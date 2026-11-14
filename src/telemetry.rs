//! Logging to stderr, with a level that can change while running.

use std::io::IsTerminal;

use tracing::Level;
use tracing_subscriber::filter::Targets;
use tracing_subscriber::layer::SubscriberExt;
use tracing_subscriber::util::SubscriberInitExt;
use tracing_subscriber::{Layer, Registry, fmt, reload};

use crate::config::LogLevel;

/// Handle for changing the log level after startup.
pub struct Telemetry {
    handle: reload::Handle<Targets, Registry>,
    debug: bool,
}

/// Sets up logging at `info`, or `debug` when forced.
#[must_use]
pub fn init(debug: bool) -> Telemetry {
    let (filter, handle) = reload::Layer::new(filter_for(debug, None));

    // A second init in the same process (tests) keeps the first subscriber.
    let _ = Registry::default()
        .with(
            fmt::layer()
                .with_writer(std::io::stderr)
                .with_ansi(use_ansi(
                    std::io::stderr().is_terminal(),
                    std::env::var_os("NO_COLOR").is_some_and(|value| !value.is_empty()),
                ))
                .with_filter(filter),
        )
        .try_init();

    Telemetry { handle, debug }
}

/// Colour is for people at a terminal: syslog and log files would show the escape codes.
fn use_ansi(stderr_is_terminal: bool, no_color: bool) -> bool {
    stderr_is_terminal && !no_color
}

/// Dependencies stay at warnings; upd logs at `level`. The `--debug` flag beats the configuration's
/// `logLevel`, which beats `info`.
fn filter_for(debug: bool, level: Option<LogLevel>) -> Targets {
    let upd = if debug {
        Level::DEBUG
    } else {
        level.map_or(Level::INFO, LogLevel::as_level)
    };
    Targets::new()
        .with_default(Level::WARN)
        .with_target("upd", upd)
}

impl Telemetry {
    /// Applies the configuration's `logLevel`; without one, goes back to `info`.
    pub fn apply(&self, level: Option<LogLevel>) {
        let _ = self.handle.reload(filter_for(self.debug, level));
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn colour_is_for_terminals_and_not_when_asked_otherwise() {
        assert!(use_ansi(true, false));
        assert!(
            !use_ansi(false, false),
            "log files and syslog get no escape codes"
        );
        assert!(!use_ansi(true, true), "NO_COLOR is honoured");
    }

    #[test]
    fn upd_and_its_dependencies_are_filtered_separately() {
        let filter = filter_for(false, None);

        assert!(filter.would_enable("upd::monitor", &Level::INFO));
        assert!(!filter.would_enable("upd::monitor", &Level::DEBUG));
        assert!(filter.would_enable("hickory_resolver", &Level::WARN));
        assert!(!filter.would_enable("hickory_resolver", &Level::INFO));
    }

    #[test]
    fn the_configured_level_applies_unless_debug_is_forced() {
        let debug = |filter: &Targets| filter.would_enable("upd", &Level::DEBUG);

        assert!(!debug(&filter_for(false, None)));
        assert!(!debug(&filter_for(false, Some(LogLevel::Warn))));
        assert!(debug(&filter_for(false, Some(LogLevel::Debug))));
        assert!(debug(&filter_for(true, Some(LogLevel::Warn))));
    }

    #[test]
    fn applying_levels_repeatedly_does_not_fail() {
        let telemetry = init(false);

        telemetry.apply(Some(LogLevel::Debug));
        telemetry.apply(None);
    }
}
