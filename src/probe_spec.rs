//! What to probe, parsed from the URL-like strings in the configuration.

use std::fmt;
use std::net::{IpAddr, SocketAddr};

use url::{Host, Url};

pub const DEFAULT_DNS_PORT: u16 = 53;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Scheme {
    Http,
    Https,
    Tcp,
    Dns,
}

impl Scheme {
    #[must_use]
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Http => "http",
            Self::Https => "https",
            Self::Tcp => "tcp",
            Self::Dns => "dns",
        }
    }
}

impl fmt::Display for Scheme {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

#[derive(Debug, Clone, PartialEq, Eq, serde::Deserialize)]
#[serde(try_from = "String")]
pub enum ProbeSpec {
    Http {
        url: Url,
    },
    Tcp {
        host: String,
        port: u16,
    },
    Dns {
        resolver: SocketAddr,
        domain: String,
    },
}

#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum ProbeSpecError {
    #[error("must be a valid URI: {0}")]
    InvalidUri(String),
    #[error("unsupported scheme {0:?}")]
    UnsupportedScheme(String),
    #[error("{0} check is missing a host")]
    MissingHost(&'static str),
    #[error("tcp check is missing a port")]
    MissingPort,
    #[error("dns check is missing the domain to resolve")]
    MissingDomain,
    #[error("dns resolver must be an IP address, got {0:?}")]
    ResolverNotAnIp(String),
}

impl TryFrom<String> for ProbeSpec {
    type Error = String;

    fn try_from(text: String) -> Result<Self, String> {
        Self::parse(&text).map_err(|err| format!("{text:?}: {err}"))
    }
}

impl ProbeSpec {
    /// # Errors
    ///
    /// Fails when the string is not a URL of a supported scheme, or lacks what the scheme needs.
    pub fn parse(text: &str) -> Result<Self, ProbeSpecError> {
        let url = Url::parse(text).map_err(|err| ProbeSpecError::InvalidUri(err.to_string()))?;

        match url.scheme() {
            "http" | "https" => {
                if url.host().is_none() {
                    return Err(ProbeSpecError::MissingHost("http"));
                }
                Ok(Self::Http { url })
            }
            "tcp" => {
                let host = host_text(&url).ok_or(ProbeSpecError::MissingHost("tcp"))?;
                let port = url.port().ok_or(ProbeSpecError::MissingPort)?;
                Ok(Self::Tcp { host, port })
            }
            "dns" => {
                let host = host_text(&url).ok_or(ProbeSpecError::MissingHost("dns"))?;
                let ip: IpAddr = host
                    .parse()
                    .map_err(|_| ProbeSpecError::ResolverNotAnIp(host))?;
                let domain = url.path().trim_prefix('/');
                if domain.is_empty() {
                    return Err(ProbeSpecError::MissingDomain);
                }
                Ok(Self::Dns {
                    resolver: SocketAddr::new(ip, url.port().unwrap_or(DEFAULT_DNS_PORT)),
                    domain: domain.to_owned(),
                })
            }
            other => Err(ProbeSpecError::UnsupportedScheme(other.to_owned())),
        }
    }

    #[must_use]
    pub fn scheme(&self) -> Scheme {
        match self {
            Self::Http { url } if url.scheme() == "https" => Scheme::Https,
            Self::Http { .. } => Scheme::Http,
            Self::Tcp { .. } => Scheme::Tcp,
            Self::Dns { .. } => Scheme::Dns,
        }
    }

    /// What the probe talks to: the URL, `host:port`, or the resolver address.
    #[must_use]
    pub fn target(&self) -> String {
        match self {
            Self::Http { url } => url.to_string(),
            Self::Tcp { host, port } if host.contains(':') => format!("[{host}]:{port}"),
            Self::Tcp { host, port } => format!("{host}:{port}"),
            Self::Dns { resolver, .. } => resolver.to_string(),
        }
    }
}

fn host_text(url: &Url) -> Option<String> {
    match url.host()? {
        Host::Domain("") => None,
        Host::Domain(domain) => Some(domain.to_owned()),
        Host::Ipv4(ip) => Some(ip.to_string()),
        Host::Ipv6(ip) => Some(ip.to_string()),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn parse(text: &str) -> Result<ProbeSpec, ProbeSpecError> {
        ProbeSpec::parse(text)
    }

    #[test]
    fn http_and_https_keep_the_url() {
        let spec = parse("https://example.com/generate_204").unwrap();

        assert_eq!(spec.scheme(), Scheme::Https);
        assert_eq!(spec.target(), "https://example.com/generate_204");
        assert_eq!(parse("http://10.10.1.4/").unwrap().scheme(), Scheme::Http);
    }

    #[test]
    fn tcp_needs_a_host_and_a_port() {
        assert_eq!(
            parse("tcp://1.1.1.1:53/").unwrap(),
            ProbeSpec::Tcp {
                host: "1.1.1.1".into(),
                port: 53
            }
        );
        assert_eq!(parse("tcp://1.1.1.1/"), Err(ProbeSpecError::MissingPort));
        assert!(matches!(
            parse("tcp://:53/"),
            Err(ProbeSpecError::InvalidUri(_))
        ));
    }

    #[test]
    fn tcp_targets_bracket_ipv6_hosts() {
        let spec = parse("tcp://[2606:4700:4700::1111]:53").unwrap();

        assert_eq!(spec.target(), "[2606:4700:4700::1111]:53");
    }

    #[test]
    fn dns_defaults_to_port_53() {
        let spec = parse("dns://1.1.1.1/www.google.com").unwrap();

        assert_eq!(
            spec,
            ProbeSpec::Dns {
                resolver: "1.1.1.1:53".parse().unwrap(),
                domain: "www.google.com".into()
            }
        );
        assert_eq!(spec.target(), "1.1.1.1:53");
    }

    #[test]
    fn dns_accepts_a_port_and_ipv6_resolvers() {
        let ProbeSpec::Dns { resolver, .. } =
            parse("dns://[2001:4860:4860::8888]:5353/a.b").unwrap()
        else {
            panic!("not a dns probe");
        };

        assert_eq!(resolver, "[2001:4860:4860::8888]:5353".parse().unwrap());
    }

    #[test]
    fn dns_needs_a_resolver_and_a_domain() {
        assert_eq!(parse("dns://8.8.4.4/"), Err(ProbeSpecError::MissingDomain));
        assert_eq!(
            parse("dns:///example.com"),
            Err(ProbeSpecError::MissingHost("dns"))
        );
        assert_eq!(
            parse("dns://resolver.example/example.com"),
            Err(ProbeSpecError::ResolverNotAnIp("resolver.example".into()))
        );
    }

    #[test]
    fn other_schemes_and_junk_are_rejected() {
        assert_eq!(
            parse("ftp://foo.bar/"),
            Err(ProbeSpecError::UnsupportedScheme("ftp".into()))
        );
        assert!(matches!(
            parse("not a url"),
            Err(ProbeSpecError::InvalidUri(_))
        ));
    }
}
