use std::sync::{Arc, Mutex, MutexGuard, PoisonError};

use super::status::Status;

/// The status, shared by the monitor (the writer) and the stats server.
#[derive(Debug, Clone)]
pub struct Shared(Arc<Mutex<Status>>);

impl Shared {
    #[must_use]
    pub fn new(status: Status) -> Self {
        Self(Arc::new(Mutex::new(status)))
    }

    /// Locks the status. A panic elsewhere cannot leave it half-updated, so poisoning is ignored.
    pub fn lock(&self) -> MutexGuard<'_, Status> {
        self.0.lock().unwrap_or_else(PoisonError::into_inner)
    }
}
