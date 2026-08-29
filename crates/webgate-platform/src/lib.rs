#![forbid(unsafe_code)]

use webgate_core::Platform;

/// Platform hooks exposed to the application without leaking OS-native types.
pub trait PlatformRuntime {
    fn platform(&self) -> Platform;
}

/// Compile-time platform selection used before OS adapters exist.
#[must_use]
pub const fn current_platform() -> Platform {
    Platform::current()
}

#[cfg(test)]
mod tests {
    use super::current_platform;
    use webgate_core::Platform;

    #[test]
    fn platform_adapter_uses_shared_core_identity() {
        assert_eq!(current_platform(), Platform::current());
    }
}
