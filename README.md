# upd: monitoring of network connection

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
   --config value, -c value  use the specified YAML configuration file (default: ".upd.yaml")
   --debug, -d               display debugging output in the console (default: false)
   --help, -h                show help
   --version, -v             print the version
```

## Configuration

Configuration by default is located in `.upd.yaml` in the working directory.

An example is:

```yaml
checks:
  every: # Will be retrieved from env vars
    normal: ${UPD_NORMAL_CHECK}
    down: ${UPD_DOWN_CHECK}
  list:
    ordered: # Sequential list of checks to run first
      # From https://en.wikipedia.org/wiki/Captive_portal
      - http://10.10.1.4/
      - http://captive.apple.com/hotspot-detect.html
      - http://connectivitycheck.gstatic.com/generate_204
    shuffled: # Random list of checks to run if the sequential ones all failed
      - http://clients3.google.com/generate_204
      - http://www.msftconnecttest.com/connecttest.txt
      - tcp://1.1.1.1:53/
      - tcp://1.0.0.1:53/
      - tcp://8.8.8.8:53/
      - tcp://8.8.4.4:53/
      - dns://1.1.1.1/www.google.com
  timeout: 2s
downAction:
  exec: cowsay
  everySec:
    after: 1s
    repeat: 3s
  stopExec: ./testdata/echo-reboot-count.sh
# Options = debug, info, warn, error
logLevel: trace
stats:
  port: :8080
  retention: 10080m
  reports:
    - 10s
    - 15m
    - 60m
    - 1440m
    - 10080m
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
  "totalChecksRun": 612,
  "timeSinceLastUpdate": "35s",
  "updUptime": "20h25m15s",
  "updVersion": "2.0.0",
  "generatedAt": "2024-12-12T17:38:01.738046722-06:00"
}
```
