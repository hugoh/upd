//! Human-readable JSON values for durations and percentages.

use std::time::Duration;

use serde::{Serialize, Serializer};

pub const NOT_COMPUTED: &str = "Not computed";

/// A duration serialized as `1h 2m 3s`, or "Not computed" when absent.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct HumanDuration(pub Option<Duration>);

impl From<Duration> for HumanDuration {
    fn from(duration: Duration) -> Self {
        Self(Some(duration))
    }
}

/// A ratio in `0.0..=1.0` serialized as `99.912 %`, or "Not computed" when absent.
#[derive(Debug, Clone, Copy, PartialEq)]
pub struct HumanPercent(pub Option<f64>);

impl From<f64> for HumanPercent {
    fn from(ratio: f64) -> Self {
        Self(Some(ratio))
    }
}

impl Serialize for HumanDuration {
    fn serialize<S: Serializer>(&self, serializer: S) -> Result<S::Ok, S::Error> {
        match self.0 {
            Some(duration) => serializer.serialize_str(&format_duration(duration)),
            None => serializer.serialize_str(NOT_COMPUTED),
        }
    }
}

impl Serialize for HumanPercent {
    fn serialize<S: Serializer>(&self, serializer: S) -> Result<S::Ok, S::Error> {
        match self.0 {
            Some(ratio) => serializer.serialize_str(&format!("{:.3} %", ratio * 100.0)),
            None => serializer.serialize_str(NOT_COMPUTED),
        }
    }
}

/// Rounds to the second, then formats like `1h 5s`.
#[must_use]
pub fn format_duration(duration: Duration) -> String {
    let rounded =
        Duration::from_secs(u64::try_from((duration.as_millis() + 500) / 1000).unwrap_or(u64::MAX));
    humantime::format_duration(rounded).to_string()
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::time::Duration;

    #[test]
    fn durations_drop_zero_trailing_units() {
        let secs = Duration::from_secs;
        assert_eq!(format_duration(secs(0)), "0s");
        assert_eq!(format_duration(secs(10)), "10s");
        assert_eq!(format_duration(secs(60)), "1m");
        assert_eq!(format_duration(secs(61)), "1m 1s");
        assert_eq!(format_duration(secs(3600)), "1h");
        assert_eq!(format_duration(secs(3605)), "1h 5s");
        assert_eq!(format_duration(secs(3660)), "1h 1m");
        assert_eq!(format_duration(secs(168 * 3600)), "7days");
    }

    #[test]
    fn durations_round_to_the_nearest_second() {
        assert_eq!(format_duration(Duration::from_millis(1499)), "1s");
        assert_eq!(format_duration(Duration::from_millis(1500)), "2s");
        assert_eq!(format_duration(Duration::from_millis(400)), "0s");
    }

    #[test]
    fn percentages_have_three_decimals() {
        let text = |ratio: f64| serde_json::to_value(HumanPercent::from(ratio)).unwrap();
        assert_eq!(text(1.0), "100.000 %");
        assert_eq!(text(0.99912), "99.912 %");
        assert_eq!(text(0.0), "0.000 %");
    }

    #[test]
    fn missing_values_serialize_as_not_computed() {
        let json = serde_json::json!([
            HumanDuration(None),
            HumanPercent(None),
            HumanDuration::from(Duration::from_secs(90)),
            HumanPercent::from(0.5),
        ]);
        assert_eq!(
            json,
            serde_json::json!(["Not computed", "Not computed", "1m 30s", "50.000 %"])
        );
    }
}
