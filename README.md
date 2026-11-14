# upd: monitoring of network connection

This is a small utility built as a single binary for easy deployment to monitor internet connections and reboot appropriate networking equipment if the connection is down.

It works by:

- Running HTTP, TCP or DNS checks on a regular basis.
- If all checks fail, runs a specified command on a regular basis until the connection is back up.

## Installation

See [INSTALL.md](INSTALL.md) for installation instructions.

## Help

`upd -h` shows:

```text
Tool to monitor if the network connection is up

Usage: upd [OPTIONS]

Options:
  -c, --config <FILE>  Use the specified TOML configuration file [env: UPD_CONFIG=]
  -d, --debug          Display debugging output in the console
  -h, --help           Print help
  -V, --version        Print version
```

Send `SIGHUP` to reload the configuration. If the new file is invalid, the error is logged and
the running configuration stays in force. `SIGINT` and `SIGTERM` shut down gracefully. A running
down-action command is killed; `stopExec` runs only when the connection comes back after the
command has run at least once.

## Configuration

Without `--config` (or `UPD_CONFIG`), `upd` uses the first of these that exists: `./.upd.toml`,
`upd/upd.toml` in `$XDG_CONFIG_HOME` (`~/.config` when unset), the same in each directory of
`$XDG_CONFIG_DIRS` (`/etc/xdg` when unset), and `/etc/upd/upd.toml`.

See [`upd.toml`](upd.toml) for a complete, runnable example. Values can
reference environment variables with `${VAR}` syntax inside string values, e.g.:

```toml
[checks.every]
normal = "${UPD_NORMAL_CHECK}"
down = "${UPD_DOWN_CHECK}"
```

Probe-stat bucket granularity per report period is also tunable: each report
period is split into at least `min` buckets (default 100), and a single
bucket never aggregates more than `maxSpan` (default 30m):

```toml
[stats.buckets]
min = 100
maxSpan = "30m"
```

Unknown keys in the file are rejected, so a typo fails at startup instead of being ignored. A
setting that is not there is off: leave out `stats.port` to have no stats server, and leave out
`downAction.every.repeat` to run the command once.
`dns://` checks need the resolver as an IP address (`dns://1.1.1.1/example.com`, optionally with a
port).

## Status

Set `stats.port` (the examples use 42080) and `upd` serves statistics as JSON at `http://<ip>:<port>/stats.json`, on all interfaces and without authentication. With `reports = ["10s", "15m", "1h", "24h", "168h"]` it looks like this:

```json
{
  "isUp": true,
  "reports": [
    {
      "period": "10s",
      "availability": "100.000 %",
      "downTime": "0s",
      "totalProbes": 1,
      "failedProbes": 0,
      "failureRate": "0.000 %"
    },
    {
      "period": "15m",
      "availability": "100.000 %",
      "downTime": "0s",
      "totalProbes": 15,
      "failedProbes": 0,
      "failureRate": "0.000 %"
    },
    {
      "period": "1h",
      "availability": "100.000 %",
      "downTime": "0s",
      "totalProbes": 60,
      "failedProbes": 0,
      "failureRate": "0.000 %"
    },
    {
      "period": "24h",
      "coverage": "12h 33m 23s",
      "availability": "100.000 %",
      "downTime": "0s",
      "totalProbes": 753,
      "failedProbes": 0,
      "failureRate": "0.000 %"
    },
    {
      "period": "168h",
      "coverage": "12h 33m 23s",
      "availability": "100.000 %",
      "downTime": "0s",
      "totalProbes": 753,
      "failedProbes": 0,
      "failureRate": "0.000 %"
    }
  ],
  "loop": {
    "lastSuccess": "11s",
    "nextCheck": "49s",
    "interval": "1m",
    "timeSinceLastUpdate": "11s",
    "totalChecksRun": 753
  },
  "updUptime": "12h 33m 23s",
  "updVersion": "5.0.0",
  "generatedAt": "2026-06-09T12:37:00.708Z"
}
```

While the connection is down, a `downAction` object (`iteration`, `sleepTime`, `backoffCapped`) is added.
`coverage` appears when `upd` has watched for less than the period. Availability is the share of
observed time spent up, accurate to one time bucket (see `stats.buckets`).

## Upgrading from 4.x

Version 5 is a rewrite in Rust. The configuration format is the same, but it is stricter, and a
few behaviours changed. Read this before upgrading an existing setup.

### Configuration changes

- Unknown keys are errors, so a typo fails at startup. Remove `stats.retention` (the retention
  comes from `reports`), `stats.writeTimeout` and `stats.idleTimeout`, which 4.x ignored or
  accepted. A key that sits in the wrong table is also caught now: a top-level `logLevel` written
  below `[checks.list]` used to be silently ignored.
- `0` no longer means "off". Leave the key out instead: `stats.port` (no stats server),
  `downAction.every.repeat` (run once), `downAction.every.expBackoffLimit` (no limit). `repeat`,
  `expBackoffLimit`, `stats.buckets.min`, `stats.buckets.maxSpan`, `stats.readTimeout`, `reports`
  entries, `checks.timeout` and `checks.every.*` must be greater than 0, and `checks.every.*`
  at most 24 hours.
- `${VAR}` is expanded inside string values only, so a number such as `port = ${PORT}` can no
  longer come from a variable, and comments are no longer expanded.
- `dns://` checks need the resolver as an IP address: `dns://1.1.1.1/example.com`, not
  `dns://dns.google/example.com`. A hostname in `http://` and `tcp://` checks is fine.
- `exec` and `stopExec` are checked when the file is loaded, so an empty or badly quoted command
  fails at startup, not at the first outage.
- `logLevel` also accepts `error` and `trace`. With `--debug`, debug output wins over `logLevel`.
- `upd` looks for its configuration in more places (see [Configuration](#configuration)), and
  `UPD_CONFIG` can name the file. `./.upd.toml` still comes first.

### Behaviour changes

- `stopExec` runs only when the connection comes back after `exec` ran at least once. It no longer
  runs when `upd` shuts down or reloads. `UPD_ITERATION` works as before.
- A reload (`SIGHUP`) that finds an invalid file keeps the running configuration and logs the
  error. 4.x exited.
- HTTP checks do not follow redirects: any response means the connection works. 4.x followed them.
- A down-action command runs in its own process group, and anything it started is killed with it.
- Statistics are computed from time buckets (`stats.buckets`), so figures can differ slightly from
  4.x. `generatedAt` is now UTC with milliseconds. The JSON field names are unchanged.
- The log format changed from `time=… level=INFO msg="…" component=…` to
  `2026-11-14T17:46:03.976Z  INFO upd::monitor: connection status changed up=true`. Colour is only
  used on a terminal. Anything that parses `upd`'s logs needs updating.

### Command line and platforms

- `-v` is gone: use `-V` or `--version`. `-c`, `-d` and `-h` are unchanged.
- Release binaries are built for Linux (x86_64 and aarch64, in glibc and static musl builds) and
  macOS (Apple silicon). Windows, FreeBSD, 32-bit and macOS on Intel are no longer built; see
  [INSTALL.md](INSTALL.md) for the archive names.
