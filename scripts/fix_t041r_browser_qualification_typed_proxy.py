#!/usr/bin/env python3
from pathlib import Path

path = Path('crates/webgate-browser/src/qualification.rs')
text = path.read_text()
text = text.replace('use crate::BrowserKind;\n', 'use crate::{BrowserKind, HttpProxyEndpoint};\n', 1)
text = text.replace('use std::net::SocketAddr;\n', '', 1)
text = text.replace('    proxy_endpoint: SocketAddr,', '    proxy_endpoint: HttpProxyEndpoint,', 1)
text = text.replace(
    '    pub const fn new(proxy_endpoint: SocketAddr) -> Self {',
    '    pub const fn new(proxy_endpoint: HttpProxyEndpoint) -> Self {',
    1,
)
text = text.replace('        capsule.attach_proxy(self.proxy_endpoint)?;', '        capsule.attach_proxy(self.proxy_endpoint);', 1)
old = '''    fn test_loopback_proxy() -> SocketAddr {\n        SocketAddr::new(IpAddr::V4(Ipv4Addr::LOCALHOST), 41080)\n    }\n'''
new = '''    fn test_loopback_proxy() -> HttpProxyEndpoint {\n        HttpProxyEndpoint::new(IpAddr::V4(Ipv4Addr::LOCALHOST), 41080).unwrap()\n    }\n'''
assert text.count(old) == 1
text = text.replace(old, new, 1)
path.write_text(text)
