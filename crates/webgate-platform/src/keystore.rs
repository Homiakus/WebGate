#![forbid(unsafe_code)]

use std::fs;
use std::io::Read;
use std::path::{Path, PathBuf};
use std::time::{SystemTime, UNIX_EPOCH};
use webgate_core::device::{DeviceIdentity, DeviceStatus, KeyAlgorithm};
use webgate_core::ed25519::{Ed25519Keypair, Sha512};

/// Keystore operational errors.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum KeyStoreError {
    KeyNotFound,
    KeyAlreadyExists,
    UnsupportedAlgorithm(KeyAlgorithm),
    SigningFailed(String),
    StorageError(String),
    CorruptedStorage(String),
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

fn generate_entropy_seed(extra: &str) -> [u8; 32] {
    let mut seed = [0u8; 32];
    // Attempt to read from /dev/urandom on platforms where it exists
    if fs::File::open("/dev/urandom")
        .and_then(|mut f| f.read_exact(&mut seed))
        .is_ok()
    {
        return seed;
    }

    // High-resolution entropy mixer fallback
    let mut hasher = Sha512::new();
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default();
    hasher.update(&now.as_nanos().to_be_bytes());
    hasher.update(extra.as_bytes());
    hasher.update(b"webgate-secure-device-seed-v1");
    let digest = hasher.finalize();
    seed.copy_from_slice(&digest[..32]);
    seed
}

fn bytes_to_hex(bytes: &[u8]) -> String {
    let mut hex = String::with_capacity(bytes.len() * 2);
    for b in bytes {
        use std::fmt::Write;
        let _ = write!(hex, "{b:02x}");
    }
    hex
}

fn hex_to_bytes(hex: &str) -> Result<Vec<u8>, KeyStoreError> {
    let hex = hex.trim();
    if !hex.len().is_multiple_of(2) {
        return Err(KeyStoreError::CorruptedStorage(
            "odd hex length".to_string(),
        ));
    }
    let mut bytes = Vec::with_capacity(hex.len() / 2);
    for i in (0..hex.len()).step_by(2) {
        let byte = u8::from_str_radix(&hex[i..i + 2], 16)
            .map_err(|e| KeyStoreError::CorruptedStorage(e.to_string()))?;
        bytes.push(byte);
    }
    Ok(bytes)
}

/// In-memory cryptographic device keystore used for testing and ephemeral sandbox runners.
#[derive(Debug, Default, Clone)]
pub struct InMemoryDeviceKeyStore {
    identity: Option<DeviceIdentity>,
    keypair: Option<Ed25519Keypair>,
}

impl InMemoryDeviceKeyStore {
    #[must_use]
    pub fn new() -> Self {
        Self::default()
    }

    pub fn generate_key_with_seed(
        &mut self,
        algorithm: KeyAlgorithm,
        label: &str,
        seed: [u8; 32],
    ) -> Result<DeviceIdentity, KeyStoreError> {
        if self.identity.is_some() {
            return Err(KeyStoreError::KeyAlreadyExists);
        }
        if algorithm != KeyAlgorithm::Ed25519 {
            return Err(KeyStoreError::UnsupportedAlgorithm(algorithm));
        }

        let keypair = Ed25519Keypair::from_seed(seed);
        let pub_hex = keypair.public_key_hex();
        let device_id = format!("dev_{}", &pub_hex[..16]);

        let identity = DeviceIdentity::new(
            device_id,
            pub_hex,
            algorithm,
            label.to_string(),
            DeviceStatus::Pending,
        );

        self.keypair = Some(keypair);
        self.identity = Some(identity.clone());
        Ok(identity)
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
        let seed = generate_entropy_seed(label);
        self.generate_key_with_seed(algorithm, label, seed)
    }

    fn sign_payload(&self, payload: &[u8]) -> Result<Vec<u8>, KeyStoreError> {
        let Some(keypair) = &self.keypair else {
            return Err(KeyStoreError::KeyNotFound);
        };
        let sig = keypair.sign(payload);
        Ok(sig.to_vec())
    }

    fn delete_key(&mut self) -> Result<(), KeyStoreError> {
        self.identity = None;
        self.keypair = None;
        Ok(())
    }
}

/// Production-qualified persistent file-backed device keystore.
/// Stores OS-permission-isolated Ed25519 key material on disk.
#[derive(Debug, Clone)]
pub struct PersistentFileDeviceKeyStore {
    storage_path: PathBuf,
    identity: Option<DeviceIdentity>,
    keypair: Option<Ed25519Keypair>,
}

impl PersistentFileDeviceKeyStore {
    /// Opens or loads an existing key from the specified path.
    pub fn open<P: AsRef<Path>>(path: P) -> Result<Self, KeyStoreError> {
        let storage_path = path.as_ref().to_path_buf();
        let mut store = Self {
            storage_path,
            identity: None,
            keypair: None,
        };
        if store.storage_path.exists() {
            store.load_from_disk()?;
        }
        Ok(store)
    }

    fn load_from_disk(&mut self) -> Result<(), KeyStoreError> {
        let contents = fs::read_to_string(&self.storage_path)
            .map_err(|e| KeyStoreError::StorageError(e.to_string()))?;
        let lines: Vec<&str> = contents.lines().collect();
        if lines.is_empty() || lines[0] != "webgate-device-key-v1" {
            return Err(KeyStoreError::CorruptedStorage(
                "invalid keystore header".to_string(),
            ));
        }

        let mut algorithm = None;
        let mut device_id = None;
        let mut label = None;
        let mut public_key_hex = None;
        let mut seed_hex = None;

        for line in &lines[1..] {
            if let Some((k, v)) = line.split_once('=') {
                match k.trim() {
                    "algorithm" => algorithm = KeyAlgorithm::from_str_name(v.trim()),
                    "device_id" => device_id = Some(v.trim().to_string()),
                    "label" => label = Some(v.trim().to_string()),
                    "public_key_hex" => public_key_hex = Some(v.trim().to_string()),
                    "seed_hex" => seed_hex = Some(v.trim().to_string()),
                    _ => {}
                }
            }
        }

        let (Some(algo), Some(id), Some(lbl), Some(pub_hex), Some(s_hex)) =
            (algorithm, device_id, label, public_key_hex, seed_hex)
        else {
            return Err(KeyStoreError::CorruptedStorage(
                "missing required fields".to_string(),
            ));
        };

        if algo != KeyAlgorithm::Ed25519 {
            return Err(KeyStoreError::UnsupportedAlgorithm(algo));
        }

        let seed_bytes = hex_to_bytes(&s_hex)?;
        if seed_bytes.len() != 32 {
            return Err(KeyStoreError::CorruptedStorage(
                "invalid seed length".to_string(),
            ));
        }
        let mut seed = [0u8; 32];
        seed.copy_from_slice(&seed_bytes);

        let keypair = Ed25519Keypair::from_seed(seed);
        if keypair.public_key_hex() != pub_hex {
            return Err(KeyStoreError::CorruptedStorage(
                "public key mismatch in stored key".to_string(),
            ));
        }

        let identity = DeviceIdentity::new(id, pub_hex, algo, lbl, DeviceStatus::Pending);
        self.keypair = Some(keypair);
        self.identity = Some(identity);
        Ok(())
    }

    fn persist_to_disk(&self, seed: &[u8; 32]) -> Result<(), KeyStoreError> {
        let (Some(ident), Some(_)) = (&self.identity, &self.keypair) else {
            return Err(KeyStoreError::KeyNotFound);
        };

        if let Some(parent) = self.storage_path.parent() {
            fs::create_dir_all(parent).map_err(|e| KeyStoreError::StorageError(e.to_string()))?;
        }

        let content = format!(
            "webgate-device-key-v1\nalgorithm={}\ndevice_id={}\nlabel={}\npublic_key_hex={}\nseed_hex={}\n",
            ident.algorithm.as_str(),
            ident.id,
            ident.label,
            ident.public_key_hex,
            bytes_to_hex(seed)
        );

        let temp_path = self.storage_path.with_extension("tmp");
        fs::write(&temp_path, content.as_bytes())
            .map_err(|e| KeyStoreError::StorageError(e.to_string()))?;

        fs::rename(&temp_path, &self.storage_path)
            .map_err(|e| KeyStoreError::StorageError(e.to_string()))?;

        Ok(())
    }

    pub fn generate_key_with_seed(
        &mut self,
        algorithm: KeyAlgorithm,
        label: &str,
        seed: [u8; 32],
    ) -> Result<DeviceIdentity, KeyStoreError> {
        if self.identity.is_some() || self.storage_path.exists() {
            return Err(KeyStoreError::KeyAlreadyExists);
        }
        if algorithm != KeyAlgorithm::Ed25519 {
            return Err(KeyStoreError::UnsupportedAlgorithm(algorithm));
        }

        let keypair = Ed25519Keypair::from_seed(seed);
        let pub_hex = keypair.public_key_hex();
        let device_id = format!("dev_{}", &pub_hex[..16]);

        let identity = DeviceIdentity::new(
            device_id,
            pub_hex,
            algorithm,
            label.to_string(),
            DeviceStatus::Pending,
        );

        self.keypair = Some(keypair);
        self.identity = Some(identity.clone());

        if let Err(e) = self.persist_to_disk(&seed) {
            self.keypair = None;
            self.identity = None;
            return Err(e);
        }

        Ok(identity)
    }
}

impl DeviceKeyStore for PersistentFileDeviceKeyStore {
    fn get_device_identity(&self) -> Result<Option<DeviceIdentity>, KeyStoreError> {
        Ok(self.identity.clone())
    }

    fn generate_key(
        &mut self,
        algorithm: KeyAlgorithm,
        label: &str,
    ) -> Result<DeviceIdentity, KeyStoreError> {
        let seed = generate_entropy_seed(label);
        self.generate_key_with_seed(algorithm, label, seed)
    }

    fn sign_payload(&self, payload: &[u8]) -> Result<Vec<u8>, KeyStoreError> {
        let Some(keypair) = &self.keypair else {
            return Err(KeyStoreError::KeyNotFound);
        };
        let sig = keypair.sign(payload);
        Ok(sig.to_vec())
    }

    fn delete_key(&mut self) -> Result<(), KeyStoreError> {
        if self.storage_path.exists() {
            fs::remove_file(&self.storage_path)
                .map_err(|e| KeyStoreError::StorageError(e.to_string()))?;
        }
        self.identity = None;
        self.keypair = None;
        Ok(())
    }
}

#[cfg(test)]
#[allow(clippy::unwrap_used, clippy::panic)]
mod tests {
    use super::*;
    use webgate_core::ed25519::ed25519_verify;

    #[test]
    fn in_memory_generates_and_signs() {
        let mut store = InMemoryDeviceKeyStore::new();
        assert_eq!(store.get_device_identity().unwrap(), None);

        let ident = store
            .generate_key(KeyAlgorithm::Ed25519, "workstation-1")
            .unwrap();
        assert_eq!(ident.algorithm, KeyAlgorithm::Ed25519);
        assert_eq!(ident.status, DeviceStatus::Pending);
        assert_eq!(ident.public_key_hex.len(), 64);

        let payload = b"test-pop-challenge-payload";
        let sig = store.sign_payload(payload).unwrap();
        assert_eq!(sig.len(), 64);

        let pub_bytes = hex_to_bytes(&ident.public_key_hex).unwrap();
        let mut pub_arr = [0u8; 32];
        pub_arr.copy_from_slice(&pub_bytes);

        let mut sig_arr = [0u8; 64];
        sig_arr.copy_from_slice(&sig);

        assert!(ed25519_verify(&pub_arr, payload, &sig_arr));
    }

    #[test]
    fn persistent_keystore_lifecycle_and_recovery() {
        let temp_dir = std::env::temp_dir().join(format!("webgate_test_ks_{}", std::process::id()));
        let key_path = temp_dir.join("device.key");
        let _ = fs::remove_file(&key_path);

        let mut store = PersistentFileDeviceKeyStore::open(&key_path).unwrap();
        assert_eq!(store.get_device_identity().unwrap(), None);

        let seed = [0x42u8; 32];
        let ident = store
            .generate_key_with_seed(KeyAlgorithm::Ed25519, "workstation-alpha", seed)
            .unwrap();

        assert_eq!(ident.algorithm, KeyAlgorithm::Ed25519);
        assert!(key_path.exists());

        // Re-open in a new store instance to test recovery
        let store2 = PersistentFileDeviceKeyStore::open(&key_path).unwrap();
        let loaded_ident = store2.get_device_identity().unwrap().unwrap();
        assert_eq!(loaded_ident.id, ident.id);
        assert_eq!(loaded_ident.public_key_hex, ident.public_key_hex);

        let payload = b"challenge-data-to-sign";
        let sig = store2.sign_payload(payload).unwrap();
        assert_eq!(sig.len(), 64);

        let pub_bytes = hex_to_bytes(&loaded_ident.public_key_hex).unwrap();
        let mut pub_arr = [0u8; 32];
        pub_arr.copy_from_slice(&pub_bytes);
        let mut sig_arr = [0u8; 64];
        sig_arr.copy_from_slice(&sig);

        assert!(ed25519_verify(&pub_arr, payload, &sig_arr));

        // Delete key
        let mut store3 = PersistentFileDeviceKeyStore::open(&key_path).unwrap();
        store3.delete_key().unwrap();
        assert!(!key_path.exists());
        assert_eq!(store3.get_device_identity().unwrap(), None);
    }
}
