//! Logging to stderr, with a level that can change while running.

use std::io::IsTerminal;

use tracing_subscriber::filter::Targets;
use tracing_subscriber::layer::SubscriberExt;
use tracing_subscriber::util::SubscriberInitExt;
use tracing_subscriber::{Layer, Registry, fmt, reload};

use crate::config::LogLevel;

const DEFAULT_LEVEL: &str = "info";

/// Handle for changing the log level after startup.
pub struct Telemetry {
    handle: reload::Handle<Targets, Registry>,
    debug: bool,
    base: String,
}

/// Sets up logging. `RUST_LOG` can set the initial filter; `debug` forces debug output for upd.
#[must_use]
pub fn init(debug: bool) -> Telemetry {
    let base = std::env::var("RUST_LOG").unwrap_or_else(|_| directive(DEFAULT_LEVEL));
    let filter = parse_filter(&filter_for(debug, &base, None)).unwrap_or_else(default_filter);
    let (filter, handle) = reload::Layer::new(filter);

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

    Telemetry {
        handle,
        debug,
        base,
    }
}

/// A `RUST_LOG`-style list such as `warn,upd=debug`.
fn parse_filter(text: &str) -> Option<Targets> {
    text.parse().ok()
}

fn default_filter() -> Targets {
    Targets::new()
        .with_default(tracing::Level::WARN)
        .with_target("upd", tracing::Level::INFO)
}

/// Colour is for people at a terminal: syslog and log files would show the escape codes.
fn use_ansi(stderr_is_terminal: bool, no_color: bool) -> bool {
    stderr_is_terminal && !no_color
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
        if let Some(filter) = parse_filter(&filter_for(self.debug, &self.base, level)) {
            let _ = self.handle.reload(filter);
        }
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
    fn our_directives_filter_upd_and_its_dependencies_separately() {
        use tracing::Level;

        let filter = parse_filter(&directive("info")).unwrap();

        assert!(filter.would_enable("upd::monitor", &Level::INFO));
        assert!(!filter.would_enable("upd::monitor", &Level::DEBUG));
        assert!(filter.would_enable("hickory_resolver", &Level::WARN));
        assert!(!filter.would_enable("hickory_resolver", &Level::INFO));
    }

    #[test]
    fn rust_log_style_directives_parse_and_junk_does_not() {
        assert!(parse_filter("warn,upd=debug,hyper_util=info").is_some());
        assert!(parse_filter("debug").is_some());
        assert!(parse_filter("upd=loud").is_none());
    }

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
