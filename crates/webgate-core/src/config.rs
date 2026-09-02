#![forbid(unsafe_code)]

use std::collections::HashMap;
use std::path::Path;

use crate::device::KeyAlgorithm;
use crate::policy::NavigationPolicy;

/// Represents a single selectable protected destination service.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DestinationTarget {
    pub id: String,
    pub name: String,
    pub url: String,
    pub description: String,
    pub category: String,
}

/// Represents a relay endpoint configured for the client session.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RelayEndpointConfig {
    pub name: String,
    pub address: String,
    pub port: u16,
}

/// A comprehensive client profile loaded from an external configuration file (.toml / .json / config).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ClientConfigProfile {
    pub profile_id: String,
    pub profile_name: String,
    pub version: String,
    pub device_label: String,
    pub key_algorithm: KeyAlgorithm,
    pub primary_relay: RelayEndpointConfig,
    pub fallback_relay: Option<RelayEndpointConfig>,
    pub destinations: Vec<DestinationTarget>,
    pub allowed_domains: Vec<String>,
    pub auto_connect: bool,
    pub default_destination_url: Option<String>,
    pub metadata: HashMap<String, String>,
}

impl Default for ClientConfigProfile {
    fn default() -> Self {
        Self {
            profile_id: "default-fleet".to_string(),
            profile_name: "Standard Fleet Operations".to_string(),
            version: "1.0.0".to_string(),
            device_label: "primary-workstation".to_string(),
            key_algorithm: KeyAlgorithm::Ed25519,
            primary_relay: RelayEndpointConfig {
                name: "Relay-Alpha (Primary)".to_string(),
                address: "127.0.0.1".to_string(),
                port: 43111,
            },
            fallback_relay: Some(RelayEndpointConfig {
                name: "Relay-Beta (Failover)".to_string(),
                address: "127.0.0.1".to_string(),
                port: 43112,
            }),
            destinations: vec![
                DestinationTarget {
                    id: "docs".to_string(),
                    name: "Corporate Knowledge Base".to_string(),
                    url: "webgate://service/docs/overview".to_string(),
                    description: "Encrypted architecture documentation and operational guides"
                        .to_string(),
                    category: "Documentation".to_string(),
                },
                DestinationTarget {
                    id: "factory".to_string(),
                    name: "FactoryOS Production Terminal".to_string(),
                    url: "webgate://service/factory/terminal".to_string(),
                    description: "Assembly line telemetry and real-time controller mesh"
                        .to_string(),
                    category: "Operations".to_string(),
                },
                DestinationTarget {
                    id: "monitoring".to_string(),
                    name: "Telemetry & Security Metrics".to_string(),
                    url: "webgate://service/monitoring/telemetry".to_string(),
                    description: "Relay latency, packet health and device posture analytics"
                        .to_string(),
                    category: "Infrastructure".to_string(),
                },
            ],
            allowed_domains: vec!["service".to_string(), "docs.internal".to_string()],
            auto_connect: true,
            default_destination_url: Some("webgate://service/docs/overview".to_string()),
            metadata: HashMap::new(),
        }
    }
}

/// Errors occurring during configuration profile loading and binding.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ConfigError {
    FileNotFound(String),
    ParseError(String),
    ValidationError(String),
    EmptyDestinations,
    InvalidRelayAddress(String),
    LockPoisoned,
}

impl std::fmt::Display for ConfigError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::FileNotFound(msg) => write!(f, "File not found: {msg}"),
            Self::ParseError(msg) => write!(f, "Parse error: {msg}"),
            Self::ValidationError(msg) => write!(f, "Validation error: {msg}"),
            Self::EmptyDestinations => write!(f, "Destinations list cannot be empty"),
            Self::InvalidRelayAddress(msg) => write!(f, "Invalid relay address: {msg}"),
            Self::LockPoisoned => write!(f, "Internal state lock poisoned"),
        }
    }
}

impl std::error::Error for ConfigError {}

impl ClientConfigProfile {
    /// Constructs a `NavigationPolicy` directly derived from this profile.
    #[must_use]
    pub fn build_navigation_policy(&self) -> NavigationPolicy {
        NavigationPolicy::new(self.allowed_domains.clone())
    }

    /// Finds a destination by its ID or canonical URL.
    #[must_use]
    pub fn find_destination(&self, query: &str) -> Option<&DestinationTarget> {
        self.destinations
            .iter()
            .find(|d| d.id == query || d.url == query)
    }

    /// Validates the profile integrity.
    pub fn validate(&self) -> Result<(), ConfigError> {
        if self.profile_id.trim().is_empty() {
            return Err(ConfigError::ValidationError(
                "profile_id cannot be empty".to_string(),
            ));
        }
        if self.primary_relay.address.trim().is_empty() {
            return Err(ConfigError::InvalidRelayAddress(
                "primary relay address cannot be empty".to_string(),
            ));
        }
        if self.primary_relay.port == 0 {
            return Err(ConfigError::InvalidRelayAddress(
                "primary relay port cannot be 0".to_string(),
            ));
        }
        if let Some(ref fb) = self.fallback_relay {
            if fb.address.trim().is_empty() {
                return Err(ConfigError::InvalidRelayAddress(
                    "fallback relay address cannot be empty".to_string(),
                ));
            }
            if fb.port == 0 {
                return Err(ConfigError::InvalidRelayAddress(
                    "fallback relay port cannot be 0".to_string(),
                ));
            }
        }
        if self.destinations.is_empty() {
            return Err(ConfigError::EmptyDestinations);
        }
        if self.allowed_domains.is_empty() {
            return Err(ConfigError::ValidationError(
                "allowed_domains cannot be empty".to_string(),
            ));
        }
        for dest in &self.destinations {
            if dest.id.trim().is_empty() {
                return Err(ConfigError::ValidationError(
                    "destination id cannot be empty".to_string(),
                ));
            }
            if dest.name.trim().is_empty() {
                return Err(ConfigError::ValidationError(
                    "destination name cannot be empty".to_string(),
                ));
            }
            if !dest.url.starts_with("webgate://") && !dest.url.starts_with("https://") {
                return Err(ConfigError::ValidationError(format!(
                    "Destination URL '{}' must use webgate:// or https:// scheme",
                    dest.url
                )));
            }
        }
        Ok(())
    }

    /// Parses a simple key-value / TOML-like profile format.
    pub fn from_toml_str(content: &str) -> Result<Self, ConfigError> {
        if content.trim().is_empty() {
            return Err(ConfigError::ParseError(
                "Config content is empty".to_string(),
            ));
        }

        let mut profile = Self::default();
        let mut custom_destinations = Vec::new();
        let mut custom_allowed_domains = Vec::new();

        for (line_no, line) in content.lines().enumerate() {
            let line = line.trim();
            if line.is_empty() || line.starts_with('#') {
                continue;
            }

            if let Some((k, v)) = line.split_once('=') {
                let key = k.trim();
                let val = v.trim().trim_matches('"').trim_matches('\'');

                match key {
                    "profile_id" => profile.profile_id = val.to_string(),
                    "profile_name" => profile.profile_name = val.to_string(),
                    "version" => profile.version = val.to_string(),
                    "device_label" => profile.device_label = val.to_string(),
                    "primary_relay_addr" => profile.primary_relay.address = val.to_string(),
                    "primary_relay_port" => match val.parse::<u16>() {
                        Ok(p) => profile.primary_relay.port = p,
                        Err(_) => {
                            return Err(ConfigError::ParseError(format!(
                                "Invalid primary_relay_port '{}' on line {}",
                                val,
                                line_no + 1
                            )));
                        }
                    },
                    "fallback_relay_addr" => {
                        if let Some(ref mut fb) = profile.fallback_relay {
                            fb.address = val.to_string();
                        } else {
                            profile.fallback_relay = Some(RelayEndpointConfig {
                                name: "Relay-Beta (Failover)".to_string(),
                                address: val.to_string(),
                                port: 43112,
                            });
                        }
                    }
                    "fallback_relay_port" => {
                        let p = val.parse::<u16>().map_err(|_| {
                            ConfigError::ParseError(format!(
                                "Invalid fallback_relay_port '{}' on line {}",
                                val,
                                line_no + 1
                            ))
                        })?;
                        if let Some(ref mut fb) = profile.fallback_relay {
                            fb.port = p;
                        } else {
                            profile.fallback_relay = Some(RelayEndpointConfig {
                                name: "Relay-Beta (Failover)".to_string(),
                                address: "127.0.0.1".to_string(),
                                port: p,
                            });
                        }
                    }
                    "allowed_domains" => {
                        let domains: Vec<String> = val
                            .split(',')
                            .map(|s| s.trim().trim_matches('"').trim_matches('\'').to_string())
                            .filter(|s| !s.is_empty())
                            .collect();
                        if !domains.is_empty() {
                            custom_allowed_domains = domains;
                        }
                    }
                    "default_destination" => {
                        profile.default_destination_url = Some(val.to_string())
                    }
                    "destination" => {
                        // Format: id|name|url|category|description
                        let parts: Vec<&str> = val.split('|').map(|s| s.trim()).collect();
                        if parts.len() >= 3 {
                            custom_destinations.push(DestinationTarget {
                                id: parts[0].to_string(),
                                name: parts[1].to_string(),
                                url: parts[2].to_string(),
                                category: parts.get(3).unwrap_or(&"General").to_string(),
                                description: parts.get(4).unwrap_or(&"").to_string(),
                            });
                        } else {
                            return Err(ConfigError::ParseError(format!(
                                "Invalid destination definition '{}' on line {}",
                                val,
                                line_no + 1
                            )));
                        }
                    }
                    _ => {
                        // Save extra attributes to metadata
                        profile.metadata.insert(key.to_string(), val.to_string());
                    }
                }
            } else {
                return Err(ConfigError::ParseError(format!(
                    "Syntax error on line {}: expected 'key = value'",
                    line_no + 1
                )));
            }
        }

        if !custom_destinations.is_empty() {
            profile.destinations = custom_destinations;
        }
        if !custom_allowed_domains.is_empty() {
            profile.allowed_domains = custom_allowed_domains;
        }

        profile.validate()?;
        Ok(profile)
    }

    /// Loads and binds a configuration file from path.
    pub fn load_from_file<P: AsRef<Path>>(path: P) -> Result<Self, ConfigError> {
        let path_ref = path.as_ref();
        let content = std::fs::read_to_string(path_ref).map_err(|e| {
            ConfigError::FileNotFound(format!("Failed to read '{:?}': {}", path_ref, e))
        })?;
        Self::from_toml_str(&content)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_default_profile_valid() {
        let profile = ClientConfigProfile::default();
        assert!(profile.validate().is_ok());
        assert_eq!(profile.destinations.len(), 3);
        assert!(profile.find_destination("docs").is_some());
    }

    #[test]
    fn test_parse_custom_toml() {
        let raw = r#"
        profile_id = "staging-mesh"
        profile_name = "Staging Field Mesh"
        primary_relay_addr = "10.0.0.5"
        primary_relay_port = 50001
        allowed_domains = "service, infra.internal"
        destination = "k8s|Kubernetes Cluster|webgate://service/k8s|Infra|Internal K8s Dashboard"
        default_destination = "webgate://service/k8s"
        "#;

        let res = ClientConfigProfile::from_toml_str(raw);
        assert!(res.is_ok());
        if let Ok(profile) = res {
            assert_eq!(profile.profile_id, "staging-mesh");
            assert_eq!(profile.primary_relay.port, 50001);
            assert_eq!(profile.destinations.len(), 1);
            assert_eq!(profile.destinations[0].id, "k8s");
            assert_eq!(profile.allowed_domains, vec!["service", "infra.internal"]);
            assert_eq!(
                profile.default_destination_url.as_deref(),
                Some("webgate://service/k8s")
            );
        }
    }

    #[test]
    fn test_validation_rejects_empty_profile_id() {
        let profile = ClientConfigProfile {
            profile_id: "   ".to_string(),
            ..Default::default()
        };
        assert!(matches!(
            profile.validate(),
            Err(ConfigError::ValidationError(_))
        ));
    }

    #[test]
    fn test_validation_rejects_zero_primary_port() {
        let profile = ClientConfigProfile {
            primary_relay: RelayEndpointConfig {
                name: "Relay".to_string(),
                address: "127.0.0.1".to_string(),
                port: 0,
            },
            ..Default::default()
        };
        assert!(matches!(
            profile.validate(),
            Err(ConfigError::InvalidRelayAddress(_))
        ));
    }

    #[test]
    fn test_validation_rejects_disallowed_destination_scheme() {
        let mut profile = ClientConfigProfile::default();
        profile.destinations.push(DestinationTarget {
            id: "bad".to_string(),
            name: "Bad Scheme".to_string(),
            url: "ftp://service/bad".to_string(),
            category: "Bad".to_string(),
            description: "".to_string(),
        });
        assert!(matches!(
            profile.validate(),
            Err(ConfigError::ValidationError(_))
        ));
    }

    #[test]
    fn test_parse_rejects_invalid_syntax() {
        let raw = "not a valid key value pair";
        assert!(matches!(
            ClientConfigProfile::from_toml_str(raw),
            Err(ConfigError::ParseError(_))
        ));
    }

    #[test]
    fn test_parse_rejects_invalid_port() {
        let raw = "primary_relay_port = notanumber";
        assert!(matches!(
            ClientConfigProfile::from_toml_str(raw),
            Err(ConfigError::ParseError(_))
        ));
    }
}
