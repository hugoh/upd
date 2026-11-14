//! Turns process signals into shutdown and reload requests.

use tokio::sync::mpsc;
use tokio_util::sync::CancellationToken;

/// Cancels `shutdown` on SIGINT or SIGTERM and sends a message on the returned channel on SIGHUP.
///
/// # Errors
///
/// Fails when the signal handlers cannot be installed.
#[cfg(unix)]
pub fn listen(shutdown: CancellationToken) -> std::io::Result<mpsc::Receiver<()>> {
    use tokio::signal::unix::{SignalKind, signal};

    let mut interrupt = signal(SignalKind::interrupt())?;
    let mut terminate = signal(SignalKind::terminate())?;
    let mut hangup = signal(SignalKind::hangup())?;
    let (reload, reloads) = mpsc::channel(1);

    tokio::spawn(async move {
        loop {
            tokio::select! {
                _ = interrupt.recv() => break,
                _ = terminate.recv() => break,
                _ = hangup.recv() => {
                    let _ = reload.try_send(());
                }
            }
        }
        shutdown.cancel();
    });
    Ok(reloads)
}

/// Cancels `shutdown` on Ctrl-C. There is no reload signal on this platform.
///
/// # Errors
///
/// Never fails on this platform.
#[cfg(not(unix))]
pub fn listen(shutdown: CancellationToken) -> std::io::Result<mpsc::Receiver<()>> {
    let (_reload, reloads) = mpsc::channel(1);
    tokio::spawn(async move {
        let _ = tokio::signal::ctrl_c().await;
        shutdown.cancel();
    });
    Ok(reloads)
}
