#[derive(Debug, thiserror::Error)]
pub enum Error {
    #[error("channel closed")]
    SendError,
    #[error(transparent)]
    IoError(#[from] std::io::Error),
    #[error("anyhow error")]
    AnyhowError(#[from] anyhow::Error),
    #[error(transparent)]
    LapinError(#[from] lapin::Error),
    #[error(transparent)]
    LapinBuildError(#[from] deadpool::managed::BuildError),
    #[error("rmq pool error: {0}")]
    RMQPoolError(#[from] deadpool_lapin::PoolError),
    #[error(transparent)]
    ParseAccountError(#[from] near_indexer::near_primitives::account::id::ParseAccountError),
    #[error(transparent)]
    MailboxError(#[from] actix::MailboxError),
    #[error(transparent)]
    GetExecutionOutcomeError(#[from] near_client_primitives::types::GetExecutionOutcomeError),
}

pub type Result<T, E = Error> = std::result::Result<T, E>;
