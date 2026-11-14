//! Runs the monitor and the stats server, and replaces them when the configuration is reloaded.

use std::path::Path;
use std::time::Instant;

use anyhow::Context;
use tokio::sync::mpsc;
use tokio::task::JoinHandle;
use tokio_util::sync::CancellationToken;
use tracing::{error, info};

use crate::config::Settings;
use crate::monitor::Monitor;
use crate::probe::Prober;
use crate::server::{self, ServeOptions, StatsService};
use crate::stats::shared::Shared;
use crate::stats::status::Status;
use crate::telemetry::Telemetry;
use crate::version::VERSION;

/// Runs until `shutdown` is cancelled, reloading the configuration at each message on `reloads`.
///
/// A configuration that cannot be loaded at startup is an error; one that cannot be loaded on a
/// reload is logged, and the current one stays in force.
///
/// # Errors
///
/// Fails when the first configuration cannot be loaded or the HTTP client cannot be built.
pub async fn supervise(
    config_path: &Path,
    telemetry: &Telemetry,
    shutdown: CancellationToken,
    mut reloads: mpsc::Receiver<()>,
) -> anyhow::Result<()> {
    let settings = Settings::load(config_path).context("cannot configure app")?;
    telemetry.apply(settings.log_level);

    let status = Shared::new(Status::new(Instant::now()));
    let prober = Prober::new(VERSION).context("cannot build the HTTP client")?;
    let mut generation = Generation::start(settings, &status, &prober, &shutdown);
    let mut reloads_open = true;

    loop {
        tokio::select! {
            () = shutdown.cancelled() => break,
            died = generation.died() => {
                if shutdown.is_cancelled() {
                    break;
                }
                return Err(died);
            }
            reload = reloads.recv(), if reloads_open => {
                if reload.is_none() {
                    reloads_open = false;
                    continue;
                }
                info!("reloading configuration");
                match Settings::load(config_path) {
                    Ok(settings) => {
                        generation.stop().await;
                        telemetry.apply(settings.log_level);
                        generation = Generation::start(settings, &status, &prober, &shutdown);
                    }
                    Err(err) => error!(%err, "cannot reload configuration; keeping the current one"),
                }
            }
        }
    }

    info!("shutting down");
    generation.stop().await;
    Ok(())
}

/// The monitor and stats server for one configuration.
struct Generation {
    cancel: CancellationToken,
    join: Option<JoinHandle<()>>,
}

impl Generation {
    fn start(
        settings: Settings,
        status: &Shared,
        prober: &Prober,
        parent: &CancellationToken,
    ) -> Self {
        let cancel = parent.child_token();
        let join = tokio::spawn(run_generation(
            settings,
            status.clone(),
            prober.clone(),
            cancel.clone(),
        ));
        Self {
            cancel,
            join: Some(join),
        }
    }

    /// Resolves, with the reason, if the generation ends without having been told to stop.
    async fn died(&mut self) -> anyhow::Error {
        let Some(join) = self.join.as_mut() else {
            return std::future::pending().await;
        };
        let result = join.await;
        self.join = None;
        match result {
            Ok(()) => anyhow::anyhow!("the monitor stopped unexpectedly"),
            Err(err) => anyhow::anyhow!("the monitor failed: {err}"),
        }
    }

    async fn stop(mut self) {
        self.cancel.cancel();
        if let Some(join) = self.join.take()
            && let Err(err) = join.await
        {
            error!(%err, "monitor task failed");
        }
    }
}

async fn run_generation(
    settings: Settings,
    status: Shared,
    prober: Prober,
    cancel: CancellationToken,
) {
    status.lock().configure(
        &settings.stats.reports,
        settings.stats.buckets,
        Instant::now(),
    );

    let server = match settings.stats.port {
        None => None,
        Some(port) => match server::bind(port.get()).await {
            Ok(listener) => Some(tokio::spawn(server::serve(
                listener,
                StatsService::new(status.clone(), &settings.stats.reports),
                cancel.clone(),
                ServeOptions::new(settings.stats.read_timeout),
            ))),
            Err(err) => {
                error!(port, %err, "error starting stats server");
                None
            }
        },
    };

    Monitor::new(
        settings.checks,
        settings.delays,
        settings.down_action,
        prober,
        status,
    )
    .run(cancel.clone())
    .await;

    cancel.cancel();
    if let Some(server) = server {
        let _ = server.await;
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::time::Duration;
    use tokio::io::{AsyncReadExt, AsyncWriteExt};
    use tokio::net::{TcpListener, TcpStream};

    use crate::telemetry;

    fn config(target_port: u16, stats_port: u16, reports: &str) -> String {
        format!(
            r#"
            [checks]
            timeout = "1s"
            [checks.every]
            normal = "50ms"
            down = "50ms"
            [checks.list]
            ordered = ["tcp://127.0.0.1:{target_port}"]
            [stats]
            port = {stats_port}
            reports = {reports}
            "#
        )
    }

    async fn free_port() -> u16 {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        listener.local_addr().unwrap().port()
    }

    async fn fetch(port: u16) -> Option<serde_json::Value> {
        let mut stream = TcpStream::connect(("127.0.0.1", port)).await.ok()?;
        stream
            .write_all(b"GET /stats.json HTTP/1.1\r\nhost: x\r\nconnection: close\r\n\r\n")
            .await
            .ok()?;
        let mut reply = String::new();
        stream.read_to_string(&mut reply).await.ok()?;
        serde_json::from_str(reply.split_once("\r\n\r\n")?.1).ok()
    }

    async fn eventually_json(
        port: u16,
        mut condition: impl FnMut(&serde_json::Value) -> bool,
    ) -> serde_json::Value {
        for _ in 0..200 {
            if let Some(json) = fetch(port).await.filter(|json| condition(json)) {
                return json;
            }
            tokio::time::sleep(Duration::from_millis(25)).await;
        }
        panic!("timed out waiting for the stats");
    }

    struct Running {
        shutdown: CancellationToken,
        reload: mpsc::Sender<()>,
        join: JoinHandle<anyhow::Result<()>>,
    }

    fn run(path: &Path) -> Running {
        let shutdown = CancellationToken::new();
        let (reload, reloads) = mpsc::channel(1);
        let join = tokio::spawn({
            let (path, shutdown) = (path.to_owned(), shutdown.clone());
            async move { supervise(&path, &telemetry::init(false), shutdown, reloads).await }
        });
        Running {
            shutdown,
            reload,
            join,
        }
    }

    impl Running {
        async fn stop(self) {
            self.shutdown.cancel();
            self.join.await.unwrap().unwrap();
        }
    }

    #[tokio::test]
    async fn an_unusable_config_is_an_error_at_startup() {
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("upd.toml");
        std::fs::write(&path, "nonsense = [").unwrap();

        let err = supervise(
            &path,
            &telemetry::init(false),
            CancellationToken::new(),
            mpsc::channel(1).1,
        )
        .await
        .unwrap_err();

        assert!(
            format!("{err:#}").contains("cannot configure app"),
            "{err:#}"
        );
    }

    #[tokio::test]
    async fn monitors_and_serves_stats_until_shut_down() {
        let target = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let stats_port = free_port().await;
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("upd.toml");
        std::fs::write(
            &path,
            config(
                target.local_addr().unwrap().port(),
                stats_port,
                r#"["10s"]"#,
            ),
        )
        .unwrap();
        let running = run(&path);

        let json = eventually_json(stats_port, |json| json["isUp"] == true).await;

        assert_eq!(json["reports"][0]["period"], "10s");
        running.stop().await;
        assert!(fetch(stats_port).await.is_none(), "the server stopped");
    }

    #[tokio::test]
    async fn a_reload_applies_the_new_config_and_keeps_the_history() {
        let target = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let target_port = target.local_addr().unwrap().port();
        let stats_port = free_port().await;
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("upd.toml");
        std::fs::write(&path, config(target_port, stats_port, r#"["10s"]"#)).unwrap();
        let running = run(&path);
        let before = eventually_json(stats_port, |json| {
            json["loop"]["totalChecksRun"].as_u64() >= Some(2)
        })
        .await;

        std::fs::write(&path, config(target_port, stats_port, r#"["10s", "15m"]"#)).unwrap();
        running.reload.send(()).await.unwrap();

        let after = eventually_json(stats_port, |json| {
            json["reports"].as_array().is_some_and(|r| r.len() == 2)
        })
        .await;
        assert_eq!(after["reports"][1]["period"], "15m");
        assert!(
            after["loop"]["totalChecksRun"].as_u64() >= before["loop"]["totalChecksRun"].as_u64()
        );
        running.stop().await;
    }

    #[tokio::test]
    async fn a_bad_reload_keeps_the_running_config() {
        let target = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let stats_port = free_port().await;
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("upd.toml");
        std::fs::write(
            &path,
            config(
                target.local_addr().unwrap().port(),
                stats_port,
                r#"["10s"]"#,
            ),
        )
        .unwrap();
        let running = run(&path);
        eventually_json(stats_port, |json| json["isUp"] == true).await;

        std::fs::write(&path, "broken [").unwrap();
        running.reload.send(()).await.unwrap();
        tokio::time::sleep(Duration::from_millis(300)).await;

        let json = fetch(stats_port).await.expect("still serving");
        assert_eq!(json["isUp"], true);
        running.stop().await;
    }

    #[tokio::test]
    async fn a_taken_stats_port_does_not_stop_the_monitoring() {
        let target = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let taken = TcpListener::bind("0.0.0.0:0").await.unwrap();
        let dir = tempfile::tempdir().unwrap();
        let path = dir.path().join("upd.toml");
        std::fs::write(
            &path,
            config(
                target.local_addr().unwrap().port(),
                taken.local_addr().unwrap().port(),
                "[]",
            ),
        )
        .unwrap();
        let running = run(&path);

        tokio::time::sleep(Duration::from_millis(200)).await;

        running.stop().await;
    }

    #[tokio::test]
    async fn a_monitor_that_dies_is_reported() {
        let mut panicked = Generation {
            cancel: CancellationToken::new(),
            join: Some(tokio::spawn(async { panic!("boom") })),
        };
        let mut finished = Generation {
            cancel: CancellationToken::new(),
            join: Some(tokio::spawn(async {})),
        };

        assert!(panicked.died().await.to_string().contains("failed"));
        assert!(
            finished
                .died()
                .await
                .to_string()
                .contains("stopped unexpectedly")
        );
        panicked.stop().await;
        finished.stop().await;
    }

    #[tokio::test]
    async fn a_generation_that_was_told_to_stop_does_not_report_a_death() {
        let cancel = CancellationToken::new();
        let mut generation = Generation {
            cancel: cancel.clone(),
            join: Some(tokio::spawn({
                let cancel = cancel.clone();
                async move { cancel.cancelled().await }
            })),
        };

        let waited = tokio::time::timeout(Duration::from_millis(100), generation.died()).await;

        assert!(waited.is_err(), "nothing to report while it runs");
        generation.stop().await;
    }

    #[tokio::test]
    async fn the_stop_command_runs_neither_on_reload_nor_on_shutdown() {
        let dir = tempfile::tempdir().unwrap();
        let (exec, stop) = (dir.path().join("exec"), dir.path().join("stop"));
        let closed = free_port().await;
        let stats_port = free_port().await;
        let path = dir.path().join("upd.toml");
        let write = |reports: &str| {
            std::fs::write(
                &path,
                format!(
                    r#"{}
                    [downAction]
                    exec = "sh -c 'echo run >> {}'"
                    stopExec = "sh -c 'echo stop >> {}'"
                    [downAction.every]
                    after = "10ms"
                    "#,
                    config(closed, stats_port, reports),
                    exec.display(),
                    stop.display()
                ),
            )
            .unwrap();
        };
        write(r#"["10s"]"#);
        let running = run(&path);
        wait_until(|| std::fs::read_to_string(&exec).is_ok_and(|text| text.lines().count() == 1))
            .await;

        write(r#"["10s", "15m"]"#);
        running.reload.send(()).await.unwrap();
        wait_until(|| std::fs::read_to_string(&exec).is_ok_and(|text| text.lines().count() == 2))
            .await;
        running.stop().await;

        assert!(!stop.exists(), "the connection never came back");
    }

    async fn wait_until(mut condition: impl FnMut() -> bool) {
        for _ in 0..200 {
            if condition() {
                return;
            }
            tokio::time::sleep(Duration::from_millis(25)).await;
        }
        panic!("timed out");
    }
}
