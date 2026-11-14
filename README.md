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

In the configuration, `stats` can be used to capture statistics that are made available via a web interface at `http://<ip>:42080/stats.json`.

The sample configuration above will provide data looking like this:

```json
{
  "isUp": true,
  "reports": [
    {
      "period": "10s",
      "availability": "100.00 %",
      "downTime": "0s",
      "totalProbes": 1,
      "failedProbes": 0,
      "failureRate": "0.00 %"
    },
    {
      "period": "15m",
      "availability": "100.00 %",
      "downTime": "0s",
      "totalProbes": 15,
      "failedProbes": 0,
      "failureRate": "0.00 %"
    },
    {
      "period": "1h",
      "availability": "100.00 %",
      "downTime": "0s",
      "totalProbes": 60,
      "failedProbes": 0,
      "failureRate": "0.00 %"
    },
    {
      "period": "24h",
      "coverage": "12h33m23s",
      "availability": "100.00 %",
      "downTime": "0s",
      "totalProbes": 753,
      "failedProbes": 0,
      "failureRate": "0.00 %"
    },
    {
      "period": "168h",
      "coverage": "12h33m23s",
      "availability": "100.00 %",
      "downTime": "0s",
      "totalProbes": 753,
      "failedProbes": 0,
      "failureRate": "0.00 %"
    }
  ],
  "loop": {
    "lastSuccess": "11s",
    "nextCheck": "49s",
    "interval": "1m",
    "timeSinceLastUpdate": "11s",
    "totalChecksRun": 753
  },
  "updUptime": "12h33m23s",
  "updVersion": "4.4.0",
  "generatedAt": "2026-06-09T07:37:00.70887063-05:00"
}
```
