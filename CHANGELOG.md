# Changelog

## [Unreleased]

### Added
- Multi-source support: gestiona varios proyectos GSM y Vaults simultáneamente.
- HashiCorp Vault provider (KV v1 + KV v2).
- Vault auth: token, AppRole, OIDC con callback browser.
- TUI: columna `PROVIDER`, filtro por fuente con Tab cycle, picker de fuentes con `Ctrl+P`.
- Subcomando `sources` (`list/add/edit/remove/toggle/login/set-default`).
- Templates per-source con override por título; nuevas variables `{{.SourceID}}` y `{{.Provider}}`.
- Audit log incluye `source_id` y `provider`; filtros `--source` y `--provider` en `audit logs`.
- Migración automática de configs antiguos (`project_id` raíz) al nuevo shape `sources`.
- `docker-compose.yml` + dex para E2E de OIDC en local.

### Changed
- `--project` está deprecated en favor de `--source <id>` (warning a stderr cuando se usa).
- `internal/gcp` movido a `internal/providers/gsm` (cambio interno, sin impacto en usuarios).

### Fixed
- `templates generate --copy` ahora copia realmente al portapapeles (regresión introducida en la PR #8).
