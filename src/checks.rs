//! A round of checks: probes tried in order until one succeeds.

use std::time::Duration;

use tokio_util::sync::CancellationToken;

use crate::probe::{Outcome, Prober};
use crate::probe_spec::ProbeSpec;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CheckList {
    /// Tried first, in this order.
    pub ordered: Vec<ProbeSpec>,
    /// Tried after the ordered ones, in a different random order every round.
    pub shuffled: Vec<ProbeSpec>,
    /// How long each probe may take.
    pub timeout: Duration,
}

impl CheckList {
    /// The probes in the order this round should try them.
    #[must_use]
    pub fn plan(&self) -> Vec<&ProbeSpec> {
        let mut shuffled: Vec<&ProbeSpec> = self.shuffled.iter().collect();
        fastrand::shuffle(&mut shuffled);
        self.ordered.iter().chain(shuffled).collect()
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Round {
    /// A probe succeeded.
    Up,
    /// Every probe failed.
    Down,
    /// The round was interrupted before it could tell.
    Cancelled,
}

/// Tries each probe until one succeeds, reporting every outcome to `observe`.
pub async fn run_round(
    prober: &Prober,
    list: &CheckList,
    cancel: &CancellationToken,
    mut observe: impl FnMut(&Outcome),
) -> Round {
    for spec in list.plan() {
        let outcome = tokio::select! {
            () = cancel.cancelled() => return Round::Cancelled,
            outcome = prober.run(spec, list.timeout) => outcome,
        };
        observe(&outcome);
        if outcome.is_success() {
            return Round::Up;
        }
    }
    Round::Down
}

#[cfg(test)]
mod tests {
    use super::*;
    use tokio::net::TcpListener;

    fn tcp(port: u16) -> ProbeSpec {
        ProbeSpec::parse(&format!("tcp://127.0.0.1:{port}")).unwrap()
    }

    async fn open_port() -> (TcpListener, u16) {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let port = listener.local_addr().unwrap().port();
        (listener, port)
    }

    async fn closed_port() -> u16 {
        open_port().await.1
    }

    fn list(ordered: Vec<ProbeSpec>, shuffled: Vec<ProbeSpec>) -> CheckList {
        CheckList {
            ordered,
            shuffled,
            timeout: Duration::from_secs(2),
        }
    }

    fn prober() -> Prober {
        Prober::new("test").unwrap()
    }

    #[test]
    fn the_plan_keeps_ordered_probes_first_and_includes_every_shuffled_one() {
        let checks = list(vec![tcp(1), tcp(2)], (10..30).map(tcp).collect());

        let plan = checks.plan();

        assert_eq!(plan.len(), 22);
        assert_eq!(plan[0], &tcp(1));
        assert_eq!(plan[1], &tcp(2));
        let mut rest: Vec<_> = plan[2..].to_vec();
        rest.sort_by_key(|spec| spec.target());
        let mut expected: Vec<_> = checks.shuffled.iter().collect();
        expected.sort_by_key(|spec| spec.target());
        assert_eq!(rest, expected);
    }

    #[test]
    fn the_shuffled_probes_come_in_varying_orders() {
        let checks = list(vec![], (10..40).map(tcp).collect());

        let first = checks.plan();

        assert!((0..20).any(|_| checks.plan() != first));
    }

    #[tokio::test]
    async fn the_round_stops_at_the_first_success() {
        let (_listener, open) = open_port().await;
        let dead = closed_port().await;
        let checks = list(vec![tcp(dead), tcp(open), tcp(dead)], vec![]);
        let mut seen = Vec::new();

        let round = run_round(&prober(), &checks, &CancellationToken::new(), |outcome| {
            seen.push(outcome.is_success());
        })
        .await;

        assert_eq!(round, Round::Up);
        assert_eq!(seen, [false, true]);
    }

    #[tokio::test]
    async fn the_round_is_down_when_everything_fails() {
        let dead = closed_port().await;
        let checks = list(vec![tcp(dead)], vec![tcp(dead)]);
        let mut seen = 0;

        let round = run_round(&prober(), &checks, &CancellationToken::new(), |_| seen += 1).await;

        assert_eq!(round, Round::Down);
        assert_eq!(seen, 2);
    }

    #[tokio::test]
    async fn the_round_can_be_cancelled() {
        let cancel = CancellationToken::new();
        cancel.cancel();
        let (_listener, open) = open_port().await;

        let round = run_round(&prober(), &list(vec![tcp(open)], vec![]), &cancel, |_| {}).await;

        assert_eq!(round, Round::Cancelled);
    }
}
