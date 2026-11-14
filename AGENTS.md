# Development Instructions

`upd` is a Rust crate (edition 2024, toolchain pinned in `rust-toolchain.toml`). See
[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for how it is organised.

## Build & Development Tasks

This project uses `mise` for task management. Use the following Mise tasks for all build, test, and lint operations:

### Building

```bash
mise build    # Release binary in target/release/upd
mise dev      # Run from source (cargo run); arguments go to the binary
mise cross    # Cross-compile with cargo-zigbuild; defaults to aarch64-unknown-linux-musl (OpenWrt),
              # `mise cross <target-triple>` for another
```

### Testing

```bash
mise test     # Run tests with cargo nextest
mise coverage # Run tests with coverage and check the threshold (cargo llvm-cov)
mise crap     # CRAP score report (warns on regressions against crap-baseline.json)
```

Individual tasks (`mise lint`, `mise fix`, `hk check --all`) are all wrapped by `mise full-check` / `mise ci`.

### Dependencies

```bash
mise deps:upgrade # Upgrade dependencies, including major bumps
mise clean        # Clean build artifacts
```

### CI

```bash
mise full-check # Fix (rustfmt, clippy, formatters), CI tests, then every hk check
mise ci         # Full CI pipeline (lint, tests, coverage, build) run in the pipeline
hk check        # Run hooks
```

## Agent Workflow

When making changes to this codebase:

1. **Before editing**: Run `mise full-check` to establish a clean baseline
2. **After editing**: Run `mise full-check`: auto-fixes lint and formatting, runs tests with coverage
3. **Before completion**: Run `mise ci` for the full CI pipeline (fresh lint from scratch)

Always use `mise` tasks rather than calling tools directly.

Write the failing test first. Time-dependent code takes injected `Instant`s; tests that need
processes or sockets use real ones on loopback.

## Conventions

- `unsafe` is forbidden, and `unwrap`/`expect` are denied outside tests (`clippy.toml`).
- Validation happens when the configuration is parsed (`src/config.rs`), so the rest of the code
  works with types that cannot hold invalid values.

## Releases

A push to `main` releases automatically from its conventional commits (a `feat!:` is a major
version). To release a pre-release, merge with a title that does not release (such as `chore:`),
then run the CI workflow by hand on `main` with `version` set, for example `5.0.0-rc.1`. A version
with a suffix is marked as a pre-release on GitHub and is not published to Cloudsmith. Run it with
`dry_run` first to rehearse; that works on any branch.

## Test Coverage

The threshold is the `--fail-under-lines` value in `mise-tasks/coverage`.
