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
  every:
    normal: 2m # Run check every 2 minutes
    down: 10s # Run check every 10 seconds if the connection is detected as down
  list: # List of checks to run
    # From https://en.wikipedia.org/wiki/Captive_portal
    - http://captive.apple.com/hotspot-detect.html
    - http://connectivitycheck.gstatic.com/generate_204
    - http://clients3.google.com/generate_204
    - http://www.msftconnecttest.com/connecttest.txt
    - dns://1.1.1.1/www.google.com
    - dns://1.0.0.1/www.google.com
    - dns://8.8.8.8/www.google.com
    - dns://8.8.4.4/www.google.com
    - dns://9.9.9.9/www.google.com
    - dns://149.112.112.112/www.google.com
    - tcp://1.1.1.1:80/
    - tcp://www.google.com:80/
  timeout: 2000ms # After 2s, consider connection down
  shuffled: true # Pick a random check every time
downAction: # What to do if the connection is detected as down
  exec: /internet-monitor/tmhi-cli -c /etc/internet-monitor/tmhi-cli.yaml reboot # Command to run
  every:
    after: 90s # Run after 90 seconds of the connection being down
    repeat: 4m # Re-run after 4 minutes
    expBackoffLimit: 20m # Next re-runs will be exponentially delayed, but at most will be run every 20 minutes
  stopExec: /etc/internet-monitor/reboot_notify.sh # Command to run once the connection becomes up again
logLevel: debug # Logging level; default is info
stats:
  port: :42080 # Start a stat server at http://<ip>:42080/stats
  retention: 168h # Keep data for 1 week
  reports: # Generate availability stats for the last
    - 1m
    - 15m
    - 1h
    - 24h
    - 168h
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
