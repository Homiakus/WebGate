#!/usr/bin/env python3
from pathlib import Path

path = Path('crates/webgate-transport/src/restricted_http_connect.rs')
text = path.read_text()

replacements = {
'''        assert_eq!(
            parse_connect_request(b"GET https://app.internal/ HTTP/1.1\r\n\r\n"),
            Err(RequestError::MethodNotAllowed)
        );
''': '''        assert!(matches!(
            parse_connect_request(b"GET https://app.internal/ HTTP/1.1\r\n\r\n"),
            Err(RequestError::MethodNotAllowed)
        ));
''',
'''        assert_eq!(
            parse_connect_request(b"CONNECT user@app.internal:443 HTTP/1.1\r\n\r\n"),
            Err(RequestError::Malformed)
        );
''': '''        assert!(matches!(
            parse_connect_request(b"CONNECT user@app.internal:443 HTTP/1.1\r\n\r\n"),
            Err(RequestError::Malformed)
        ));
''',
'''        assert_eq!(
            parse_connect_request(b"CONNECT app.internal:443 HTTP/1.1\r\n folded\r\n\r\n"),
            Err(RequestError::Malformed)
        );
''': '''        assert!(matches!(
            parse_connect_request(b"CONNECT app.internal:443 HTTP/1.1\r\n folded\r\n\r\n"),
            Err(RequestError::Malformed)
        ));
''',
}

for old, new in replacements.items():
    count = text.count(old)
    assert count == 1, (old[:80], count)
    text = text.replace(old, new, 1)

path.write_text(text)
