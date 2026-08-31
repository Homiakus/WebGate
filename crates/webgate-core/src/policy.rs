#![forbid(unsafe_code)]

/// Errors that occur during strict navigation or URL verification.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum PolicyError {
    EmptyUrl,
    DisallowedScheme(String),
    MissingHost,
    UserInfoForbidden,
    InvalidPort(u16),
    InvalidPathTraversal,
    InvalidCharacters,
    HostNotAllowed(String),
    MalformedUrl(String),
}

/// Strict navigation policy configuration.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct NavigationPolicy {
    allowed_schemes: Vec<String>,
    allowed_domains: Vec<String>,
    allow_custom_scheme: bool,
}

impl Default for NavigationPolicy {
    fn default() -> Self {
        Self {
            allowed_schemes: vec!["https".to_string(), "webgate".to_string()],
            allowed_domains: Vec::new(),
            allow_custom_scheme: true,
        }
    }
}

impl NavigationPolicy {
    /// Creates a new policy with custom allowed domains.
    #[must_use]
    pub fn new(allowed_domains: Vec<String>) -> Self {
        Self {
            allowed_schemes: vec!["https".to_string(), "webgate".to_string()],
            allowed_domains,
            allow_custom_scheme: true,
        }
    }

    /// Verifies if a given raw URL string complies with the strict security policy.
    pub fn validate_url(&self, raw_url: &str) -> Result<ValidatedUrl, PolicyError> {
        let trimmed = raw_url.trim();
        if trimmed.is_empty() {
            return Err(PolicyError::EmptyUrl);
        }

        // Check for control characters or null bytes
        if trimmed.chars().any(|c| c.is_control() || c == '\0') {
            return Err(PolicyError::InvalidCharacters);
        }

        // Split scheme up to first colon
        let Some((scheme, after_colon)) = trimmed.split_once(':') else {
            return Err(PolicyError::MalformedUrl(
                "missing scheme colon".to_string(),
            ));
        };

        let scheme_lower = scheme.to_ascii_lowercase();
        if !self.allowed_schemes.contains(&scheme_lower) {
            return Err(PolicyError::DisallowedScheme(scheme.to_string()));
        }

        if scheme_lower == "webgate" && !self.allow_custom_scheme {
            return Err(PolicyError::DisallowedScheme(
                "webgate scheme disabled".to_string(),
            ));
        }

        // Allowed schemes must be hierarchical and have //
        let Some(rest) = after_colon.strip_prefix("//") else {
            return Err(PolicyError::MalformedUrl(
                "missing // after scheme".to_string(),
            ));
        };

        // Parse host and path
        let (authority, path_and_query) = match rest.split_once('/') {
            Some((auth, path)) => (auth, format!("/{path}")),
            None => (rest, "/".to_string()),
        };

        // Reject userinfo (e.g. user:pass@host)
        if authority.contains('@') {
            return Err(PolicyError::UserInfoForbidden);
        }

        // Parse host and optional port
        let (host, port) = if authority.starts_with('[') {
            // IPv6 literal
            let Some(end_bracket) = authority.find(']') else {
                return Err(PolicyError::MalformedUrl(
                    "unclosed IPv6 bracket".to_string(),
                ));
            };
            let ipv6_host = &authority[1..end_bracket];
            let remainder = &authority[end_bracket + 1..];
            if remainder.is_empty() {
                (ipv6_host, None)
            } else if let Some(p_str) = remainder.strip_prefix(':') {
                let parsed_port = p_str
                    .parse::<u16>()
                    .map_err(|_| PolicyError::InvalidPort(0))?;
                (ipv6_host, Some(parsed_port))
            } else {
                return Err(PolicyError::MalformedUrl(
                    "invalid IPv6 authority format".to_string(),
                ));
            }
        } else if let Some((h, p_str)) = authority.split_once(':') {
            let parsed_port = p_str
                .parse::<u16>()
                .map_err(|_| PolicyError::InvalidPort(0))?;
            (h, Some(parsed_port))
        } else {
            (authority, None)
        };

        if host.is_empty() {
            return Err(PolicyError::MissingHost);
        }

        let host_lower = host.to_ascii_lowercase();

        // Check path for directory traversal attempts
        for segment in path_and_query.split('/') {
            if segment == ".." {
                return Err(PolicyError::InvalidPathTraversal);
            }
        }

        // If domain whitelist is configured, enforce it
        if !self.allowed_domains.is_empty() {
            let is_allowed = self.allowed_domains.iter().any(|allowed| {
                if let Some(suffix) = allowed.strip_prefix("*.") {
                    host_lower.ends_with(suffix) && host_lower.len() > suffix.len()
                } else {
                    host_lower == allowed.to_ascii_lowercase()
                }
            });

            if !is_allowed {
                return Err(PolicyError::HostNotAllowed(host_lower));
            }
        }

        Ok(ValidatedUrl {
            scheme: scheme_lower,
            host: host_lower,
            port,
            path: path_and_query,
        })
    }
}

/// A parsed and verified URL safe for navigation within the WebGate boundary.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ValidatedUrl {
    pub scheme: String,
    pub host: String,
    pub port: Option<u16>,
    pub path: String,
}

impl ValidatedUrl {
    #[must_use]
    pub fn is_webgate_internal(&self) -> bool {
        self.scheme == "webgate"
    }

    /// Reconstructs the canonical serialized URL string.
    #[must_use]
    pub fn as_url_string(&self) -> String {
        let authority = match self.port {
            Some(p) => format!("{}:{}", self.host, p),
            None => self.host.clone(),
        };
        let path = if self.path.starts_with('/') {
            self.path.clone()
        } else {
            format!("/{}", self.path)
        };
        format!("{}://{}{}", self.scheme, authority, path)
    }

    /// Extracts service slug from deep link `webgate://service/<slug>/...`
    #[must_use]
    pub fn target_service_slug(&self) -> Option<&str> {
        if self.scheme == "webgate" {
            if self.host == "service" {
                let trimmed_path = self.path.trim_start_matches('/');
                let mut parts = trimmed_path.split('/');
                return parts.next().filter(|s| !s.is_empty());
            }
            Some(&self.host)
        } else {
            None
        }
    }
}

#[cfg(test)]
#[allow(clippy::unwrap_used, clippy::panic)]
mod tests {
    use super::*;

    #[test]
    fn accepts_valid_https_url() {
        let policy = NavigationPolicy::default();
        let validated = policy.validate_url("https://gateway.internal/app").unwrap();
        assert_eq!(validated.scheme, "https");
        assert_eq!(validated.host, "gateway.internal");
        assert_eq!(validated.path, "/app");
        assert_eq!(validated.port, None);
        assert_eq!(validated.as_url_string(), "https://gateway.internal/app");
    }

    #[test]
    fn accepts_webgate_deep_link() {
        let policy = NavigationPolicy::default();
        let validated = policy
            .validate_url("webgate://service/docs/v1/page")
            .unwrap();
        assert_eq!(validated.scheme, "webgate");
        assert_eq!(validated.target_service_slug(), Some("docs"));
        assert_eq!(validated.as_url_string(), "webgate://service/docs/v1/page");
    }

    #[test]
    fn rejects_disallowed_schemes() {
        let policy = NavigationPolicy::default();
        assert!(matches!(
            policy.validate_url("file:///etc/passwd"),
            Err(PolicyError::DisallowedScheme(_))
        ));
        assert!(matches!(
            policy.validate_url("javascript:alert(1)"),
            Err(PolicyError::DisallowedScheme(_))
        ));
        assert!(matches!(
            policy.validate_url("data:text/html,test"),
            Err(PolicyError::DisallowedScheme(_))
        ));
        assert!(matches!(
            policy.validate_url("http://insecure.example.com"),
            Err(PolicyError::DisallowedScheme(_))
        ));
    }

    #[test]
    fn rejects_userinfo_in_url() {
        let policy = NavigationPolicy::default();
        assert_eq!(
            policy.validate_url("https://admin:secret@gateway.internal/"),
            Err(PolicyError::UserInfoForbidden)
        );
    }

    #[test]
    fn rejects_path_traversal() {
        let policy = NavigationPolicy::default();
        assert_eq!(
            policy.validate_url("https://gateway.internal/app/../secret"),
            Err(PolicyError::InvalidPathTraversal)
        );
    }

    #[test]
    fn rejects_control_characters() {
        let policy = NavigationPolicy::default();
        assert_eq!(
            policy.validate_url("https://gateway.internal\r\n/app"),
            Err(PolicyError::InvalidCharacters)
        );
    }

    #[test]
    fn enforces_allowed_domains() {
        let policy =
            NavigationPolicy::new(vec!["*.webgate.corp".to_string(), "auth.corp".to_string()]);
        assert!(
            policy
                .validate_url("https://app.webgate.corp/login")
                .is_ok()
        );
        assert!(policy.validate_url("https://auth.corp/status").is_ok());
        assert!(matches!(
            policy.validate_url("https://evil.corp/login"),
            Err(PolicyError::HostNotAllowed(_))
        ));
    }
}
