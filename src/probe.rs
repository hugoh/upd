//! Runs one probe: an HTTP request, a TCP connection or a DNS lookup.

use std::time::{Duration, Instant};

use std::net::{IpAddr, SocketAddr};
use std::sync::Arc;

use hickory_resolver::TokioResolver;
use hickory_resolver::config::{NameServerConfig, ResolveHosts, ResolverConfig};
use hickory_resolver::net::runtime::TokioRuntimeProvider;
use tokio::net::TcpStream;

use crate::probe_spec::{ProbeSpec, Scheme};

#[derive(Debug, thiserror::Error)]
pub enum ProbeError {
    #[error("timed out after {0:?}")]
    Timeout(Duration),
    #[error("error making request to {url}: {source}")]
    Http { url: String, source: reqwest::Error },
    #[error("error connecting to {target}: {source}")]
    Connect {
        target: String,
        source: std::io::Error,
    },
    #[error("error resolving {domain}: {message}")]
    Dns { domain: String, message: String },
    #[error("error resolving {0}: no addresses returned")]
    NoAddresses(String),
}

/// What happened when one probe ran.
#[derive(Debug)]
pub struct Outcome {
    pub scheme: Scheme,
    pub target: String,
    pub elapsed: Duration,
    /// A short description of the answer, or why there was none.
    pub result: Result<String, ProbeError>,
}

impl Outcome {
    #[must_use]
    pub fn is_success(&self) -> bool {
        self.result.is_ok()
    }
}

#[derive(Debug, thiserror::Error)]
pub enum ProberError {
    #[error("cannot build the HTTP client: {0}")]
    Client(#[from] reqwest::Error),
    #[error("cannot read the system DNS configuration: {0}")]
    Resolver(String),
}

/// Looks host names up with the system's DNS settings, without the blocking threads of
/// `getaddrinfo`: a lookup that hangs while the network is down is dropped with its probe.
#[derive(Clone)]
struct SystemResolver(Arc<TokioResolver>);

impl SystemResolver {
    fn new() -> Result<Self, ProberError> {
        let resolver = TokioResolver::builder_tokio()
            .and_then(hickory_resolver::ResolverBuilder::build)
            .map_err(|err| ProberError::Resolver(err.to_string()))?;
        Ok(Self(Arc::new(resolver)))
    }

    async fn lookup(&self, host: &str) -> Result<Vec<IpAddr>, String> {
        let answer = self
            .0
            .lookup_ip(host)
            .await
            .map_err(|err| err.to_string())?;
        Ok(answer.iter().collect())
    }
}

impl reqwest::dns::Resolve for SystemResolver {
    fn resolve(&self, name: reqwest::dns::Name) -> reqwest::dns::Resolving {
        let resolver = self.clone();
        Box::pin(async move {
            let ips = resolver.lookup(name.as_str()).await?;
            let addrs: reqwest::dns::Addrs = Box::new(
                ips.into_iter()
                    .map(|ip| SocketAddr::new(ip, 0))
                    .collect::<Vec<_>>()
                    .into_iter(),
            );
            Ok(addrs)
        })
    }
}

/// Runs probes. Holds the HTTP client and the resolver, which every probe shares.
#[derive(Clone)]
pub struct Prober {
    client: reqwest::Client,
    resolver: SystemResolver,
}

impl Prober {
    /// # Errors
    ///
    /// Fails if the HTTP client cannot be built or the system DNS settings cannot be read.
    pub fn new(version: &str) -> Result<Self, ProberError> {
        crate::tls::init();
        let resolver = SystemResolver::new()?;
        let client = reqwest::Client::builder()
            .user_agent(format!("upd/{version}"))
            .redirect(reqwest::redirect::Policy::none())
            .pool_max_idle_per_host(0)
            .dns_resolver(Arc::new(resolver.clone()))
            .build()?;
        Ok(Self { client, resolver })
    }

    pub async fn run(&self, spec: &ProbeSpec, timeout: Duration) -> Outcome {
        let start = Instant::now();
        let result = match tokio::time::timeout(timeout, self.attempt(spec, timeout)).await {
            Ok(result) => result,
            Err(_) => Err(ProbeError::Timeout(timeout)),
        };
        Outcome {
            scheme: spec.scheme(),
            target: spec.target(),
            elapsed: start.elapsed(),
            result,
        }
    }

    async fn attempt(&self, spec: &ProbeSpec, timeout: Duration) -> Result<String, ProbeError> {
        match spec {
            ProbeSpec::Http { url } => {
                let response = self
                    .client
                    .get(url.clone())
                    .send()
                    .await
                    .map_err(|source| ProbeError::Http {
                        url: url.to_string(),
                        source,
                    })?;
                Ok(response.status().to_string())
            }
            ProbeSpec::Tcp { host, port } => {
                let stream = self.connect(host, *port, &spec.target()).await?;
                Ok(stream
                    .local_addr()
                    .map_or_else(|_| String::new(), |addr| addr.to_string()))
            }
            ProbeSpec::Dns { resolver, domain } => lookup(*resolver, domain, timeout).await,
        }
    }
}

impl Prober {
    /// Connects to the first address of `host` that accepts.
    async fn connect(&self, host: &str, port: u16, target: &str) -> Result<TcpStream, ProbeError> {
        let addresses = match host.parse::<IpAddr>() {
            Ok(ip) => vec![ip],
            Err(_) => self
                .resolver
                .lookup(host)
                .await
                .map_err(|message| ProbeError::Dns {
                    domain: host.to_owned(),
                    message,
                })?,
        };

        let mut failure = None;
        for ip in addresses {
            match TcpStream::connect((ip, port)).await {
                Ok(stream) => return Ok(stream),
                Err(source) => failure = Some(source),
            }
        }
        Err(ProbeError::Connect {
            target: target.to_owned(),
            source: failure.unwrap_or_else(|| std::io::ErrorKind::NotFound.into()),
        })
    }
}

async fn lookup(
    resolver: std::net::SocketAddr,
    domain: &str,
    timeout: Duration,
) -> Result<String, ProbeError> {
    let dns_error = |message: String| ProbeError::Dns {
        domain: domain.to_owned(),
        message,
    };

    let mut server = NameServerConfig::udp_and_tcp(resolver.ip());
    for connection in &mut server.connections {
        connection.port = resolver.port();
    }
    let mut builder = TokioResolver::builder_with_config(
        ResolverConfig::from_name_servers(vec![server]),
        TokioRuntimeProvider::default(),
    );
    let options = builder.options_mut();
    options.attempts = 1;
    options.cache_size = 0;
    options.timeout = timeout;
    options.use_hosts_file = ResolveHosts::Never;

    let resolver_client = builder.build().map_err(|err| dns_error(err.to_string()))?;
    let answer = resolver_client
        .lookup_ip(domain)
        .await
        .map_err(|err| dns_error(err.to_string()))?;
    let address = answer
        .iter()
        .next()
        .ok_or_else(|| ProbeError::NoAddresses(domain.to_owned()))?;
    Ok(format!("{address} @ {resolver}"))
}

#[cfg(test)]
mod tests {
    use super::*;
    use tokio::io::{AsyncReadExt, AsyncWriteExt};
    use tokio::net::{TcpListener, UdpSocket};

    const TIMEOUT: Duration = Duration::from_secs(2);

    fn prober() -> Prober {
        Prober::new("test").unwrap()
    }

    async fn http_server(response: &'static str) -> std::net::SocketAddr {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap();
        tokio::spawn(async move {
            loop {
                let Ok((mut socket, _)) = listener.accept().await else {
                    return;
                };
                tokio::spawn(async move {
                    let mut buf = [0_u8; 1024];
                    let _ = socket.read(&mut buf).await;
                    let _ = socket.write_all(response.as_bytes()).await;
                });
            }
        });
        addr
    }

    fn http_spec(addr: std::net::SocketAddr) -> ProbeSpec {
        ProbeSpec::parse(&format!("http://{addr}/")).unwrap()
    }

    #[tokio::test]
    async fn http_succeeds_on_any_response_including_errors_and_redirects() {
        for response in [
            "HTTP/1.1 204 No Content\r\nconnection: close\r\n\r\n",
            "HTTP/1.1 503 Service Unavailable\r\ncontent-length: 0\r\nconnection: close\r\n\r\n",
            "HTTP/1.1 302 Found\r\nlocation: http://127.0.0.1:1/\r\ncontent-length: 0\r\nconnection: close\r\n\r\n",
        ] {
            let addr = http_server(response).await;

            let outcome = prober().run(&http_spec(addr), TIMEOUT).await;

            assert!(outcome.is_success(), "{response}: {:?}", outcome.result);
            assert_eq!(outcome.scheme, Scheme::Http);
            assert_eq!(outcome.target, format!("http://{addr}/"));
        }
    }

    #[tokio::test]
    async fn http_reports_the_status_line() {
        let addr = http_server("HTTP/1.1 204 No Content\r\nconnection: close\r\n\r\n").await;

        let outcome = prober().run(&http_spec(addr), TIMEOUT).await;

        assert_eq!(outcome.result.unwrap(), "204 No Content");
    }

    #[tokio::test]
    async fn http_sends_the_upd_user_agent() {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap();
        let request = tokio::spawn(async move {
            let (mut socket, _) = listener.accept().await.unwrap();
            let mut buf = vec![0_u8; 2048];
            let read = socket.read(&mut buf).await.unwrap();
            let _ = socket
                .write_all(b"HTTP/1.1 200 OK\r\ncontent-length: 0\r\nconnection: close\r\n\r\n")
                .await;
            String::from_utf8_lossy(&buf[..read]).to_lowercase()
        });

        prober().run(&http_spec(addr), TIMEOUT).await;

        assert!(request.await.unwrap().contains("user-agent: upd/test"));
    }

    #[tokio::test]
    async fn http_fails_when_nothing_listens() {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap();
        drop(listener);

        let outcome = prober().run(&http_spec(addr), TIMEOUT).await;

        assert!(matches!(outcome.result, Err(ProbeError::Http { .. })));
    }

    #[tokio::test]
    async fn a_silent_server_times_out() {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let addr = listener.local_addr().unwrap();
        tokio::spawn(async move {
            let _held = listener.accept().await;
            std::future::pending::<()>().await;
        });

        let outcome = prober()
            .run(&http_spec(addr), Duration::from_millis(100))
            .await;

        assert!(matches!(outcome.result, Err(ProbeError::Timeout(_))));
        assert!(outcome.elapsed < Duration::from_secs(1));
    }

    #[tokio::test]
    async fn tcp_connects_and_reports_the_local_address() {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let port = listener.local_addr().unwrap().port();
        let spec = ProbeSpec::parse(&format!("tcp://127.0.0.1:{port}")).unwrap();

        let outcome = prober().run(&spec, TIMEOUT).await;

        assert!(outcome.result.unwrap().starts_with("127.0.0.1:"));
        assert_eq!(outcome.scheme, Scheme::Tcp);
    }

    #[tokio::test]
    async fn host_names_are_resolved_for_tcp_and_http_probes() {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let port = listener.local_addr().unwrap().port();
        tokio::spawn(async move {
            while let Ok((mut socket, _)) = listener.accept().await {
                tokio::spawn(async move {
                    let mut buf = [0_u8; 1024];
                    let _ = socket.read(&mut buf).await;
                    let _ = socket
                        .write_all(b"HTTP/1.1 204 No Content\r\nconnection: close\r\n\r\n")
                        .await;
                });
            }
        });

        for target in [
            format!("tcp://localhost:{port}"),
            format!("http://localhost:{port}/"),
        ] {
            let outcome = prober()
                .run(&ProbeSpec::parse(&target).unwrap(), TIMEOUT)
                .await;

            assert!(outcome.is_success(), "{target}: {:?}", outcome.result);
        }
    }

    #[tokio::test]
    async fn an_unknown_host_fails_the_probe() {
        let spec = ProbeSpec::parse("tcp://no-such-host.invalid:80").unwrap();

        let outcome = prober().run(&spec, TIMEOUT).await;

        assert!(matches!(outcome.result, Err(ProbeError::Dns { .. })));
    }

    #[tokio::test]
    async fn tcp_fails_on_a_closed_port() {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let port = listener.local_addr().unwrap().port();
        drop(listener);
        let spec = ProbeSpec::parse(&format!("tcp://127.0.0.1:{port}")).unwrap();

        let outcome = prober().run(&spec, TIMEOUT).await;

        assert!(matches!(outcome.result, Err(ProbeError::Connect { .. })));
    }

    /// Answers every query with `answers` A records (0 or 1) and the given response code.
    async fn dns_server(rcode: u8, answers: u8) -> std::net::SocketAddr {
        let socket = UdpSocket::bind("127.0.0.1:0").await.unwrap();
        let addr = socket.local_addr().unwrap();
        tokio::spawn(async move {
            let mut buf = [0_u8; 512];
            loop {
                let Ok((len, peer)) = socket.recv_from(&mut buf).await else {
                    return;
                };
                let query = &buf[..len];
                let mut end = 12;
                while query[end] != 0 {
                    end += usize::from(query[end]) + 1;
                }
                end += 5;

                let mut reply = vec![
                    query[0],
                    query[1],
                    0x81,
                    0x80 | rcode,
                    0,
                    1,
                    0,
                    answers,
                    0,
                    0,
                    0,
                    0,
                ];
                reply.extend_from_slice(&query[12..end]);
                if answers > 0 {
                    reply.extend_from_slice(&[
                        0xC0, 0x0C, 0, 1, 0, 1, 0, 0, 0, 60, 0, 4, 192, 0, 2, 1,
                    ]);
                }
                let _ = socket.send_to(&reply, peer).await;
            }
        });
        addr
    }

    fn dns_spec(addr: std::net::SocketAddr) -> ProbeSpec {
        ProbeSpec::parse(&format!("dns://{addr}/example.test")).unwrap()
    }

    #[tokio::test]
    async fn dns_succeeds_when_the_resolver_answers() {
        let addr = dns_server(0, 1).await;

        let outcome = prober().run(&dns_spec(addr), TIMEOUT).await;

        assert_eq!(outcome.result.unwrap(), format!("192.0.2.1 @ {addr}"));
        assert_eq!(outcome.scheme, Scheme::Dns);
        assert_eq!(outcome.target, addr.to_string());
    }

    #[tokio::test]
    async fn dns_fails_on_nxdomain_and_on_empty_answers() {
        for (rcode, answers) in [(3, 0), (0, 0)] {
            let addr = dns_server(rcode, answers).await;

            let outcome = prober().run(&dns_spec(addr), TIMEOUT).await;

            assert!(
                matches!(outcome.result, Err(ProbeError::Dns { .. })),
                "rcode {rcode}: {:?}",
                outcome.result
            );
        }
    }

    #[tokio::test]
    async fn dns_times_out_when_the_resolver_is_silent() {
        let silent = UdpSocket::bind("127.0.0.1:0").await.unwrap();

        let outcome = prober()
            .run(
                &dns_spec(silent.local_addr().unwrap()),
                Duration::from_millis(200),
            )
            .await;

        assert!(outcome.result.is_err());
        assert!(outcome.elapsed < Duration::from_secs(2));
    }
}
