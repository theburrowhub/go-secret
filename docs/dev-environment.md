# Dev environment

## Vault local (KV v2 + OIDC dex)

```bash
docker-compose up -d
```

Ese comando levanta Vault en `:8200` (token raíz `root-token`) y dex en `:5556`.

### Configurar OIDC en Vault

```bash
export VAULT_ADDR=http://localhost:8200
export VAULT_TOKEN=root-token
vault auth enable oidc
vault write auth/oidc/config \
  oidc_discovery_url=http://localhost:5556 \
  oidc_client_id=vault \
  oidc_client_secret=vault-secret \
  default_role=developer
vault write auth/oidc/role/developer \
  bound_audiences=vault \
  allowed_redirect_uris=http://localhost:8200/v1/auth/oidc/oidc/callback,http://127.0.0.1:8250/oidc/callback \
  user_claim=email \
  policies=default
vault secrets enable -path=secret -version=2 kv
```

### Añadir source en go-secret

```bash
go-secret sources add
# provider: vault
# id: vault-local
# address: http://localhost:8200
# auth method: oidc
# role: developer
# mount: secret, version: 2
```

### Login

```bash
go-secret sources login vault-local
# → abre el browser con dex (user: dev@local, pass: password)
```

Después de eso `go-secret list` debería listar tanto los secretos GSM como los de Vault.
