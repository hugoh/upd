# Installing upd

## Debian/Ubuntu (apt)

```sh
curl -1sLf 'https://dl.cloudsmith.io/public/hugoh-oss/upd/setup.deb.sh' | sudo -E bash
sudo apt install upd
```

## RHEL/Fedora (dnf)

```sh
curl -1sLf 'https://dl.cloudsmith.io/public/hugoh-oss/upd/setup.rpm.sh' | sudo -E bash
sudo dnf install upd
```

Package repository hosting is graciously provided by
[Cloudsmith](https://cloudsmith.com).

## Package files

Download the `.deb` or `.rpm` from the
[releases page](https://github.com/hugoh/upd/releases) and install it
with your package manager:

```sh
# Debian/Ubuntu
sudo apt install ./upd_*.deb

# RHEL/Fedora
sudo dnf install ./upd_*.rpm
```

## Release binaries

The [releases page](https://github.com/hugoh/upd/releases) has an archive per platform; each holds
the `upd` binary, with a `checksums.txt` for all of them:

| Archive | For |
|---|---|
| `upd_<version>_linux_amd64.tar.gz`, `…_linux_arm64.tar.gz` | Linux with glibc (Debian, Ubuntu, Fedora, ...) |
| `upd_<version>_linux_amd64_musl.tar.gz`, `…_linux_arm64_musl.tar.gz` | Static binaries: Alpine, OpenWrt, containers without glibc |
| `upd_<version>_darwin_arm64.tar.gz` | macOS on Apple silicon |

There are no builds for Windows, FreeBSD, 32-bit systems or macOS on Intel.
Pre-releases (such as `5.0.0-rc.1`) are published here only, not to the package repositories.

## mise

```sh
mise use -g github:hugoh/upd
```

## Containers

Commands run by the down action may start background processes. In a container, run `upd` under
an init (`docker run --init`, or `tini`) so that any such processes left behind are reaped.
