use prometheus::Registry;
use tokio::sync::mpsc;
use tokio::task::JoinHandle;

use crate::{
    candidates_validator,
    errors::Result,
    metrics::{make_candidates_validator_metrics, CandidatesListener, Metricable},
    rabbit_publisher::PublishData,
    types,
};

#[derive(Clone)]
pub(crate) struct CandidatesValidator {
    view_client: actix::Addr<near_client::ViewClientActor>,
    listener: Option<CandidatesListener>,
}

impl CandidatesValidator {
    pub(crate) fn new(view_client: actix::Addr<near_client::ViewClientActor>) -> Self {
        Self {
            view_client,
            listener: None,
        }
    }

    pub(crate) fn run(
        &self,
        candidates_receiver: mpsc::Receiver<types::CandidateData>,
    ) -> (
        JoinHandle<Result<mpsc::Receiver<types::CandidateData>>>,
        mpsc::Receiver<PublishData>,
    ) {
        let (sender, receiver) = mpsc::channel(1000);
        let task = candidates_validator::task::Task::new(
            self.view_client.clone(),
            self.listener.clone(),
            crate::rabbit_publisher::RabbitPublisherHandle { sender },
        );

        (task.run(candidates_receiver), receiver)
    }
}

impl Metricable for CandidatesValidator {
    fn enable_metrics(&mut self, registry: Registry) -> Result<()> {
        let listener = make_candidates_validator_metrics(registry)?;
        self.listener = Some(listener);

        Ok(())
    }
}
