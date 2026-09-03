#![forbid(unsafe_code)]

use crate::adapter::{
    RendererQualificationEvidence, RendererQualificationSnapshot, ServoContractAdapter,
    ServoEmbeddingConfig,
};
use crate::{BrowserConfig, BrowserKind, BrowserLifecycleEvent, BrowserState, ProtectedBrowser};
use std::net::SocketAddr;
use webgate_core::Platform;
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
#[derive(Debug)]
pub struct BrowserCapsule {
    kind: BrowserKind,
    state: BrowserState,
    proxy_config: Option<CapsuleProxyConfig>,
    navigation_policy: NavigationPolicy,
    current_url: Option<ValidatedUrl>,
    cache_purged: bool,
    adapter: Option<ServoContractAdapter>,
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
            adapter: None,
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

    #[must_use]
    pub fn adapter(&self) -> Option<&ServoContractAdapter> {
        self.adapter.as_ref()
    }

    /// Returns renderer-observed proof. Contract-only adapters intentionally
    /// return an unqualified snapshot even when BrowserCapsule policy/proxy setup
    /// itself is ready.
    #[must_use]
    pub fn renderer_qualification(&self) -> RendererQualificationSnapshot {
        self.adapter
            .as_ref()
            .map(RendererQualificationEvidence::qualification_snapshot)
            .unwrap_or_default()
    }

    /// Attaches the mandatory loopback proxy. All outbound traffic MUST flow through it.
    pub fn attach_proxy(&mut self, endpoint: SocketAddr) -> Result<(), CapsuleError> {
        let config = CapsuleProxyConfig::new(endpoint)?;
        self.proxy_config = Some(config);
        Ok(())
    }

    /// Starts the browser capsule. Fails closed if no verified loopback proxy is attached.
    pub fn start(&mut self) -> Result<(), CapsuleError> {
        let Some(proxy) = self.proxy_config else {
            self.state = BrowserState::Failed;
            return Err(CapsuleError::ProxyMissingFailClosed);
        };

        let browser_cfg = BrowserConfig::new(Platform::current());
        let servo_cfg = ServoEmbeddingConfig::new(browser_cfg).with_proxy(proxy.proxy_endpoint);
        let mut adapter = ServoContractAdapter::new(servo_cfg);

        if let Err(e) = adapter.initialize() {
            self.state = BrowserState::Failed;
            return Err(CapsuleError::InvalidProxyAddress(e.to_string()));
        }

        self.adapter = Some(adapter);
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

        if let Some(adapter) = &mut self.adapter {
            let _ = adapter.load_url(raw_url);
        }

        self.current_url = Some(validated.clone());
        Ok(validated)
    }

    /// Dispatches a subresource fetch through the verified proxy pipeline.
    pub fn dispatch_subresource_fetch(&self, resource_url: &str) -> Result<String, CapsuleError> {
        if self.state != BrowserState::Ready {
            return Err(CapsuleError::BrowserNotReady(self.state));
        }

        let validated = self
            .navigation_policy
            .validate_url(resource_url)
            .map_err(CapsuleError::NavigationPolicyViolation)?;

        let Some(adapter) = &self.adapter else {
            return Err(CapsuleError::BrowserNotReady(self.state));
        };

        adapter
            .execute_proxied_fetch(&validated.as_url_string())
            .map_err(|e| CapsuleError::InvalidProxyAddress(e.to_string()))
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
                    if let Some(adapter) = &mut self.adapter {
                        let _ = adapter.handle_lifecycle_event(BrowserLifecycleEvent::Pause);
                    }
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
                    if let Some(adapter) = &mut self.adapter {
                        let _ = adapter.handle_lifecycle_event(BrowserLifecycleEvent::Resume);
                    }
                    self.state = BrowserState::Ready;
                    Ok(())
                } else {
                    Err(CapsuleError::InvalidLifecycleTransition(
                        "cannot resume from non-paused state",
                    ))
                }
            }
            BrowserLifecycleEvent::SaveState => {
                if let Some(adapter) = &mut self.adapter {
                    let _ = adapter.handle_lifecycle_event(BrowserLifecycleEvent::SaveState);
                }
                Ok(())
            }
            BrowserLifecycleEvent::RestoreState(raw_url) => {
                // When restoring state (e.g. after activity recreation), validate URL strictly
                let validated = self
                    .navigation_policy
                    .validate_url(&raw_url)
                    .map_err(CapsuleError::NavigationPolicyViolation)?;
                if let Some(adapter) = &mut self.adapter {
                    let _ = adapter.handle_lifecycle_event(BrowserLifecycleEvent::RestoreState(
                        raw_url.clone(),
                    ));
                }
                self.current_url = Some(validated);
                Ok(())
            }
            BrowserLifecycleEvent::LowMemory => {
                self.cache_purged = true;
                if let Some(adapter) = &mut self.adapter {
                    let _ = adapter.handle_lifecycle_event(BrowserLifecycleEvent::LowMemory);
                }
                Ok(())
            }
        }
    }

    /// Gracefully terminates the browser capsule and clears runtime state.
    pub fn shutdown(&mut self) {
        if let Some(adapter) = &mut self.adapter {
            adapter.shutdown();
        }
        self.state = BrowserState::Stopped;
        self.current_url = None;
        self.proxy_config = None;
        self.cache_purged = false;
        self.adapter = None;
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
        assert!(capsule.adapter().is_some());
        assert!(!capsule.renderer_qualification().qualifies_open());
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
    fn capsule_navigates_to_valid_service_and_dispatches_subresource() {
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

        // Proxied subresource fetch
        let sub = capsule
            .dispatch_subresource_fetch("webgate://service/factory/static/app.js")
            .unwrap();
        assert!(sub.contains("proxied_response"));

        // Negative navigation (file:// scheme)
        assert!(matches!(
            capsule.navigate("file:///C:/Windows/system.ini"),
            Err(CapsuleError::NavigationPolicyViolation(
                PolicyError::DisallowedScheme(_)
            ))
        ));

        // Negative subresource fetch (disallowed scheme)
        assert!(matches!(
            capsule.dispatch_subresource_fetch("file:///C:/Windows/malicious.js"),
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
