#!/usr/bin/env python3
from pathlib import Path

path = Path('crates/webgate-app/src/main.rs')
text = path.read_text()

enum_block = '''#[derive(Debug)]
enum ClientTransportStartError {
    Primary(RestrictedProxyError),
    Dual(DualRelayError),
    StatusHandleUnavailable,
}
'''
with_display = enum_block + '''
impl std::fmt::Display for ClientTransportStartError {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Primary(error) => write!(formatter, "primary transport: {error:?}"),
            Self::Dual(error) => write!(formatter, "dual transport: {error:?}"),
            Self::StatusHandleUnavailable => formatter.write_str("live status handle unavailable"),
        }
    }
}
'''
assert text.count(enum_block) == 1
text = text.replace(enum_block, with_display, 1)

old_log = '''                "[Транспорт] Protected transport failed to start and remains OFFLINE: {error:?}"
'''
new_log = '''                "[Транспорт] Protected transport failed to start and remains OFFLINE: {error}"
'''
assert text.count(old_log) == 1
text = text.replace(old_log, new_log, 1)

old_direct = '''        let session_manager_clone = Arc::clone(&session_manager);

        let server_thread = thread::spawn(move || {
            let (stream, _) = listener.accept().unwrap();
            handle_client_stream(
                stream,
                &profile_clone,
                &session_manager_clone,
                "dev_test_123",
                TransportState::Ready,
                None,
            );
        });
'''
new_direct = '''        let session_manager_clone = Arc::clone(&session_manager);
        let transport_status = ClientTransportStatus::fixed(TransportState::Ready, None);

        let server_thread = thread::spawn(move || {
            let (stream, _) = listener.accept().unwrap();
            handle_client_stream(
                stream,
                &profile_clone,
                &session_manager_clone,
                "dev_test_123",
                &transport_status,
            );
        });
'''
count = text.count(old_direct)
assert count == 2, count
text = text.replace(old_direct, new_direct, 2)

path.write_text(text)
