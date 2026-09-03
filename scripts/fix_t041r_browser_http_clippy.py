#!/usr/bin/env python3
from pathlib import Path

path = Path('crates/webgate-browser/tests/proxy_enforcement.rs')
text = path.read_text()
old = 'use std::net::{IpAddr, Ipv4Addr, SocketAddr, TcpListener};\n'
new = 'use std::net::{IpAddr, Ipv4Addr, TcpListener};\n'
assert text.count(old) == 1
path.write_text(text.replace(old, new, 1))
