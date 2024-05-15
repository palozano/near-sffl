use prometheus::Registry;

use crate::{
    candidates_validator::spawner::CandidatesValidator, configs::RunConfigArgs, errors::Result,
    indexer_wrapper::IndexerWrapper, metrics::Metricable, rabbit_publisher::RabbitPublisher,
};

pub(crate) struct Manager {
    indexer: IndexerWrapper,
    candidates_validator: CandidatesValidator,
    rmq_publisher: RabbitPublisher,
}

impl Manager {
    pub fn new(config: &RunConfigArgs, indexer_config: near_indexer::IndexerConfig) -> Result<Self> {
        let addresses_to_rollup_ids = config.compile_addresses_to_ids_map()?;
        let indexer = IndexerWrapper::new(indexer_config, addresses_to_rollup_ids);

        let (view_client, _) = indexer.client_actors();
        let candidates_validator = CandidatesValidator::new(view_client);
        let rmq_publisher = RabbitPublisher::new(&config.rmq_address)?;

        Ok(Self {
            indexer,
            candidates_validator,
            rmq_publisher,
        })
    }

    pub async fn run(self) -> Result<()> {
        let Self {
            indexer,
            candidates_validator,
            rmq_publisher,
        } = self;

        let (block_handle, candidates_stream) = indexer.run();
        let (validation_handle, validated_stream) = candidates_validator.run(candidates_stream);
        let rmq_handle = rmq_publisher.run(validated_stream);

        // TODO
        tokio::select! {
            _ = block_handle => {},
            _ = validation_handle => {},
            _ = rmq_handle => {}
        };

        Ok(())
    }
}

impl Metricable for Manager {
    fn enable_metrics(&mut self, registry: Registry) -> crate::errors::Result<()> {
        self.indexer.enable_metrics(registry.clone())?;
        self.rmq_publisher.enable_metrics(registry.clone())?;
        self.candidates_validator.enable_metrics(registry.clone())?;

        Ok(())
    }
}
