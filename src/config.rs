//! The TOML configuration: loading, `${VAR}` expansion, validation and conversion to typed settings.
//!
//! Types that cannot hold an invalid value (`Interval`, `Positive`, `NonZeroU16`, `Command`,
//! `ProbeSpec`) do most of the validation while serde reads the file, so the first problem is
//! reported with the key it belongs to.

use std::num::{NonZeroU16, NonZeroU32};
use std::path::{Path, PathBuf};
use std::time::Duration;

use serde::{Deserialize, Deserializer};

use crate::checks::CheckList;
use crate::command::Command;
use crate::down_action::DownAction;
use crate::probe_spec::ProbeSpec;
use crate::server::DEFAULT_READ_TIMEOUT;
use crate::stats::tracker::{BucketConfig, MAX_BUCKETS_PER_PERIOD};

/// The longest pause between checks.
const MAX_INTERVAL: Duration = Duration::from_hours(24);

#[derive(Debug, thiserror::Error)]
pub enum ConfigError {
    #[error("could not read config {}: {source}", path.display())]
    Read {
        path: PathBuf,
        source: std::io::Error,
    },
    #[error(
        "no configuration file found (looked for {}); use --config",
        list(searched)
    )]
    NoDefault { searched: Vec<PathBuf> },
    #[error("{key}: environment variable {name:?} is not set")]
    MissingEnv { key: String, name: String },
    #[error("invalid configuration: {0}")]
    Parse(#[from] toml::de::Error),
    #[error("invalid configuration: {key}: {problem}")]
    Invalid { key: &'static str, problem: String },
}

fn list(paths: &[PathBuf]) -> String {
    paths
        .iter()
        .map(|path| path.display().to_string())
        .collect::<Vec<_>>()
        .join(", ")
}

/// The configuration file to use when none was named: the first that exists of `./.upd.toml`,
/// `upd/upd.toml` in `$XDG_CONFIG_HOME` (default `~/.config`), the same in each of
/// `$XDG_CONFIG_DIRS` (default `/etc/xdg`), and `/etc/upd/upd.toml`.
///
/// # Errors
///
/// Fails, naming the places looked at, when none of them exists.
pub fn default_path(exists: impl Fn(&Path) -> bool) -> Result<PathBuf, ConfigError> {
    let dirs = xdg::BaseDirectories::with_prefix("upd");
    first_existing(
        default_candidates(dirs.get_config_home(), dirs.get_config_dirs()),
        exists,
    )
}

/// Where to look, most preferred first.
fn default_candidates(config_home: Option<PathBuf>, config_dirs: Vec<PathBuf>) -> Vec<PathBuf> {
    std::iter::once(PathBuf::from(".upd.toml"))
        .chain(
            config_home
                .into_iter()
                .chain(config_dirs)
                .map(|dir| dir.join("upd.toml")),
        )
        .chain([PathBuf::from("/etc/upd/upd.toml")])
        .collect()
}

fn first_existing(
    searched: Vec<PathBuf>,
    exists: impl Fn(&Path) -> bool,
) -> Result<PathBuf, ConfigError> {
    searched
        .iter()
        .find(|path| exists(path))
        .cloned()
        .ok_or(ConfigError::NoDefault { searched })
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum LogLevel {
    Error,
    Warn,
    Info,
    Debug,
    Trace,
}

impl LogLevel {
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Error => "error",
            Self::Warn => "warn",
            Self::Info => "info",
            Self::Debug => "debug",
            Self::Trace => "trace",
        }
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Delays {
    pub up: Duration,
    pub down: Duration,
}

impl Delays {
    #[must_use]
    pub fn for_state(&self, up: bool) -> Duration {
        if up { self.up } else { self.down }
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct StatsSettings {
    /// Where the stats server listens; `None` leaves it off.
    pub port: Option<NonZeroU16>,
    pub reports: Vec<Duration>,
    pub buckets: BucketConfig,
    /// How long a client has to send its request.
    pub read_timeout: Duration,
}

/// A validated configuration.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Settings {
    pub checks: CheckList,
    pub delays: Delays,
    pub down_action: Option<DownAction>,
    pub stats: StatsSettings,
    pub log_level: Option<LogLevel>,
}

impl Settings {
    /// Reads the file at `path`, expanding `${VAR}` in its string values from the environment.
    ///
    /// # Errors
    ///
    /// Fails when the file is unreadable, is not valid TOML, refers to an unset variable, or
    /// holds a setting that is not valid.
    pub fn load(path: &Path) -> Result<Self, ConfigError> {
        let text = std::fs::read_to_string(path).map_err(|source| ConfigError::Read {
            path: path.to_owned(),
            source,
        })?;
        Self::parse(&text, |name| std::env::var(name).ok())
    }

    /// # Errors
    ///
    /// As for [`Settings::load`], with `lookup` standing in for the environment.
    pub fn parse(text: &str, lookup: impl Fn(&str) -> Option<String>) -> Result<Self, ConfigError> {
        let mut table: toml::Table = toml::from_str(text)?;
        expand_table(&mut table, "", &lookup)?;
        let raw: Raw = table.try_into()?;
        raw.into_settings()
    }
}

fn expand_table(
    table: &mut toml::Table,
    prefix: &str,
    lookup: &impl Fn(&str) -> Option<String>,
) -> Result<(), ConfigError> {
    for (key, value) in table.iter_mut() {
        let path = if prefix.is_empty() {
            key.clone()
        } else {
            format!("{prefix}.{key}")
        };
        expand_value(value, &path, lookup)?;
    }
    Ok(())
}

fn expand_value(
    value: &mut toml::Value,
    path: &str,
    lookup: &impl Fn(&str) -> Option<String>,
) -> Result<(), ConfigError> {
    match value {
        toml::Value::String(text) => *text = expand_env(text, path, lookup)?,
        toml::Value::Table(table) => expand_table(table, path, lookup)?,
        toml::Value::Array(items) => {
            for (index, item) in items.iter_mut().enumerate() {
                expand_value(item, &format!("{path}[{index}]"), lookup)?;
            }
        }
        _ => {}
    }
    Ok(())
}

/// Replaces each `${NAME}` with the variable's value; an unset variable is an error.
fn expand_env(
    text: &str,
    key: &str,
    lookup: &impl Fn(&str) -> Option<String>,
) -> Result<String, ConfigError> {
    let mut out = String::with_capacity(text.len());
    let mut rest = text;

    while let Some(start) = rest.find("${") {
        let after = &rest[start + 2..];
        let Some(end) = after
            .find('}')
            .filter(|end| is_variable_name(&after[..*end]))
        else {
            out.push_str(&rest[..start + 2]);
            rest = after;
            continue;
        };

        let name = &after[..end];
        let value = lookup(name).ok_or_else(|| ConfigError::MissingEnv {
            key: key.to_owned(),
            name: name.to_owned(),
        })?;
        out.push_str(&rest[..start]);
        out.push_str(&value);
        rest = &after[end + 1..];
    }

    out.push_str(rest);
    Ok(out)
}

fn is_variable_name(name: &str) -> bool {
    !name.is_empty() && name.chars().all(|c| c.is_ascii_alphanumeric() || c == '_')
}

/// Reads a duration such as `"2s"`, `"5m"` or `"1h30m"`, and applies `check` to it.
fn duration_with<'de, D: Deserializer<'de>>(
    deserializer: D,
    check: fn(Duration) -> Result<(), &'static str>,
) -> Result<Duration, D::Error> {
    let text = String::deserialize(deserializer)?;
    let duration = humantime::parse_duration(&text)
        .map_err(|err| serde::de::Error::custom(format!("invalid duration {text:?}: {err}")))?;
    check(duration).map_err(|problem| serde::de::Error::custom(format!("{text:?} {problem}")))?;
    Ok(duration)
}

/// Any duration, zero included.
#[derive(Debug, Clone, Copy)]
struct Span(Duration);

/// A duration greater than zero.
#[derive(Debug, Clone, Copy)]
struct Positive(Duration);

/// A pause between checks: greater than zero, at most a day.
#[derive(Debug, Clone, Copy)]
struct Interval(Duration);

impl<'de> Deserialize<'de> for Span {
    fn deserialize<D: Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        duration_with(deserializer, |_| Ok(())).map(Self)
    }
}

impl<'de> Deserialize<'de> for Positive {
    fn deserialize<D: Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        duration_with(deserializer, |duration| {
            if duration.is_zero() {
                Err("must be greater than 0")
            } else {
                Ok(())
            }
        })
        .map(Self)
    }
}

impl<'de> Deserialize<'de> for Interval {
    fn deserialize<D: Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        duration_with(deserializer, |duration| {
            if duration.is_zero() {
                Err("must be greater than 0")
            } else if duration > MAX_INTERVAL {
                Err("must be at most 24h")
            } else {
                Ok(())
            }
        })
        .map(Self)
    }
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields, rename_all = "camelCase")]
struct Raw {
    checks: RawChecks,
    down_action: Option<RawDownAction>,
    #[serde(default)]
    stats: RawStats,
    log_level: Option<LogLevel>,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct RawChecks {
    every: RawEvery,
    #[serde(default)]
    list: RawList,
    timeout: Positive,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields)]
struct RawEvery {
    normal: Interval,
    down: Interval,
}

#[derive(Debug, Default, Deserialize)]
#[serde(deny_unknown_fields)]
struct RawList {
    #[serde(default)]
    ordered: Vec<ProbeSpec>,
    #[serde(default)]
    shuffled: Vec<ProbeSpec>,
}

#[derive(Debug, Deserialize)]
#[serde(deny_unknown_fields, rename_all = "camelCase")]
struct RawDownAction {
    exec: Command,
    stop_exec: Option<Command>,
    #[serde(default)]
    every: RawDownEvery,
}

#[derive(Debug, Default, Deserialize)]
#[serde(deny_unknown_fields, rename_all = "camelCase")]
struct RawDownEvery {
    after: Option<Span>,
    repeat: Option<Positive>,
    exp_backoff_limit: Option<Positive>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(deny_unknown_fields, rename_all = "camelCase")]
struct RawStats {
    port: Option<NonZeroU16>,
    #[serde(default)]
    reports: Vec<Positive>,
    #[serde(default)]
    buckets: RawBuckets,
    read_timeout: Option<Positive>,
}

#[derive(Debug, Default, Deserialize)]
#[serde(deny_unknown_fields, rename_all = "camelCase")]
struct RawBuckets {
    min: Option<NonZeroU32>,
    max_span: Option<Positive>,
}

impl Raw {
    /// Checks what no single value can, and converts to the settings the rest of upd uses.
    fn into_settings(self) -> Result<Settings, ConfigError> {
        let invalid = |key, problem: String| ConfigError::Invalid { key, problem };

        let list = self.checks.list;
        if list.ordered.is_empty() && list.shuffled.is_empty() {
            return Err(invalid("checks.list", "no checks found in config".into()));
        }

        let defaults = BucketConfig::default();
        let buckets = BucketConfig {
            min: self.stats.buckets.min.map_or(defaults.min, NonZeroU32::get),
            max_span: self
                .stats
                .buckets
                .max_span
                .map_or(defaults.max_span, |span| span.0),
        };
        let reports: Vec<Duration> = self.stats.reports.iter().map(|period| period.0).collect();
        if let Some(period) = reports
            .iter()
            .find(|period| buckets.bucket_count(**period) > MAX_BUCKETS_PER_PERIOD)
        {
            return Err(invalid(
                "stats.reports",
                format!(
                    "{period:?} needs {} buckets, more than the {MAX_BUCKETS_PER_PERIOD} allowed: increase stats.buckets.maxSpan",
                    buckets.bucket_count(*period)
                ),
            ));
        }

        Ok(Settings {
            checks: CheckList {
                ordered: list.ordered,
                shuffled: list.shuffled,
                timeout: self.checks.timeout.0,
            },
            delays: Delays {
                up: self.checks.every.normal.0,
                down: self.checks.every.down.0,
            },
            down_action: self.down_action.map(|down| DownAction {
                exec: down.exec,
                stop_exec: down.stop_exec,
                after: down.every.after.map_or(Duration::ZERO, |span| span.0),
                repeat: down.every.repeat.map(|span| span.0),
                backoff_limit: down.every.exp_backoff_limit.map(|span| span.0),
            }),
            stats: StatsSettings {
                port: self.stats.port,
                reports,
                buckets,
                read_timeout: self
                    .stats
                    .read_timeout
                    .map_or(DEFAULT_READ_TIMEOUT, |span| span.0),
            },
            log_level: self.log_level,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    const MINIMAL: &str = r#"
        [checks]
        timeout = "2s"
        [checks.every]
        normal = "120s"
        down = "20s"
        [checks.list]
        ordered = ["http://captive.apple.com/hotspot-detect.html", "tcp://1.1.1.1:53"]
    "#;

    fn no_env(_: &str) -> Option<String> {
        None
    }

    fn parse(text: &str) -> Result<Settings, ConfigError> {
        Settings::parse(text, no_env)
    }

    fn error(text: &str) -> String {
        parse(text).unwrap_err().to_string()
    }

    fn secs(n: u64) -> Duration {
        Duration::from_secs(n)
    }

    #[test]
    fn a_minimal_config_gets_the_defaults() {
        let settings = parse(MINIMAL).unwrap();

        assert_eq!(settings.checks.ordered.len(), 2);
        assert!(settings.checks.shuffled.is_empty());
        assert_eq!(settings.checks.timeout, secs(2));
        assert_eq!(settings.delays.for_state(true), secs(120));
        assert_eq!(settings.delays.for_state(false), secs(20));
        assert_eq!(settings.down_action, None);
        assert_eq!(settings.stats.port, None);
        assert!(settings.stats.reports.is_empty());
        assert_eq!(settings.stats.buckets, BucketConfig::default());
        assert_eq!(settings.stats.read_timeout, DEFAULT_READ_TIMEOUT);
        assert_eq!(settings.log_level, None);
    }

    #[test]
    fn the_example_config_is_valid() {
        let text =
            std::fs::read_to_string(concat!(env!("CARGO_MANIFEST_DIR"), "/upd.toml")).unwrap();

        let settings = parse(&text).unwrap();

        assert_eq!(settings.checks.ordered.len(), 3);
        assert_eq!(settings.checks.shuffled.len(), 7);
        assert_eq!(settings.stats.port, NonZeroU16::new(8080));
        assert_eq!(settings.stats.reports.len(), 5);
        assert_eq!(settings.log_level, Some(LogLevel::Debug));
        let down = settings.down_action.unwrap();
        assert_eq!(down.after, secs(1));
        assert_eq!(down.repeat, Some(secs(3)));
        assert!(down.stop_exec.is_some());
    }

    #[test]
    fn down_action_settings_are_converted() {
        let settings = parse(&format!(
            r#"{MINIMAL}
            [downAction]
            exec = "reboot-modem --now"
            stopExec = "notify up"
            [downAction.every]
            after = "120s"
            repeat = "5m"
            expBackoffLimit = "1h"
            "#
        ))
        .unwrap();

        let down = settings.down_action.unwrap();
        assert_eq!(down.exec.program(), "reboot-modem");
        assert_eq!(down.exec.args(), ["--now"]);
        assert_eq!(down.stop_exec.unwrap().program(), "notify");
        assert_eq!(down.after, secs(120));
        assert_eq!(down.repeat, Some(secs(300)));
        assert_eq!(down.backoff_limit, Some(secs(3600)));
    }

    #[test]
    fn a_down_action_needs_only_a_command() {
        let settings = parse(&format!("{MINIMAL}\n[downAction]\nexec = \"x\"")).unwrap();

        let down = settings.down_action.unwrap();
        assert_eq!(down.after, Duration::ZERO);
        assert_eq!(down.repeat, None);
        assert_eq!(down.backoff_limit, None);
        assert_eq!(down.stop_exec, None);
    }

    #[test]
    fn stats_settings_are_converted() {
        let settings = parse(&format!(
            r#"logLevel = "trace"
            {MINIMAL}
            [stats]
            port = 42080
            reports = ["10s", "1h"]
            readTimeout = "750ms"
            [stats.buckets]
            min = 50
            maxSpan = "10m"
            "#
        ))
        .unwrap();

        assert_eq!(settings.stats.port, NonZeroU16::new(42080));
        assert_eq!(settings.stats.reports, [secs(10), secs(3600)]);
        assert_eq!(settings.stats.buckets.min, 50);
        assert_eq!(settings.stats.buckets.max_span, secs(600));
        assert_eq!(settings.stats.read_timeout, Duration::from_millis(750));
        assert_eq!(settings.log_level, Some(LogLevel::Trace));
    }

    #[test]
    fn every_log_level_has_a_name() {
        for (level, name) in [
            (LogLevel::Error, "error"),
            (LogLevel::Warn, "warn"),
            (LogLevel::Info, "info"),
            (LogLevel::Debug, "debug"),
            (LogLevel::Trace, "trace"),
        ] {
            assert_eq!(level.as_str(), name);
            assert_eq!(
                parse(&format!("logLevel = \"{name}\"\n{MINIMAL}"))
                    .unwrap()
                    .log_level,
                Some(level)
            );
        }
    }

    #[test]
    fn durations_must_be_positive_where_zero_makes_no_sense() {
        for (from, to) in [
            ("normal = \"120s\"", "normal = \"0s\""),
            ("down = \"20s\"", "down = \"0s\""),
            ("timeout = \"2s\"", "timeout = \"0s\""),
        ] {
            let message = error(&MINIMAL.replace(from, to));

            assert!(message.contains("must be greater than 0"), "{message}");
        }
        for key in ["repeat", "expBackoffLimit"] {
            let message = error(&format!(
                "{MINIMAL}\n[downAction]\nexec = \"x\"\n[downAction.every]\n{key} = \"0s\""
            ));

            assert!(
                message.contains("must be greater than 0"),
                "{key}: {message}"
            );
        }
        assert!(
            error(&format!("{MINIMAL}\n[stats]\nreports = [\"0s\"]")).contains("greater than 0")
        );
        assert!(
            parse(&format!(
                "{MINIMAL}\n[downAction]\nexec = \"x\"\n[downAction.every]\nafter = \"0s\""
            ))
            .is_ok()
        );
    }

    #[test]
    fn check_intervals_are_capped_at_a_day() {
        let message = error(&MINIMAL.replace("\"120s\"", "\"400000000000y\""));

        assert!(message.contains("must be at most 24h"), "{message}");
        assert!(parse(&MINIMAL.replace("\"120s\"", "\"24h\"")).is_ok());
    }

    #[test]
    fn a_bad_probe_is_reported_with_its_text() {
        let message = error(&MINIMAL.replace("tcp://1.1.1.1:53", "ftp://foo.bar/"));

        assert!(message.contains("\"ftp://foo.bar/\""), "{message}");
        assert!(message.contains("unsupported scheme"), "{message}");
    }

    #[test]
    fn a_bad_command_is_rejected() {
        let message = error(&format!("{MINIMAL}\n[downAction]\nexec = \"\""));

        assert!(message.contains("command is empty"), "{message}");
    }

    #[test]
    fn at_least_one_check_is_required() {
        let message = error(
            r#"
            [checks]
            timeout = "1s"
            [checks.every]
            normal = "1s"
            down = "1s"
            "#,
        );

        assert!(
            message.contains("checks.list: no checks found"),
            "{message}"
        );
    }

    #[test]
    fn report_periods_needing_too_many_buckets_are_rejected() {
        let message = error(&format!(
            r#"{MINIMAL}
            [stats]
            reports = ["100000d"]
            [stats.buckets]
            maxSpan = "1s"
            "#
        ));

        assert!(message.contains("stats.reports"), "{message}");
        assert!(
            message.contains("increase stats.buckets.maxSpan"),
            "{message}"
        );
    }

    #[test]
    fn syntax_and_type_errors_are_parse_errors() {
        assert!(matches!(parse("not toml ["), Err(ConfigError::Parse(_))));
        assert!(matches!(
            parse(&MINIMAL.replace("\"2s\"", "\"soon\"")),
            Err(ConfigError::Parse(_))
        ));
        assert!(matches!(
            parse(&format!("logLevel = \"loud\"\n{MINIMAL}")),
            Err(ConfigError::Parse(_))
        ));
    }

    #[test]
    fn unknown_keys_are_rejected() {
        assert!(error(&format!("verbosity = \"debug\"\n{MINIMAL}")).contains("verbosity"));
        for key in ["writeTimeout", "idleTimeout"] {
            assert!(error(&format!("{MINIMAL}\n[stats]\n{key} = \"3s\"")).contains(key));
        }
    }

    #[test]
    fn a_missing_or_zero_port_is_rejected_or_disables_the_server() {
        assert_eq!(parse(MINIMAL).unwrap().stats.port, None);
        assert!(matches!(
            parse(&format!("{MINIMAL}\n[stats]\nport = 0")),
            Err(ConfigError::Parse(_))
        ));
        assert!(matches!(
            parse(&format!("{MINIMAL}\n[stats]\nport = 70000")),
            Err(ConfigError::Parse(_))
        ));
    }

    #[test]
    fn environment_variables_are_expanded_in_string_values() {
        let text = MINIMAL
            .replace("\"120s\"", "\"${NORMAL}\"")
            .replace("\"20s\"", "\"${DOWN}s\"");

        let settings = Settings::parse(&text, |name| match name {
            "NORMAL" => Some("90s".to_owned()),
            "DOWN" => Some("15".to_owned()),
            _ => None,
        })
        .unwrap();

        assert_eq!(settings.delays.up, secs(90));
        assert_eq!(settings.delays.down, secs(15));
    }

    #[test]
    fn variables_inside_arrays_and_comments() {
        let text = format!(
            "# every = \"${{NOT_SET}}\"\n{}",
            MINIMAL.replace("tcp://1.1.1.1:53", "tcp://${HOST}:53")
        );

        let settings =
            Settings::parse(&text, |name| (name == "HOST").then(|| "9.9.9.9".to_owned())).unwrap();

        assert_eq!(settings.checks.ordered[1].target(), "9.9.9.9:53");
    }

    #[test]
    fn an_unset_variable_is_an_error_naming_it_and_its_key() {
        let message = error(&MINIMAL.replace("\"120s\"", "\"${NORMAL}\""));

        assert_eq!(
            message,
            "checks.every.normal: environment variable \"NORMAL\" is not set"
        );
    }

    #[test]
    fn expansion_leaves_other_dollar_text_alone() {
        let expanded = expand_env("a $x ${} ${unclosed b ${V} c", "k", &|name: &str| {
            (name == "V").then(|| "1".to_owned())
        })
        .unwrap();

        assert_eq!(expanded, "a $x ${} ${unclosed b 1 c");
    }

    #[test]
    fn loading_a_missing_file_names_it() {
        let err = Settings::load(Path::new("/nonexistent/upd.toml")).unwrap_err();

        assert!(matches!(err, ConfigError::Read { .. }));
        assert!(err.to_string().contains("/nonexistent/upd.toml"));
    }

    #[test]
    fn loading_reads_the_file() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("upd.toml");
        std::fs::write(&path, MINIMAL).unwrap();

        assert_eq!(Settings::load(&path).unwrap(), parse(MINIMAL).unwrap());
    }

    fn p(text: &str) -> PathBuf {
        PathBuf::from(text)
    }

    #[test]
    fn candidates_run_from_the_working_directory_to_the_system_config() {
        let candidates = default_candidates(
            Some(p("/home/me/.config/upd")),
            vec![p("/etc/xdg/upd"), p("/opt/etc/upd")],
        );

        assert_eq!(
            candidates,
            [
                p(".upd.toml"),
                p("/home/me/.config/upd/upd.toml"),
                p("/etc/xdg/upd/upd.toml"),
                p("/opt/etc/upd/upd.toml"),
                p("/etc/upd/upd.toml"),
            ]
        );
    }

    #[test]
    fn without_a_home_the_user_config_dir_is_skipped() {
        let candidates = default_candidates(None, vec![p("/etc/xdg/upd")]);

        assert_eq!(
            candidates,
            [
                p(".upd.toml"),
                p("/etc/xdg/upd/upd.toml"),
                p("/etc/upd/upd.toml")
            ]
        );
    }

    #[test]
    fn the_first_candidate_that_exists_wins() {
        let candidates = default_candidates(Some(p("/u/upd")), vec![p("/etc/xdg/upd")]);

        let local = first_existing(candidates.clone(), |_| true).unwrap();
        let user = first_existing(candidates.clone(), |path| path != Path::new(".upd.toml"));
        let system = first_existing(candidates, |path| path.starts_with("/etc/xdg"));

        assert_eq!(local, p(".upd.toml"));
        assert_eq!(user.unwrap(), p("/u/upd/upd.toml"));
        assert_eq!(system.unwrap(), p("/etc/xdg/upd/upd.toml"));
    }

    #[test]
    fn when_nothing_exists_the_places_looked_at_are_named() {
        let err = first_existing(
            default_candidates(Some(p("/u/upd")), vec![p("/etc/xdg/upd")]),
            |_| false,
        )
        .unwrap_err();

        assert_eq!(
            err.to_string(),
            "no configuration file found (looked for .upd.toml, /u/upd/upd.toml, /etc/xdg/upd/upd.toml, /etc/upd/upd.toml); use --config"
        );
    }
}
