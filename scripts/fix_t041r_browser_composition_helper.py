#!/usr/bin/env python3
from pathlib import Path

path = Path('scripts/patch_t041r_browser_http_composition.py')
text = path.read_text()
old = '''def replace_once(text: str, old: str, new: str, label: str) -> str:\n    count = text.count(old)\n    assert count == 1, (label, count)\n    return text.replace(old, new, 1)\n'''
new = '''def replace_once(text: str, old: str, new: str, label: str) -> str:\n    count = text.count(old)\n    expected = 2 if label == "browser imports" else 1\n    assert count == expected, (label, count, expected)\n    return text.replace(old, new, 1)\n'''
assert text.count(old) == 1
path.write_text(text.replace(old, new, 1))
