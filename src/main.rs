use std::process::ExitCode;

use clap::Parser;
use tokio_util::sync::CancellationToken;

use upd::{app, cli::Cli, config, signals, telemetry};

#[tokio::main]
async fn main() -> ExitCode {
    let cli = Cli::parse();
    let config_path = match cli.config {
        Some(path) => Ok(path),
        None => config::default_path(std::path::Path::is_file),
    };
    let config_path = match config_path {
        Ok(path) => path,
        Err(err) => {
            eprintln!("failed to run app: {err}");
            return ExitCode::FAILURE;
        }
    };
    let telemetry = telemetry::init(cli.debug);
    let shutdown = CancellationToken::new();

    let reloads = match signals::listen(shutdown.clone()) {
        Ok(reloads) => reloads,
        Err(err) => {
            eprintln!("cannot install signal handlers: {err}");
            return ExitCode::FAILURE;
        }
    };

    match app::supervise(&config_path, &telemetry, shutdown, reloads).await {
        Ok(()) => ExitCode::SUCCESS,
        Err(err) => {
            eprintln!("failed to run app: {err:#}");
            ExitCode::FAILURE
        }
    }
}
