#![forbid(unsafe_code)]

use std::collections::HashMap;
use webgate_core::device::{DeviceIdentity, DeviceStatus, KeyAlgorithm};

/// Keystore operational errors.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum KeyStoreError {
    KeyNotFound,
    KeyAlreadyExists,
    UnsupportedAlgorithm(KeyAlgorithm),
    SigningFailed(String),
    StorageError(String),
}

/// Abstract device secure keystore contract (Windows CNG / TPM / Android Keystore / Secure Enclave).
pub trait DeviceKeyStore {
    fn get_device_identity(&self) -> Result<Option<DeviceIdentity>, KeyStoreError>;
    fn generate_key(
        &mut self,
        algorithm: KeyAlgorithm,
        label: &str,
    ) -> Result<DeviceIdentity, KeyStoreError>;
    fn sign_payload(&self, payload: &[u8]) -> Result<Vec<u8>, KeyStoreError>;
    fn delete_key(&mut self) -> Result<(), KeyStoreError>;
}

/// In-memory software device keystore used for testing and portable sandbox runners.
#[derive(Debug, Default, Clone)]
pub struct InMemoryDeviceKeyStore {
    identity: Option<DeviceIdentity>,
    private_key_material: HashMap<String, Vec<u8>>,
}

impl InMemoryDeviceKeyStore {
    #[must_use]
    pub fn new() -> Self {
        Self::default()
    }
}

impl DeviceKeyStore for InMemoryDeviceKeyStore {
    fn get_device_identity(&self) -> Result<Option<DeviceIdentity>, KeyStoreError> {
        Ok(self.identity.clone())
    }

    fn generate_key(
        &mut self,
        algorithm: KeyAlgorithm,
        label: &str,
    ) -> Result<DeviceIdentity, KeyStoreError> {
        if self.identity.is_some() {
            return Err(KeyStoreError::KeyAlreadyExists);
        }

        // Generate synthetic key material for portable software keystore
        let fake_priv = format!("priv_material_{label}_{:?}", algorithm).into_bytes();
        let pub_hex = format!("pubkey_{label}_{:?}", algorithm);
        let device_id = format!("dev_{}", &pub_hex[..pub_hex.len().min(16)]);

        let identity = DeviceIdentity::new(
            device_id.clone(),
            pub_hex,
            algorithm,
            label.to_string(),
            DeviceStatus::Pending,
        );

        self.private_key_material.insert(device_id, fake_priv);
        self.identity = Some(identity.clone());
        Ok(identity)
    }

    fn sign_payload(&self, payload: &[u8]) -> Result<Vec<u8>, KeyStoreError> {
        let Some(ident) = &self.identity else {
            return Err(KeyStoreError::KeyNotFound);
        };

        let Some(priv_bytes) = self.private_key_material.get(&ident.id) else {
            return Err(KeyStoreError::KeyNotFound);
        };

        // Deterministic synthetic HMAC-like signature over payload with priv_bytes
        let mut signature = Vec::new();
        signature.extend_from_slice(b"sig:");
        signature.extend_from_slice(priv_bytes);
        signature.extend_from_slice(b":");
        signature.extend_from_slice(payload);
        Ok(signature)
    }

    fn delete_key(&mut self) -> Result<(), KeyStoreError> {
        if let Some(ident) = self.identity.take() {
            self.private_key_material.remove(&ident.id);
        }
        Ok(())
    }
}

#[cfg(test)]
#[allow(clippy::unwrap_used, clippy::panic)]
mod tests {
    use super::*;

    #[test]
    fn generates_and_stores_key() {
        let mut store = InMemoryDeviceKeyStore::new();
        assert_eq!(store.get_device_identity().unwrap(), None);

        let ident = store
            .generate_key(KeyAlgorithm::Ed25519, "workstation-1")
            .unwrap();
        assert_eq!(ident.algorithm, KeyAlgorithm::Ed25519);
        assert_eq!(ident.status, DeviceStatus::Pending);

        assert_eq!(store.get_device_identity().unwrap(), Some(ident));
    }

    #[test]
    fn rejects_duplicate_key_generation() {
        let mut store = InMemoryDeviceKeyStore::new();
        store.generate_key(KeyAlgorithm::Ed25519, "dev-1").unwrap();
        assert_eq!(
            store.generate_key(KeyAlgorithm::P256, "dev-2"),
            Err(KeyStoreError::KeyAlreadyExists)
        );
    }

    #[test]
    fn signs_payload_and_deletes_key() {
        let mut store = InMemoryDeviceKeyStore::new();
        store.generate_key(KeyAlgorithm::Ed25519, "dev-1").unwrap();

        let sig = store.sign_payload(b"challenge_nonce_12345").unwrap();
        assert!(!sig.is_empty());

        store.delete_key().unwrap();
        assert_eq!(store.get_device_identity().unwrap(), None);
        assert_eq!(store.sign_payload(b"test"), Err(KeyStoreError::KeyNotFound));
    }
}
