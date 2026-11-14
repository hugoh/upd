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

## mise

```sh
mise use -g github:hugoh/upd
```

## Containers

Commands run by the down action may start background processes. In a container, run `upd` under
an init (`docker run --init`, or `tini`) so that any such processes left behind are reaped.
