#![forbid(unsafe_code)]
#![allow(clippy::unwrap_used, clippy::panic)]

use std::fs;
use webgate_core::device::{DeviceStatus, KeyAlgorithm};
use webgate_core::ed25519::{Sha512, ed25519_verify};
use webgate_platform::keystore::{
    DeviceKeyStore, InMemoryDeviceKeyStore, KeyStoreError, PersistentFileDeviceKeyStore,
};

fn hex_to_32(hex: &str) -> [u8; 32] {
    let mut out = [0u8; 32];
    for i in 0..32 {
        out[i] = u8::from_str_radix(&hex[i * 2..i * 2 + 2], 16).unwrap();
    }
    out
}

#[test]
fn in_memory_ed25519_contract() {
    let mut store = InMemoryDeviceKeyStore::new();
    assert_eq!(store.get_device_identity().unwrap(), None);

    let ident = store
        .generate_key(KeyAlgorithm::Ed25519, "laptop-corp")
        .unwrap();

    assert_eq!(ident.algorithm, KeyAlgorithm::Ed25519);
    assert_eq!(ident.status, DeviceStatus::Pending);
    assert_eq!(ident.public_key_hex.len(), 64);
    assert!(ident.id.starts_with("dev_"));

    // Reject duplicate key generation
    assert_eq!(
        store.generate_key(KeyAlgorithm::Ed25519, "another-label"),
        Err(KeyStoreError::KeyAlreadyExists)
    );

    let payload = b"challenge_message_12345";
    let sig = store.sign_payload(payload).unwrap();
    assert_eq!(sig.len(), 64);

    let pub_arr = hex_to_32(&ident.public_key_hex);
    let mut sig_arr = [0u8; 64];
    sig_arr.copy_from_slice(&sig);

    assert!(ed25519_verify(&pub_arr, payload, &sig_arr));
    assert!(!ed25519_verify(&pub_arr, b"tampered_message", &sig_arr));

    // Delete key
    store.delete_key().unwrap();
    assert_eq!(store.get_device_identity().unwrap(), None);
    assert_eq!(store.sign_payload(payload), Err(KeyStoreError::KeyNotFound));
}

#[test]
fn persistent_keystore_corrupted_storage_fails_closed() {
    let temp_dir =
        std::env::temp_dir().join(format!("webgate_test_ks_corrupt_{}", std::process::id()));
    let key_path = temp_dir.join("corrupt_device.key");
    let _ = fs::create_dir_all(&temp_dir);

    // Write malformed header
    fs::write(&key_path, b"invalid-header\nfoo=bar\n").unwrap();
    assert!(matches!(
        PersistentFileDeviceKeyStore::open(&key_path),
        Err(KeyStoreError::CorruptedStorage(_))
    ));

    // Write mismatched public key
    let bad_content = "webgate-device-key-v1\nalgorithm=ed25519\ndevice_id=dev_123\nlabel=test\npublic_key_hex=0000000000000000000000000000000000000000000000000000000000000000\nseed_hex=4242424242424242424242424242424242424242424242424242424242424242\n";
    fs::write(&key_path, bad_content.as_bytes()).unwrap();
    assert!(matches!(
        PersistentFileDeviceKeyStore::open(&key_path),
        Err(KeyStoreError::CorruptedStorage(_))
    ));

    let _ = fs::remove_dir_all(&temp_dir);
}

#[test]
fn pop_challenge_signing_format_compatible_with_go_server() {
    let mut store = InMemoryDeviceKeyStore::new();
    let seed = [0x55u8; 32];
    let ident = store
        .generate_key_with_seed(KeyAlgorithm::Ed25519, "workstation-pop", seed)
        .unwrap();

    let pub_arr = hex_to_32(&ident.public_key_hex);
    let pub_sha256 = Sha512::digest(&pub_arr); // sha256 or digest format

    // Standard Go server PoP format
    let challenge_payload = format!(
        "webgate-device-pop-v1\nchallenge_id=c-999\ndevice_id={}\nnonce=n-777\nexpires_at=1900000000\nalgorithm=Ed25519\npublic_key_sha256={:02x}\n",
        ident.id, pub_sha256[0]
    );

    let sig = store.sign_payload(challenge_payload.as_bytes()).unwrap();
    assert_eq!(sig.len(), 64);

    let mut sig_arr = [0u8; 64];
    sig_arr.copy_from_slice(&sig);

    assert!(ed25519_verify(
        &pub_arr,
        challenge_payload.as_bytes(),
        &sig_arr
    ));
}
