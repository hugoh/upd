//! The HTTP endpoint that serves the statistics as JSON.

use std::convert::Infallible;
use std::net::SocketAddr;
use std::sync::Arc;
use std::time::{Duration, Instant, SystemTime};

use http_body_util::Full;
use hyper::body::Bytes;
use hyper::http::HeaderValue;
use hyper::server::conn::http1;
use hyper::service::service_fn;
use hyper::{Method, Request, Response, StatusCode, header};
use hyper_util::rt::{TokioIo, TokioTimer};
use tokio::net::{TcpListener, TcpStream};
use tokio_util::sync::CancellationToken;
use tokio_util::task::TaskTracker;
use tracing::{debug, error, info, warn};

use crate::stats::shared::Shared;
use crate::version::VERSION;

pub const STATS_ROUTE: &str = "/stats.json";

/// Answers requests for the statistics.
#[derive(Clone)]
pub struct StatsService {
    status: Shared,
    periods: Arc<[Duration]>,
}

impl StatsService {
    /// A service reporting on `periods`.
    #[must_use]
    pub fn new(status: Shared, periods: &[Duration]) -> Self {
        Self {
            status,
            periods: periods.into(),
        }
    }

    /// The response to `request`: the report for `GET /stats.json`, an error for anything else.
    pub fn respond<B>(&self, request: &Request<B>) -> Response<Full<Bytes>> {
        let mut response = if request.uri().path() != STATS_ROUTE {
            plain(StatusCode::NOT_FOUND)
        } else if request.method() != Method::GET {
            let mut refused = plain(StatusCode::METHOD_NOT_ALLOWED);
            refused
                .headers_mut()
                .insert(header::ALLOW, HeaderValue::from_static("GET"));
            refused
        } else {
            self.report()
        };

        if let Ok(value) = HeaderValue::from_str(&format!("upd/{VERSION}")) {
            response.headers_mut().insert(header::SERVER, value);
        }
        response
    }

    fn report(&self) -> Response<Full<Bytes>> {
        debug!("stats requested");
        let report =
            self.status
                .lock()
                .report(&self.periods, Instant::now(), SystemTime::now(), VERSION);

        match serde_json::to_string_pretty(&report) {
            Ok(body) => {
                let mut response = Response::new(Full::new(Bytes::from(body)));
                response.headers_mut().insert(
                    header::CONTENT_TYPE,
                    HeaderValue::from_static("application/json"),
                );
                response
            }
            Err(err) => {
                error!(%err, "error writing JSON response");
                plain(StatusCode::INTERNAL_SERVER_ERROR)
            }
        }
    }
}

fn plain(status: StatusCode) -> Response<Full<Bytes>> {
    let mut response = Response::new(Full::new(Bytes::new()));
    *response.status_mut() = status;
    response
}

/// Listens on all interfaces at `port`.
///
/// # Errors
///
/// Fails when the port cannot be bound.
pub async fn bind(port: u16) -> std::io::Result<TcpListener> {
    TcpListener::bind(SocketAddr::from(([0, 0, 0, 0], port))).await
}

/// How long connections may take to send a request, and to finish when the server stops.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct ServeOptions {
    /// A client that has not sent its request headers by then is disconnected.
    pub read_timeout: Duration,
    /// After `cancel`, requests in flight get this long before their connections are dropped.
    pub shutdown_grace: Duration,
}

pub const DEFAULT_READ_TIMEOUT: Duration = Duration::from_secs(3);
pub const DEFAULT_SHUTDOWN_GRACE: Duration = Duration::from_secs(5);

impl ServeOptions {
    #[must_use]
    pub fn new(read_timeout: Duration) -> Self {
        Self {
            read_timeout,
            shutdown_grace: DEFAULT_SHUTDOWN_GRACE,
        }
    }
}

/// Serves until `cancel` is cancelled, then waits up to the grace period for requests in flight.
pub async fn serve(
    listener: TcpListener,
    service: StatsService,
    cancel: CancellationToken,
    options: ServeOptions,
) {
    if let Ok(addr) = listener.local_addr() {
        info!(url = %format!("http://localhost:{}{STATS_ROUTE}", addr.port()), "stats server started");
    }

    let connections = TaskTracker::new();
    let hard_stop = CancellationToken::new();
    loop {
        tokio::select! {
            () = cancel.cancelled() => break,
            accepted = listener.accept() => match accepted {
                Ok((stream, _)) => {
                    connections.spawn(serve_connection(
                        stream,
                        service.clone(),
                        options.read_timeout,
                        cancel.clone(),
                        hard_stop.clone(),
                    ));
                }
                Err(err) => warn!(%err, "error accepting a stats connection"),
            },
        }
    }

    drop(listener);
    connections.close();
    if tokio::time::timeout(options.shutdown_grace, connections.wait())
        .await
        .is_err()
    {
        warn!("closing stats connections that did not finish in time");
        hard_stop.cancel();
        connections.wait().await;
    }
    info!("stats server stopped");
}

async fn serve_connection(
    stream: TcpStream,
    service: StatsService,
    read_timeout: Duration,
    cancel: CancellationToken,
    hard_stop: CancellationToken,
) {
    let connection = http1::Builder::new()
        .timer(TokioTimer::new())
        .header_read_timeout(read_timeout)
        .serve_connection(
            TokioIo::new(stream),
            service_fn(move |request: Request<hyper::body::Incoming>| {
                std::future::ready(Ok::<_, Infallible>(service.respond(&request)))
            }),
        );
    tokio::pin!(connection);

    tokio::select! {
        result = connection.as_mut() => log_connection_end(result),
        () = cancel.cancelled() => {
            connection.as_mut().graceful_shutdown();
            tokio::select! {
                result = connection.as_mut() => log_connection_end(result),
                () = hard_stop.cancelled() => {}
            }
        }
    }
}

fn log_connection_end(result: Result<(), hyper::Error>) {
    if let Err(err) = result {
        debug!(%err, "stats connection ended with an error");
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use http_body_util::BodyExt;
    use tokio::io::{AsyncReadExt, AsyncWriteExt};

    use crate::stats::status::Status;
    use crate::stats::tracker::BucketConfig;

    const PERIODS: [Duration; 2] = [Duration::from_secs(10), Duration::from_secs(3600)];

    fn shared() -> Shared {
        let now = Instant::now();
        let mut status = Status::new(now);
        status.configure(&PERIODS, BucketConfig::default(), now);
        status.record_probe(false, now);
        status.record_round(true, now);
        Shared::new(status)
    }

    fn request(method: Method, uri: &str) -> Response<Full<Bytes>> {
        let request = Request::builder().method(method).uri(uri).body(()).unwrap();
        StatsService::new(shared(), &PERIODS).respond(&request)
    }

    #[tokio::test]
    async fn serves_the_report_as_pretty_json_with_a_server_header() {
        let response = request(Method::GET, STATS_ROUTE);

        assert_eq!(response.status(), StatusCode::OK);
        assert_eq!(response.headers()[header::CONTENT_TYPE], "application/json");
        assert_eq!(
            response.headers()[header::SERVER],
            format!("upd/{VERSION}").as_str()
        );
        let body = String::from_utf8(
            response
                .into_body()
                .collect()
                .await
                .unwrap()
                .to_bytes()
                .to_vec(),
        )
        .unwrap();
        assert!(body.contains("\n  \"isUp\": true"), "{body}");
        let json: serde_json::Value = serde_json::from_str(&body).unwrap();
        assert_eq!(json["reports"].as_array().unwrap().len(), 2);
        assert_eq!(json["reports"][0]["period"], "10s");
        assert_eq!(json["reports"][1]["period"], "1h");
        assert_eq!(json["reports"][0]["totalProbes"], 1);
        assert_eq!(json["updVersion"], VERSION);
        assert_eq!(json["loop"]["totalChecksRun"], 1);
    }

    #[tokio::test]
    async fn other_paths_and_methods_are_refused() {
        let unknown = request(Method::GET, "/");
        let wrong_method = request(Method::POST, STATS_ROUTE);

        assert_eq!(unknown.status(), StatusCode::NOT_FOUND);
        assert_eq!(wrong_method.status(), StatusCode::METHOD_NOT_ALLOWED);
        assert_eq!(wrong_method.headers()[header::ALLOW], "GET");
    }

    const OPTIONS: ServeOptions = ServeOptions {
        read_timeout: Duration::from_millis(200),
        shutdown_grace: Duration::from_millis(300),
    };

    struct Running {
        port: u16,
        cancel: CancellationToken,
        server: tokio::task::JoinHandle<()>,
    }

    async fn start() -> Running {
        let listener = bind(0).await.unwrap();
        let port = listener.local_addr().unwrap().port();
        let cancel = CancellationToken::new();
        let server = tokio::spawn(serve(
            listener,
            StatsService::new(shared(), &PERIODS),
            cancel.clone(),
            OPTIONS,
        ));
        Running {
            port,
            cancel,
            server,
        }
    }

    impl Running {
        async fn stop_within(self, limit: Duration) {
            self.cancel.cancel();
            tokio::time::timeout(limit, self.server)
                .await
                .expect("the server stops in time")
                .unwrap();
        }
    }

    #[tokio::test]
    async fn listens_until_cancelled() {
        let running = start().await;

        let mut stream = TcpStream::connect(("127.0.0.1", running.port))
            .await
            .unwrap();
        stream
            .write_all(b"GET /stats.json HTTP/1.1\r\nhost: localhost\r\nconnection: close\r\n\r\n")
            .await
            .unwrap();
        let mut reply = String::new();
        stream.read_to_string(&mut reply).await.unwrap();
        assert!(reply.starts_with("HTTP/1.1 200 OK"), "{reply}");
        assert!(reply.contains("\"isUp\": true"));

        running.stop_within(Duration::from_secs(5)).await;
    }

    #[tokio::test]
    async fn a_client_that_never_finishes_its_request_is_disconnected() {
        let running = start().await;
        let mut slow = TcpStream::connect(("127.0.0.1", running.port))
            .await
            .unwrap();
        slow.write_all(b"GET /stats.json HTTP/1.1\r\nhost: x\r\n")
            .await
            .unwrap();

        let mut buf = Vec::new();
        let closed = tokio::time::timeout(Duration::from_secs(3), slow.read_to_end(&mut buf)).await;

        assert!(closed.is_ok(), "the connection should have been closed");
        running.stop_within(Duration::from_secs(5)).await;
    }

    #[tokio::test]
    async fn an_idle_connection_does_not_hold_up_shutdown() {
        let running = start().await;
        let _idle = TcpStream::connect(("127.0.0.1", running.port))
            .await
            .unwrap();
        tokio::time::sleep(Duration::from_millis(50)).await;

        running.stop_within(Duration::from_secs(2)).await;
    }

    #[tokio::test]
    async fn a_stuck_request_is_cut_off_after_the_grace_period() {
        let running = start().await;
        let mut stuck = TcpStream::connect(("127.0.0.1", running.port))
            .await
            .unwrap();
        stuck
            .write_all(
                b"POST /stats.json HTTP/1.1\r\nhost: x\r\ncontent-length: 100\r\n\r\npartial",
            )
            .await
            .unwrap();
        tokio::time::sleep(Duration::from_millis(50)).await;

        running.stop_within(Duration::from_secs(2)).await;
    }

    #[tokio::test]
    async fn binding_a_taken_port_fails() {
        let first = bind(0).await.unwrap();
        let port = first.local_addr().unwrap().port();

        assert!(bind(port).await.is_err());
    }
}
