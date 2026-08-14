use std::fmt;

/// Parsed and resolved loza:// DSN.
#[derive(Clone, PartialEq)]
pub struct LozaDSN {
    pub scheme: String,
    pub host: String,
    pub port: u16,
    pub collector_name: String,
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
    pub username: Option<String>,
    pub password: Option<String>,
}

impl fmt::Debug for LozaDSN {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("LozaDSN")
            .field("scheme", &self.scheme)
            .field("host", &self.host)
            .field("port", &self.port)
            .field("collector_name", &self.collector_name)
            .field("project", &self.project)
            .field("env", &self.env)
            .field("service", &self.service)
            .field("tls", &self.tls)
            .field("transport", &self.transport)
            .field("base_url", &self.base_url)
            .field("events_url", &self.events_url)
            .field("batch_url", &self.batch_url)
            .field("otlp_url", &self.otlp_url)
            .field("tail_ws_url", &self.tail_ws_url)
            .field("credentials", &self.username.as_ref().map(|_| "<redacted>"))
            .finish()
    }
}

impl fmt::Display for LozaDSN {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(&self.base_url)
    }
}

#[derive(Debug, Clone)]
pub struct DsnError(pub String);

impl fmt::Display for DsnError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "invalid Loza DSN: {}", self.0)
    }
}

impl std::error::Error for DsnError {}

/// Parse a loza:// connection URI into a resolved LozaDSN.
///
/// Validation rules:
/// - Scheme must be `loza://`
/// - Host is required
/// - Collector path is required
/// - Private userinfo must contain a non-empty username and password
/// - Public lx_pub_... userinfo uses an explicitly empty password
/// - tls must be "true", "false", or "auto"
/// - transport must be "http", "otlp", or "grpc"
/// - Port must be 1-65535 if specified
///
/// TLS defaults: localhost/127.0.0.1/::1 -> false; everything else -> true.
/// Port defaults: tls=true -> 443; tls=false -> 80; localhost without port -> 9308.
pub fn parse(raw: &str) -> Result<LozaDSN, DsnError> {
    if raw.is_empty() {
        return Err(DsnError("empty string".into()));
    }

    if !raw.starts_with("loza://") {
        return Err(DsnError("scheme must be loza://".into()));
    }

    let rest = &raw[7..]; // skip "loza://"

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

    // Split optional userinfo from the authority. The final `@` is the
    // delimiter; an additional raw `@` belongs to malformed userinfo.
    let (userinfo, authority) = match authority.rsplit_once('@') {
        Some((userinfo, authority)) => {
            if userinfo.contains('@') {
                return Err(DsnError("userinfo contains an unescaped @".into()));
            }
            (Some(userinfo), authority)
        }
        None => (None, authority),
    };

    let (username, password) = match userinfo {
        None => (None, None),
        Some(value) => {
            let (raw_username, raw_password) = value
                .split_once(':')
                .ok_or_else(|| DsnError("userinfo must be username:password".into()))?;
            if raw_username.is_empty() {
                return Err(DsnError(
                    "userinfo requires username:password or lx_pub_...:".into(),
                ));
            }
            let username = percent_decode_userinfo(raw_username, "username")?;
            let password = percent_decode_userinfo(raw_password, "password")?;
            if username.is_empty()
                || (password.is_empty() && !is_public_credential_username(&username))
            {
                return Err(DsnError(
                    "userinfo requires username:password or lx_pub_...:".into(),
                ));
            }
            if username.contains(':') || username.chars().any(char::is_whitespace) {
                return Err(DsnError(
                    "userinfo username must not contain ':' or whitespace".into(),
                ));
            }
            (Some(username), Some(password))
        }
    };

    if authority.is_empty() {
        return Err(DsnError("host is required".into()));
    }

    if authority.contains('@') {
        return Err(DsnError("authority contains an unescaped @".into()));
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

    let collector_name = path_part.trim_start_matches('/');
    if collector_name.is_empty() {
        return Err(DsnError(
            "collector path is required, e.g. loza://host/my-collector".into(),
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

    let collector_base_url = format!("{base_url}/collectors/{collector_name}");
    let collector_tail_base_url =
        format!("{ws_scheme}://{host_part}:{port}/collectors/{collector_name}");

    Ok(LozaDSN {
        scheme: "loza".to_string(),
        host: host.to_string(),
        port,
        collector_name: collector_name.to_string(),
        project: collector_name.to_string(),
        env,
        service,
        tls,
        transport,
        base_url: base_url.clone(),
        events_url: format!("{collector_base_url}/events"),
        batch_url: format!("{collector_base_url}/events/batch"),
        otlp_url: format!("{collector_base_url}/otlp/logs"),
        tail_ws_url: format!("{collector_tail_base_url}/tail"),
        username,
        password,
    })
}

fn percent_decode_userinfo(value: &str, field: &str) -> Result<String, DsnError> {
    let bytes = value.as_bytes();
    let mut decoded = Vec::with_capacity(bytes.len());
    let mut index = 0;
    while index < bytes.len() {
        match bytes[index] {
            b'%' => {
                if index + 2 >= bytes.len() {
                    return Err(DsnError(format!(
                        "userinfo {field} has an incomplete percent escape"
                    )));
                }
                let high = hex_value(bytes[index + 1]).ok_or_else(|| {
                    DsnError(format!("userinfo {field} has an invalid percent escape"))
                })?;
                let low = hex_value(bytes[index + 2]).ok_or_else(|| {
                    DsnError(format!("userinfo {field} has an invalid percent escape"))
                })?;
                decoded.push((high << 4) | low);
                index += 3;
            }
            byte if is_unreserved(byte) => {
                decoded.push(byte);
                index += 1;
            }
            _ => {
                return Err(DsnError(format!(
                    "userinfo {field} contains a character that must be percent-encoded"
                )))
            }
        }
    }
    String::from_utf8(decoded).map_err(|_| DsnError(format!("userinfo {field} is not valid UTF-8")))
}

fn is_unreserved(byte: u8) -> bool {
    byte.is_ascii_alphanumeric() || matches!(byte, b'-' | b'.' | b'_' | b'~')
}

fn hex_value(byte: u8) -> Option<u8> {
    match byte {
        b'0'..=b'9' => Some(byte - b'0'),
        b'a'..=b'f' => Some(byte - b'a' + 10),
        b'A'..=b'F' => Some(byte - b'A' + 10),
        _ => None,
    }
}

pub fn is_public_credential_username(username: &str) -> bool {
    const PREFIX: &str = "lx_pub_";
    username.starts_with(PREFIX) && username.len() > PREFIX.len()
}

fn is_localhost(host: &str) -> bool {
    host == "localhost" || host == "127.0.0.1" || host == "::1"
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn basic_localhost() {
        let dsn = parse("loza://localhost:9308/demo?tls=false").unwrap();
        assert_eq!(dsn.host, "localhost");
        assert_eq!(dsn.port, 9308);
        assert_eq!(dsn.project, "demo");
        assert!(!dsn.tls);
        assert_eq!(dsn.base_url, "http://localhost:9308");
    }

    #[test]
    fn parses_and_redacts_credentials() {
        let dsn = parse("loza://key%2Did:s%40cret%3Avalue@host/project").unwrap();
        assert_eq!(dsn.username.as_deref(), Some("key-id"));
        assert_eq!(dsn.password.as_deref(), Some("s@cret:value"));
        assert!(!dsn.base_url.contains("s@cret"));
        assert!(!format!("{dsn:?}").contains("s@cret"));
    }

    #[test]
    fn rejects_invalid_credentials() {
        assert!(parse("loza://key@host/project").is_err());
        assert!(parse("loza://key:@host/project").is_err());
        assert!(parse("loza://:secret@host/project").is_err());
        assert!(parse("loza://key:secret%ZZ@host/project").is_err());
        assert!(parse("loza://key%3Aname:secret@host/project").is_err());
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
        assert!(parse("loza://").is_err());
    }

    #[test]
    fn reject_no_project() {
        assert!(parse("loza://host").is_err());
    }
}
