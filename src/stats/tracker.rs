//! What happened in each report period: time up and down, and probes made and failed.
//!
//! One ring of time buckets per period, each bucket counting what fell into its slice of time,
//! so a report sums the buckets that lie inside its window.

use std::collections::VecDeque;
use std::time::{Duration, Instant};

pub const DEFAULT_BUCKETS_MIN: u32 = 100;
pub const DEFAULT_BUCKET_MAX_SPAN: Duration = Duration::from_mins(30);
pub const MIN_BUCKET_INTERVAL: Duration = Duration::from_secs(1);
/// Bounds a ring to about 800 KiB.
pub const MAX_BUCKETS_PER_PERIOD: u64 = 100_000;

/// The sum of the buckets inside a report window.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub struct Totals {
    /// Time the connection was seen up.
    pub up: Duration,
    /// Time the connection was seen down.
    pub down: Duration,
    pub probes: u64,
    pub failed: u64,
}

/// Tunes how finely each report period is divided.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct BucketConfig {
    /// The least number of buckets a report period is split into. Not zero.
    pub min: u32,
    /// The longest time a single bucket may cover.
    pub max_span: Duration,
}

impl Default for BucketConfig {
    fn default() -> Self {
        Self {
            min: DEFAULT_BUCKETS_MIN,
            max_span: DEFAULT_BUCKET_MAX_SPAN,
        }
    }
}

impl BucketConfig {
    /// `period / min`, at most `max_span`, at least one second.
    #[must_use]
    pub fn interval(&self, period: Duration) -> Duration {
        (period / self.min.max(1))
            .min(self.max_span)
            .max(MIN_BUCKET_INTERVAL)
    }

    /// The ring size needed to cover `period`.
    #[must_use]
    pub fn bucket_count(&self, period: Duration) -> u64 {
        let interval = self.interval(period).as_nanos();
        u64::try_from(period.as_nanos() / interval + 1).unwrap_or(u64::MAX)
    }
}

#[derive(Debug, Clone, Copy, Default)]
struct Bucket {
    up_ms: u32,
    down_ms: u32,
    probes: u32,
    failed: u32,
}

#[derive(Debug, Clone)]
struct Ring {
    period: Duration,
    interval_nanos: u128,
    capacity: usize,
    buckets: VecDeque<Bucket>,
    /// Absolute index (intervals since the tracker's origin) of the newest bucket.
    newest: u128,
}

impl Ring {
    fn new(period: Duration, config: BucketConfig) -> Self {
        let capacity = usize::try_from(config.bucket_count(period)).unwrap_or(usize::MAX);
        Self {
            period,
            interval_nanos: config.interval(period).as_nanos(),
            capacity,
            buckets: VecDeque::new(),
            newest: 0,
        }
    }

    fn index_at(&self, origin: Instant, at: Instant) -> u128 {
        at.saturating_duration_since(origin).as_nanos() / self.interval_nanos
    }

    fn update(&mut self, index: u128, change: impl FnOnce(&mut Bucket)) {
        self.advance_to(index);
        if let Some(bucket) = self.buckets.back_mut() {
            change(bucket);
        }
    }

    fn advance_to(&mut self, index: u128) {
        if self.buckets.is_empty() {
            self.buckets.push_back(Bucket::default());
            self.newest = index;
            return;
        }
        if index <= self.newest {
            return;
        }

        let steps = index - self.newest;
        self.newest = index;
        if steps >= self.capacity as u128 {
            self.buckets.clear();
            self.buckets.push_back(Bucket::default());
            return;
        }
        for _ in 0..steps {
            self.buckets.push_back(Bucket::default());
        }
        while self.buckets.len() > self.capacity {
            self.buckets.pop_front();
        }
    }

    fn totals(&self, origin: Instant, now: Instant) -> Totals {
        let Some(count) = u128::try_from(self.buckets.len()).ok().filter(|n| *n > 0) else {
            return Totals::default();
        };

        let cutoff = now
            .saturating_duration_since(origin)
            .saturating_sub(self.period);
        // A bucket counts only when it starts at or after the cutoff.
        let first_included = cutoff.as_nanos().div_ceil(self.interval_nanos);
        let oldest = self.newest + 1 - count;
        let skip = usize::try_from(first_included.saturating_sub(oldest))
            .unwrap_or(usize::MAX)
            .min(self.buckets.len());

        self.buckets
            .iter()
            .skip(skip)
            .fold(Totals::default(), |sum, bucket| Totals {
                up: sum.up + Duration::from_millis(u64::from(bucket.up_ms)),
                down: sum.down + Duration::from_millis(u64::from(bucket.down_ms)),
                probes: sum.probes + u64::from(bucket.probes),
                failed: sum.failed + u64::from(bucket.failed),
            })
    }
}

/// Records probes and check rounds for each configured report period.
#[derive(Debug, Clone)]
pub struct Tracker {
    origin: Instant,
    rings: Vec<Ring>,
    /// When the last round ran and what it found: the state that held until the next round.
    last_round: Option<(Instant, bool)>,
}

impl Tracker {
    #[must_use]
    pub fn new(periods: &[Duration], config: BucketConfig, origin: Instant) -> Self {
        Self {
            origin,
            rings: periods
                .iter()
                .map(|period| Ring::new(*period, config))
                .collect(),
            last_round: None,
        }
    }

    pub fn record_probe(&mut self, failed: bool, now: Instant) {
        for ring in &mut self.rings {
            let index = ring.index_at(self.origin, now);
            ring.update(index, |bucket| {
                bucket.probes += 1;
                bucket.failed += u32::from(failed);
            });
        }
    }

    /// Counts the time since the previous round as spent in the state that round found.
    pub fn record_round(&mut self, up: bool, now: Instant) {
        let Some((previous, was_up)) = self.last_round.replace((now, up)) else {
            return;
        };
        let elapsed_ms =
            u32::try_from(now.saturating_duration_since(previous).as_millis()).unwrap_or(u32::MAX);

        for ring in &mut self.rings {
            let index = ring.index_at(self.origin, now);
            ring.update(index, |bucket| {
                let spent = if was_up {
                    &mut bucket.up_ms
                } else {
                    &mut bucket.down_ms
                };
                *spent = spent.saturating_add(elapsed_ms);
            });
        }
    }

    /// One entry per period, in the order the tracker was built with.
    #[must_use]
    pub fn totals(&self, now: Instant) -> Vec<Totals> {
        self.rings
            .iter()
            .map(|ring| ring.totals(self.origin, now))
            .collect()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn secs(n: u64) -> Duration {
        Duration::from_secs(n)
    }

    #[test]
    fn interval_is_period_over_min_within_bounds() {
        let config = BucketConfig::default();

        assert_eq!(config.interval(secs(10)), MIN_BUCKET_INTERVAL);
        assert_eq!(config.interval(secs(1000)), secs(10));
        assert_eq!(config.interval(secs(7 * 86_400)), DEFAULT_BUCKET_MAX_SPAN);
    }

    #[test]
    fn interval_honours_explicit_settings() {
        let config = BucketConfig {
            min: 10,
            max_span: secs(60),
        };

        assert_eq!(config.interval(secs(300)), secs(30));
        assert_eq!(config.interval(secs(3600)), secs(60));
    }

    #[test]
    fn bucket_count_covers_the_period_plus_the_current_bucket() {
        let config = BucketConfig::default();

        assert_eq!(config.bucket_count(secs(1000)), 101);
        assert_eq!(config.bucket_count(secs(10)), 11);
    }

    #[test]
    fn counts_totals_and_failures_within_the_period() {
        let t0 = Instant::now();
        let mut tracker = Tracker::new(&[secs(60)], BucketConfig::default(), t0);

        tracker.record_probe(false, t0 + secs(1));
        tracker.record_probe(true, t0 + secs(2));
        tracker.record_probe(true, t0 + secs(3));

        assert_eq!(
            tracker.totals(t0 + secs(4)),
            vec![Totals {
                probes: 3,
                failed: 2,
                ..Totals::default()
            }]
        );
    }

    #[test]
    fn results_older_than_the_period_drop_out() {
        let t0 = Instant::now();
        let mut tracker = Tracker::new(&[secs(10)], BucketConfig::default(), t0);

        tracker.record_probe(true, t0 + secs(1));
        tracker.record_probe(false, t0 + secs(50));

        assert_eq!(
            tracker.totals(t0 + secs(51)),
            vec![Totals {
                probes: 1,
                failed: 0,
                ..Totals::default()
            }]
        );
    }

    #[test]
    fn a_gap_longer_than_the_ring_clears_it() {
        let t0 = Instant::now();
        let mut tracker = Tracker::new(&[secs(10)], BucketConfig::default(), t0);

        tracker.record_probe(true, t0);
        tracker.record_probe(false, t0 + secs(100_000));

        assert_eq!(
            tracker.totals(t0 + secs(100_000)),
            vec![Totals {
                probes: 1,
                failed: 0,
                ..Totals::default()
            }]
        );
    }

    #[test]
    fn each_period_has_its_own_window() {
        let t0 = Instant::now();
        let mut tracker = Tracker::new(&[secs(10), secs(1000)], BucketConfig::default(), t0);

        tracker.record_probe(true, t0 + secs(1));
        tracker.record_probe(false, t0 + secs(500));

        let stats = tracker.totals(t0 + secs(501));
        assert_eq!(
            stats[0],
            Totals {
                probes: 1,
                failed: 0,
                ..Totals::default()
            }
        );
        assert_eq!(
            stats[1],
            Totals {
                probes: 2,
                failed: 1,
                ..Totals::default()
            }
        );
    }

    #[test]
    fn an_empty_tracker_reports_nothing() {
        let t0 = Instant::now();
        let tracker = Tracker::new(&[secs(10)], BucketConfig::default(), t0);

        assert_eq!(tracker.totals(t0), vec![Totals::default()]);
    }

    #[test]
    fn the_ring_never_grows_past_its_capacity() {
        let t0 = Instant::now();
        let mut tracker = Tracker::new(&[secs(10)], BucketConfig::default(), t0);

        for second in 0..200 {
            tracker.record_probe(false, t0 + secs(second));
        }

        assert!(tracker.rings[0].buckets.len() <= tracker.rings[0].capacity);
        assert_eq!(tracker.totals(t0 + secs(200))[0].probes, 10);
    }

    #[test]
    fn counts_agree_with_a_naive_tally() {
        let t0 = Instant::now();
        let period = secs(600);
        let mut tracker = Tracker::new(&[period], BucketConfig::default(), t0);
        let events: Vec<(u64, bool)> = (0..2000).map(|i| (i * 3, i % 7 == 0)).collect();

        for &(at, failed) in &events {
            tracker.record_probe(failed, t0 + secs(at));
        }

        let now = t0 + secs(6000);
        let interval = BucketConfig::default().interval(period).as_secs();
        let cutoff: u64 = 6000 - 600;
        let first_bucket_start = cutoff.div_ceil(interval) * interval;
        let (total, failed) = events
            .iter()
            .filter(|(at, _)| *at >= first_bucket_start)
            .fold((0, 0), |(t, f), (_, failed)| {
                (t + 1, f + u64::from(*failed))
            });

        assert_eq!(
            tracker.totals(now)[0],
            Totals {
                probes: total,
                failed,
                ..Totals::default()
            }
        );
    }

    #[test]
    fn time_since_the_previous_round_goes_to_the_state_it_found() {
        let t0 = Instant::now();
        let mut tracker = Tracker::new(&[secs(100)], BucketConfig::default(), t0);

        tracker.record_round(true, t0 + secs(1));
        tracker.record_round(true, t0 + secs(11));
        tracker.record_round(false, t0 + secs(16));
        tracker.record_round(true, t0 + secs(20));

        let totals = tracker.totals(t0 + secs(21))[0];
        assert_eq!(totals.up, secs(15));
        assert_eq!(totals.down, secs(4));
    }

    #[test]
    fn the_first_round_has_no_time_to_account_for() {
        let t0 = Instant::now();
        let mut tracker = Tracker::new(&[secs(100)], BucketConfig::default(), t0);

        tracker.record_round(true, t0 + secs(5));

        assert_eq!(tracker.totals(t0 + secs(6))[0], Totals::default());
    }

    #[test]
    fn time_outside_the_window_is_not_counted() {
        let t0 = Instant::now();
        let mut tracker = Tracker::new(&[secs(30)], BucketConfig::default(), t0);

        for at in (0..=15).step_by(5) {
            tracker.record_round(false, t0 + secs(at));
        }
        for at in (20..=100).step_by(5) {
            tracker.record_round(true, t0 + secs(at));
        }

        let totals = tracker.totals(t0 + secs(101))[0];
        assert_eq!(totals.down, Duration::ZERO);
        assert!(
            totals.up >= secs(25) && totals.up <= secs(30),
            "{:?}",
            totals.up
        );
    }

    #[test]
    fn rounds_and_probes_are_counted_independently() {
        let t0 = Instant::now();
        let mut tracker = Tracker::new(&[secs(100)], BucketConfig::default(), t0);

        tracker.record_probe(true, t0 + secs(1));
        tracker.record_round(false, t0 + secs(1));
        tracker.record_probe(false, t0 + secs(5));
        tracker.record_round(true, t0 + secs(5));

        assert_eq!(
            tracker.totals(t0 + secs(6))[0],
            Totals {
                up: Duration::ZERO,
                down: secs(4),
                probes: 2,
                failed: 1
            }
        );
    }
}
