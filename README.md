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

An example is:

```toml
[checks]
timeout = "2s"

[checks.every]
# Will be retrieved from env vars
normal = "${UPD_NORMAL_CHECK}"
down = "${UPD_DOWN_CHECK}"

[checks.list]
ordered = [
  # From https://en.wikipedia.org/wiki/Captive_portal
  "http://10.10.1.4/",
  "http://captive.apple.com/hotspot-detect.html",
  "http://connectivitycheck.gstatic.com/generate_204",
]
shuffled = [
  "http://clients3.google.com/generate_204",
  "http://www.msftconnecttest.com/connecttest.txt",
  "tcp://1.1.1.1:53/",
  "tcp://1.0.0.1:53/",
  "tcp://8.8.8.8:53/",
  "tcp://8.8.4.4:53/",
  "dns://1.1.1.1/www.google.com",
]

[downAction]
exec = "cowsay"
stopExec = "./testdata/echo-reboot-count.sh"

[downAction.every]
after = "1s"
repeat = "3s"

[stats]
port = 8080
retention = "10080m"
reports = ["10s", "15m", "60m", "1440m", "10080m"]

# Optional: probe-stat bucket granularity per report period.
# Each report period is split into at least `min` buckets (default 100),
# and a single bucket never aggregates more than `maxSpan` (default 30m).
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
      "period": "1m",
      "availability": "100.00 %",
      "downTime": "0s"
    },
    {
      "period": "15m",
      "availability": "100.00 %",
      "downTime": "0s"
    },
    {
      "period": "1h",
      "availability": "100.00 %",
      "downTime": "0s"
    },
    {
      "period": "24h",
      "availability": "Not computed",
      "downTime": "0s"
    },
    {
      "period": "168h",
      "availability": "Not computed",
      "downTime": "0s"
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
  "updVersion": "4.2.0",
  "generatedAt": "2026-06-09T07:37:00.70887063-05:00"
}
```
