#![forbid(unsafe_code)]

use crate::policy::ValidatedUrl;

/// Scoped capabilities granted to an isolated browser capsule instance.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum BrokerCapability {
    NavigateService,
    QueryDeviceStatus,
    RefreshSession,
    ReportError,
    CloseCapsule,
}

/// Errors returned by the trusted broker to the isolated browser capsule.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum BrokerError {
    UnauthorizedCapability(String),
    SessionExpired,
    ServiceNotFound(String),
    NavigationBlocked(String),
    InvalidPayload(String),
    InternalBrokerError(String),
}

/// Incoming semantic request payload from the browser capsule.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum BrokerRequestPayload {
    Navigate { target_url: String },
    GetDeviceStatus,
    RefreshSession,
    CloseCapsule,
    ReportTelemetry { event: String },
}

/// Outgoing semantic response payload to the browser capsule.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum BrokerResponsePayload {
    NavigationApproved { validated_url: ValidatedUrl },
    DeviceStatus { is_active: bool, device_id: String },
    SessionRefreshed { expires_at_epoch_sec: u64 },
    Ack,
}

/// A versioned, bounded semantic message sent from the capsule to the trusted broker.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct BrokerRequest {
    pub request_id: String,
    pub session_token: String,
    pub payload: BrokerRequestPayload,
}

/// A structured response from the trusted broker back to the capsule.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct BrokerResponse {
    pub request_id: String,
    pub result: Result<BrokerResponsePayload, BrokerError>,
}

/// Capability security gate that enforces principle of least privilege on capsule requests.
#[derive(Debug, Clone)]
pub struct BrokerSecurityGate {
    granted_capabilities: Vec<BrokerCapability>,
    allowed_session_token: String,
    is_session_active: bool,
}

impl BrokerSecurityGate {
    #[must_use]
    pub fn new(granted_capabilities: Vec<BrokerCapability>, session_token: String) -> Self {
        Self {
            granted_capabilities,
            allowed_session_token: session_token,
            is_session_active: true,
        }
    }

    pub fn set_session_active(&mut self, active: bool) {
        self.is_session_active = active;
    }

    /// Verifies if a given request is permitted by the granted capabilities and active session.
    pub fn verify_request(&self, request: &BrokerRequest) -> Result<BrokerCapability, BrokerError> {
        if !self.is_session_active {
            return Err(BrokerError::SessionExpired);
        }

        if request.session_token != self.allowed_session_token {
            return Err(BrokerError::UnauthorizedCapability(
                "invalid session token".to_string(),
            ));
        }

        let required_cap = match &request.payload {
            BrokerRequestPayload::Navigate { .. } => BrokerCapability::NavigateService,
            BrokerRequestPayload::GetDeviceStatus => BrokerCapability::QueryDeviceStatus,
            BrokerRequestPayload::RefreshSession => BrokerCapability::RefreshSession,
            BrokerRequestPayload::CloseCapsule => BrokerCapability::CloseCapsule,
            BrokerRequestPayload::ReportTelemetry { .. } => BrokerCapability::ReportError,
        };

        if self.granted_capabilities.contains(&required_cap) {
            Ok(required_cap)
        } else {
            Err(BrokerError::UnauthorizedCapability(format!(
                "{required_cap:?} not granted to capsule"
            )))
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn allows_permitted_navigation_request() {
        let gate = BrokerSecurityGate::new(
            vec![
                BrokerCapability::NavigateService,
                BrokerCapability::QueryDeviceStatus,
            ],
            "sess_token_123".to_string(),
        );

        let request = BrokerRequest {
            request_id: "req_1".to_string(),
            session_token: "sess_token_123".to_string(),
            payload: BrokerRequestPayload::Navigate {
                target_url: "webgate://service/docs".to_string(),
            },
        };

        assert_eq!(
            gate.verify_request(&request),
            Ok(BrokerCapability::NavigateService)
        );
    }

    #[test]
    fn rejects_unauthorized_capability() {
        let gate = BrokerSecurityGate::new(
            vec![BrokerCapability::NavigateService],
            "sess_token_123".to_string(),
        );

        let request = BrokerRequest {
            request_id: "req_2".to_string(),
            session_token: "sess_token_123".to_string(),
            payload: BrokerRequestPayload::RefreshSession,
        };

        assert!(matches!(
            gate.verify_request(&request),
            Err(BrokerError::UnauthorizedCapability(_))
        ));
    }

    #[test]
    fn rejects_inactive_session() {
        let mut gate = BrokerSecurityGate::new(
            vec![BrokerCapability::NavigateService],
            "sess_token_123".to_string(),
        );
        gate.set_session_active(false);

        let request = BrokerRequest {
            request_id: "req_3".to_string(),
            session_token: "sess_token_123".to_string(),
            payload: BrokerRequestPayload::Navigate {
                target_url: "webgate://service/docs".to_string(),
            },
        };

        assert_eq!(
            gate.verify_request(&request),
            Err(BrokerError::SessionExpired)
        );
    }
}
