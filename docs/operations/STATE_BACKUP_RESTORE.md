# WebGate durable state — backup and restore

This runbook covers the WebGate-owned SQLite state only. SecureAcces owns its own identity/session/audit persistence and must be backed up separately.

## What is stored

`--state-db` / `WEBGATE_STATE_DB` contains WebGate-owned:

- protected service configuration and lifecycle status;
- enrolled device metadata with explicit SecureAcces `AccountID`;
- release metadata/artifacts;
- non-secret control configuration metadata;
- append-only WebGate management audit events.

The database intentionally does **not** store process PID/runtime state, device PoP challenges, `WEBGATE_ADMIN_TOKEN`, `WEBGATE_AUTHORITY_TOKEN`, or the Telegram bot token.

`WEBGATE_TELEGRAM_BOT_TOKEN` is the preferred runtime-only Telegram credential and overrides any token from an external config file.

## Create a consistent backup

A backup can be created while the normal state database is not being modified by this one-shot process invocation:

```bash
webgate-server \
  --state-db /var/lib/webgate/webgate-state.db \
  --backup-state /var/backups/webgate/webgate-state-$(date +%Y%m%d-%H%M%S).db
```

The command:

1. opens the normal state database with the same schema checks as server startup;
2. validates service/device/release/control/audit records and their checksums;
3. creates a SQLite-consistent snapshot with `VACUUM INTO`;
4. restricts the resulting file permissions where the platform supports it;
5. synchronizes the snapshot and containing directory before returning success.

The destination must not already exist and must differ from the live database path. Never treat a copied `*.db` file from an active WAL database as a qualified backup.

## Validate by restore drill

Restore is intentionally **offline** and create-only. It will not overwrite an existing target.

1. Stop WebGate.
2. Keep the current state file as the rollback copy; move it out of the target path rather than deleting it.
3. Run:

```bash
webgate-server \
  --state-db /var/lib/webgate/webgate-state.db \
  --restore-state /var/backups/webgate/webgate-state-20260901-120000.db
```

Before installing the target, WebGate copies the backup into a staging file, opens it through the normal registry/control loaders, validates schema versions and record checksums, checkpoints SQLite state, and only then atomically renames the staged database into place. A failed validation leaves the requested target path absent.

4. Start WebGate with the normal runtime secrets (`WEBGATE_ADMIN_TOKEN`, optional `WEBGATE_AUTHORITY_TOKEN`, optional `WEBGATE_TELEGRAM_BOT_TOKEN`).
5. Verify Admin/Data listeners, service inventory, device/release inventory and audit history before reopening access to users.

## Rollback

If post-restore verification fails:

1. Stop WebGate.
2. Move the newly restored database out of the live path for forensic inspection.
3. Move the preserved pre-restore database back to the configured `--state-db` path.
4. Start WebGate with the same runtime-only secrets.

Do not merge two WebGate state databases by hand. Do not edit audit rows: database triggers reject UPDATE/DELETE and checksum validation treats inconsistent records as corruption.

## Important bootstrap rule

Service defaults/config entries seed the registry only when the state database did not exist before startup and the new registry is empty. Once a state database exists, even an intentionally empty service registry is authoritative; restart does not resurrect deleted services from defaults or a config file.
