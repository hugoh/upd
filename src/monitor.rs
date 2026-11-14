//! The monitoring loop: runs check rounds, tracks the connection state and drives the down action.

use std::time::{Duration, Instant};

use tokio_util::sync::CancellationToken;
use tokio_util::task::TaskTracker;
use tracing::{debug, info, warn};

use crate::checks::{CheckList, Round, run_round};
use crate::config::Delays;
use crate::down_action::{DownAction, DownActionHandle};
use crate::probe::{Outcome, Prober};
use crate::stats::shared::Shared;

pub struct Monitor {
    checks: CheckList,
    delays: Delays,
    down_action: Option<DownAction>,
    prober: Prober,
    status: Shared,
}

impl Monitor {
    #[must_use]
    pub fn new(
        checks: CheckList,
        delays: Delays,
        down_action: Option<DownAction>,
        prober: Prober,
        status: Shared,
    ) -> Self {
        Self {
            checks,
            delays,
            down_action,
            prober,
            status,
        }
    }

    /// Runs until `cancel` is cancelled, then ends the down action without its stop command:
    /// that one is for the connection coming back, which has not happened.
    pub async fn run(self, cancel: CancellationToken) {
        let mut state = State::default();
        let stops = TaskTracker::new();

        loop {
            let round = run_round(&self.prober, &self.checks, &cancel, |outcome| {
                self.observe(outcome);
            })
            .await;
            let up = match round {
                Round::Up => true,
                Round::Down => false,
                Round::Cancelled => break,
            };

            let delay = self.delays.for_state(up);
            self.process(&mut state, up, delay, &stops);
            debug!(wait = ?delay, "waiting for next loop iteration");

            tokio::select! {
                () = cancel.cancelled() => break,
                () = tokio::time::sleep(delay) => {}
            }
        }

        debug!("monitor loop stopping");
        if let Some(handle) = state.down.take() {
            stops.spawn(handle.abandon());
        }
        stops.close();
        stops.wait().await;
    }

    fn observe(&self, outcome: &Outcome) {
        let failed = !outcome.is_success();
        match &outcome.result {
            Ok(response) => debug!(
                protocol = %outcome.scheme,
                target = %outcome.target,
                elapsed = ?outcome.elapsed,
                %response,
                "probe succeeded"
            ),
            Err(error) => warn!(
                protocol = %outcome.scheme,
                target = %outcome.target,
                elapsed = ?outcome.elapsed,
                %error,
                "probe failed"
            ),
        }
        self.status.lock().record_probe(failed, Instant::now());
    }

    fn process(&self, state: &mut State, up: bool, delay: Duration, stops: &TaskTracker) {
        let now = Instant::now();
        let changed = state.link != Some(up);
        state.link = Some(up);
        self.status.lock().record_round(up, now);

        if changed {
            info!(up, "connection status changed");
            self.on_change(state, up, stops);
        }

        let mut status = self.status.lock();
        status.set_schedule(now + delay, delay);
        status.set_down_action(state.down.as_ref().map(DownActionHandle::status));
    }

    fn on_change(&self, state: &mut State, up: bool, stops: &TaskTracker) {
        let Some(action) = &self.down_action else {
            return;
        };
        if up {
            // The stop command may run for a while and must not hold up the next check.
            if let Some(handle) = state.down.take() {
                stops.spawn(handle.stop());
            }
        } else if state.down.is_none() {
            state.down = Some(action.start());
        }
    }
}

#[derive(Default)]
struct State {
    /// `None` until the first round, which always counts as a change.
    link: Option<bool>,
    down: Option<DownActionHandle>,
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::net::SocketAddr;
    use std::path::Path;
    use std::sync::Arc;
    use std::sync::atomic::{AtomicBool, Ordering};
    use std::time::SystemTime;
    use tokio::io::AsyncWriteExt;
    use tokio::net::TcpListener;
    use tokio::task::JoinHandle;

    use crate::command::Command;
    use crate::probe_spec::ProbeSpec;
    use crate::stats::status::Status;
    use crate::stats::tracker::BucketConfig;

    fn ms(n: u64) -> Duration {
        Duration::from_millis(n)
    }

    fn append(file: &Path, text: &str) -> Command {
        Command::parse(&format!(
            "sh -c 'echo {text}$UPD_ITERATION >> {}'",
            file.display()
        ))
        .unwrap()
    }

    fn lines(file: &Path) -> Vec<String> {
        std::fs::read_to_string(file)
            .unwrap_or_default()
            .lines()
            .map(str::to_owned)
            .collect()
    }

    async fn eventually(what: &str, mut condition: impl FnMut() -> bool) {
        for _ in 0..200 {
            if condition() {
                return;
            }
            tokio::time::sleep(ms(25)).await;
        }
        panic!("timed out waiting for {what}");
    }

    /// An HTTP server whose connectivity can be switched: when off it accepts connections but
    /// never answers, so probes time out. Unlike closing a port, this cannot lose the port to
    /// another test.
    struct Switch {
        addr: SocketAddr,
        on: Arc<AtomicBool>,
    }

    impl Switch {
        async fn new(on: bool) -> Self {
            let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
            let addr = listener.local_addr().unwrap();
            let on = Arc::new(AtomicBool::new(on));
            tokio::spawn({
                let on = Arc::clone(&on);
                async move {
                    let mut silent = Vec::new();
                    while let Ok((mut socket, _)) = listener.accept().await {
                        if on.load(Ordering::Acquire) {
                            let _ = socket
                                .write_all(b"HTTP/1.1 204 No Content\r\nconnection: close\r\n\r\n")
                                .await;
                        } else {
                            silent.push(socket);
                        }
                    }
                }
            });
            Self { addr, on }
        }

        fn set(&self, on: bool) {
            self.on.store(on, Ordering::Release);
        }
    }

    struct Fixture {
        status: Shared,
        cancel: CancellationToken,
        join: JoinHandle<()>,
    }

    fn start(target: SocketAddr, down_action: Option<DownAction>) -> Fixture {
        let status = Shared::new(Status::new(Instant::now()));
        status.lock().configure(
            &[Duration::from_secs(60)],
            BucketConfig::default(),
            Instant::now(),
        );
        let monitor = Monitor::new(
            CheckList {
                ordered: vec![ProbeSpec::parse(&format!("http://{target}/")).unwrap()],
                shuffled: vec![],
                timeout: ms(150),
            },
            Delays {
                up: ms(20),
                down: ms(20),
            },
            down_action,
            Prober::new("test").unwrap(),
            status.clone(),
        );
        let cancel = CancellationToken::new();
        let join = tokio::spawn(monitor.run(cancel.clone()));
        Fixture {
            status,
            cancel,
            join,
        }
    }

    impl Fixture {
        async fn stop(self) {
            self.cancel.cancel();
            self.join.await.unwrap();
        }

        fn report(&self) -> crate::stats::status::Report {
            self.status.lock().report(
                &[Duration::from_secs(60)],
                Instant::now(),
                SystemTime::now(),
                "t",
            )
        }
    }

    fn down_action(dir: &Path, repeat: Option<Duration>) -> DownAction {
        DownAction {
            exec: append(&dir.join("exec"), "run"),
            stop_exec: Some(append(&dir.join("stop"), "stop")),
            after: ms(10),
            repeat,
            backoff_limit: None,
        }
    }

    #[tokio::test]
    async fn tracks_the_connection_and_runs_the_down_action_only_while_down() {
        let dir = tempfile::tempdir().unwrap();
        let (exec, stop) = (dir.path().join("exec"), dir.path().join("stop"));
        let target = Switch::new(true).await;
        let fixture = start(target.addr, Some(down_action(dir.path(), Some(ms(30)))));

        eventually("the connection to be seen up", || {
            fixture.status.lock().is_up()
        })
        .await;
        tokio::time::sleep(ms(150)).await;
        assert!(lines(&exec).is_empty(), "no action while up");

        target.set(false);
        eventually("the connection to be seen down", || {
            !fixture.status.lock().is_up()
        })
        .await;
        eventually("the down action to run twice", || lines(&exec).len() >= 2).await;
        assert!(fixture.report().down_action.is_some());

        target.set(true);
        eventually("the connection to be seen up again", || {
            fixture.status.lock().is_up()
        })
        .await;
        eventually("the stop command to run", || lines(&stop).len() == 1).await;
        let runs = lines(&exec).len();
        // The count is of commands started; the last one may have been killed before it wrote.
        let told: usize = lines(&stop)[0].trim_start_matches("stop").parse().unwrap();
        assert!(
            (runs..=runs + 1).contains(&told),
            "the stop command was told {told} runs, {runs} wrote their line"
        );
        tokio::time::sleep(ms(200)).await;
        assert_eq!(
            lines(&exec).len(),
            runs,
            "the action stops when the connection returns"
        );
        assert!(fixture.report().down_action.is_none());

        fixture.stop().await;
    }

    #[tokio::test]
    async fn recovering_before_the_command_first_ran_does_not_run_the_stop_command() {
        let dir = tempfile::tempdir().unwrap();
        let (exec, stop) = (dir.path().join("exec"), dir.path().join("stop"));
        let target = Switch::new(false).await;
        let mut action = down_action(dir.path(), Some(ms(30)));
        action.after = Duration::from_secs(60);
        let fixture = start(target.addr, Some(action));
        eventually("the connection to be seen down", || {
            fixture.report().down_action.is_some()
        })
        .await;

        target.set(true);
        eventually("the connection to be seen up", || {
            fixture.status.lock().is_up()
        })
        .await;
        eventually("the action to be stopped", || {
            fixture.report().down_action.is_none()
        })
        .await;
        tokio::time::sleep(ms(200)).await;

        assert!(lines(&exec).is_empty());
        assert!(
            lines(&stop).is_empty(),
            "nothing was attempted to report on"
        );
        fixture.stop().await;
    }

    #[tokio::test]
    async fn shutting_down_while_down_ends_the_action_without_the_stop_command() {
        let dir = tempfile::tempdir().unwrap();
        let (exec, stop) = (dir.path().join("exec"), dir.path().join("stop"));
        let fixture = start(
            Switch::new(false).await.addr,
            Some(down_action(dir.path(), None)),
        );
        eventually("the action to run", || lines(&exec) == ["run0"]).await;

        fixture.stop().await;

        assert!(lines(&stop).is_empty(), "the connection did not come back");
    }

    #[tokio::test]
    async fn without_a_down_action_it_only_monitors() {
        let target = Switch::new(true).await;
        let fixture = start(target.addr, None);

        eventually("the connection to be seen up", || {
            fixture.status.lock().is_up()
        })
        .await;
        target.set(false);
        eventually("the connection to be seen down", || {
            !fixture.status.lock().is_up()
        })
        .await;

        fixture.stop().await;
    }

    #[tokio::test]
    async fn records_probes_and_schedule_for_the_report() {
        let fixture = start(Switch::new(true).await.addr, None);

        eventually("a few checks", || {
            let report = fixture.report();
            report.loop_status.total_checks_run >= 3 && report.periods[0].total_probes >= 3
        })
        .await;

        fixture.stop().await;
    }
}
