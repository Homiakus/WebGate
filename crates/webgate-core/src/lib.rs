#![forbid(unsafe_code)]

pub mod broker;
pub mod config;
pub mod device;
pub mod policy;
pub mod release;

/// Operating-system families supported by WebGate's portable core.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum Platform {
    Windows,
    Android,
    Linux,
    MacOs,
    Other,
}

impl Platform {
    /// Returns the platform selected at compile time without calling platform APIs.
    #[must_use]
    pub const fn current() -> Self {
        if cfg!(target_os = "windows") {
            Self::Windows
        } else if cfg!(target_os = "android") {
            Self::Android
        } else if cfg!(target_os = "linux") {
            Self::Linux
        } else if cfg!(target_os = "macos") {
            Self::MacOs
        } else {
            Self::Other
        }
    }
}

/// High-level capability boundaries. Platform adapters implement capabilities;
/// the portable core never imports operating-system implementations.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum Capability {
    Browser,
    Transport,
    Platform,
}

#[cfg(test)]
mod tests {
    use super::Platform;

    #[test]
    fn current_platform_matches_target_cfg() {
        #[cfg(target_os = "windows")]
        assert_eq!(Platform::current(), Platform::Windows);

        #[cfg(target_os = "android")]
        assert_eq!(Platform::current(), Platform::Android);

        #[cfg(target_os = "linux")]
        assert_eq!(Platform::current(), Platform::Linux);

        #[cfg(target_os = "macos")]
        assert_eq!(Platform::current(), Platform::MacOs);
    }
}
