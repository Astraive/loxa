use std::fmt;

/// Parsed and resolved loxa:// DSN.
#[derive(Debug, Clone, PartialEq)]
pub struct LoxaDSN {
    pub scheme: String,
    pub host: String,
    pub port: u16,
    pub project: String,
    pub env: String,
    pub service: String,
    pub tls: bool,
    pub transport: String,
    pub base_url: String,
    pub events_url: String,
    pub batch_url: String,
    pub otlp_url: String,
    pub tail_ws_url: String,
}

#[derive(Debug, Clone)]
pub struct DsnError(pub String);

impl fmt::Display for DsnError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "invalid Loxa DSN: {}", self.0)
    }
}

impl std::error::Error for DsnError {}

/// Parse a loxa:// connection URI into a resolved LoxaDSN.
///
/// Validation rules:
/// - Scheme must be `loxa://`
/// - Host is required
/// - Project path is required
/// - No userinfo allowed
/// - tls must be "true", "false", or "auto"
/// - transport must be "http", "otlp", or "grpc"
/// - Port must be 1-65535 if specified
///
/// TLS defaults: localhost/127.0.0.1/::1 -> false; everything else -> true.
/// Port defaults: tls=true -> 443; tls=false -> 80; localhost without port -> 9308.
pub fn parse(raw: &str) -> Result<LoxaDSN, DsnError> {
    if raw.is_empty() {
        return Err(DsnError("empty string".into()));
    }

    if !raw.starts_with("loxa://") {
        return Err(DsnError("scheme must be loxa://".into()));
    }

    let rest = &raw[7..]; // skip "loxa://"

    // Strip fragment if present.
    let rest = match rest.find('#') {
        Some(i) => &rest[..i],
        None => rest,
    };

    // Split into authority+path.
    let (authority, path_query) = match rest.find('/') {
        Some(i) => (&rest[..i], &rest[i..]),
        None => (rest, ""),
    };

    // Reject userinfo (API keys must not be in the URL).
    if authority.contains('@') {
        return Err(DsnError(
            "do not put API keys in the URL, use LOXA_API_KEY instead".into(),
        ));
    }

    if authority.is_empty() {
        return Err(DsnError("host is required".into()));
    }

    // Parse host and port from authority.
    let (host, port_str) = if authority.starts_with('[') {
        // IPv6: [::1] or [::1]:port
        let close = authority
            .find(']')
            .ok_or_else(|| DsnError("invalid IPv6 address".into()))?;
        let h = &authority[1..close];
        let after = &authority[close + 1..];
        if let Some(stripped) = after.strip_prefix(':') {
            (h, Some(stripped))
        } else if after.is_empty() {
            (h, None)
        } else {
            return Err(DsnError("invalid IPv6 address".into()));
        }
    } else {
        match authority.rfind(':') {
            Some(i) => (&authority[..i], Some(&authority[i + 1..])),
            None => (authority, None),
        }
    };

    if host.is_empty() {
        return Err(DsnError("host is required".into()));
    }

    // Validate port if specified.
    if let Some(ps) = port_str {
        let p: u16 = ps
            .parse()
            .map_err(|_| DsnError(format!("invalid port {ps:?}")))?;
        if p == 0 {
            return Err(DsnError(format!("invalid port {ps:?}")));
        }
        // u16 is always <= 65535, so we only need to check != 0.
        let _ = p;
    }

    // Parse path and query.
    let (path_part, query) = match path_query.find('?') {
        Some(i) => (&path_query[..i], &path_query[i + 1..]),
        None => (path_query, ""),
    };

    let project = path_part.trim_start_matches('/');
    if project.is_empty() {
        return Err(DsnError(
            "project path is required, e.g. loxa://host/my-project".into(),
        ));
    }

    // Parse query parameters.
    let mut tls_param: Option<&str> = None;
    let mut env_param: Option<&str> = None;
    let mut service_param: Option<&str> = None;
    let mut transport_param: Option<&str> = None;

    for pair in query.split('&') {
        if pair.is_empty() {
            continue;
        }
        let (k, v) = match pair.find('=') {
            Some(i) => (&pair[..i], &pair[i + 1..]),
            None => (pair, ""),
        };
        match k {
            "tls" => tls_param = Some(v),
            "env" => env_param = Some(v),
            "service" => service_param = Some(v),
            "transport" => transport_param = Some(v),
            _ => {}
        }
    }

    // TLS default.
    let mut tls = !is_localhost(host);
    if let Some(v) = tls_param {
        match v {
            "true" => tls = true,
            "false" => tls = false,
            "auto" => { /* keep computed default */ }
            _ => {
                return Err(DsnError(format!(
                    "tls must be true, false, or auto, got {v:?}"
                )))
            }
        }
    }

    // Port default.
    let port: u16 = if let Some(ps) = port_str {
        ps.parse().unwrap() // already validated above
    } else if is_localhost(host) {
        9308
    } else if tls {
        443
    } else {
        80
    };

    // Transport.
    let transport = match transport_param {
        None => "http".to_string(),
        Some("http") | Some("otlp") | Some("grpc") => transport_param.unwrap().to_string(),
        Some(v) => {
            return Err(DsnError(format!(
                "transport must be http, otlp, or grpc, got {v:?}"
            )))
        }
    };

    // Env and service.
    let env = match env_param {
        Some(v) if !v.is_empty() => v.to_string(),
        _ => "default".to_string(),
    };
    let service = service_param.unwrap_or("").to_string();

    // Build resolved URLs.
    let scheme = if tls { "https" } else { "http" };
    let ws_scheme = if tls { "wss" } else { "ws" };

    // IPv6 addresses must be bracketed in URLs per RFC 2732/3986.
    let host_part = if host.contains(':') {
        format!("[{host}]")
    } else {
        host.to_string()
    };

    let base_url = format!("{scheme}://{host_part}:{port}");

    Ok(LoxaDSN {
        scheme: "loxa".to_string(),
        host: host.to_string(),
        port,
        project: project.to_string(),
        env,
        service,
        tls,
        transport,
        base_url: base_url.clone(),
        events_url: format!("{base_url}/events"),
        batch_url: format!("{base_url}/events/batch"),
        otlp_url: format!("{base_url}/otlp/logs"),
        tail_ws_url: format!("{ws_scheme}://{host_part}:{port}/tail"),
    })
}

fn is_localhost(host: &str) -> bool {
    host == "localhost" || host == "127.0.0.1" || host == "::1"
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn basic_localhost() {
        let dsn = parse("loxa://localhost:9308/demo?tls=false").unwrap();
        assert_eq!(dsn.host, "localhost");
        assert_eq!(dsn.port, 9308);
        assert_eq!(dsn.project, "demo");
        assert!(!dsn.tls);
        assert_eq!(dsn.base_url, "http://localhost:9308");
    }

    #[test]
    fn reject_empty() {
        assert!(parse("").is_err());
    }

    #[test]
    fn reject_wrong_scheme() {
        assert!(parse("https://host/project").is_err());
    }

    #[test]
    fn reject_no_host() {
        assert!(parse("loxa://").is_err());
    }

    #[test]
    fn reject_no_project() {
        assert!(parse("loxa://host").is_err());
    }

    #[test]
    fn reject_userinfo() {
        assert!(parse("loxa://key@host/project").is_err());
    }
}
