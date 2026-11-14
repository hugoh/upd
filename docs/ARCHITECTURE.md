# Architecture

`upd` checks that the network connection works and runs a command while it does not. It is one
binary on the Tokio runtime.

## What it does

1. Every `checks.every.normal` (up) or `checks.every.down` (down), a **round** of checks runs. The
   `ordered` probes are tried in order, then the `shuffled` ones in random order. The first
   success makes the round "up"; if every probe fails it is "down".
2. A probe is chosen by URL scheme: `http(s)://` (any response counts), `tcp://host:port` (a
   connection opens) or `dns://resolver[:port]/domain` (the resolver answers). Each probe is
   bounded by `checks.timeout`.
3. When the state changes to down (the first round counts as a change), the **down action** starts:
   wait `after`, run `exec`, then run it again after `repeat`, growing 1.5 times each time up to
   `expBackoffLimit`. `UPD_ITERATION` tells the command which run it is. Without `repeat` it runs
   once. When the connection comes back, the running command and anything it started (it runs in
   its own process group) are killed and `stopExec` runs once (30 s limit), but only if `exec` ran
   at least once. Shutting down or reloading kills the command too, but does not run `stopExec`.
4. With `stats.port`, `GET /stats.json` reports availability and downtime per report period, probe
   counts, loop timing, the down action and the uptime.

## Modules

| Module | Role |
|---|---|
| `main`, `cli`, `signals` | Arguments, SIGINT/SIGTERM (shutdown) and SIGHUP (reload) |
| `app` | `supervise`: loads the config, runs one *generation* (monitor and stats server) and replaces it on reload |
| `config` | TOML, `${VAR}` expansion and validation (the first problem, with its key) into typed `Settings` |
| `probe_spec` | `ProbeSpec`: what to probe, parsed from the URL strings |
| `probe` | `Prober`: runs one probe and returns an `Outcome` |
| `checks` | `CheckList` (ordered plus shuffled) and `run_round` |
| `monitor` | The loop: rounds, state changes, starting and stopping the down action |
| `down_action`, `command` | The repeated command with backoff, and its subprocesses |
| `stats` | `Tracker` (time-bucketed up and down time and probe counts per report period), `Status` (shared state and the JSON report) |
| `server` | `StatsService` for `/stats.json`, and a small hyper accept loop with timeouts and a shutdown grace period |
| `telemetry` | `tracing` setup with a log level that can change on reload |

## Concurrency

- The **monitor task** drives the checks. It writes the shared `Status` behind a short-lived
  `Mutex` (never held across an `.await`); the stats server reads it per request.
- The **down action** is its own task with a `CancellationToken`. Stopping it kills the running
  command and then, when the connection came back, runs `stopExec` in the same task, so the stop
  command always runs after the loop has quiesced. The monitor stops it in a tracked background
  task so a slow `stopExec` does not delay the next check. Shutdown and reload abandon it
  instead, which skips `stopExec`.
- **Reload.** `supervise` keeps the `Status` across reloads, so history survives; the new
  generation starts with an unknown link state, so a down connection restarts its down action. The
  old down action is killed without running `stopExec`, since the connection has not come back. A
  config that fails to load on reload is logged and ignored.
- **A monitor that dies** (a panic) makes `supervise` return an error, so the process exits and a
  service manager can restart it.
- **The stats server** drops clients that are slow to send their request (`stats.readTimeout`,
  3 s by default) and gives requests in flight 5 s on shutdown before cutting them off.
- **Name lookups** for `http://` and `tcp://` probes go through `hickory-resolver` with the system
  DNS settings, not `getaddrinfo`, so a lookup that hangs while the network is down is cancelled
  with its probe instead of holding a thread.
- Backoff delays come from `backon`'s `ExponentialBuilder`, used as a plain iterator.

## Choices

- **Process groups by hand, not `process-wrap`.** A down-action command starts in its own process
  group (`process_group(0)`) and is killed with `rustix::process::kill_process_group`, so anything
  a wrapper script started dies with it. `process-wrap` would do the same with
  `ProcessGroup::leader()` and `KillOnDrop`, and I tried it: the tests pass and `down_action.rs`
  loses about 9 lines and its `cfg(unix)` gating. Its `wait()` also reaps leftover members of the
  group, and with its `job-object` feature it would cover Windows. Against that, it adds four
  crates (`nix` replaces `rustix`), and it calls `libc::waitpid` itself in an `unsafe` block,
  where `rustix` wraps the syscall in a safe API. That does not touch upd's own
  `unsafe_code = "forbid"`, and the tree already holds plenty of `unsafe` in `tokio`, `hyper` and
  `ring`, so it is a minor point. Orphans left by a killed group are adopted by init, so nothing
  builds up, unless upd is itself PID 1 (a container with no init). That is where
  `process-wrap`'s reaping would help; running such a container with an init (`INSTALL.md`) is the
  usual answer. We do not ship Windows, so the by-hand version stays. Revisit it if Windows builds
  come back.

- **The `xdg` crate for the default config path.** It implements the whole XDG Base Directory
  specification (`XDG_CONFIG_HOME`, `XDG_CONFIG_DIRS`, relative values ignored, home found as
  `std::env::home_dir()` does) with no dependencies. upd asks it only for the directories and
  adds `./.upd.toml` first and `/etc/upd/upd.toml` last. `etcetera` and `dirs` do not search
  `XDG_CONFIG_DIRS`, and `dirs` uses `~/Library/Application Support` on macOS. `xdg` reads the
  real environment, so the lookup is tested through the binary with the variables set. It is
  Unix-only, which is all we ship.

## Differences from the Go version

The user-facing list, with what to change, is "Upgrading from 4.x" in the README. This is the
same list with the reasons.

- An invalid config on SIGHUP keeps the old one running instead of exiting.
- Unknown TOML keys are errors, including `stats.writeTimeout` and `idleTimeout`, which the Go
  version accepted. `readTimeout` limits how long a client may take to send its request.
- A setting that is absent is off. `0` is not a way to turn things off: `stats.port`, `repeat`,
  `expBackoffLimit`, `buckets.min`, `buckets.maxSpan` and the timeouts must be positive when set.
- `${VAR}` is expanded in string values after parsing, not in the raw text, so comments are
  untouched. The first problem found is reported, with its key, instead of all of them.
- Commands are split and checked when the configuration is loaded.
- `logLevel` accepts `error` to `trace`; `--debug` wins over it.
- The default config file is searched for along the XDG base directories (the `xdg` crate), after
  `./.upd.toml` and before `/etc/upd/upd.toml`.
- `checks.every.normal` and `down` are limited to 24 h.
- HTTP probes do not follow redirects: any response means the connection works.
- `dns://` resolvers must be IP addresses.
- Statistics use a monotonic clock and one time-bucketed structure. Availability is the share of
  observed time spent up, accurate to one bucket (at most 30 min, about 1% of the period by
  default), and each stretch between two rounds counts in the bucket of the later round. The
  report keeps its history across a reload as long as `stats.reports` and `stats.buckets` are
  unchanged. `generatedAt` is UTC with milliseconds.
- `stopExec` runs only when the connection comes back after `exec` ran at least once, not when
  upd shuts down or reloads.
- Captured stderr of down-action commands is capped at 8 KiB, and a command's process group is
  killed with it.
- Logging is `tracing`'s text format, coloured only on a terminal, instead of `slog`'s.
- The version flag is `-V`, as clap spells it, not `-v`.
- Release builds are glibc binaries for Linux x86_64 and aarch64 and a macOS aarch64 binary, plus
  static musl binaries for Linux, which run on systems without glibc such as OpenWrt. The glibc
  archives keep the Go names (`upd_<version>_linux_arm64.tar.gz`); the musl ones add `_musl`
  (`upd_<version>_linux_arm64_musl.tar.gz`). The deb and rpm packages use the glibc binaries.
  Windows, FreeBSD, 32-bit and macOS on Intel are not built: the `xdg` crate and the signal
  handling are Unix-only, and the Go release's other targets were not carried over.
