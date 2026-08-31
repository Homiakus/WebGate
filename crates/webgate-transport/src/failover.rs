#![forbid(unsafe_code)]

use crate::{LocalProxyEndpoint, TransportProvider, TransportState};

/// Role of a transport in the multi-relay redundancy hierarchy.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum TransportRole {
    Primary,
    Fallback,
}

/// Dynamic health metrics for an active or standby transport channel.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct TransportHealth {
    pub consecutive_failures: u32,
    pub last_latency_ms: u64,
    pub is_responsive: bool,
    pub last_probe_epoch_sec: u64,
}

impl Default for TransportHealth {
    fn default() -> Self {
        Self {
            consecutive_failures: 0,
            last_latency_ms: 0,
            is_responsive: true,
            last_probe_epoch_sec: 0,
        }
    }
}

/// Configuration parameters governing failover triggers and switchback cooldowns.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct FailoverConfig {
    pub max_consecutive_failures: u32,
    pub high_latency_threshold_ms: u64,
    pub switchback_cooldown_sec: u64,
}

impl Default for FailoverConfig {
    fn default() -> Self {
        Self {
            max_consecutive_failures: 3,
            high_latency_threshold_ms: 1500,
            switchback_cooldown_sec: 30,
        }
    }
}

/// Deterministic transport failover coordinator ensuring fail-closed safety.
#[derive(Debug)]
pub struct TransportFailoverController<P: TransportProvider, F: TransportProvider> {
    primary: P,
    fallback: F,
    config: FailoverConfig,
    active_role: Option<TransportRole>,
    primary_health: TransportHealth,
    fallback_health: TransportHealth,
    last_failover_epoch_sec: u64,
}

impl<P: TransportProvider, F: TransportProvider> TransportFailoverController<P, F> {
    #[must_use]
    pub fn new(primary: P, fallback: F, config: FailoverConfig) -> Self {
        Self {
            primary,
            fallback,
            config,
            active_role: None,
            primary_health: TransportHealth::default(),
            fallback_health: TransportHealth::default(),
            last_failover_epoch_sec: 0,
        }
    }

    /// Initializes and starts the transport hierarchy, selecting Primary by default.
    pub fn start(&mut self) -> TransportState {
        self.active_role = Some(TransportRole::Primary);
        self.primary_health.is_responsive = true;
        self.primary_health.consecutive_failures = 0;
        self.primary.state()
    }

    #[must_use]
    pub fn active_role(&self) -> Option<TransportRole> {
        self.active_role
    }

    /// Returns the currently active local proxy endpoint (if any).
    #[must_use]
    pub fn active_proxy_endpoint(&self) -> Option<LocalProxyEndpoint> {
        match self.active_role {
            Some(TransportRole::Primary) => self.primary.local_proxy(),
            Some(TransportRole::Fallback) => self.fallback.local_proxy(),
            None => None,
        }
    }

    /// Returns overall aggregate transport state.
    #[must_use]
    pub fn aggregate_state(&self) -> TransportState {
        match self.active_role {
            Some(TransportRole::Primary) => {
                if self.primary_health.consecutive_failures > 0 {
                    TransportState::Degraded
                } else {
                    self.primary.state()
                }
            }
            Some(TransportRole::Fallback) => {
                if self.fallback_health.consecutive_failures > 0 {
                    TransportState::Degraded
                } else {
                    self.fallback.state()
                }
            }
            None => TransportState::Offline,
        }
    }

    /// Records a probe/traffic observation for the currently active transport.
    pub fn record_observation(&mut self, success: bool, latency_ms: u64, now_epoch_sec: u64) {
        match self.active_role {
            Some(TransportRole::Primary) => {
                self.primary_health.last_probe_epoch_sec = now_epoch_sec;
                self.primary_health.last_latency_ms = latency_ms;
                if success {
                    self.primary_health.consecutive_failures = 0;
                    self.primary_health.is_responsive = true;
                } else {
                    self.primary_health.consecutive_failures =
                        self.primary_health.consecutive_failures.saturating_add(1);
                    if self.primary_health.consecutive_failures
                        >= self.config.max_consecutive_failures
                    {
                        self.primary_health.is_responsive = false;
                        self.trigger_failover_to_fallback(now_epoch_sec);
                    }
                }
            }
            Some(TransportRole::Fallback) => {
                self.fallback_health.last_probe_epoch_sec = now_epoch_sec;
                self.fallback_health.last_latency_ms = latency_ms;
                if success {
                    self.fallback_health.consecutive_failures = 0;
                    self.fallback_health.is_responsive = true;
                } else {
                    self.fallback_health.consecutive_failures =
                        self.fallback_health.consecutive_failures.saturating_add(1);
                    if self.fallback_health.consecutive_failures
                        >= self.config.max_consecutive_failures
                    {
                        self.fallback_health.is_responsive = false;
                        // Both failed -> fail closed
                        self.active_role = None;
                    }
                }
            }
            None => {}
        }
    }

    /// Probes standby primary when operating on fallback; switches back if primary is healthy and cooldown expired.
    pub fn probe_standby_primary(&mut self, probe_successful: bool, now_epoch_sec: u64) -> bool {
        if self.active_role != Some(TransportRole::Fallback) {
            return false;
        }

        let time_since_failover = now_epoch_sec.saturating_sub(self.last_failover_epoch_sec);
        if time_since_failover < self.config.switchback_cooldown_sec {
            return false;
        }

        if probe_successful {
            self.primary_health.consecutive_failures = 0;
            self.primary_health.is_responsive = true;
            self.primary_health.last_probe_epoch_sec = now_epoch_sec;
            self.active_role = Some(TransportRole::Primary);
            true
        } else {
            false
        }
    }

    fn trigger_failover_to_fallback(&mut self, now_epoch_sec: u64) {
        self.active_role = Some(TransportRole::Fallback);
        self.last_failover_epoch_sec = now_epoch_sec;
        self.fallback_health.consecutive_failures = 0;
        self.fallback_health.is_responsive = true;
    }

    /// Gracefully stops all transport providers.
    pub fn shutdown(&mut self) {
        self.active_role = None;
        self.primary.stop();
        self.fallback.stop();
    }
}

#[cfg(test)]
#[allow(clippy::unwrap_used, clippy::panic)]
mod tests {
    use super::*;
    use std::net::{IpAddr, Ipv4Addr};

    #[derive(Debug)]
    struct MockTransport {
        name: &'static str,
        state: TransportState,
        endpoint: Option<LocalProxyEndpoint>,
    }

    impl MockTransport {
        fn new(name: &'static str, port: u16) -> Self {
            let endpoint = LocalProxyEndpoint::new(IpAddr::V4(Ipv4Addr::LOCALHOST), port).ok();
            Self {
                name,
                state: TransportState::Ready,
                endpoint,
            }
        }
    }

    impl TransportProvider for MockTransport {
        fn name(&self) -> &str {
            self.name
        }

        fn state(&self) -> TransportState {
            self.state
        }

        fn local_proxy(&self) -> Option<LocalProxyEndpoint> {
            self.endpoint
        }

        fn stop(&mut self) {
            self.state = TransportState::Stopped;
        }
    }

    #[test]
    fn starts_on_primary_and_returns_primary_proxy() {
        let primary = MockTransport::new("Primary-Relay", 40001);
        let fallback = MockTransport::new("Fallback-Relay", 40002);
        let mut controller =
            TransportFailoverController::new(primary, fallback, FailoverConfig::default());

        controller.start();
        assert_eq!(controller.active_role(), Some(TransportRole::Primary));
        assert_eq!(
            controller
                .active_proxy_endpoint()
                .unwrap()
                .socket_addr()
                .port(),
            40001
        );
        assert_eq!(controller.aggregate_state(), TransportState::Ready);
    }

    #[test]
    fn fails_over_to_fallback_after_consecutive_failures() {
        let primary = MockTransport::new("Primary-Relay", 40001);
        let fallback = MockTransport::new("Fallback-Relay", 40002);
        let config = FailoverConfig {
            max_consecutive_failures: 3,
            high_latency_threshold_ms: 1000,
            switchback_cooldown_sec: 20,
        };
        let mut controller = TransportFailoverController::new(primary, fallback, config);
        controller.start();

        // 1st failure -> degraded
        controller.record_observation(false, 200, 100);
        assert_eq!(controller.active_role(), Some(TransportRole::Primary));
        assert_eq!(controller.aggregate_state(), TransportState::Degraded);

        // 2nd failure -> still primary degraded
        controller.record_observation(false, 200, 101);
        assert_eq!(controller.active_role(), Some(TransportRole::Primary));

        // 3rd failure -> triggers failover to fallback
        controller.record_observation(false, 200, 102);
        assert_eq!(controller.active_role(), Some(TransportRole::Fallback));
        assert_eq!(
            controller
                .active_proxy_endpoint()
                .unwrap()
                .socket_addr()
                .port(),
            40002
        );
    }

    #[test]
    fn fails_closed_when_both_transports_fail() {
        let primary = MockTransport::new("Primary-Relay", 40001);
        let fallback = MockTransport::new("Fallback-Relay", 40002);
        let config = FailoverConfig {
            max_consecutive_failures: 2,
            ..Default::default()
        };
        let mut controller = TransportFailoverController::new(primary, fallback, config);
        controller.start();

        // Fail primary
        controller.record_observation(false, 100, 10);
        controller.record_observation(false, 100, 11);
        assert_eq!(controller.active_role(), Some(TransportRole::Fallback));

        // Fail fallback
        controller.record_observation(false, 100, 12);
        controller.record_observation(false, 100, 13);

        // Both down -> fails closed (active_role is None, proxy is None, state is Offline)
        assert_eq!(controller.active_role(), None);
        assert_eq!(controller.active_proxy_endpoint(), None);
        assert_eq!(controller.aggregate_state(), TransportState::Offline);
    }

    #[test]
    fn switches_back_to_primary_after_cooldown_and_successful_probe() {
        let primary = MockTransport::new("Primary-Relay", 40001);
        let fallback = MockTransport::new("Fallback-Relay", 40002);
        let config = FailoverConfig {
            max_consecutive_failures: 2,
            switchback_cooldown_sec: 30,
            ..Default::default()
        };
        let mut controller = TransportFailoverController::new(primary, fallback, config);
        controller.start();

        // Failover at t=100
        controller.record_observation(false, 100, 99);
        controller.record_observation(false, 100, 100);
        assert_eq!(controller.active_role(), Some(TransportRole::Fallback));

        // Attempt probe before cooldown (t=115 < 100+30) -> rejected
        assert!(!controller.probe_standby_primary(true, 115));
        assert_eq!(controller.active_role(), Some(TransportRole::Fallback));

        // Attempt probe after cooldown (t=131 >= 100+30) -> successful switchback
        assert!(controller.probe_standby_primary(true, 131));
        assert_eq!(controller.active_role(), Some(TransportRole::Primary));
        assert_eq!(
            controller
                .active_proxy_endpoint()
                .unwrap()
                .socket_addr()
                .port(),
            40001
        );
    }
}
