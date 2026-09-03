# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest (main) | ✅ |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

If you discover a security vulnerability, please report it privately:

1. **Email**: Open a private security advisory at https://github.com/[your-org]/FaultIQ/security/advisories/new
2. **Response time**: We aim to acknowledge reports within 48 hours and provide a fix within 14 days for critical issues.

### What to include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Suggested fix (optional)

## Security Considerations for Self-Hosted Deployments

### Secrets management
- Never commit `.env` to version control
- Use Docker secrets or a vault (see `docs/SECRETS.md`)
- Generate strong random values: `openssl rand -hex 32`

### Default credentials
The default `.env.example` contains placeholder values. **Change all defaults** before production:
- `NEO4J_PASSWORD` — use a strong random password
- `POSTGRES_PASSWORD` — use a strong random password  
- `KEYCLOAK_ADMIN_PASSWORD` — change from default `admin`
- `API_KEY_SECRET` — generate with `openssl rand -hex 32`

### Network security
- Do not expose Neo4j (7474/7687), Postgres (5432), Redis (6379) ports to the public internet
- Use the TLS profile (`docker compose --profile tls up`) for production
- Place api-gateway behind a reverse proxy (nginx/traefik) with rate limiting

### Authentication
- FaultIQ uses Keycloak for user authentication — keep Keycloak updated
- The `API_KEY_SECRET` for CI/CD should be rotated periodically
- OAuth2 health check secrets should use dedicated service accounts with minimal permissions

## Known Security Limitations

- The demo-services mock server (port 8091) has no authentication — **never expose in production**
- The default Keycloak realm (`faultiq`) is configured for development — harden for production
