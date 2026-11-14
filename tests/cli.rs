//! Runs the real binary against local listeners.

use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::Path;
use std::process::{Child, Command, Stdio};
use std::time::{Duration, Instant};

const BIN: &str = env!("CARGO_BIN_EXE_upd");

fn free_port() -> u16 {
    TcpListener::bind("127.0.0.1:0")
        .unwrap()
        .local_addr()
        .unwrap()
        .port()
}

fn config(target: u16, stats: u16, extra: &str) -> String {
    format!(
        r#"
        [checks]
        timeout = "1s"
        [checks.every]
        normal = "50ms"
        down = "50ms"
        [checks.list]
        ordered = ["tcp://127.0.0.1:{target}"]
        [stats]
        port = {stats}
        reports = ["10s"]
        {extra}
        "#
    )
}

fn spawn(config_path: &Path) -> Child {
    Command::new(BIN)
        .arg("-c")
        .arg(config_path)
        .stderr(Stdio::null())
        .spawn()
        .unwrap()
}

fn signal(child: &Child, name: &str) {
    let status = Command::new("kill")
        .args([&format!("-{name}"), &child.id().to_string()])
        .status()
        .unwrap();
    assert!(status.success());
}

fn fetch(port: u16) -> Option<serde_json::Value> {
    let mut stream = TcpStream::connect(("127.0.0.1", port)).ok()?;
    stream
        .write_all(b"GET /stats.json HTTP/1.1\r\nhost: x\r\nconnection: close\r\n\r\n")
        .ok()?;
    let mut reply = String::new();
    stream.read_to_string(&mut reply).ok()?;
    serde_json::from_str(reply.split_once("\r\n\r\n")?.1).ok()
}

fn wait_for<T>(what: &str, mut probe: impl FnMut() -> Option<T>) -> T {
    let deadline = Instant::now() + Duration::from_secs(10);
    while Instant::now() < deadline {
        if let Some(found) = probe() {
            return found;
        }
        std::thread::sleep(Duration::from_millis(25));
    }
    panic!("timed out waiting for {what}");
}

fn wait_for_exit(child: &mut Child) -> std::process::ExitStatus {
    wait_for("the process to exit", || child.try_wait().unwrap())
}

#[test]
fn prints_its_version() {
    for flag in ["-V", "--version"] {
        let output = Command::new(BIN).arg(flag).output().unwrap();

        assert!(output.status.success());
        assert!(String::from_utf8_lossy(&output.stdout).starts_with("upd "));
    }
}

#[test]
fn a_missing_config_file_fails_with_a_message() {
    let output = Command::new(BIN)
        .args(["-c", "/nonexistent/upd.toml"])
        .output()
        .unwrap();

    assert_eq!(output.status.code(), Some(1));
    assert!(String::from_utf8_lossy(&output.stderr).contains("/nonexistent/upd.toml"));
}

#[test]
fn an_invalid_config_names_the_key_at_fault() {
    let dir = tempfile::tempdir().unwrap();
    let path = dir.path().join("upd.toml");
    std::fs::write(
        &path,
        r#"
        [checks]
        timeout = "2s"
        [checks.every]
        normal = "0s"
        down = "1s"
        [checks.list]
        ordered = ["tcp://127.0.0.1:1"]
        "#,
    )
    .unwrap();

    let output = Command::new(BIN).arg("-c").arg(&path).output().unwrap();

    let stderr = String::from_utf8_lossy(&output.stderr);
    assert_eq!(output.status.code(), Some(1));
    assert!(stderr.contains("checks.every.normal"), "{stderr}");
    assert!(stderr.contains("must be greater than 0"), "{stderr}");
}

#[test]
fn without_a_config_file_it_says_where_it_looked() {
    let dir = tempfile::tempdir().unwrap();

    let output = Command::new(BIN)
        .current_dir(dir.path())
        .env("HOME", dir.path())
        .env_remove("XDG_CONFIG_HOME")
        .env_remove("XDG_CONFIG_DIRS")
        .env_remove("UPD_CONFIG")
        .output()
        .unwrap();

    let stderr = String::from_utf8_lossy(&output.stderr);
    assert_eq!(output.status.code(), Some(1));
    assert!(stderr.contains("no configuration file found"), "{stderr}");
    assert!(stderr.contains("/etc/xdg/upd/upd.toml"), "{stderr}");
    assert!(stderr.contains("/etc/upd/upd.toml"), "{stderr}");
}

/// Runs upd in an empty directory with `vars` set, looking for the config in an XDG directory.
fn finds_the_config_through(vars: &[(&str, std::path::PathBuf)], config_dir: &Path) {
    let target = TcpListener::bind("127.0.0.1:0").unwrap();
    let stats = free_port();
    std::fs::create_dir_all(config_dir.join("upd")).unwrap();
    std::fs::write(
        config_dir.join("upd/upd.toml"),
        config(target.local_addr().unwrap().port(), stats, ""),
    )
    .unwrap();
    let empty = tempfile::tempdir().unwrap();
    let mut command = Command::new(BIN);
    command
        .current_dir(empty.path())
        .env("HOME", empty.path())
        .env_remove("XDG_CONFIG_HOME")
        .env_remove("XDG_CONFIG_DIRS")
        .env_remove("UPD_CONFIG")
        .stderr(Stdio::null());
    for (name, value) in vars {
        command.env(name, value);
    }
    let mut child = command.spawn().unwrap();

    wait_for("the stats", || {
        fetch(stats).filter(|json| json["isUp"] == true)
    });

    signal(&child, "TERM");
    assert!(wait_for_exit(&mut child).success());
}

#[test]
fn the_config_is_found_in_xdg_config_home() {
    let dir = tempfile::tempdir().unwrap();

    finds_the_config_through(&[("XDG_CONFIG_HOME", dir.path().to_owned())], dir.path());
}

#[test]
fn the_config_is_found_in_xdg_config_dirs() {
    let dir = tempfile::tempdir().unwrap();

    finds_the_config_through(&[("XDG_CONFIG_DIRS", dir.path().to_owned())], dir.path());
}

#[test]
fn the_config_is_found_in_the_home_config_directory() {
    let home = tempfile::tempdir().unwrap();
    let config_dir = home.path().join(".config");

    finds_the_config_through(&[("HOME", home.path().to_owned())], &config_dir);
}

#[test]
fn the_config_file_can_come_from_the_environment() {
    let dir = tempfile::tempdir().unwrap();
    let path = dir.path().join("elsewhere.toml");
    let target = TcpListener::bind("127.0.0.1:0").unwrap();
    let stats = free_port();
    std::fs::write(
        &path,
        config(target.local_addr().unwrap().port(), stats, ""),
    )
    .unwrap();
    let mut child = Command::new(BIN)
        .env("UPD_CONFIG", &path)
        .stderr(Stdio::null())
        .spawn()
        .unwrap();

    wait_for("the stats", || {
        fetch(stats).filter(|json| json["isUp"] == true)
    });

    signal(&child, "TERM");
    assert!(wait_for_exit(&mut child).success());
}

#[test]
fn environment_variables_are_expanded_in_the_config() {
    let dir = tempfile::tempdir().unwrap();
    let path = dir.path().join("upd.toml");
    let target = TcpListener::bind("127.0.0.1:0").unwrap();
    let stats = free_port();
    std::fs::write(
        &path,
        config(target.local_addr().unwrap().port(), stats, "")
            .replace("\"50ms\"", "\"${UPD_TEST_EVERY}\""),
    )
    .unwrap();
    let mut child = Command::new(BIN)
        .arg("-c")
        .arg(&path)
        .env("UPD_TEST_EVERY", "50ms")
        .stderr(Stdio::null())
        .spawn()
        .unwrap();

    wait_for("the stats", || {
        fetch(stats).filter(|json| json["isUp"] == true)
    });

    signal(&child, "TERM");
    assert!(wait_for_exit(&mut child).success());
}

#[test]
fn serves_stats_and_exits_cleanly_on_sigterm() {
    let dir = tempfile::tempdir().unwrap();
    let path = dir.path().join("upd.toml");
    let target = TcpListener::bind("127.0.0.1:0").unwrap();
    let stats = free_port();
    std::fs::write(
        &path,
        config(target.local_addr().unwrap().port(), stats, ""),
    )
    .unwrap();
    let mut child = spawn(&path);

    let json = wait_for("the stats", || {
        fetch(stats).filter(|json| json["isUp"] == true)
    });

    assert_eq!(json["reports"][0]["period"], "10s");
    assert!(json["updVersion"].is_string());
    signal(&child, "TERM");
    assert!(wait_for_exit(&mut child).success());
}

#[test]
fn sighup_reloads_the_config() {
    let dir = tempfile::tempdir().unwrap();
    let path = dir.path().join("upd.toml");
    let target = TcpListener::bind("127.0.0.1:0").unwrap();
    let target_port = target.local_addr().unwrap().port();
    let stats = free_port();
    std::fs::write(&path, config(target_port, stats, "")).unwrap();
    let mut child = spawn(&path);
    wait_for("the stats", || fetch(stats));

    std::fs::write(
        &path,
        config(target_port, stats, "").replace("[\"10s\"]", "[\"10s\", \"1h\"]"),
    )
    .unwrap();
    signal(&child, "HUP");

    wait_for("the new report period", || {
        fetch(stats).filter(|json| json["reports"].as_array().is_some_and(|r| r.len() == 2))
    });
    signal(&child, "INT");
    assert!(wait_for_exit(&mut child).success());
}

#[test]
fn runs_the_down_action_while_down_and_leaves_the_stop_command_for_recovery() {
    let dir = tempfile::tempdir().unwrap();
    let path = dir.path().join("upd.toml");
    let (exec, stop) = (dir.path().join("exec"), dir.path().join("stop"));
    let closed = free_port();
    let stats = free_port();
    let down_action = format!(
        r#"
        [downAction]
        exec = "sh -c 'echo $UPD_ITERATION >> {}'"
        stopExec = "sh -c 'echo stopped >> {}'"
        [downAction.every]
        after = "10ms"
        repeat = "30ms"
        "#,
        exec.display(),
        stop.display()
    );
    std::fs::write(
        &path,
        format!("{down_action}\n{}", config(closed, stats, "")),
    )
    .unwrap();
    let mut child = spawn(&path);

    wait_for("repeated runs", || {
        let runs = std::fs::read_to_string(&exec).unwrap_or_default();
        (runs.lines().count() >= 2).then_some(())
    });
    let json = wait_for("a down report", || {
        fetch(stats).filter(|json| json["isUp"] == false)
    });
    assert!(json["downAction"]["iteration"].as_u64().is_some());

    signal(&child, "TERM");
    assert!(wait_for_exit(&mut child).success());
    assert!(!stop.exists(), "the connection never came back");
}
