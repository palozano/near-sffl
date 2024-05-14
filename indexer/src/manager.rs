use prometheus::Registry;

use crate::{
    candidates_validator::CandidatesValidator, configs::RunConfigArgs, errors::Result, indexer_wrapper::IndexerWrapper,
    metrics::Metricable, rabbit_publisher::RabbitPublisher,
};

struct Manager {
    indexer: IndexerWrapper,
    candidates_validator: CandidatesValidator,
    rmq_publisher: RabbitPublisher,
}

impl Manager {
    pub fn new(config: RunConfigArgs, indexer_config: near_indexer::IndexerConfig) -> Result<Self> {
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

    pub fn run(self) {
        let Self {
            indexer,
            candidates_validator,
            rmq_publisher,
        } = self;

        let (block_handle, candidates_stream) = indexer.run();
        let validated_stream = candidates_validator.run(candidates_stream);
        rmq_publisher.run(validated_stream);
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
