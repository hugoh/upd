//! The state shared between the monitor, which writes it, and the stats server, which reads it.

use std::time::{Duration, Instant, SystemTime};

use serde::Serialize;

use super::format::{HumanDuration, HumanPercent};
use super::tracker::{BucketConfig, Totals, Tracker};

/// Progress of the down-action loop.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct DownActionStatus {
    pub iteration: u32,
    #[serde(serialize_with = "serialize_duration")]
    pub sleep_time: Duration,
    pub backoff_capped: bool,
}

fn serialize_duration<S: serde::Serializer>(
    duration: &Duration,
    serializer: S,
) -> Result<S::Ok, S::Error> {
    HumanDuration::from(*duration).serialize(serializer)
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct PeriodReport {
    pub period: HumanDuration,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub coverage: Option<HumanDuration>,
    pub availability: HumanPercent,
    pub down_time: HumanDuration,
    pub total_probes: u64,
    pub failed_probes: u64,
    pub failure_rate: HumanPercent,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct LoopReport {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub last_success: Option<HumanDuration>,
    pub next_check: HumanDuration,
    pub interval: HumanDuration,
    #[serde(rename = "timeSinceLastUpdate")]
    pub time_since_update: HumanDuration,
    pub total_checks_run: u64,
}

#[derive(Debug, Serialize)]
pub struct Report {
    #[serde(rename = "isUp")]
    pub up: bool,
    #[serde(rename = "reports")]
    pub periods: Vec<PeriodReport>,
    #[serde(rename = "loop")]
    pub loop_status: LoopReport,
    #[serde(rename = "downAction", skip_serializing_if = "Option::is_none")]
    pub down_action: Option<DownActionStatus>,
    #[serde(rename = "updUptime")]
    pub uptime: HumanDuration,
    #[serde(rename = "updVersion")]
    pub version: String,
    #[serde(rename = "generatedAt")]
    pub generated_at: String,
}

#[derive(Debug)]
pub struct Status {
    started: Instant,
    up: Option<bool>,
    tracker: Option<Tracker>,
    /// The periods and tuning the tracker was built for.
    configured: Option<(Vec<Duration>, BucketConfig)>,
    checks: u64,
    last_checked: Option<Instant>,
    last_success: Option<Instant>,
    next_check: Option<Instant>,
    interval: Duration,
    down_action: Option<DownActionStatus>,
}

impl Status {
    #[must_use]
    pub fn new(started: Instant) -> Self {
        Self {
            started,
            up: None,
            tracker: None,
            configured: None,
            checks: 0,
            last_checked: None,
            last_success: None,
            next_check: None,
            interval: Duration::ZERO,
            down_action: None,
        }
    }

    /// Applies the report periods. The history is kept when they and the bucket tuning are
    /// unchanged, as on a reload that touches something else.
    pub fn configure(&mut self, periods: &[Duration], buckets: BucketConfig, now: Instant) {
        if self
            .configured
            .as_ref()
            .is_some_and(|(kept, tuning)| kept == periods && *tuning == buckets)
        {
            return;
        }

        self.configured = Some((periods.to_vec(), buckets));
        self.tracker = (!periods.is_empty()).then(|| Tracker::new(periods, buckets, now));
    }

    pub fn record_probe(&mut self, failed: bool, now: Instant) {
        if let Some(tracker) = &mut self.tracker {
            tracker.record_probe(failed, now);
        }
    }

    pub fn record_round(&mut self, up: bool, now: Instant) {
        self.up = Some(up);
        self.checks += 1;
        self.last_checked = Some(now);
        if up {
            self.last_success = Some(now);
        }
        if let Some(tracker) = &mut self.tracker {
            tracker.record_round(up, now);
        }
    }

    pub fn set_schedule(&mut self, next_check: Instant, interval: Duration) {
        self.next_check = Some(next_check);
        self.interval = interval;
    }

    pub fn set_down_action(&mut self, status: Option<DownActionStatus>) {
        self.down_action = status;
    }

    #[must_use]
    pub fn is_up(&self) -> bool {
        self.up.unwrap_or(false)
    }

    #[must_use]
    pub fn report(
        &self,
        periods: &[Duration],
        now: Instant,
        generated_at: SystemTime,
        version: &str,
    ) -> Report {
        Report {
            up: self.is_up(),
            periods: self.period_reports(periods, now),
            loop_status: LoopReport {
                last_success: self
                    .last_success
                    .map(|at| now.saturating_duration_since(at).into()),
                next_check: self
                    .next_check
                    .map_or(Duration::ZERO, |at| at.saturating_duration_since(now))
                    .into(),
                interval: self.interval.into(),
                time_since_update: self
                    .last_checked
                    .map_or(Duration::ZERO, |at| now.saturating_duration_since(at))
                    .into(),
                total_checks_run: self.checks,
            },
            down_action: self.down_action,
            uptime: now.saturating_duration_since(self.started).into(),
            version: version.to_owned(),
            generated_at: humantime::format_rfc3339_millis(generated_at).to_string(),
        }
    }

    fn period_reports(&self, periods: &[Duration], now: Instant) -> Vec<PeriodReport> {
        let Some(tracker) = &self.tracker else {
            return Vec::new();
        };

        periods
            .iter()
            .zip(tracker.totals(now))
            .map(|(&period, totals)| PeriodReport::new(period, totals))
            .collect()
    }
}

impl PeriodReport {
    /// Availability is the share of observed time spent up. Downtime and coverage are scaled to
    /// the period, since the buckets at the window's edge reach slightly beyond it.
    fn new(period: Duration, totals: Totals) -> Self {
        let observed = totals.up + totals.down;
        let availability =
            (!observed.is_zero()).then(|| totals.up.as_secs_f64() / observed.as_secs_f64());
        let coverage = observed.min(period);
        #[allow(clippy::cast_precision_loss)]
        let failure_rate = (totals.probes > 0).then(|| totals.failed as f64 / totals.probes as f64);

        Self {
            period: period.into(),
            coverage: (coverage < period).then(|| coverage.into()),
            availability: HumanPercent(availability),
            down_time: HumanDuration(
                availability.map(|available| coverage.mul_f64(1.0 - available)),
            ),
            total_probes: totals.probes,
            failed_probes: totals.failed,
            failure_rate: HumanPercent(failure_rate),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    fn secs(n: u64) -> Duration {
        Duration::from_secs(n)
    }

    fn render(status: &Status, periods: &[Duration], now: Instant) -> serde_json::Value {
        let report = status.report(periods, now, SystemTime::UNIX_EPOCH, "9.9.9");
        serde_json::to_value(report).unwrap()
    }

    #[test]
    fn an_unconfigured_status_reports_only_loop_state() {
        let t0 = Instant::now();
        let mut status = Status::new(t0);
        status.record_round(true, t0 + secs(1));
        status.set_schedule(t0 + secs(11), secs(10));

        let json = render(&status, &[], t0 + secs(5));

        assert_eq!(json["isUp"], true);
        assert_eq!(json["reports"], json!([]));
        assert_eq!(json["loop"]["totalChecksRun"], 1);
        assert_eq!(json["loop"]["lastSuccess"], "4s");
        assert_eq!(json["loop"]["nextCheck"], "6s");
        assert_eq!(json["loop"]["interval"], "10s");
        assert_eq!(json["updUptime"], "5s");
        assert_eq!(json["updVersion"], "9.9.9");
        assert_eq!(json["generatedAt"], "1970-01-01T00:00:00.000Z");
        assert!(json.get("downAction").is_none());
    }

    #[test]
    fn before_the_first_round_nothing_is_up_and_there_is_no_last_success() {
        let t0 = Instant::now();
        let status = Status::new(t0);

        let json = render(&status, &[], t0);

        assert_eq!(json["isUp"], false);
        assert!(json["loop"].get("lastSuccess").is_none());
        assert_eq!(json["loop"]["nextCheck"], "0s");
    }

    #[test]
    fn period_reports_combine_availability_and_probe_counts() {
        let t0 = Instant::now();
        let periods = [secs(60), secs(3600)];
        let mut status = Status::new(t0);
        status.configure(&periods, BucketConfig::default(), t0);
        status.record_probe(false, t0 + secs(1));
        status.record_round(true, t0 + secs(1));
        status.record_probe(true, t0 + secs(31));
        status.record_round(false, t0 + secs(31));
        status.record_round(false, t0 + secs(61));

        let json = render(&status, &periods, t0 + secs(61));

        let minute = &json["reports"][0];
        assert_eq!(minute["period"], "1m");
        assert_eq!(minute["availability"], "50.000 %");
        assert_eq!(minute["downTime"], "30s");
        assert_eq!(minute["totalProbes"], 2);
        assert_eq!(minute["failedProbes"], 1);
        assert_eq!(minute["failureRate"], "50.000 %");
        assert!(minute.get("coverage").is_none());

        let hour = &json["reports"][1];
        assert_eq!(hour["coverage"], "1m");
    }

    #[test]
    fn periods_without_probes_have_no_failure_rate() {
        let t0 = Instant::now();
        let periods = [secs(60)];
        let mut status = Status::new(t0);
        status.configure(&periods, BucketConfig::default(), t0);
        status.record_round(true, t0 + secs(1));

        let json = render(&status, &periods, t0 + secs(2));

        assert_eq!(json["reports"][0]["failureRate"], "Not computed");
    }

    #[test]
    fn availability_is_not_computed_until_a_second_round_gives_it_time() {
        let t0 = Instant::now();
        let periods = [secs(60)];
        let mut status = Status::new(t0);
        status.configure(&periods, BucketConfig::default(), t0);
        status.record_round(true, t0 + secs(1));

        let json = render(&status, &periods, t0 + secs(2));

        assert_eq!(json["reports"][0]["availability"], "Not computed");
        assert_eq!(json["reports"][0]["downTime"], "Not computed");
    }

    #[test]
    fn coverage_shows_how_much_of_the_period_has_been_observed() {
        let t0 = Instant::now();
        let periods = [secs(3600)];
        let mut status = Status::new(t0);
        status.configure(&periods, BucketConfig::default(), t0);
        status.record_round(true, t0 + secs(1));
        status.record_round(true, t0 + secs(121));

        let json = render(&status, &periods, t0 + secs(122));

        assert_eq!(json["reports"][0]["coverage"], "2m");
        assert_eq!(json["reports"][0]["availability"], "100.000 %");
    }

    #[test]
    fn the_down_action_appears_when_set() {
        let t0 = Instant::now();
        let mut status = Status::new(t0);
        status.set_down_action(Some(DownActionStatus {
            iteration: 2,
            sleep_time: secs(90),
            backoff_capped: true,
        }));

        let json = render(&status, &[], t0);

        assert_eq!(
            json["downAction"],
            json!({"iteration": 2, "sleepTime": "1m 30s", "backoffCapped": true})
        );
    }

    #[test]
    fn reconfiguring_with_the_same_periods_keeps_the_history() {
        let t0 = Instant::now();
        let periods = [secs(60)];
        let mut status = Status::new(t0);
        status.configure(&periods, BucketConfig::default(), t0);
        status.record_probe(true, t0 + secs(1));

        status.configure(&periods, BucketConfig::default(), t0 + secs(2));

        let json = render(&status, &periods, t0 + secs(3));
        assert_eq!(json["reports"][0]["totalProbes"], 1);
    }

    #[test]
    fn reconfiguring_with_other_periods_starts_afresh() {
        let t0 = Instant::now();
        let mut status = Status::new(t0);
        status.configure(&[secs(60)], BucketConfig::default(), t0);
        status.record_probe(true, t0 + secs(1));

        status.configure(&[secs(120)], BucketConfig::default(), t0 + secs(2));

        let json = render(&status, &[secs(120)], t0 + secs(3));
        assert_eq!(json["reports"][0]["totalProbes"], 0);
    }

    #[test]
    fn dropping_all_periods_stops_the_period_reports() {
        let t0 = Instant::now();
        let mut status = Status::new(t0);
        status.configure(&[secs(60)], BucketConfig::default(), t0);

        status.configure(&[], BucketConfig::default(), t0);

        assert_eq!(render(&status, &[secs(60)], t0)["reports"], json!([]));
    }
}
