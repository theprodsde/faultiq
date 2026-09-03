package main

import (
    "context"
    "crypto/rsa"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "math/big"
    "net/http"
    "os"
    "strings"
    "sync"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "github.com/prometheus/client_golang/prometheus"
)

type contextKey string

const ctxUserKey contextKey = "user"

// JWKSManager handles fetching and refreshing JWKS from a URL.
type JWKSManager struct {
    url     string
    mu      sync.RWMutex
    keys    map[string]map[string]interface{}
    ticker  *time.Ticker
    stopCh  chan struct{}
    // configured refresh interval
    refreshInterval time.Duration
    // last successful refresh time
    lastSuccessful time.Time
    // metrics
    fetchSuccess prometheus.Counter
    fetchFailure prometheus.Counter
    lastRefresh  prometheus.Gauge
}

func NewJWKSManager(url string, refreshInterval time.Duration) *JWKSManager {
    m := &JWKSManager{url: url, keys: make(map[string]map[string]interface{}), stopCh: make(chan struct{})}
    m.refreshInterval = refreshInterval
    // init metrics
    m.fetchSuccess = prometheus.NewCounter(prometheus.CounterOpts{Name: "jwks_fetch_success_total", Help: "Total successful JWKS fetches"})
    m.fetchFailure = prometheus.NewCounter(prometheus.CounterOpts{Name: "jwks_fetch_failure_total", Help: "Total failed JWKS fetch attempts"})
    m.lastRefresh = prometheus.NewGauge(prometheus.GaugeOpts{Name: "jwks_last_refresh_timestamp", Help: "Last successful JWKS refresh as unix timestamp"})
    // register metrics (ignore errs)
    _ = prometheus.Register(m.fetchSuccess)
    _ = prometheus.Register(m.fetchFailure)
    _ = prometheus.Register(m.lastRefresh)
    // initial fetch with retry/backoff
    go func() {
        backoff := time.Second
        for i := 0; i < 6; i++ {
            if err := m.fetch(); err == nil {
                break
            } else {
                Infof("jwks initial fetch attempt %d failed: %v", i+1, err)
            }
            time.Sleep(backoff)
            backoff *= 2
        }
    }()
    if refreshInterval <= 0 {
        refreshInterval = time.Hour
    }
    m.ticker = time.NewTicker(refreshInterval)
    go func() {
        for {
            select {
            case <-m.ticker.C:
                // attempt fetch with simple backoff on failure
                if err := m.fetch(); err != nil {
                    m.fetchFailure.Inc()
                    Infof("jwks refresh error, will backoff: %v", err)
                    // exponential backoff loop until next successful fetch or cap
                    b := time.Second
                    for i := 0; i < 6; i++ {
                        select {
                        case <-m.stopCh:
                            return
                        case <-time.After(b):
                        }
                        if err := m.fetch(); err == nil {
                            break
                        }
                        b *= 2
                        if b > 30*time.Second {
                            b = 30 * time.Second
                        }
                    }
                }
            case <-m.stopCh:
                return
            }
        }
    }()
    return m
}

func (m *JWKSManager) Stop() {
    if m.ticker != nil {
        m.ticker.Stop()
    }
    close(m.stopCh)
}

func (m *JWKSManager) fetch() error {
    resp, err := http.Get(m.url)
    if err != nil {
        if m != nil && m.fetchFailure != nil {
            m.fetchFailure.Inc()
        }
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        b, _ := io.ReadAll(resp.Body)
        if m != nil && m.fetchFailure != nil {
            m.fetchFailure.Inc()
        }
        return fmt.Errorf("jwks fetch status %d: %s", resp.StatusCode, string(b))
    }
    var data struct{ Keys []map[string]interface{} `json:"keys"` }
    if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
        return err
    }
    tmp := make(map[string]map[string]interface{})
    for _, k := range data.Keys {
        if kid, ok := k["kid"].(string); ok {
            tmp[kid] = k
        }
    }
    m.mu.Lock()
    m.keys = tmp
    m.lastSuccessful = time.Now()
    m.mu.Unlock()
    Infof("jwks fetched %d keys from %s", len(tmp), m.url)
    if m != nil && m.fetchSuccess != nil && m.lastRefresh != nil {
        m.fetchSuccess.Inc()
        m.lastRefresh.Set(float64(time.Now().Unix()))
    }
    return nil
}

// Healthy reports whether the JWKS manager has a recent successful refresh.
func (m *JWKSManager) Healthy() bool {
    if m == nil {
        return false
    }
    m.mu.RLock()
    defer m.mu.RUnlock()
    if m.lastSuccessful.IsZero() {
        return false
    }
    // consider healthy if last successful refresh within 5x refresh interval (or 10m if unset)
    capDur := 10 * time.Minute
    if m.refreshInterval > 0 {
        capDur = m.refreshInterval * 5
    }
    return time.Since(m.lastSuccessful) < capDur
}

func (m *JWKSManager) GetKey(kid string) (map[string]interface{}, bool) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    k, ok := m.keys[kid]
    return k, ok
}

// RequireAuth validates incoming requests using JWKS (if configured) or Keycloak userinfo fallback.
func RequireAuth(next http.Handler) http.Handler {
    // Use the global JWKS manager initialized at startup (if any)
    var mgr *JWKSManager = globalJWKS

    userinfoURL := "http://keycloak:8080/realms/faultiq/protocol/openid-connect/userinfo"

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // API key check — for CI/CD machine-to-machine auth (no browser flow needed)
        if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
            expectedKey := os.Getenv("API_KEY_SECRET")
            if expectedKey != "" && apiKey == expectedKey {
                // Synthetic claims for CI/CD service accounts
                claims := map[string]interface{}{
                    "sub":    "ci-cd-service-account",
                    "email":  "ci-cd@system",
                    "tenant": os.Getenv("DEFAULT_TENANT"),
                    "roles":  []string{"service_account"},
                }
                ctx := context.WithValue(r.Context(), ctxUserKey, claims)
                next.ServeHTTP(w, r.WithContext(ctx))
                return
            } else if expectedKey != "" {
                http.Error(w, "invalid API key", http.StatusUnauthorized)
                return
            }
        }

        auth := r.Header.Get("Authorization")
        if auth == "" {
            http.Error(w, "missing Authorization header", http.StatusUnauthorized)
            return
        }
        parts := strings.SplitN(auth, " ", 2)
        if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
            http.Error(w, "invalid Authorization header", http.StatusUnauthorized)
            return
        }
        raw := parts[1]

        // JWKS verification path
        if mgr != nil {
            token, err := jwt.Parse(raw, func(t *jwt.Token) (interface{}, error) {
                kid, _ := t.Header["kid"].(string)
                Debugf("jwks: token kid header=%s", kid)
                if kid == "" {
                    Debugf("jwks: token missing kid header")
                    return nil, fmt.Errorf("no kid in token header")
                }
                keyEntry, ok := mgr.GetKey(kid)
                if !ok {
                    Infof("jwks: kid %s not found in jwks cache", kid)
                    return nil, fmt.Errorf("kid not found")
                }
                kty, _ := keyEntry["kty"].(string)
                if kty != "RSA" {
                    Infof("jwks: unsupported kty: %s", kty)
                    return nil, fmt.Errorf("unsupported kty: %s", kty)
                }
                nStr, _ := keyEntry["n"].(string)
                eStr, _ := keyEntry["e"].(string)
                nBytes, err := base64.RawURLEncoding.DecodeString(nStr)
                if err != nil {
                    return nil, err
                }
                eBytes, err := base64.RawURLEncoding.DecodeString(eStr)
                if err != nil {
                    return nil, err
                }
                n := new(big.Int).SetBytes(nBytes)
                e := 0
                for _, b := range eBytes {
                    e = e<<8 + int(b)
                }
                return &rsa.PublicKey{N: n, E: e}, nil
            })
            if err != nil || !token.Valid {
                Errorf("token validation error: %v; tokenValid=%v", err, token != nil && token.Valid)
                http.Error(w, fmt.Sprintf("invalid token: %v", err), http.StatusUnauthorized)
                return
            }
            if c, ok := token.Claims.(jwt.MapClaims); ok {
                claims := map[string]interface{}(c)

                // Optional issuer/audience validation (configure via env)
                if expIss := os.Getenv("AUTH_ISSUER"); expIss != "" {
                    if iss, _ := claims["iss"].(string); iss != expIss {
                        Infof("token issuer mismatch: got=%s expected=%s", iss, expIss)
                        http.Error(w, "invalid token issuer", http.StatusUnauthorized)
                        return
                    }
                }
                if expAud := os.Getenv("AUTH_AUDIENCE"); expAud != "" {
                    audClaim := claims["aud"]
                    audOk := false
                    switch v := audClaim.(type) {
                    case string:
                        audOk = (v == expAud)
                    case []interface{}:
                        for _, it := range v {
                            if s, ok := it.(string); ok && s == expAud {
                                audOk = true
                                break
                            }
                        }
                    }
                    if !audOk {
                        Infof("token audience mismatch: got=%v expected=%s", audClaim, expAud)
                        http.Error(w, "invalid token audience", http.StatusUnauthorized)
                        return
                    }
                }

                ctx := context.WithValue(r.Context(), ctxUserKey, claims)
                next.ServeHTTP(w, r.WithContext(ctx))
                return
            }
            http.Error(w, "invalid token claims", http.StatusUnauthorized)
            return
        }

        // Fallback: call Keycloak userinfo endpoint
        req, _ := http.NewRequestWithContext(r.Context(), "GET", userinfoURL, nil)
        req.Header.Set("Authorization", "Bearer "+raw)
        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            http.Error(w, fmt.Sprintf("auth check failed: %v", err), http.StatusServiceUnavailable)
            return
        }
        defer resp.Body.Close()
        if resp.StatusCode != http.StatusOK {
            b, _ := io.ReadAll(resp.Body)
            http.Error(w, fmt.Sprintf("invalid token: %s", string(b)), http.StatusUnauthorized)
            return
        }
        var claims map[string]interface{}
        if err := json.NewDecoder(resp.Body).Decode(&claims); err != nil {
            http.Error(w, "invalid token claims", http.StatusUnauthorized)
            return
        }
        ctx := context.WithValue(r.Context(), ctxUserKey, claims)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}

// RequireRole enforces that the token claims include the given realm role.
func RequireRole(role string, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        claims := GetClaims(r)
        if claims == nil {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        if hasRole(claims, "super_admin") {
            next.ServeHTTP(w, r)
            return
        }
        if hasRole(claims, role) {
            next.ServeHTTP(w, r)
            return
        }
        http.Error(w, "forbidden", http.StatusForbidden)
    })
}

// RequireTenant enforces that the token contains the tenant claim matching the provided tenant id.
func RequireTenant(tenant string, next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        claims := GetClaims(r)
        if claims == nil {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        if v, ok := claims["tenant"].(string); ok {
            if v == tenant || hasRole(claims, "super_admin") {
                next.ServeHTTP(w, r)
                return
            }
        }
        http.Error(w, "forbidden - tenant mismatch", http.StatusForbidden)
    })
}

func hasRole(claims map[string]interface{}, role string) bool {
    // Keycloak may put roles under realm_access.roles
    if ra, ok := claims["realm_access"].(map[string]interface{}); ok {
        if rr, ok := ra["roles"].([]interface{}); ok {
            for _, r := range rr {
                if s, ok := r.(string); ok && s == role {
                    return true
                }
            }
        }
    }
    // fallback: top-level roles array
    if rr, ok := claims["roles"].([]interface{}); ok {
        for _, r := range rr {
            if s, ok := r.(string); ok && s == role {
                return true
            }
        }
    }
    return false
}

// GetClaims returns claims from request context
func GetClaims(r *http.Request) map[string]interface{} {
    v := r.Context().Value(ctxUserKey)
    if v == nil {
        return nil
    }
    if m, ok := v.(map[string]interface{}); ok {
        return m
    }
    return nil
}
