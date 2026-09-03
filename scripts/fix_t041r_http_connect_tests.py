#!/usr/bin/env python3
from pathlib import Path

path = Path('crates/webgate-transport/src/restricted_http_connect.rs')
text = path.read_text()

replacements = {
r'''        assert_eq!(
            parse_connect_request(b"GET https://app.internal/ HTTP/1.1\r\n\r\n"),
            Err(RequestError::MethodNotAllowed)
        );
''': r'''        assert!(matches!(
            parse_connect_request(b"GET https://app.internal/ HTTP/1.1\r\n\r\n"),
            Err(RequestError::MethodNotAllowed)
        ));
''',
r'''        assert_eq!(
            parse_connect_request(b"CONNECT user@app.internal:443 HTTP/1.1\r\n\r\n"),
            Err(RequestError::Malformed)
        );
''': r'''        assert!(matches!(
            parse_connect_request(b"CONNECT user@app.internal:443 HTTP/1.1\r\n\r\n"),
            Err(RequestError::Malformed)
        ));
''',
r'''        assert_eq!(
            parse_connect_request(b"CONNECT app.internal:443 HTTP/1.1\r\n folded\r\n\r\n"),
            Err(RequestError::Malformed)
        );
''': r'''        assert!(matches!(
            parse_connect_request(b"CONNECT app.internal:443 HTTP/1.1\r\n folded\r\n\r\n"),
            Err(RequestError::Malformed)
        ));
''',
r'''        let mut denied_response = [0_u8; 64];
        let count = denied.read(&mut denied_response).unwrap();
        assert!(std::str::from_utf8(&denied_response[..count]).unwrap().contains("403 Forbidden"));
        assert_eq!(bridge.state(), TransportState::Ready);

        let mut malformed = TcpStream::connect(endpoint.socket_addr()).unwrap();
        malformed.write_all(b"GET / HTTP/1.1\r\n\r\n").unwrap();
        let count = malformed.read(&mut denied_response).unwrap();
        assert!(std::str::from_utf8(&denied_response[..count]).unwrap().contains("405 Method Not Allowed"));
        assert_eq!(bridge.state(), TransportState::Ready);
''': r'''        let mut denied_response = String::new();
        denied.read_to_string(&mut denied_response).unwrap();
        assert!(denied_response.starts_with("HTTP/1.1 403 Forbidden"));
        assert_eq!(bridge.state(), TransportState::Ready);

        let mut malformed = TcpStream::connect(endpoint.socket_addr()).unwrap();
        malformed.write_all(b"GET / HTTP/1.1\r\n\r\n").unwrap();
        let mut malformed_response = String::new();
        malformed.read_to_string(&mut malformed_response).unwrap();
        assert!(malformed_response.starts_with("HTTP/1.1 405 Method Not Allowed"));
        assert_eq!(bridge.state(), TransportState::Ready);
''',
}

for old, new in replacements.items():
    count = text.count(old)
    assert count == 1, (old[:100], count)
    text = text.replace(old, new, 1)

path.write_text(text)
