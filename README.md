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
Usage:
  upd [flags]

Flags:
  -c, --config string   config file (default is $HOME/.up.yaml)
  -d, --debug           display debugging output in the console
  -D, --dump            dump parsed configuration and quit
  -h, --help            help for upd
  -v, --version         version for upd
```

## Configuration

Configuration by default is located in `.upd.yaml` either in the working directory or the home directory.

An example is:

```yaml
checks:
  everySec:
    normal: 120 # Run check every 2 minutes
    down: 10 # Run check every 10 seconds if the connection is detected as down
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
  timeoutMilli: 2000 # After 2s, consider connection down
  shuffled: true # Pick a random check every time
downAction: # What to do if the connection is detected as down
  exec: /internet-monitor/tmhi-cli -c /etc/internet-monitor/tmhi-cli.yaml reboot # Command to run
  everySec:
    after: 90 # Run after 90 seconds of the connection being down
    repeat: 240 # Re-run after 4 minutes
    expBackoffLimit: 1200 # Next re-runs will be exponentially delayed, but at most will be run every 20 minutes
  stopExec: /etc/internet-monitor/reboot_notify.sh # Command to run once the connection becomes up again
logLevel: debug # Logging level; default is info
```
