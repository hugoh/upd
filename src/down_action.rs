//! The command run while the connection is down, repeated with exponential backoff.

use std::process::Stdio;
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};
use std::time::Duration;

use backon::{BackoffBuilder, ExponentialBuilder};
use tokio::io::{AsyncRead, AsyncReadExt};
use tokio::process::{Child, Command as Process};
use tokio::sync::watch;
use tokio::task::JoinHandle;
use tokio_util::sync::CancellationToken;
use tracing::{debug, error, info, warn};

use crate::command::Command;
use crate::stats::status::DownActionStatus;

/// How long the stop command may run.
pub const STOP_EXEC_TIMEOUT: Duration = Duration::from_secs(30);
const BACKOFF_FACTOR: f32 = 1.5;
const STDERR_CAP: usize = 8 * 1024;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DownAction {
    pub exec: Command,
    /// Run once when the connection comes back, if `exec` ran at least once.
    pub stop_exec: Option<Command>,
    /// Wait before the first run.
    pub after: Duration,
    /// Wait before the second run, growing by half each time after. `None` runs once.
    pub repeat: Option<Duration>,
    /// Ceiling for the growing wait.
    pub backoff_limit: Option<Duration>,
}

/// The waits between runs: `repeat`, then 1.5 times longer each time, up to `limit`.
fn schedule(repeat: Duration, limit: Option<Duration>) -> impl Iterator<Item = Duration> {
    let builder = ExponentialBuilder::new()
        .with_min_delay(repeat)
        .with_factor(BACKOFF_FACTOR)
        .without_max_times();
    match limit {
        Some(limit) => builder.with_max_delay(limit),
        None => builder.without_max_delay(),
    }
    .build()
}

/// A running down-action loop.
#[derive(Debug)]
pub struct DownActionHandle {
    cancel: CancellationToken,
    skip_stop: Arc<AtomicBool>,
    join: JoinHandle<()>,
    status: watch::Receiver<DownActionStatus>,
}

impl DownActionHandle {
    #[must_use]
    pub fn status(&self) -> DownActionStatus {
        *self.status.borrow()
    }

    /// Ends the loop, kills the running command, then runs the stop command to completion.
    pub async fn stop(self) {
        self.end().await;
    }

    /// Like [`stop`](Self::stop), without the stop command: for when the connection has not
    /// come back, as when upd shuts down or the monitor is replaced by a reload.
    pub async fn abandon(self) {
        self.skip_stop.store(true, Ordering::Release);
        self.end().await;
    }

    async fn end(self) {
        self.cancel.cancel();
        if let Err(err) = self.join.await {
            error!(%err, "down action task failed");
        }
    }
}

impl DownAction {
    /// Starts the loop; it runs until the handle is stopped.
    #[must_use]
    pub fn start(&self) -> DownActionHandle {
        let cancel = CancellationToken::new();
        let skip_stop = Arc::new(AtomicBool::new(false));
        let (sender, status) = watch::channel(DownActionStatus {
            iteration: 0,
            sleep_time: self.after,
            backoff_capped: false,
        });
        debug!("starting down action loop");
        let join = tokio::spawn(
            self.clone()
                .run(cancel.clone(), Arc::clone(&skip_stop), sender),
        );
        DownActionHandle {
            cancel,
            skip_stop,
            join,
            status,
        }
    }

    async fn run(
        self,
        cancel: CancellationToken,
        skip_stop: Arc<AtomicBool>,
        status: watch::Sender<DownActionStatus>,
    ) {
        let mut waits = self
            .repeat
            .map(|repeat| schedule(repeat, self.backoff_limit));
        let mut wait = self.after;
        let mut iteration = 0_u32;
        let mut running: Option<Running> = None;
        let mut ran = false;

        loop {
            tokio::select! {
                () = cancel.cancelled() => break,
                () = tokio::time::sleep(wait) => {}
            }

            if let Some(previous) = running.take() {
                previous.kill().await;
            }
            ran = true;
            running = Running::spawn(&self.exec, iteration);

            let Some(next) = waits.as_mut().and_then(Iterator::next) else {
                debug!("down action runs once; waiting to be stopped");
                cancel.cancelled().await;
                break;
            };
            iteration += 1;
            wait = next;
            status.send_replace(DownActionStatus {
                iteration,
                sleep_time: next,
                backoff_capped: self.backoff_limit.is_some_and(|limit| next >= limit),
            });
        }

        debug!("down action loop canceled");
        if let Some(last) = running {
            last.kill().await;
        }
        // The stop command reports on what the command did, so it has nothing to say if it never ran.
        if let Some(stop) = self
            .stop_exec
            .as_ref()
            .filter(|_| ran && !skip_stop.load(Ordering::Acquire))
        {
            run_stop_command(stop, iteration).await;
        }
    }
}

/// A started command, watched by its own task so its exit is always logged.
struct Running {
    kill: CancellationToken,
    watcher: JoinHandle<()>,
}

impl Running {
    fn spawn(command: &Command, iteration: u32) -> Option<Self> {
        info!(exec = %command, iteration, "executing command");
        let child = match start(command, iteration) {
            Ok(child) => child,
            Err(err) => {
                error!(exec = %command, %err, "failed to run");
                return None;
            }
        };

        let kill = CancellationToken::new();
        let watcher = tokio::spawn({
            let kill = kill.clone();
            let command = command.clone();
            async move {
                let mut child = child;
                tokio::select! {
                    () = kill.cancelled() => {
                        warn!(exec = %command, "killing current command");
                        kill_tree(&mut child).await;
                    }
                    () = wait_logged(&mut child, &command) => {}
                }
            }
        });
        Some(Self { kill, watcher })
    }

    async fn kill(self) {
        self.kill.cancel();
        let _ = self.watcher.await;
    }
}

fn start(command: &Command, iteration: u32) -> std::io::Result<Child> {
    let mut process = Process::new(command.program());
    process
        .args(command.args())
        .env("UPD_ITERATION", iteration.to_string())
        .stdin(Stdio::null())
        .stdout(Stdio::null())
        .stderr(Stdio::piped())
        .kill_on_drop(true);
    // Its own process group, so that killing it also reaches what it started.
    #[cfg(unix)]
    process.process_group(0);
    process.spawn()
}

/// Kills the command and anything it started, then reaps it.
async fn kill_tree(child: &mut Child) {
    #[cfg(unix)]
    if let Some(group) = child
        .id()
        .and_then(|id| i32::try_from(id).ok())
        .and_then(rustix::process::Pid::from_raw)
    {
        let _ = rustix::process::kill_process_group(group, rustix::process::Signal::KILL);
    }
    if let Err(err) = child.kill().await {
        warn!(%err, "failed to kill command");
    }
}

async fn run_stop_command(command: &Command, iteration: u32) {
    info!(exec = %command, iteration, "executing stop command");
    let mut child = match start(command, iteration) {
        Ok(child) => child,
        Err(err) => {
            warn!(exec = %command, %err, "failed to execute stop command");
            return;
        }
    };

    if tokio::time::timeout(STOP_EXEC_TIMEOUT, wait_logged(&mut child, command))
        .await
        .is_err()
    {
        warn!(exec = %command, "stop command timed out");
        kill_tree(&mut child).await;
    }
}

/// Waits for exit and logs a failure with whatever the command wrote to stderr.
async fn wait_logged(child: &mut Child, command: &Command) {
    let stderr = child.stderr.take();
    let (status, stderr) = tokio::join!(child.wait(), read_capped(stderr, STDERR_CAP));

    match status {
        Ok(status) if status.success() => debug!(exec = %command, "command succeeded"),
        Ok(status) => warn!(
            exec = %command,
            %status,
            stderr = %String::from_utf8_lossy(&stderr).trim(),
            "error executing command"
        ),
        Err(err) => warn!(exec = %command, %err, "error waiting for command"),
    }
}

/// Reads to the end, keeping only the first `cap` bytes so a chatty command cannot fill the pipe.
async fn read_capped<R: AsyncRead + Unpin>(reader: Option<R>, cap: usize) -> Vec<u8> {
    let Some(mut reader) = reader else {
        return Vec::new();
    };
    let mut kept = Vec::new();
    let mut chunk = [0_u8; 1024];
    loop {
        match reader.read(&mut chunk).await {
            Ok(0) | Err(_) => return kept,
            Ok(read) => {
                let room = cap.saturating_sub(kept.len());
                kept.extend_from_slice(&chunk[..read.min(room)]);
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::path::Path;

    fn ms(n: u64) -> Duration {
        Duration::from_millis(n)
    }

    fn secs(n: u64) -> Duration {
        Duration::from_secs(n)
    }

    #[test]
    fn waits_grow_by_half_up_to_the_limit() {
        let waits: Vec<_> = schedule(secs(10), Some(secs(30))).take(6).collect();

        assert_eq!(
            waits,
            [
                secs(10),
                ms(15_000),
                ms(22_500),
                secs(30),
                secs(30),
                secs(30)
            ]
        );
    }

    #[test]
    fn without_a_limit_the_waits_keep_growing_past_a_minute() {
        let waits: Vec<_> = schedule(secs(10), None).take(8).collect();

        assert!(waits.windows(2).all(|pair| pair[1] > pair[0]));
        assert!(waits[7] > secs(60));
    }

    #[test]
    fn the_schedule_never_ends() {
        assert_eq!(schedule(ms(1), None).take(1000).count(), 1000);
    }

    #[tokio::test]
    async fn stderr_is_capped_but_fully_drained() {
        let data = vec![b'x'; 5000];

        let kept = read_capped(Some(data.as_slice()), 100).await;

        assert_eq!(kept.len(), 100);
        assert!(read_capped(None::<&[u8]>, 100).await.is_empty());
    }

    fn append_command(file: &Path, text: &str) -> Command {
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

    fn action(dir: &Path, repeat: Option<Duration>) -> DownAction {
        DownAction {
            exec: append_command(&dir.join("exec"), "run"),
            stop_exec: Some(append_command(&dir.join("stop"), "stop")),
            after: ms(10),
            repeat,
            backoff_limit: None,
        }
    }

    #[tokio::test]
    async fn repeats_with_an_increasing_iteration_then_runs_the_stop_command() {
        let dir = tempfile::tempdir().unwrap();
        let handle = action(dir.path(), Some(ms(20))).start();

        tokio::time::sleep(ms(400)).await;
        handle.stop().await;

        let runs = lines(&dir.path().join("exec"));
        assert!(runs.len() >= 3, "expected repeated runs, got {runs:?}");
        assert_eq!(&runs[..3], ["run0", "run1", "run2"]);
        assert_eq!(lines(&dir.path().join("stop")).len(), 1);

        tokio::time::sleep(ms(100)).await;
        assert_eq!(lines(&dir.path().join("exec")).len(), runs.len());
    }

    #[tokio::test]
    async fn without_repeat_the_command_runs_once() {
        let dir = tempfile::tempdir().unwrap();
        let handle = action(dir.path(), None).start();

        tokio::time::sleep(ms(300)).await;
        handle.stop().await;

        assert_eq!(lines(&dir.path().join("exec")), ["run0"]);
        assert_eq!(lines(&dir.path().join("stop")), ["stop0"]);
    }

    #[tokio::test]
    async fn stopping_before_the_first_run_skips_the_stop_command() {
        let dir = tempfile::tempdir().unwrap();
        let mut down = action(dir.path(), Some(ms(20)));
        down.after = secs(60);
        let handle = down.start();

        handle.stop().await;

        assert!(lines(&dir.path().join("exec")).is_empty());
        assert!(lines(&dir.path().join("stop")).is_empty());
    }

    #[tokio::test]
    async fn status_tracks_the_iteration_and_the_next_wait() {
        let dir = tempfile::tempdir().unwrap();
        let mut down = action(dir.path(), Some(ms(20)));
        down.after = secs(60);
        down.backoff_limit = Some(ms(20));
        let handle = down.start();

        assert_eq!(
            handle.status(),
            DownActionStatus {
                iteration: 0,
                sleep_time: secs(60),
                backoff_capped: false
            }
        );
        handle.stop().await;
    }

    #[tokio::test]
    async fn a_command_that_cannot_start_is_survived() {
        let dir = tempfile::tempdir().unwrap();
        let mut down = action(dir.path(), Some(ms(20)));
        down.exec = Command::parse("/nonexistent/upd-test-binary").unwrap();
        let handle = down.start();

        tokio::time::sleep(ms(100)).await;
        assert!(handle.status().iteration >= 1);
        handle.stop().await;

        assert_eq!(lines(&dir.path().join("stop")).len(), 1);
    }

    #[tokio::test]
    async fn the_running_command_is_killed_on_stop() {
        let dir = tempfile::tempdir().unwrap();
        let marker = dir.path().join("survived");
        let mut down = action(dir.path(), None);
        down.exec =
            Command::parse(&format!("sh -c 'sleep 1; touch {}'", marker.display())).unwrap();
        let handle = down.start();

        tokio::time::sleep(ms(200)).await;
        handle.stop().await;
        tokio::time::sleep(ms(1200)).await;

        assert!(!marker.exists());
    }

    #[tokio::test]
    async fn abandoning_kills_the_command_but_skips_the_stop_command() {
        let dir = tempfile::tempdir().unwrap();
        let handle = action(dir.path(), Some(ms(20))).start();
        tokio::time::sleep(ms(150)).await;

        handle.abandon().await;

        assert!(!lines(&dir.path().join("exec")).is_empty());
        assert!(lines(&dir.path().join("stop")).is_empty());
    }

    #[cfg(unix)]
    #[tokio::test]
    async fn programs_started_by_the_command_are_killed_too() {
        let dir = tempfile::tempdir().unwrap();
        let pid_file = dir.path().join("pid");
        let mut down = action(dir.path(), None);
        down.exec = Command::parse(&format!(
            "sh -c 'sleep 30 & echo $! > {}; wait'",
            pid_file.display()
        ))
        .unwrap();
        let handle = down.start();
        let pid = loop {
            if let Ok(text) = std::fs::read_to_string(&pid_file)
                && let Ok(pid) = text.trim().parse::<u32>()
            {
                break pid;
            }
            tokio::time::sleep(ms(20)).await;
        };
        assert!(process_exists(pid));

        handle.stop().await;
        tokio::time::sleep(ms(100)).await;

        assert!(!process_exists(pid), "the grandchild survived");
    }

    #[cfg(unix)]
    fn process_exists(pid: u32) -> bool {
        std::process::Command::new("kill")
            .args(["-0", &pid.to_string()])
            .stderr(Stdio::null())
            .status()
            .is_ok_and(|status| status.success())
    }

    #[tokio::test]
    async fn the_status_reports_when_the_backoff_hits_its_limit() {
        let dir = tempfile::tempdir().unwrap();
        let mut down = action(dir.path(), Some(ms(20)));
        down.backoff_limit = Some(ms(20));
        let handle = down.start();

        tokio::time::sleep(ms(200)).await;

        let status = handle.status();
        assert!(status.backoff_capped);
        assert_eq!(status.sleep_time, ms(20));
        handle.stop().await;
    }
}
