#![forbid(unsafe_code)]

use crate::{BrowserKind, BrowserLifecycleEvent, BrowserState};
use std::net::SocketAddr;
use webgate_core::policy::{NavigationPolicy, PolicyError, ValidatedUrl};

/// Failures encountered when configuring, launching, or navigating the browser capsule.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum CapsuleError {
    ProxyMissingFailClosed,
    DirectEgressForbidden,
    InvalidProxyAddress(String),
    NavigationPolicyViolation(PolicyError),
    BrowserNotReady(BrowserState),
    InvalidLifecycleTransition(&'static str),
}

/// Loopback proxy configuration attached to the browser capsule.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct CapsuleProxyConfig {
    pub proxy_endpoint: SocketAddr,
}

impl CapsuleProxyConfig {
    pub fn new(proxy_endpoint: SocketAddr) -> Result<Self, CapsuleError> {
        if !proxy_endpoint.ip().is_loopback() {
            return Err(CapsuleError::DirectEgressForbidden);
        }
        if proxy_endpoint.port() == 0 {
            return Err(CapsuleError::InvalidProxyAddress(
                "port cannot be zero".to_string(),
            ));
        }
        Ok(Self { proxy_endpoint })
    }
}

/// Isolated browser capsule running with strict fail-closed network policies.
#[derive(Debug, Clone)]
pub struct BrowserCapsule {
    kind: BrowserKind,
    state: BrowserState,
    proxy_config: Option<CapsuleProxyConfig>,
    navigation_policy: NavigationPolicy,
    current_url: Option<ValidatedUrl>,
    cache_purged: bool,
}

impl BrowserCapsule {
    #[must_use]
    pub fn new(kind: BrowserKind, navigation_policy: NavigationPolicy) -> Self {
        Self {
            kind,
            state: BrowserState::Stopped,
            proxy_config: None,
            navigation_policy,
            current_url: None,
            cache_purged: false,
        }
    }

    #[must_use]
    pub fn kind(&self) -> BrowserKind {
        self.kind
    }

    #[must_use]
    pub fn state(&self) -> BrowserState {
        self.state
    }

    #[must_use]
    pub fn current_url(&self) -> Option<&ValidatedUrl> {
        self.current_url.as_ref()
    }

    #[must_use]
    pub const fn is_cache_purged(&self) -> bool {
        self.cache_purged
    }

    /// Attaches the mandatory loopback proxy. All outbound traffic MUST flow through it.
    pub fn attach_proxy(&mut self, endpoint: SocketAddr) -> Result<(), CapsuleError> {
        let config = CapsuleProxyConfig::new(endpoint)?;
        self.proxy_config = Some(config);
        Ok(())
    }

    /// Starts the browser capsule. Fails closed if no verified loopback proxy is attached.
    pub fn start(&mut self) -> Result<(), CapsuleError> {
        if self.proxy_config.is_none() {
            self.state = BrowserState::Failed;
            return Err(CapsuleError::ProxyMissingFailClosed);
        }

        self.state = BrowserState::Ready;
        Ok(())
    }

    /// Validates and navigates to a target URL, enforcing the strict navigation policy.
    pub fn navigate(&mut self, raw_url: &str) -> Result<ValidatedUrl, CapsuleError> {
        if self.state != BrowserState::Ready {
            return Err(CapsuleError::BrowserNotReady(self.state));
        }

        let validated = self
            .navigation_policy
            .validate_url(raw_url)
            .map_err(CapsuleError::NavigationPolicyViolation)?;

        self.current_url = Some(validated.clone());
        Ok(validated)
    }

    /// Handles platform lifecycle events (Android pause/resume/recreate/memory-trim).
    pub fn handle_lifecycle_event(
        &mut self,
        event: BrowserLifecycleEvent,
    ) -> Result<(), CapsuleError> {
        match event {
            BrowserLifecycleEvent::Pause => {
                if self.state == BrowserState::Ready {
                    self.state = BrowserState::Paused;
                    Ok(())
                } else {
                    Err(CapsuleError::InvalidLifecycleTransition(
                        "cannot pause from non-ready state",
                    ))
                }
            }
            BrowserLifecycleEvent::Resume => {
                if self.state == BrowserState::Paused {
                    // Fail-closed verification: proxy must still be valid loopback
                    if self.proxy_config.is_none() {
                        self.state = BrowserState::Failed;
                        return Err(CapsuleError::ProxyMissingFailClosed);
                    }
                    self.state = BrowserState::Ready;
                    Ok(())
                } else {
                    Err(CapsuleError::InvalidLifecycleTransition(
                        "cannot resume from non-paused state",
                    ))
                }
            }
            BrowserLifecycleEvent::SaveState => Ok(()),
            BrowserLifecycleEvent::RestoreState(raw_url) => {
                // When restoring state (e.g. after activity recreation), validate URL strictly
                let validated = self
                    .navigation_policy
                    .validate_url(&raw_url)
                    .map_err(CapsuleError::NavigationPolicyViolation)?;
                self.current_url = Some(validated);
                Ok(())
            }
            BrowserLifecycleEvent::LowMemory => {
                self.cache_purged = true;
                Ok(())
            }
        }
    }

    /// Gracefully terminates the browser capsule and clears runtime state.
    pub fn shutdown(&mut self) {
        self.state = BrowserState::Stopped;
        self.current_url = None;
        self.proxy_config = None;
        self.cache_purged = false;
    }
}

#[cfg(test)]
#[allow(clippy::unwrap_used, clippy::panic)]
mod tests {
    use super::*;
    use std::net::{IpAddr, Ipv4Addr};

    #[test]
    fn capsule_starts_with_valid_loopback_proxy() {
        let mut capsule = BrowserCapsule::new(BrowserKind::Servo, NavigationPolicy::default());
        let loopback = SocketAddr::new(IpAddr::V4(Ipv4Addr::LOCALHOST), 41000);

        assert_eq!(capsule.attach_proxy(loopback), Ok(()));
        assert_eq!(capsule.start(), Ok(()));
        assert_eq!(capsule.state(), BrowserState::Ready);
    }

    #[test]
    fn capsule_fails_closed_without_proxy() {
        let mut capsule = BrowserCapsule::new(BrowserKind::Servo, NavigationPolicy::default());

        assert_eq!(capsule.start(), Err(CapsuleError::ProxyMissingFailClosed));
        assert_eq!(capsule.state(), BrowserState::Failed);
    }

    #[test]
    fn capsule_rejects_non_loopback_proxy_egress() {
        let mut capsule = BrowserCapsule::new(BrowserKind::Servo, NavigationPolicy::default());
        let public_ip = SocketAddr::new(IpAddr::V4(Ipv4Addr::new(8, 8, 8, 8)), 8080);

        assert_eq!(
            capsule.attach_proxy(public_ip),
            Err(CapsuleError::DirectEgressForbidden)
        );
        assert_eq!(capsule.start(), Err(CapsuleError::ProxyMissingFailClosed));
    }

    #[test]
    fn capsule_navigates_to_valid_service_and_blocks_disallowed() {
        let mut capsule = BrowserCapsule::new(BrowserKind::Servo, NavigationPolicy::default());
        let loopback = SocketAddr::new(IpAddr::V4(Ipv4Addr::LOCALHOST), 41000);
        capsule.attach_proxy(loopback).unwrap();
        capsule.start().unwrap();

        // Positive navigation
        let nav = capsule
            .navigate("webgate://service/factory/orders")
            .unwrap();
        assert_eq!(nav.target_service_slug(), Some("factory"));
        assert_eq!(capsule.current_url(), Some(&nav));

        // Negative navigation (file:// scheme)
        assert!(matches!(
            capsule.navigate("file:///C:/Windows/system.ini"),
            Err(CapsuleError::NavigationPolicyViolation(
                PolicyError::DisallowedScheme(_)
            ))
        ));
    }

    #[test]
    fn capsule_handles_android_lifecycle_and_recreation() {
        let mut capsule = BrowserCapsule::new(BrowserKind::Servo, NavigationPolicy::default());
        let loopback = SocketAddr::new(IpAddr::V4(Ipv4Addr::LOCALHOST), 41000);
        capsule.attach_proxy(loopback).unwrap();
        capsule.start().unwrap();

        capsule.navigate("webgate://service/docs/spec").unwrap();

        // Pause
        capsule
            .handle_lifecycle_event(BrowserLifecycleEvent::Pause)
            .unwrap();
        assert_eq!(capsule.state(), BrowserState::Paused);

        // Low memory purge
        capsule
            .handle_lifecycle_event(BrowserLifecycleEvent::LowMemory)
            .unwrap();
        assert!(capsule.is_cache_purged());

        // Resume
        capsule
            .handle_lifecycle_event(BrowserLifecycleEvent::Resume)
            .unwrap();
        assert_eq!(capsule.state(), BrowserState::Ready);

        // Activity recreation with state restore
        let saved_url = capsule.current_url().unwrap().as_url_string();
        capsule.shutdown();
        assert_eq!(capsule.state(), BrowserState::Stopped);

        // Rehydrate
        capsule.attach_proxy(loopback).unwrap();
        capsule.start().unwrap();
        capsule
            .handle_lifecycle_event(BrowserLifecycleEvent::RestoreState(saved_url))
            .unwrap();
        assert_eq!(capsule.state(), BrowserState::Ready);
        assert_eq!(
            capsule.current_url().unwrap().as_url_string(),
            "webgate://service/docs/spec"
        );
    }
}
