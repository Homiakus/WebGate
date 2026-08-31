#![forbid(unsafe_code)]

/// Supported cryptographic algorithms for device key identity (algorithm-agile).
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum KeyAlgorithm {
    Ed25519,
    P256,
}

impl KeyAlgorithm {
    #[must_use]
    pub const fn as_str(self) -> &'static str {
        match self {
            Self::Ed25519 => "ed25519",
            Self::P256 => "p256",
        }
    }

    pub fn from_str_name(name: &str) -> Option<Self> {
        match name.to_ascii_lowercase().as_str() {
            "ed25519" => Some(Self::Ed25519),
            "p256" | "ecdsa_p256" => Some(Self::P256),
            _ => None,
        }
    }
}

/// Lifecycle state of a registered or enrolling client device.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum DeviceStatus {
    Pending,
    Active,
    Suspended,
    Revoked,
}

impl DeviceStatus {
    #[must_use]
    pub const fn is_allowed_access(self) -> bool {
        matches!(self, Self::Active)
    }

    #[must_use]
    pub const fn as_str(self) -> &'static str {
        match self {
            Self::Pending => "PENDING",
            Self::Active => "ACTIVE",
            Self::Suspended => "SUSPENDED",
            Self::Revoked => "REVOKED",
        }
    }
}

/// Unique identifier and metadata of a registered device.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DeviceIdentity {
    pub id: String,
    pub public_key_hex: String,
    pub algorithm: KeyAlgorithm,
    pub label: String,
    pub status: DeviceStatus,
}

impl DeviceIdentity {
    #[must_use]
    pub fn new(
        id: String,
        public_key_hex: String,
        algorithm: KeyAlgorithm,
        label: String,
        status: DeviceStatus,
    ) -> Self {
        Self {
            id,
            public_key_hex,
            algorithm,
            label,
            status,
        }
    }
}

/// Server challenge for proof-of-possession (PoP) authentication.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DeviceChallenge {
    pub challenge_id: String,
    pub nonce_hex: String,
    pub issued_at_epoch_sec: u64,
    pub expires_at_epoch_sec: u64,
}

impl DeviceChallenge {
    #[must_use]
    pub fn is_expired(&self, current_epoch_sec: u64) -> bool {
        current_epoch_sec > self.expires_at_epoch_sec
    }
}

/// Proof-of-Possession response signed by the device's hardware/secure keystore.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ProofOfPossession {
    pub device_id: String,
    pub challenge_id: String,
    pub signature_hex: String,
    pub algorithm: KeyAlgorithm,
    pub client_timestamp_epoch_sec: u64,
}

impl ProofOfPossession {
    #[must_use]
    pub fn new(
        device_id: String,
        challenge_id: String,
        signature_hex: String,
        algorithm: KeyAlgorithm,
        client_timestamp_epoch_sec: u64,
    ) -> Self {
        Self {
            device_id,
            challenge_id,
            signature_hex,
            algorithm,
            client_timestamp_epoch_sec,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn algorithm_string_mapping() {
        assert_eq!(KeyAlgorithm::Ed25519.as_str(), "ed25519");
        assert_eq!(KeyAlgorithm::P256.as_str(), "p256");
        assert_eq!(
            KeyAlgorithm::from_str_name("ED25519"),
            Some(KeyAlgorithm::Ed25519)
        );
        assert_eq!(
            KeyAlgorithm::from_str_name("p256"),
            Some(KeyAlgorithm::P256)
        );
        assert_eq!(KeyAlgorithm::from_str_name("rsa"), None);
    }

    #[test]
    fn device_status_access_check() {
        assert!(DeviceStatus::Active.is_allowed_access());
        assert!(!DeviceStatus::Pending.is_allowed_access());
        assert!(!DeviceStatus::Suspended.is_allowed_access());
        assert!(!DeviceStatus::Revoked.is_allowed_access());
    }

    #[test]
    fn challenge_expiry_evaluation() {
        let challenge = DeviceChallenge {
            challenge_id: "chal_123".to_string(),
            nonce_hex: "abcd1234".to_string(),
            issued_at_epoch_sec: 1000,
            expires_at_epoch_sec: 1060,
        };

        assert!(!challenge.is_expired(1030));
        assert!(!challenge.is_expired(1060));
        assert!(challenge.is_expired(1061));
    }
}
