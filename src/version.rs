//! The version shown by `--version`, in the stats report and in the HTTP user agent.

/// The crate version, which release tags carry.
pub const VERSION: &str = env!("CARGO_PKG_VERSION");
