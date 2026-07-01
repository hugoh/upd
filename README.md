# upd: monitoring of network connection

[![CI](https://github.com/hugoh/upd/actions/workflows/ci.yml/badge.svg)](https://github.com/hugoh/upd/actions/workflows/ci.yml)
[![codecov](https://codecov.io/github/hugoh/upd/graph/badge.svg?token=UFSZDFKCTR)](https://codecov.io/github/hugoh/upd)
[![Go Report Card](https://goreportcard.com/badge/github.com/hugoh/upd)](https://goreportcard.com/report/github.com/hugoh/upd)

This is a small utility built as a single binary for easy deployment to monitor internet connections and reboot appropriate networking equipment if the connection is down.

It works by:

- Running HTTP, TCP or DNS checks on a regular basis.
- If all checks fail, runs a specified command on a regular basis until the connection is back up.

## Installation

Download the latest release and extract the `upd` or `upd.exe` binary.

## Help

`upd -h` and `upd.exe -h` will show:

```text
NAME:
   upd - Tool to monitor if the network connection is up.

USAGE:
   upd [global options] command [command options]

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --config value, -c value  use the specified TOML configuration file (default: ".upd.toml")
   --debug, -d               display debugging output in the console (default: false)
   --help, -h                show help
   --version, -v             print the version
```

## Configuration

Configuration by default is located in `.upd.toml` in the working directory.

See [`upd.toml`](upd.toml) for a complete, runnable example. Values can
reference environment variables with `${VAR}` syntax, e.g.:

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
