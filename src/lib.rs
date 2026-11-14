//! `upd` checks that the network connection works and runs a command while it does not.

pub mod app;
pub mod checks;
pub mod cli;
pub mod command;
pub mod config;
pub mod down_action;
pub mod monitor;
pub mod probe;
pub mod probe_spec;
pub mod server;
pub mod signals;
pub mod stats;
pub mod telemetry;
pub mod tls;
pub mod version;
