#![forbid(unsafe_code)]

/// Observable Android lifecycle state machine states.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum AndroidLifecycleState {
    Uninitialized,
    Created,
    Started,
    Resumed,
    Paused,
    Stopped,
    Destroyed,
}

/// System memory trim pressure levels received by Android Activity.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TrimMemoryLevel {
    RunningModerate,
    RunningLow,
    RunningCritical,
    UiHidden,
    Background,
    Moderate,
    Complete,
}

/// Lifecycle events dispatched by the Android runtime host.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum AndroidLifecycleEvent {
    OnCreate,
    OnStart,
    OnResume,
    OnPause,
    OnStop,
    OnDestroy,
    OnSaveInstanceState(String),
    OnRestoreInstanceState(String),
    OnTrimMemory(TrimMemoryLevel),
}

/// Error returned on invalid lifecycle state transitions.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum AndroidLifecycleError {
    InvalidTransition {
        current: AndroidLifecycleState,
        event: &'static str,
    },
    CorruptedSavedState(String),
}

/// Android lifecycle probe adapter managing lifecycle events, state saving/restoration,
/// and low-memory pressure notifications without desktop-only assumptions.
#[derive(Debug, Clone)]
pub struct AndroidLifecycleProbe {
    state: AndroidLifecycleState,
    saved_instance_state: Option<String>,
    last_trim_memory: Option<TrimMemoryLevel>,
    recreation_count: u32,
}

impl Default for AndroidLifecycleProbe {
    fn default() -> Self {
        Self::new()
    }
}

impl AndroidLifecycleProbe {
    #[must_use]
    pub const fn new() -> Self {
        Self {
            state: AndroidLifecycleState::Uninitialized,
            saved_instance_state: None,
            last_trim_memory: None,
            recreation_count: 0,
        }
    }

    #[must_use]
    pub const fn state(&self) -> AndroidLifecycleState {
        self.state
    }

    #[must_use]
    pub fn saved_instance_state(&self) -> Option<&str> {
        self.saved_instance_state.as_deref()
    }

    #[must_use]
    pub const fn last_trim_memory(&self) -> Option<TrimMemoryLevel> {
        self.last_trim_memory
    }

    #[must_use]
    pub const fn recreation_count(&self) -> u32 {
        self.recreation_count
    }

    /// Dispatches an Android lifecycle event into the state machine.
    pub fn dispatch(&mut self, event: AndroidLifecycleEvent) -> Result<(), AndroidLifecycleError> {
        match event {
            AndroidLifecycleEvent::OnCreate => match self.state {
                AndroidLifecycleState::Uninitialized | AndroidLifecycleState::Destroyed => {
                    self.state = AndroidLifecycleState::Created;
                    Ok(())
                }
                _ => Err(AndroidLifecycleError::InvalidTransition {
                    current: self.state,
                    event: "OnCreate",
                }),
            },
            AndroidLifecycleEvent::OnStart => match self.state {
                AndroidLifecycleState::Created | AndroidLifecycleState::Stopped => {
                    self.state = AndroidLifecycleState::Started;
                    Ok(())
                }
                _ => Err(AndroidLifecycleError::InvalidTransition {
                    current: self.state,
                    event: "OnStart",
                }),
            },
            AndroidLifecycleEvent::OnResume => match self.state {
                AndroidLifecycleState::Started | AndroidLifecycleState::Paused => {
                    self.state = AndroidLifecycleState::Resumed;
                    Ok(())
                }
                _ => Err(AndroidLifecycleError::InvalidTransition {
                    current: self.state,
                    event: "OnResume",
                }),
            },
            AndroidLifecycleEvent::OnPause => match self.state {
                AndroidLifecycleState::Resumed => {
                    self.state = AndroidLifecycleState::Paused;
                    Ok(())
                }
                _ => Err(AndroidLifecycleError::InvalidTransition {
                    current: self.state,
                    event: "OnPause",
                }),
            },
            AndroidLifecycleEvent::OnStop => match self.state {
                AndroidLifecycleState::Paused => {
                    self.state = AndroidLifecycleState::Stopped;
                    Ok(())
                }
                _ => Err(AndroidLifecycleError::InvalidTransition {
                    current: self.state,
                    event: "OnStop",
                }),
            },
            AndroidLifecycleEvent::OnDestroy => match self.state {
                AndroidLifecycleState::Stopped | AndroidLifecycleState::Created => {
                    self.state = AndroidLifecycleState::Destroyed;
                    Ok(())
                }
                _ => Err(AndroidLifecycleError::InvalidTransition {
                    current: self.state,
                    event: "OnDestroy",
                }),
            },
            AndroidLifecycleEvent::OnSaveInstanceState(bundle) => {
                if self.state == AndroidLifecycleState::Paused
                    || self.state == AndroidLifecycleState::Stopped
                    || self.state == AndroidLifecycleState::Created
                {
                    self.saved_instance_state = Some(bundle);
                    Ok(())
                } else {
                    Err(AndroidLifecycleError::InvalidTransition {
                        current: self.state,
                        event: "OnSaveInstanceState",
                    })
                }
            }
            AndroidLifecycleEvent::OnRestoreInstanceState(bundle) => {
                if self.state == AndroidLifecycleState::Started
                    || self.state == AndroidLifecycleState::Created
                {
                    self.saved_instance_state = Some(bundle);
                    self.recreation_count = self.recreation_count.saturating_add(1);
                    Ok(())
                } else {
                    Err(AndroidLifecycleError::InvalidTransition {
                        current: self.state,
                        event: "OnRestoreInstanceState",
                    })
                }
            }
            AndroidLifecycleEvent::OnTrimMemory(level) => {
                self.last_trim_memory = Some(level);
                Ok(())
            }
        }
    }
}

#[cfg(test)]
#[allow(clippy::unwrap_used, clippy::panic)]
mod tests {
    use super::*;

    #[test]
    fn complete_normal_android_lifecycle_flow() {
        let mut probe = AndroidLifecycleProbe::new();
        assert_eq!(probe.state(), AndroidLifecycleState::Uninitialized);

        probe.dispatch(AndroidLifecycleEvent::OnCreate).unwrap();
        assert_eq!(probe.state(), AndroidLifecycleState::Created);

        probe.dispatch(AndroidLifecycleEvent::OnStart).unwrap();
        assert_eq!(probe.state(), AndroidLifecycleState::Started);

        probe.dispatch(AndroidLifecycleEvent::OnResume).unwrap();
        assert_eq!(probe.state(), AndroidLifecycleState::Resumed);

        probe.dispatch(AndroidLifecycleEvent::OnPause).unwrap();
        assert_eq!(probe.state(), AndroidLifecycleState::Paused);

        probe.dispatch(AndroidLifecycleEvent::OnStop).unwrap();
        assert_eq!(probe.state(), AndroidLifecycleState::Stopped);

        probe.dispatch(AndroidLifecycleEvent::OnDestroy).unwrap();
        assert_eq!(probe.state(), AndroidLifecycleState::Destroyed);
    }

    #[test]
    fn transient_pause_resume_cycle() {
        let mut probe = AndroidLifecycleProbe::new();
        probe.dispatch(AndroidLifecycleEvent::OnCreate).unwrap();
        probe.dispatch(AndroidLifecycleEvent::OnStart).unwrap();
        probe.dispatch(AndroidLifecycleEvent::OnResume).unwrap();

        // Screen turned off or dialog overlay
        probe.dispatch(AndroidLifecycleEvent::OnPause).unwrap();
        assert_eq!(probe.state(), AndroidLifecycleState::Paused);

        // Returned to foreground
        probe.dispatch(AndroidLifecycleEvent::OnResume).unwrap();
        assert_eq!(probe.state(), AndroidLifecycleState::Resumed);
    }

    #[test]
    fn activity_recreate_with_saved_instance_state() {
        let mut probe = AndroidLifecycleProbe::new();
        probe.dispatch(AndroidLifecycleEvent::OnCreate).unwrap();
        probe.dispatch(AndroidLifecycleEvent::OnStart).unwrap();
        probe.dispatch(AndroidLifecycleEvent::OnResume).unwrap();

        // Orientation change triggers pause -> stop -> saveState -> destroy
        probe.dispatch(AndroidLifecycleEvent::OnPause).unwrap();
        probe.dispatch(AndroidLifecycleEvent::OnStop).unwrap();
        probe
            .dispatch(AndroidLifecycleEvent::OnSaveInstanceState(
                "active_url=webgate://service/factory".to_string(),
            ))
            .unwrap();
        probe.dispatch(AndroidLifecycleEvent::OnDestroy).unwrap();
        assert_eq!(probe.state(), AndroidLifecycleState::Destroyed);

        // Recreate activity
        probe.dispatch(AndroidLifecycleEvent::OnCreate).unwrap();
        probe.dispatch(AndroidLifecycleEvent::OnStart).unwrap();
        probe
            .dispatch(AndroidLifecycleEvent::OnRestoreInstanceState(
                "active_url=webgate://service/factory".to_string(),
            ))
            .unwrap();
        probe.dispatch(AndroidLifecycleEvent::OnResume).unwrap();

        assert_eq!(probe.state(), AndroidLifecycleState::Resumed);
        assert_eq!(
            probe.saved_instance_state(),
            Some("active_url=webgate://service/factory")
        );
        assert_eq!(probe.recreation_count(), 1);
    }

    #[test]
    fn handles_low_memory_trim_events() {
        let mut probe = AndroidLifecycleProbe::new();
        probe.dispatch(AndroidLifecycleEvent::OnCreate).unwrap();
        probe.dispatch(AndroidLifecycleEvent::OnStart).unwrap();
        probe.dispatch(AndroidLifecycleEvent::OnResume).unwrap();

        probe
            .dispatch(AndroidLifecycleEvent::OnTrimMemory(
                TrimMemoryLevel::RunningCritical,
            ))
            .unwrap();
        assert_eq!(
            probe.last_trim_memory(),
            Some(TrimMemoryLevel::RunningCritical)
        );
        assert_eq!(probe.state(), AndroidLifecycleState::Resumed);
    }

    #[test]
    fn rejects_invalid_lifecycle_transitions() {
        let mut probe = AndroidLifecycleProbe::new();
        // Cannot resume directly from uninitialized
        let res = probe.dispatch(AndroidLifecycleEvent::OnResume);
        assert_eq!(
            res,
            Err(AndroidLifecycleError::InvalidTransition {
                current: AndroidLifecycleState::Uninitialized,
                event: "OnResume",
            })
        );
    }
}
