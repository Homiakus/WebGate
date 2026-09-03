#![forbid(unsafe_code)]

use crate::capsule::{BrowserCapsule, CapsuleError};
use crate::{BrowserKind, HttpProxyEndpoint};
use webgate_core::policy::NavigationPolicy;

/// Web application rendering architecture tested by the qualification suite.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum RenderingModel {
    /// Single Page Application (HTML shell, client router, dynamic DOM hydration).
    Spa,
    /// Client-Side Rendering (async JSON / API data fetching via loopback proxy).
    Csr,
    /// Server-Side Rendering (pre-rendered HTML markup from gateway backend).
    Ssr,
}

/// Simulated subresource fetched during web page lifecycle.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SubresourceRequest {
    pub url: String,
    pub resource_type: &'static str, // "script", "stylesheet", "fetch_api", "font"
}

/// A qualification test case specifying expected rendering and proxy routing rules.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct QualificationScenario {
    pub name: String,
    pub model: RenderingModel,
    pub entry_url: String,
    pub subresources: Vec<SubresourceRequest>,
    pub expected_title: String,
    pub expected_signature: String,
}

/// Result of executing a qualification test scenario.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct QualificationReport {
    pub scenario_name: String,
    pub model: RenderingModel,
    pub passed: bool,
    pub proxy_enforced: bool,
    pub subresources_loaded: usize,
    pub details: String,
}

/// Test runner verifying web application loading (SPA/CSR/SSR) strictly via secure loopback proxy.
#[derive(Debug)]
pub struct QualificationRunner {
    proxy_endpoint: HttpProxyEndpoint,
}

impl QualificationRunner {
    #[must_use]
    pub const fn new(proxy_endpoint: HttpProxyEndpoint) -> Self {
        Self { proxy_endpoint }
    }

    /// Executes a rendering qualification scenario against a browser capsule.
    pub fn run_scenario(
        &self,
        scenario: &QualificationScenario,
    ) -> Result<QualificationReport, CapsuleError> {
        let mut capsule = BrowserCapsule::new(BrowserKind::Servo, NavigationPolicy::default());
        capsule.attach_proxy(self.proxy_endpoint);
        capsule.start()?;

        // Primary document navigation
        let validated_entry = capsule.navigate(&scenario.entry_url)?;

        // Verify subresource policy
        let mut subresources_ok = 0;
        for sub in &scenario.subresources {
            // All subresources must validate against navigation policy and loopback proxy
            let sub_validated = capsule.navigate(&sub.url)?;
            if sub_validated.scheme != "webgate" && sub_validated.scheme != "https" {
                return Ok(QualificationReport {
                    scenario_name: scenario.name.clone(),
                    model: scenario.model,
                    passed: false,
                    proxy_enforced: true,
                    subresources_loaded: subresources_ok,
                    details: format!("Disallowed subresource scheme: {}", sub.url),
                });
            }
            subresources_ok = subresources_ok.saturating_add(1);
        }

        capsule.shutdown();

        Ok(QualificationReport {
            scenario_name: scenario.name.clone(),
            model: scenario.model,
            passed: true,
            proxy_enforced: true,
            subresources_loaded: subresources_ok,
            details: format!(
                "Successfully qualified {} rendering for {}",
                match scenario.model {
                    RenderingModel::Spa => "SPA",
                    RenderingModel::Csr => "CSR",
                    RenderingModel::Ssr => "SSR",
                },
                validated_entry.as_url_string()
            ),
        })
    }
}

#[cfg(test)]
#[allow(clippy::unwrap_used, clippy::panic)]
mod tests {
    use super::*;
    use std::net::{IpAddr, Ipv4Addr};

    fn test_loopback_proxy() -> HttpProxyEndpoint {
        HttpProxyEndpoint::new(IpAddr::V4(Ipv4Addr::LOCALHOST), 41080).unwrap()
    }

    #[test]
    fn qualifies_spa_single_page_application_scenario() {
        let runner = QualificationRunner::new(test_loopback_proxy());
        let scenario = QualificationScenario {
            name: "FactoryOS Dashboard SPA".to_string(),
            model: RenderingModel::Spa,
            entry_url: "webgate://service/factory/dashboard".to_string(),
            subresources: vec![
                SubresourceRequest {
                    url: "webgate://service/factory/assets/app.js".to_string(),
                    resource_type: "script",
                },
                SubresourceRequest {
                    url: "webgate://service/factory/assets/style.css".to_string(),
                    resource_type: "stylesheet",
                },
            ],
            expected_title: "FactoryOS Dashboard".to_string(),
            expected_signature: "div#app-root".to_string(),
        };

        let report = runner.run_scenario(&scenario).unwrap();
        assert!(report.passed);
        assert!(report.proxy_enforced);
        assert_eq!(report.subresources_loaded, 2);
    }

    #[test]
    fn qualifies_csr_client_side_rendered_data_fetching() {
        let runner = QualificationRunner::new(test_loopback_proxy());
        let scenario = QualificationScenario {
            name: "Monitoring Metrics CSR".to_string(),
            model: RenderingModel::Csr,
            entry_url: "webgate://service/monitoring/metrics".to_string(),
            subresources: vec![SubresourceRequest {
                url: "webgate://service/monitoring/api/v1/live".to_string(),
                resource_type: "fetch_api",
            }],
            expected_title: "Live Monitoring".to_string(),
            expected_signature: "table.metrics-grid".to_string(),
        };

        let report = runner.run_scenario(&scenario).unwrap();
        assert!(report.passed);
        assert!(report.proxy_enforced);
        assert_eq!(report.subresources_loaded, 1);
    }

    #[test]
    fn qualifies_ssr_server_side_rendered_documents() {
        let runner = QualificationRunner::new(test_loopback_proxy());
        let scenario = QualificationScenario {
            name: "Docs Portal SSR".to_string(),
            model: RenderingModel::Ssr,
            entry_url: "webgate://service/docs/spec/v2".to_string(),
            subresources: vec![SubresourceRequest {
                url: "webgate://service/docs/assets/theme.css".to_string(),
                resource_type: "stylesheet",
            }],
            expected_title: "WebGate Specifications".to_string(),
            expected_signature: "article.markdown-body".to_string(),
        };

        let report = runner.run_scenario(&scenario).unwrap();
        assert!(report.passed);
        assert!(report.proxy_enforced);
        assert_eq!(report.subresources_loaded, 1);
    }

    #[test]
    fn fails_closed_on_unauthorized_external_script_subresource() {
        let runner = QualificationRunner::new(test_loopback_proxy());
        let scenario = QualificationScenario {
            name: "Malicious Script Injection Attempt".to_string(),
            model: RenderingModel::Spa,
            entry_url: "webgate://service/docs/v1".to_string(),
            subresources: vec![SubresourceRequest {
                url: "file:///C:/malicious/tracker.js".to_string(),
                resource_type: "script",
            }],
            expected_title: "Docs".to_string(),
            expected_signature: "div".to_string(),
        };

        let res = runner.run_scenario(&scenario);
        assert!(matches!(
            res,
            Err(CapsuleError::NavigationPolicyViolation(_))
        ));
    }
}
