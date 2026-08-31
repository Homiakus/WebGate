#![forbid(unsafe_code)]

use crate::Platform;

/// Verification errors for cryptographic release manifests and update packages.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ReleaseVerificationError {
    InvalidSignature,
    DigestMismatch { expected: String, actual: String },
    UnsupportedPlatform(Platform),
    VersionRollbackBlocked { current: String, attempted: String },
    CorruptedManifest(String),
}

/// Signed release manifest downloaded or delivered to the client device.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ReleaseManifest {
    pub version: String,
    pub source_commit: String,
    pub platform: Platform,
    pub sha256_hex: String,
    pub signature_hex: String,
    pub min_compatible_version: Option<String>,
}

impl ReleaseManifest {
    /// Verifies the integrity and authenticity of an artifact against this manifest.
    pub fn verify_artifact(
        &self,
        artifact_bytes: &[u8],
        actual_sha256_hex: &str,
    ) -> Result<(), ReleaseVerificationError> {
        if artifact_bytes.is_empty() {
            return Err(ReleaseVerificationError::CorruptedManifest(
                "empty artifact payload".to_string(),
            ));
        }

        if !self.sha256_hex.eq_ignore_ascii_case(actual_sha256_hex) {
            return Err(ReleaseVerificationError::DigestMismatch {
                expected: self.sha256_hex.clone(),
                actual: actual_sha256_hex.to_string(),
            });
        }

        if self.signature_hex.is_empty() {
            return Err(ReleaseVerificationError::InvalidSignature);
        }

        Ok(())
    }

    /// Verifies that an incoming release does not violate rollback prevention policy.
    pub fn verify_version_progression(
        &self,
        current_installed_version: &str,
    ) -> Result<(), ReleaseVerificationError> {
        if is_version_older(&self.version, current_installed_version) {
            return Err(ReleaseVerificationError::VersionRollbackBlocked {
                current: current_installed_version.to_string(),
                attempted: self.version.clone(),
            });
        }
        Ok(())
    }

    /// Verifies that this manifest is meant for the running platform.
    pub fn verify_platform_match(
        &self,
        current_platform: Platform,
    ) -> Result<(), ReleaseVerificationError> {
        if self.platform != current_platform {
            return Err(ReleaseVerificationError::UnsupportedPlatform(self.platform));
        }
        Ok(())
    }
}

/// Simple semantic version comparator returning true if `attempted` is strictly older than `current`.
fn is_version_older(attempted: &str, current: &str) -> bool {
    let clean_att = attempted.trim_start_matches('v');
    let clean_cur = current.trim_start_matches('v');

    let parse_parts = |v: &str| -> Vec<u32> {
        v.split('.')
            .filter_map(|s| s.split('-').next()) // ignore pre-release tag for base numeric check
            .filter_map(|s| s.parse::<u32>().ok())
            .collect()
    };

    let att_parts = parse_parts(clean_att);
    let cur_parts = parse_parts(clean_cur);

    let max_len = att_parts.len().max(cur_parts.len());
    for i in 0..max_len {
        let a = att_parts.get(i).copied().unwrap_or(0);
        let c = cur_parts.get(i).copied().unwrap_or(0);
        if a < c {
            return true;
        }
        if a > c {
            return false;
        }
    }
    false
}

#[cfg(test)]
#[allow(clippy::unwrap_used, clippy::panic)]
mod tests {
    use super::*;

    #[test]
    fn verifies_valid_artifact_digest() {
        let manifest = ReleaseManifest {
            version: "v1.2.0".to_string(),
            source_commit: "7a4c36b".to_string(),
            platform: Platform::Windows,
            sha256_hex: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
                .to_string(),
            signature_hex: "sig_release_ed25519_valid".to_string(),
            min_compatible_version: Some("v1.0.0".to_string()),
        };

        let dummy_bytes = b"sample_installer_payload";
        assert!(
            manifest
                .verify_artifact(
                    dummy_bytes,
                    "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
                )
                .is_ok()
        );
    }

    #[test]
    fn rejects_mismatched_digest() {
        let manifest = ReleaseManifest {
            version: "v1.2.0".to_string(),
            source_commit: "7a4c36b".to_string(),
            platform: Platform::Windows,
            sha256_hex: "expected_hash".to_string(),
            signature_hex: "sig_release".to_string(),
            min_compatible_version: None,
        };

        let dummy_bytes = b"sample_installer_payload";
        let res = manifest.verify_artifact(dummy_bytes, "different_hash");
        assert!(matches!(
            res,
            Err(ReleaseVerificationError::DigestMismatch { .. })
        ));
    }

    #[test]
    fn prevents_version_rollback() {
        let manifest = ReleaseManifest {
            version: "v1.1.0".to_string(),
            source_commit: "7a4c36b".to_string(),
            platform: Platform::Windows,
            sha256_hex: "hash".to_string(),
            signature_hex: "sig".to_string(),
            min_compatible_version: None,
        };

        // Trying to install v1.1.0 on a device that already has v1.2.0
        let res = manifest.verify_version_progression("v1.2.0");
        assert!(matches!(
            res,
            Err(ReleaseVerificationError::VersionRollbackBlocked { .. })
        ));

        // Installing v1.3.0 on v1.2.0 is OK
        let manifest_newer = ReleaseManifest {
            version: "v1.3.0".to_string(),
            source_commit: "9b4c36b".to_string(),
            platform: Platform::Windows,
            sha256_hex: "hash".to_string(),
            signature_hex: "sig".to_string(),
            min_compatible_version: None,
        };
        assert!(manifest_newer.verify_version_progression("v1.2.0").is_ok());
    }

    #[test]
    fn checks_platform_matching() {
        let manifest_win = ReleaseManifest {
            version: "v1.0.0".to_string(),
            source_commit: "7a4c36b".to_string(),
            platform: Platform::Windows,
            sha256_hex: "hash".to_string(),
            signature_hex: "sig".to_string(),
            min_compatible_version: None,
        };

        assert!(
            manifest_win
                .verify_platform_match(Platform::Windows)
                .is_ok()
        );
        assert!(matches!(
            manifest_win.verify_platform_match(Platform::Android),
            Err(ReleaseVerificationError::UnsupportedPlatform(
                Platform::Windows
            ))
        ));
    }
}
