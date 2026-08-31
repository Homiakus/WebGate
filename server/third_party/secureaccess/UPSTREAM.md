# SecureAccess pinned dependency anchor

This directory is a reproducible source anchor for the private upstream module used by WebGate.

- Upstream repository: `Homiakus/SecureAcces`
- Upstream version: `v0.4.0`
- Upstream commit: `827abb1add11a9fcbd0a9944e65efbd20c675739`
- WebGate supported build line for this dependency: Go `1.26.x`; the server toolchain is pinned to `go1.26.6`.

## Scope

T-049 deliberately vendors only the immutable dependency anchor needed to make the private module path, license, provenance, version and toolchain reproducible in clean WebGate CI. It does **not** claim that SecureAccess authorization is integrated yet. The exact production implementation required by WebGate is introduced and behavior-qualified in T-050; administrator management authorization follows in T-051.

No WebGate-authored security implementation may be placed under this directory. Vendored upstream files must remain byte-identical to the source commit below.

## Upstream Git blob identities

| Local path | Upstream Git blob SHA-1 |
| --- | --- |
| `go.mod` | `594a9c44026deef24815096e8b750db3291d09f3` |
| `LICENSE` | `094569d25a7322deca171590dabdc253bb5a452b` |
| `doc.go` | `9ada938d38d5786559cb772d92aa4fc25b53eca9` |
| `secureaccess/doc.go` | `eeaa2c064abf01b8fdd21e76ce6fb3ec6cde2ccc` |
| `secureaccess/version.go` | `de3bf071e5228d94fe1df531a2c359b0046e39be` |

The repository contract test recomputes each Git blob object ID as `SHA1("blob <len>\\0" || bytes)` and rejects any byte drift.

## Update rule

1. Select an explicit reviewed SecureAccess commit/version.
2. Fetch source only through an authorized repository channel.
3. Copy required upstream files byte-for-byte and record their upstream Git blob IDs here.
4. Update the integrity contract test in the same atomic change.
5. Run the complete WebGate qualification ladder before moving `main`.

A file that needs WebGate-specific behavior belongs in a WebGate adapter outside `third_party/secureaccess`; never patch vendored security source in place.
