# Keycloak Authentication Setup Guide

## Overview

This guide covers the complete Keycloak configuration for FaultIQ, including user management, role assignment, and token claim mapping.

---

## Realm Configuration

### Realm: `faultiq`

The FaultIQ realm is pre-configured in `scripts/keycloak/realm.json` and automatically imported when Keycloak starts.

**Key Settings**:
- Token Lifespan: 3600 seconds (1 hour)
- Refresh Token Lifespan: 86400 seconds (24 hours)

---

## Clients Configuration

### 1. `faultiq-ui` (Frontend Web Application)

**Type**: Public Client  
**Purpose**: JavaScript/Next.js frontend authentication

**Configuration**:
```json
{
  "clientId": "faultiq-ui",
  "publicClient": true,
  "directAccessGrantsEnabled": true,
  "redirectUris": [
    "http://localhost:4001/*",
    "http://localhost:3000/*"
  ],
  "webOrigins": ["http://localhost:4001", "http://localhost:3000"]
}
```

**Protocol Mappers** (Token Claims):
1. **tenant** - Maps `tenant` attribute to token
2. **projects** - Maps `projects` attribute to token  
3. **roles** - Maps realm roles to token

**Usage**:
```javascript
// In frontend, after login:
const token = keycloak.token;
const decoded = jwt_decode(token);
console.log(decoded.tenant);    // "acme-corp"
console.log(decoded.projects);  // "payments-platform,orders-platform"
console.log(decoded.roles);     // ["super_admin"]
```

### 2. `faultiq-backend` (Service-to-Service)

**Type**: Confidential Client  
**Purpose**: Backend services calling each other

**Configuration**:
```json
{
  "clientId": "faultiq-backend",
  "publicClient": false,
  "directAccessGrantsEnabled": true,
  "serviceAccountsEnabled": true,
  "clientSecret": "faultiq-backend-secret-change-in-production"
}
```

**Usage** (for future service-to-service calls):
```bash
# Get service account token
curl -X POST http://localhost:8081/realms/faultiq/protocol/openid-connect/token \
  -d 'client_id=faultiq-backend' \
  -d 'client_secret=faultiq-backend-secret-change-in-production' \
  -d 'grant_type=client_credentials'
```

---

## Roles

### Role Hierarchy

```
super_admin
  ↓ (Full access, can grant other roles)
  ├─ tenant_admin
  │   ├─ project_admin
  │   └─ analyst
  │       ├─ viewer
  │       └─ signal_publisher
```

### Role Descriptions

| Role | Access Level | Permissions | Use Case |
|------|---|---|---|
| **super_admin** | Global | Create tenants, manage all projects, user admin | Platform administrators |
| **tenant_admin** | Tenant | Create projects, manage team, view incidents | Tenant admin users |
| **project_admin** | Project | Manage graph, incidents, recommendations | Project leads |
| **analyst** | Project | Analyze incidents, view graphs, submit feedback | On-call engineers |
| **viewer** | Project | Read-only access to incidents and graphs | Stakeholders, auditors |
| **signal_publisher** | API | Submit signals/metrics only | Monitoring systems, CI/CD |

---

## Users & Credentials

### Pre-configured Test Users

After Keycloak realm import, the following users are available:

#### 1. Super Admin
```
Username: super
Password: superpass
Roles: [super_admin]
Tenant: acme-corp
Projects: payments-platform, orders-platform
```

**Permissions**:
- Create/manage all tenants
- Create/manage all projects
- Manage all users
- View all incidents across platform

#### 2. Organization Admin
```
Username: org-admin
Password: orgadminpass
Roles: [tenant_admin]
Tenant: acme-corp
Projects: payments-platform, orders-platform, analytics-platform
```

**Permissions**:
- Create projects within tenant
- Manage team members
- Manage graph for tenant projects
- View all incidents in tenant

#### 3. Project Admin
```
Username: project-admin
Password: projadminpass
Roles: [project_admin]
Tenant: acme-corp
Projects: payments-platform (only)
```

**Permissions**:
- Manage graph topology
- View/manage incidents
- View recommendations
- Manage playbooks

#### 4. Analyst
```
Username: analyst
Password: analystpass
Roles: [analyst]
Tenant: acme-corp
Projects: payments-platform, orders-platform
```

**Permissions**:
- View incidents
- View graphs
- Analyze root causes
- Submit feedback

#### 5. Viewer
```
Username: viewer
Password: viewerpass
Roles: [viewer]
Tenant: acme-corp
Projects: payments-platform (only)
```

**Permissions**:
- View incidents (read-only)
- View graphs (read-only)

#### 6. Signal Publisher
```
Username: signal-publisher
Password: signalpublisherpass
Roles: [signal_publisher]
Tenant: acme-corp
Projects: payments-platform, orders-platform, analytics-platform
```

**Permissions**:
- Submit signals/metrics via API
- No console access

---

## Login Flow

### Web Browser Login

```
1. User visits http://localhost:4001 (Frontend)
   │
2. Frontend checks for token
   └─ No token found → Redirect to login
   
3. Frontend initiates Keycloak login
   ├─ POST to http://localhost:8081/realms/faultiq/protocol/openid-connect/auth
   └─ Parameters: client_id=faultiq-ui, redirect_uri=http://localhost:4001/
   
4. Keycloak login page displayed
   ├─ User enters username/password
   └─ User grants consent
   
5. Keycloak redirects to callback
   ├─ URL: http://localhost:4001/?code=...&state=...
   └─ Frontend captures code
   
6. Frontend exchanges code for token
   ├─ POST to http://localhost:8081/realms/faultiq/protocol/openid-connect/token
   ├─ Parameters: code, client_id, grant_type=authorization_code
   └─ Response: {access_token, refresh_token, ...}
   
7. Frontend stores token
   ├─ Sets in localStorage or session storage
   └─ Adds to API headers: Authorization: Bearer $token
   
8. Frontend requests user projects
   ├─ GET http://localhost:8080/api/v1/projects
   ├─ Header: Authorization: Bearer $token
   └─ API validates token with Keycloak and returns filtered projects
   
9. User dashboard displayed
   └─ Shows only projects from token.projects claim
```

### Token Contents

After successful login, the JWT token contains:

```javascript
{
  "jti": "...",
  "exp": 1621234567,
  "iat": 1621230967,
  "iss": "http://localhost:8081/realms/faultiq",
  "aud": "faultiq-ui",
  "sub": "user-id-uuid",
  "typ": "Bearer",
  "azp": "faultiq-ui",
  "session_state": "...",
  "name": "Super Admin",
  "preferred_username": "super",
  "given_name": "Super",
  "family_name": "Admin",
  "email": "super@faultiq.local",
  "email_verified": true,
  
  // Custom claims from protocol mappers:
  "tenant": "acme-corp",
  "projects": "payments-platform,orders-platform",
  "roles": ["super_admin"],
  
  // Standard OIDC claims:
  "realm_access": {
    "roles": ["super_admin"]
  }
}
```

---

## API Token Validation

### API Gateway Flow

When API receives a request with Bearer token:

```go
// 1. Extract token from header
Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5...

// 2. Validate signature using JWKS
// Fetches keys from: http://keycloak:8080/realms/faultiq/protocol/openid-connect/certs

// 3. Check token expiration
if token.Exp < time.Now() {
  return 401 Unauthorized
}

// 4. Extract claims
tenant := token.Claims["tenant"]      // "acme-corp"
projects := token.Claims["projects"]  // "payments-platform,orders-platform"
roles := token.Claims["roles"]        // ["super_admin"]
userId := token.Sub                   // UUID

// 5. Enforce access control
if project not in projects {
  return 403 Forbidden
}

// 6. Forward to downstream service with headers
X-Tenant-ID: acme-corp
X-Project-IDs: payments-platform,orders-platform
X-User-Roles: super_admin
X-User-ID: user-id-uuid
```

---

## Adding New Users

### Keycloak Admin Console

1. Open http://localhost:8181
2. Admin Console → Choose Realm: `faultiq`
3. Users → Create User
4. Fill in:
   - Username: `new-user`
   - Email: `user@example.com`
   - First Name, Last Name
5. Set Attributes:
   - Key: `tenant` → Value: `acme-corp`
   - Key: `projects` → Value: `payments-platform`
6. Assign Roles:
   - Go to Role Mappings → Assign Roles → Select `analyst`
7. Set Password:
   - Credentials tab → Set Password
   - Toggle "Temporary" to OFF

### Via Realm JSON

Edit `scripts/keycloak/realm.json`:

```json
{
  "users": [
    {
      "username": "new-user",
      "enabled": true,
      "firstName": "New",
      "lastName": "User",
      "email": "new@example.com",
      "emailVerified": true,
      "credentials": [
        {
          "type": "password",
          "value": "newpassword",
          "temporary": false
        }
      ],
      "realmRoles": ["analyst"],
      "attributes": {
        "tenant": "acme-corp",
        "projects": "payments-platform,orders-platform"
      }
    }
  ]
}
```

Then restart Keycloak:
```bash
docker-compose restart keycloak
```

---

## Modifying Roles

### Adding Permission to Role

1. Admin Console → Realm Roles → Choose Role
2. Add permissions by editing role description
3. Changes apply to all users with that role immediately

### Assigning Role to User

1. Users → Choose User → Role Mappings
2. Available Roles → Select role → Add selected

### Removing Role from User

1. Users → Choose User → Role Mappings  
2. Assigned Roles → Select role → Remove selected

---

## Project Access Control

### How Projects are Filtered

1. **Backend receives request**
   ```
   GET /api/v1/projects
   Authorization: Bearer eyJ...
   ```

2. **Token is validated, claims extracted**
   ```
   projects: "payments-platform,orders-platform"
   ```

3. **Backend filters projects**
   ```sql
   SELECT * FROM projects 
   WHERE slug IN ('payments-platform', 'orders-platform')
   AND tenant_id = 'acme-corp'
   ```

4. **Only accessible projects returned**
   ```json
   [
     { "id": "...", "name": "Payments Platform" },
     { "id": "...", "name": "Orders Platform" }
   ]
   ```

### Adding User to Project

1. Edit user in Keycloak Admin Console
2. Go to Attributes
3. Update `projects` attribute:
   ```
   Before: payments-platform
   After:  payments-platform,orders-platform
   ```
4. Save → User now has access to new project

---

## Troubleshooting

### Issue: "Invalid grant" when logging in
**Cause**: Wrong password or user doesn't exist  
**Fix**: 
```bash
# Check user exists in Keycloak
curl http://localhost:8081/admin/realms/faultiq/users \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Reset user password via Admin Console
```

### Issue: Token is missing `projects` claim
**Cause**: Protocol mapper not active or user attributes not set  
**Fix**:
1. Verify `projects` protocol mapper in realm.json
2. Check user has `projects` attribute set
3. Re-import realm:
   ```bash
   docker-compose restart keycloak
   ```

### Issue: "Invalid redirect URI"
**Cause**: Redirect URI not in whitelist  
**Fix**:
1. Admin Console → Clients → faultiq-ui
2. Valid Redirect URIs → Add missing URL
3. If running on different port, update to:
   ```
   http://localhost:YOUR_PORT/*
   http://localhost:YOUR_PORT/
   ```

### Issue: Keycloak not starting
**Cause**: Admin password error or import failure  
**Fix**:
```bash
# Check logs
docker logs $(docker ps --filter "name=keycloak" -q)

# Verify realm.json is valid JSON
jq . scripts/keycloak/realm.json

# Recreate container
docker-compose down
docker volume rm faultiq_keycloak_data
docker-compose up keycloak
```

---

## Testing Users

### Test Login as Each Role

```bash
# Test super admin
curl -X POST http://localhost:8081/realms/faultiq/protocol/openid-connect/token \
  -d 'client_id=faultiq-ui' \
  -d 'username=super' \
  -d 'password=superpass' \
  -d 'grant_type=password' \
  | jq .access_token

# Test org admin  
curl -X POST http://localhost:8081/realms/faultiq/protocol/openid-connect/token \
  -d 'client_id=faultiq-ui' \
  -d 'username=org-admin' \
  -d 'password=orgadminpass' \
  -d 'grant_type=password' \
  | jq .access_token

# Decode and verify token
echo $TOKEN | cut -d'.' -f2 | base64 -D | jq .
```

### Test API Access with Token

```bash
# Get token
TOKEN=$(curl -s -X POST http://localhost:8081/realms/faultiq/protocol/openid-connect/token \
  -d 'client_id=faultiq-ui' \
  -d 'username=analyst' \
  -d 'password=analystpass' \
  -d 'grant_type=password' | jq -r .access_token)

# Access API
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/projects

# Should return only projects user has access to
```

---

## Security Notes

⚠️ **Development Only**:
- Admin password is `admin` (change in production!)
- Users have hardcoded passwords (use generated ones in production)
- Passwords are stored in git (use Kubernetes Secrets in production)

✅ **Production Recommendations**:
1. Change all default passwords via `.env.production`
2. Use K8s Secrets for credential storage
3. Enable HTTPS for Keycloak
4. Configure SMTP for email verification
5. Set up backup/restore for Keycloak database
6. Monitor token expiration and refresh rates
