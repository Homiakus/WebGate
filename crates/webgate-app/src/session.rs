#![forbid(unsafe_code)]

use std::collections::HashMap;

use webgate_browser::BrowserKind;
use webgate_browser::capsule::{BrowserCapsule, CapsuleError};
use webgate_core::broker::{
    BrokerCapability, BrokerRequest, BrokerRequestPayload, BrokerSecurityGate,
};
use webgate_core::config::ClientConfigProfile;
use webgate_transport::{LocalProxyEndpoint, TransportState};

/// Human/API-visible lifecycle of one protected application session.
///
/// `Open` is deliberately unreachable with the current Servo adapter. The adapter
/// does not yet provide a real renderer/navigation-commit proof, so successful
/// BrowserCapsule setup terminates at `RendererUnqualified` instead of inventing
/// a protected browser success state.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ApplicationSessionState {
    Requested,
    Authorizing,
    TransportReady,
    StartingProtectedBrowser,
    Navigating,
    RendererUnqualified,
    Open,
    Denied,
    Offline,
    Failed,
    Closed,
}

impl ApplicationSessionState {
    #[must_use]
    pub const fn as_str(self) -> &'static str {
        match self {
            Self::Requested => "requested",
            Self::Authorizing => "authorizing",
            Self::TransportReady => "transport_ready",
            Self::StartingProtectedBrowser => "starting_protected_browser",
            Self::Navigating => "navigating",
            Self::RendererUnqualified => "renderer_unqualified",
            Self::Open => "open",
            Self::Denied => "denied",
            Self::Offline => "offline",
            Self::Failed => "failed",
            Self::Closed => "closed",
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ApplicationSessionSnapshot {
    pub session_id: String,
    pub target_url: String,
    pub state: ApplicationSessionState,
    pub message: String,
    pub transitions: Vec<ApplicationSessionState>,
}

#[derive(Debug)]
struct ActiveSession {
    snapshot: ApplicationSessionSnapshot,
    capsule: Option<BrowserCapsule>,
}

/// Owns protected browser capsules for GUI/CLI/deep-link orchestration.
///
/// The session identifier is a local correlation identifier only. It is not an
/// authorization token and must never be treated as a security credential.
#[derive(Debug, Default)]
pub struct ApplicationSessionManager {
    next_sequence: u64,
    sessions: HashMap<String, ActiveSession>,
}

impl ApplicationSessionManager {
    #[must_use]
    pub fn new() -> Self {
        Self::default()
    }

    fn next_session_id(&mut self) -> String {
        self.next_sequence = self.next_sequence.saturating_add(1);
        format!("wgs-{}-{}", std::process::id(), self.next_sequence)
    }

    fn insert_terminal(
        &mut self,
        session_id: String,
        target_url: String,
        state: ApplicationSessionState,
        message: impl Into<String>,
        mut transitions: Vec<ApplicationSessionState>,
    ) -> ApplicationSessionSnapshot {
        if transitions.last().copied() != Some(state) {
            transitions.push(state);
        }
        let snapshot = ApplicationSessionSnapshot {
            session_id: session_id.clone(),
            target_url,
            state,
            message: message.into(),
            transitions,
        };
        self.sessions.insert(
            session_id,
            ActiveSession {
                snapshot: snapshot.clone(),
                capsule: None,
            },
        );
        snapshot
    }

    /// Runs the shared application-open orchestration through the current
    /// BrowserCapsule boundary and retains the capsule for lifecycle ownership.
    ///
    /// Current behavior is intentionally fail-closed at renderer qualification:
    /// BrowserCapsule may validate proxy/policy and accept the navigation intent,
    /// but the current ServoEmbeddingAdapter does not yet prove a real renderer
    /// instance or committed page navigation. Therefore this method cannot return
    /// `Open` until that proof is implemented and requalified.
    pub fn open_application(
        &mut self,
        profile: &ClientConfigProfile,
        target_url: &str,
        transport_state: TransportState,
        protected_proxy: Option<LocalProxyEndpoint>,
    ) -> ApplicationSessionSnapshot {
        let session_id = self.next_session_id();
        let target_url = target_url.to_string();
        let mut transitions = vec![ApplicationSessionState::Requested];

        transitions.push(ApplicationSessionState::Authorizing);
        let internal_broker_token = format!("wg-internal-broker-{}", self.next_sequence);
        let gate = BrokerSecurityGate::new(
            vec![
                BrokerCapability::NavigateService,
                BrokerCapability::QueryDeviceStatus,
                BrokerCapability::CloseCapsule,
            ],
            internal_broker_token.clone(),
        );
        let nav_req = BrokerRequest {
            request_id: format!("req-open-{}", self.next_sequence),
            session_token: internal_broker_token,
            payload: BrokerRequestPayload::Navigate {
                target_url: target_url.clone(),
            },
        };

        if let Err(error) = gate.verify_request(&nav_req) {
            return self.insert_terminal(
                session_id,
                target_url,
                ApplicationSessionState::Denied,
                format!("navigation capability denied: {error:?}"),
                transitions,
            );
        }

        let transport_usable = matches!(
            transport_state,
            TransportState::Ready | TransportState::Degraded
        ) && protected_proxy.is_some();
        if !transport_usable {
            return self.insert_terminal(
                session_id,
                target_url,
                ApplicationSessionState::Offline,
                "protected transport is not ready",
                transitions,
            );
        }
        transitions.push(ApplicationSessionState::TransportReady);

        let Some(proxy) = protected_proxy else {
            return self.insert_terminal(
                session_id,
                target_url,
                ApplicationSessionState::Offline,
                "protected transport is missing its verified loopback proxy",
                transitions,
            );
        };
        let policy = profile.build_navigation_policy();
        let mut capsule = BrowserCapsule::new(BrowserKind::Servo, policy);

        transitions.push(ApplicationSessionState::StartingProtectedBrowser);
        if let Err(error) = capsule.attach_proxy(proxy.socket_addr()) {
            return self.insert_terminal(
                session_id,
                target_url,
                ApplicationSessionState::Failed,
                format!("protected browser proxy attachment failed: {error:?}"),
                transitions,
            );
        }

        if let Err(error) = capsule.start() {
            return self.insert_terminal(
                session_id,
                target_url,
                ApplicationSessionState::Failed,
                format!("protected browser start failed: {error:?}"),
                transitions,
            );
        }

        transitions.push(ApplicationSessionState::Navigating);
        if let Err(error) = capsule.navigate(&target_url) {
            let state = match error {
                CapsuleError::NavigationPolicyViolation(_) => ApplicationSessionState::Denied,
                _ => ApplicationSessionState::Failed,
            };
            return self.insert_terminal(
                session_id,
                target_url,
                state,
                format!("protected browser navigation failed: {error:?}"),
                transitions,
            );
        }

        // IMPORTANT: current ServoEmbeddingAdapter is a contract stub. Its
        // initialize/load_url methods do not yet own a real Servo engine/webview
        // or provide a renderer/navigation-commit proof. Retain the capsule for
        // lifecycle ownership, but never claim Open.
        transitions.push(ApplicationSessionState::RendererUnqualified);
        let snapshot = ApplicationSessionSnapshot {
            session_id: session_id.clone(),
            target_url,
            state: ApplicationSessionState::RendererUnqualified,
            message: "BrowserCapsule accepted proxy and navigation intent, but the embedded renderer is not production-qualified; protected Open is blocked".to_string(),
            transitions,
        };
        self.sessions.insert(
            session_id,
            ActiveSession {
                snapshot: snapshot.clone(),
                capsule: Some(capsule),
            },
        );
        snapshot
    }

    #[cfg(test)]
    #[must_use]
    pub fn get(&self, session_id: &str) -> Option<ApplicationSessionSnapshot> {
        self.sessions.get(session_id).map(|s| s.snapshot.clone())
    }

    pub fn close(&mut self, session_id: &str) -> Option<ApplicationSessionSnapshot> {
        let active = self.sessions.get_mut(session_id)?;
        if let Some(capsule) = &mut active.capsule {
            capsule.shutdown();
        }
        active.capsule = None;
        active.snapshot.state = ApplicationSessionState::Closed;
        active.snapshot.transitions.push(ApplicationSessionState::Closed);
        active.snapshot.message = "protected browser session closed".to_string();
        Some(active.snapshot.clone())
    }

    #[cfg(test)]
    #[must_use]
    pub fn active_capsule_count(&self) -> usize {
        self.sessions
            .values()
            .filter(|session| session.capsule.is_some())
            .count()
    }
}

#[cfg(test)]
#[allow(clippy::unwrap_used, clippy::panic)]
mod tests {
    use super::*;
    use std::net::{IpAddr, Ipv4Addr};

    fn test_proxy() -> LocalProxyEndpoint {
        LocalProxyEndpoint::new(IpAddr::V4(Ipv4Addr::LOCALHOST), 43117).unwrap()
    }

    #[test]
    fn offline_transport_never_starts_a_capsule() {
        let profile = ClientConfigProfile::default();
        let mut manager = ApplicationSessionManager::new();
        let result = manager.open_application(
            &profile,
            "webgate://service/docs/overview",
            TransportState::Offline,
            None,
        );

        assert_eq!(result.state, ApplicationSessionState::Offline);
        assert_eq!(manager.active_capsule_count(), 0);
        assert_eq!(
            result.transitions,
            vec![
                ApplicationSessionState::Requested,
                ApplicationSessionState::Authorizing,
                ApplicationSessionState::Offline,
            ]
        );
        assert_ne!(result.state, ApplicationSessionState::Open);
    }

    #[test]
    fn current_servo_stub_can_never_report_open() {
        let profile = ClientConfigProfile::default();
        let mut manager = ApplicationSessionManager::new();
        let result = manager.open_application(
            &profile,
            "webgate://service/docs/overview",
            TransportState::Ready,
            Some(test_proxy()),
        );

        assert_eq!(result.state, ApplicationSessionState::RendererUnqualified);
        assert_eq!(manager.active_capsule_count(), 1);
        assert_eq!(
            result.transitions,
            vec![
                ApplicationSessionState::Requested,
                ApplicationSessionState::Authorizing,
                ApplicationSessionState::TransportReady,
                ApplicationSessionState::StartingProtectedBrowser,
                ApplicationSessionState::Navigating,
                ApplicationSessionState::RendererUnqualified,
            ]
        );
        assert_ne!(result.state, ApplicationSessionState::Open);
        assert!(result.message.contains("not production-qualified"));
    }

    #[test]
    fn disallowed_navigation_is_denied_before_open() {
        let profile = ClientConfigProfile::default();
        let mut manager = ApplicationSessionManager::new();
        let result = manager.open_application(
            &profile,
            "file:///etc/passwd",
            TransportState::Ready,
            Some(test_proxy()),
        );

        assert_eq!(result.state, ApplicationSessionState::Denied);
        assert_eq!(manager.active_capsule_count(), 0);
        assert_eq!(result.transitions.last(), Some(&ApplicationSessionState::Denied));
        assert_ne!(result.state, ApplicationSessionState::Open);
    }

    #[test]
    fn session_ids_are_unique_correlation_ids() {
        let profile = ClientConfigProfile::default();
        let mut manager = ApplicationSessionManager::new();
        let first = manager.open_application(
            &profile,
            "webgate://service/docs/overview",
            TransportState::Offline,
            None,
        );
        let second = manager.open_application(
            &profile,
            "webgate://service/docs/overview",
            TransportState::Offline,
            None,
        );

        assert_ne!(first.session_id, second.session_id);
        assert!(first.session_id.starts_with("wgs-"));
        assert!(second.session_id.starts_with("wgs-"));
    }

    #[test]
    fn close_releases_owned_capsule_and_reports_closed() {
        let profile = ClientConfigProfile::default();
        let mut manager = ApplicationSessionManager::new();
        let opened = manager.open_application(
            &profile,
            "webgate://service/docs/overview",
            TransportState::Ready,
            Some(test_proxy()),
        );
        assert_eq!(manager.active_capsule_count(), 1);

        let closed = manager.close(&opened.session_id).unwrap();
        assert_eq!(closed.state, ApplicationSessionState::Closed);
        assert_eq!(closed.transitions.last(), Some(&ApplicationSessionState::Closed));
        assert_eq!(manager.active_capsule_count(), 0);
        assert_eq!(
            manager.get(&opened.session_id).unwrap().state,
            ApplicationSessionState::Closed
        );
    }
}
