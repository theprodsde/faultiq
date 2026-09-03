# Managing Secrets in FaultIQ

This document covers the supported approaches for keeping credentials out of source control and environment variable plaintext.

---

## Approach 1: `.env.production` (simplest, recommended for single-host deployments)

Create a `.env.production` file (already listed in `.gitignore`) and pass it at startup:

```bash
cp .env.example .env.production
# Edit .env.production with real credentials
docker compose --env-file .env.production up -d
```

Never commit `.env.production`. Add it to `.gitignore` if not already present:

```
.env.production
.env.local
*.secret
```

---

## Approach 2: Docker Swarm secrets

Docker Swarm secrets are encrypted at rest and only exposed to services that explicitly request them.

**Create secrets:**

```bash
printf "your-db-password"   | docker secret create postgres_password -
printf "your-neo4j-password" | docker secret create neo4j_password -
printf "your-api-key"        | docker secret create api_key_secret -
```

**Deploy with the secrets overlay:**

```bash
docker stack deploy \
  -c docker-compose.yml \
   \
  faultiq
```

Secrets are mounted as files under `/run/secrets/<name>` inside the container. The services that consume them read the `*_FILE` environment variable (e.g. `POSTGRES_PASSWORD_FILE`) and load the value from the file path.

**Remove a secret:**

```bash
docker secret rm postgres_password
```

---

## Approach 3: HashiCorp Vault

Vault provides dynamic secrets, lease renewal, and audit logging.

**Pattern — sidecar agent injection (recommended for Kubernetes):**

1. Install the Vault Agent sidecar injector in your cluster.
2. Annotate your pod:

```yaml
annotations:
  vault.hashicorp.com/agent-inject: "true"
  vault.hashicorp.com/role: "faultiq"
  vault.hashicorp.com/agent-inject-secret-db-password: "secret/faultiq/postgres"
  vault.hashicorp.com/agent-inject-template-db-password: |
    {{- with secret "secret/faultiq/postgres" -}}
    {{ .Data.data.password }}
    {{- end }}
```

The secret is written to `/vault/secrets/db-password` at pod start. Point your service at that path via an env var.

**Pattern — direct API call at startup:**

```bash
VAULT_TOKEN=$(vault write -field=token auth/approle/login \
  role_id=$VAULT_ROLE_ID secret_id=$VAULT_SECRET_ID)

POSTGRES_PASSWORD=$(vault kv get -field=password secret/faultiq/postgres)
export POSTGRES_PASSWORD
```

---

## Approach 4: AWS Secrets Manager

Use the AWS CLI or SDK to fetch secrets at container startup.

**Store a secret:**

```bash
aws secretsmanager create-secret \
  --name faultiq/postgres_password \
  --secret-string "your-db-password"
```

**Fetch at startup (shell init script):**

```bash
export POSTGRES_PASSWORD=$(aws secretsmanager get-secret-value \
  --secret-id faultiq/postgres_password \
  --query SecretString \
  --output text)
```

**ECS / EKS native injection:**

In your ECS task definition or Kubernetes pod spec, reference the secret ARN directly:

```json
{
  "secrets": [
    {
      "name": "POSTGRES_PASSWORD",
      "valueFrom": "arn:aws:secretsmanager:us-east-1:123456789:secret:faultiq/postgres_password"
    }
  ]
}
```

The container runtime fetches and injects the value before your process starts — no code changes required.

---

## Secret rotation

Regardless of the backend, rotate secrets by:

1. Creating the new secret value in the store.
2. Updating the consuming services (rolling restart in Swarm/K8s picks up the new value).
3. Revoking the old secret value.

For zero-downtime rotation, services should reload credentials on SIGHUP or on a configurable interval rather than only at startup.
