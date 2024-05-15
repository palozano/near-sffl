use near_indexer::near_primitives::views::{ExecutionStatusView, FinalExecutionStatus};
use near_o11y::WithSpanContextExt;
use std::{collections::VecDeque, sync, time::Duration};
use tokio::{
    sync::{mpsc, oneshot, Mutex},
    task::JoinHandle,
    time,
};
use tracing::{error, info};

use crate::{
    candidates_validator::CANDIDATES_VALIDATOR,
    errors::{Error, Result},
    metrics::CandidatesListener,
    rabbit_publisher::{
        get_routing_key, PublishData, PublishOptions, PublishPayload, PublisherContext, RabbitPublisherHandle,
    },
    types,
};

#[derive(Clone)]
pub struct Task {
    view_client: actix::Addr<near_client::ViewClientActor>,
    listener: Option<CandidatesListener>,
    queue_protected: types::ProtectedQueue<types::CandidateData>,
    rmq_handle: RabbitPublisherHandle,
}

impl Task {
    pub fn new(
        view_client: actix::Addr<near_client::ViewClientActor>,
        listener: Option<CandidatesListener>,
        rmq_handle: RabbitPublisherHandle,
    ) -> Self {
        Self {
            view_client,
            listener,
            queue_protected: sync::Arc::new(Mutex::new(VecDeque::new())),
            rmq_handle,
        }
    }

    async fn ticker(self, mut done: oneshot::Receiver<()>) {
        let Self {
            view_client,
            listener,
            queue_protected,
            mut rmq_handle,
        } = self;

        info!(target: CANDIDATES_VALIDATOR, "Starting ticker");
        let mut interval = time::interval(Duration::from_secs(2));
        loop {
            tokio::select! {
                _ = interval.tick() => {
                    let mut queue = queue_protected.lock().await;
                    let _ = Self::flush(&mut queue, &mut rmq_handle, &view_client, listener.clone()).await;
                    interval.reset();
                },
                _ = &mut done => {
                    return
                }
            }
        }
    }

    // Assumes queue is under mutex
    async fn flush(
        queue: &mut VecDeque<types::CandidateData>,
        rmq_handle: &mut RabbitPublisherHandle,
        view_client: &actix::Addr<near_client::ViewClientActor>,
        listener: Option<CandidatesListener>,
    ) -> Result<bool> {
        if queue.is_empty() {
            return Ok(true);
        }

        info!(target: CANDIDATES_VALIDATOR, "Flushing");
        while let Some(candidate_data) = queue.front() {
            let final_status = Self::fetch_execution_outcome(view_client, candidate_data).await?;
            match final_status {
                FinalExecutionStatus::NotStarted | FinalExecutionStatus::Started => {
                    info!(target: CANDIDATES_VALIDATOR, "Execution not finished, not sending to RabbitMQ");
                    return Ok(false);
                }
                FinalExecutionStatus::SuccessValue(_) => {
                    info!(target: CANDIDATES_VALIDATOR, "Candidate status now successful, candidate_data: {}", candidate_data);
                    listener.as_ref().map(|l| l.num_successful.inc());
                    Self::send(candidate_data, rmq_handle).await?;
                    queue.pop_front();
                }
                FinalExecutionStatus::Failure(_) => {
                    info!(target: CANDIDATES_VALIDATOR, "Execution failed, not sending to RabbitMQ");
                    listener.as_ref().map(|l| l.num_failed.inc());

                    queue.pop_front();
                }
            };
        }

        Ok(true)
    }

    async fn fetch_execution_outcome(
        view_client: &actix::Addr<near_client::ViewClientActor>,
        candidate_data: &types::CandidateData,
    ) -> Result<FinalExecutionStatus> {
        info!(target: CANDIDATES_VALIDATOR, "Fetching execution outcome for candidate data");
        let kek = view_client
            .send(
                near_client::TxStatus {
                    tx_hash: candidate_data.transaction.transaction.hash,
                    signer_account_id: candidate_data.transaction.transaction.signer_id.clone(),
                    fetch_receipt: true,
                }
                .with_span_context(),
            )
            .await;
        Ok(kek??
            .execution_outcome
            .map(|x| x.into_outcome().status)
            .unwrap_or(FinalExecutionStatus::NotStarted))
    }

    fn handle_error(error: impl Into<Error>, publish_data: Option<PublishData>) {
        let error = error.into();
        let msg = if let Some(data) = publish_data {
            // TODO: add display for cx
            // TODO: handle error here
            format!("Publisher Error: {}, cx: {}", error.to_string(), data.cx.block_hash)
        } else {
            format!("Publisher Error: {}", error.to_string())
        };

        error!(target: CANDIDATES_VALIDATOR, message = display(msg.as_str()));
    }

    async fn send(candidate_data: &types::CandidateData, rmq_handle: &mut RabbitPublisherHandle) -> Result<()> {
        // TODO: is sequential order important here?
        for data in candidate_data.clone().payloads {
            rmq_handle
                .publish(PublishData {
                    publish_options: PublishOptions {
                        routing_key: get_routing_key(candidate_data.rollup_id),
                        ..PublishOptions::default()
                    },
                    cx: PublisherContext {
                        block_hash: candidate_data.transaction.outcome.execution_outcome.block_hash,
                    },
                    payload: PublishPayload {
                        transaction_id: candidate_data.transaction.transaction.hash,
                        data,
                    },
                })
                .await?;
        }

        Ok(())
    }

    async fn process_candidates(
        self,
        mut receiver: mpsc::Receiver<types::CandidateData>,
        done_sender: oneshot::Sender<()>,
    ) -> Result<mpsc::Receiver<types::CandidateData>> {
        let Self {
            view_client,
            listener,
            queue_protected,
            mut rmq_handle,
        } = self;

        while let Some(candidate_data) = receiver.recv().await {
            info!(target: CANDIDATES_VALIDATOR, "Received candidate data");

            {
                let mut queue = queue_protected.lock().await;
                // TODO(edwin): handle errors/ unwrap_or(false)?
                info!(target: CANDIDATES_VALIDATOR, "Flushing enqueued candidates");
                let flushed = Self::flush(&mut queue, &mut rmq_handle, &view_client, listener.clone()).await?;

                if !flushed {
                    info!(target: CANDIDATES_VALIDATOR, "Not flushed, so enqueuing candidate data");
                    queue.push_back(candidate_data);
                    continue;
                }
            }

            let final_status = match candidate_data.transaction.outcome.execution_outcome.outcome.status {
                ExecutionStatusView::Failure(_) => {
                    info!(target: CANDIDATES_VALIDATOR, "Execution failed, not sending to RabbitMQ");
                    listener.as_ref().map(|l| l.num_failed.inc());

                    continue;
                }
                _ => Self::fetch_execution_outcome(&view_client, &candidate_data).await?,
            };

            match final_status {
                FinalExecutionStatus::NotStarted | FinalExecutionStatus::Started => {
                    info!(target: CANDIDATES_VALIDATOR, "Execution not finished, pushing back to queue");
                    let mut queue = queue_protected.lock().await;
                    queue.push_back(candidate_data);
                }
                FinalExecutionStatus::SuccessValue(_) => {
                    info!(target: CANDIDATES_VALIDATOR, "Candidate executed successfully, candidate_data: {}", candidate_data);
                    listener.as_ref().map(|l| l.num_successful.inc());

                    Self::send(&candidate_data, &mut rmq_handle).await?;
                }
                FinalExecutionStatus::Failure(_) => {
                    info!(target: CANDIDATES_VALIDATOR, "Execution failed, not sending to RabbitMQ");
                    listener.as_ref().map(|l| l.num_failed.inc());
                }
            }
        }

        let _ = done_sender.send(());
        Ok(receiver)
    }

    pub fn run(
        self,
        candidates_receiver: mpsc::Receiver<types::CandidateData>,
    ) -> JoinHandle<Result<mpsc::Receiver<types::CandidateData>>> {
        let (done_sender, done_receiver) = oneshot::channel();
        actix::spawn(self.clone().ticker(done_receiver));
        actix::spawn(self.process_candidates(candidates_receiver, done_sender))
    }
}
