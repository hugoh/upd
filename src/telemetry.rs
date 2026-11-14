//! Logging to stderr, with a level that can change while running.

use tracing_subscriber::layer::SubscriberExt;
use tracing_subscriber::util::SubscriberInitExt;
use tracing_subscriber::{EnvFilter, Registry, fmt, reload};

use crate::config::LogLevel;

const DEFAULT_LEVEL: &str = "info";

/// Handle for changing the log level after startup.
pub struct Telemetry {
    handle: reload::Handle<EnvFilter, Registry>,
    debug: bool,
    base: String,
}

/// Sets up logging. `RUST_LOG` can set the initial filter; `debug` forces debug output for upd.
#[must_use]
pub fn init(debug: bool) -> Telemetry {
    let base = std::env::var("RUST_LOG").unwrap_or_else(|_| directive(DEFAULT_LEVEL));
    let filter = EnvFilter::try_new(filter_for(debug, &base, None))
        .unwrap_or_else(|_| EnvFilter::new(directive(DEFAULT_LEVEL)));
    let (layer, handle) = reload::Layer::new(filter);

    // A second init in the same process (tests) keeps the first subscriber.
    let _ = Registry::default()
        .with(layer)
        .with(fmt::layer().with_writer(std::io::stderr))
        .try_init();

    Telemetry {
        handle,
        debug,
        base,
    }
}

/// Dependencies stay at warnings; upd logs at `level`.
fn directive(level: &str) -> String {
    format!("warn,upd={level}")
}

/// The `--debug` flag beats the configuration's `logLevel`, which beats the startup level.
fn filter_for(debug: bool, base: &str, level: Option<LogLevel>) -> String {
    if debug {
        directive("debug")
    } else {
        level.map_or_else(|| base.to_owned(), |level| directive(level.as_str()))
    }
}

impl Telemetry {
    /// Applies the configuration's `logLevel`; without one, goes back to the startup level.
    pub fn apply(&self, level: Option<LogLevel>) {
        if let Ok(filter) = EnvFilter::try_new(filter_for(self.debug, &self.base, level)) {
            let _ = self.handle.reload(filter);
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn directives_keep_dependencies_quiet() {
        assert_eq!(directive("debug"), "warn,upd=debug");
    }

    #[test]
    fn the_configured_level_applies_unless_debug_is_forced() {
        assert_eq!(filter_for(false, "warn,upd=info", None), "warn,upd=info");
        assert_eq!(
            filter_for(false, "warn,upd=info", Some(LogLevel::Warn)),
            "warn,upd=warn"
        );
        assert_eq!(
            filter_for(true, "warn,upd=info", Some(LogLevel::Warn)),
            "warn,upd=debug"
        );
    }

    #[test]
    fn applying_levels_repeatedly_does_not_fail() {
        let telemetry = init(false);

        telemetry.apply(Some(LogLevel::Debug));
        telemetry.apply(None);
    }
}
