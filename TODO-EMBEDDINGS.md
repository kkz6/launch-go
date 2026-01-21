# Go Struct Embedding Consolidation Plan

A comprehensive plan for leveraging Go's struct embedding to reduce code duplication, following Laravel's trait-like composition patterns.

**Generated:** 2026-01-21
**Updated:** 2026-01-21
**Focus:** Struct embedding & composition patterns (NOT covered in TODO-REFACTORING.md)
**Estimated LOC Reduction:** 4000+ lines

---

## Completed Infrastructure (Phase 1)

The following foundational infrastructure has been implemented:

| # | Work Package | Status | Commit |
|---|-------------|--------|--------|
| 46 | GitLab Webhook Timing Attack Fix | ✅ DONE | `4c7a6b3` |
| 47 | Broadcast Event Constants | ✅ DONE | (in broadcast/events.go) |
| 49 | Crypto Package Consolidation | ✅ DONE | `4c7a6b3` - cryptoutil/ |
| 51 | HTTP Client Base with Request Builder | ✅ DONE | `f14f33f` - httpclient/ |
| 63 | Error Type Consolidation | ✅ DONE | `fde7a4f` - errors/ |
| 75 | GORM Query Scope - Order By Latest | ✅ DONE | `ed049ad` - repository/scopes.go |
| 79 | Job Struct Boilerplate | ✅ DONE | `d699376` - jobs/builder.go, payload.go |
| 81 | DTO Timestamp Formatter | ✅ DONE | `f14f33f` - dto/response.go |
| 106 | Enum Boilerplate Consolidation | ✅ DONE | `065e17c` - enums/ |
| 108 | Slice Transformation Generic Helper | ✅ DONE | `de817a9` - utils/slice.go |
| 109 | Safe Map Access Helper | ✅ DONE | `de817a9` - utils/map.go |
| 115 | Nil-Safe Dereference Helpers | ✅ DONE | (already in ptr/) |
| 148 | Generic Slice Mapper | ✅ DONE | `de817a9` - utils/slice.go |
| 149 | Enum Response Helper | ✅ DONE | dto/enum.go |
| 150 | Pointer Dereference Utilities | ✅ DONE | (already in ptr/) |
| 151 | URL Builder Package | ✅ DONE | (already in urlbuilder/) |
| 152 | Generic UpdateStatus Repository Method | ✅ DONE | `ed049ad` - repository/base.go |
| 154 | Preload Scope Helpers | ✅ DONE | `ed049ad` - repository/scopes.go |
| - | Model Mixins (Named, Described, etc.) | ✅ DONE | `44808a1` - models/mixins.go |
| - | Constants/Limits | ✅ DONE | `1cf762a` - constants/limits.go |
| 1 | Job Base Payload Generic | ✅ DONE | `13ebce5` - jobs/context.go BaseJob[C,P] |
| 2 | Installable Repository Mixin | ✅ DONE | `5a998b7` - Go embedding promotes methods |
| 3 | Queue Dispatch Unification | ✅ DONE | `81a797c` - jobs.Base.Dispatch* methods |
| 4 | Pagination Embedding | ✅ DONE | repository.Paginate, PaginatedResult[T] |
| 5 | Task Builder Pattern | ✅ DONE | taskrunner.TaskBuilder[C], BuiltTask[C] |
| 61 | Broadcast Channel Builder | ✅ DONE | (already in broadcast/) |
| 62 | Pagination Query Helpers | ✅ DONE | `fb70324` - pagination/limit.go |
| 64 | WebSocket SSH Client Consolidation | ✅ DONE | `fb70324` - taskrunner/ssh_client.go |
| 65 | Template Engine Consolidation | ✅ DONE | `fb70324` - template/base.go |
| 66 | Slack Admin Alert Base | ✅ DONE | `fb70324` - slack/base_alert.go |
| 67 | Notification Format Helper | ✅ DONE | `fb70324` - notifications/formatter.go |
| 68 | Webhook Signature Verification | ✅ DONE | `fb70324` - webhook/base.go |
| 69 | Route Middleware Chain Helper | ✅ DONE | `fb70324` - middleware/chain.go |
| 70 | Module ServiceDeps Generic | ✅ DONE | `fb70324` - service/module.go |
| 71 | JobContext Generic Base | ✅ DONE | `fd83937` - jobs/module_context.go |
| 72 | Token Generation Consolidation | ✅ DONE | `fd83937` - removed duplicates |
| 73 | Git Ref Trimming Helper | ✅ DONE | `fd83937` - gitref/gitref.go |
| 74 | Channel Validation Rules | ✅ DONE | `fd83937` - channels/validation.go |
| 75 | GORM Query Scope | ✅ DONE | (already in repository/scopes.go) |
| 76 | Sync Associations Transaction | ✅ DONE | `fd83937` - repository/associations.go |
| 77 | WebSocket Handler Base | ✅ DONE | (already implemented) |
| 78 | WebSocket Message Builder | ✅ DONE | `fd83937` - handlers/base.go |
| 79 | Job Struct Boilerplate | ✅ DONE | (already in jobs/builder.go) |
| 80 | Cloud Provider HTTP Client | ✅ DONE | `fd83937` - dns providers updated |
| 82 | DTO List Transformation | ✅ DONE | dto/converter.go TransformSlice |
| 83 | Activity Logging Helper | ✅ DONE | activity/mixin.go ActivityMixin |
| 84 | ParseAndValidate Adoption | ✅ DONE | handlers updated to use fiberctx |
| 85 | MockRepository Generic | ✅ DONE | testutil/mock_store.go |
| 86 | Email Normalization | ✅ DONE | auth/dto uses pkgdto helpers |
| 87 | Request Context Extraction | ✅ DONE | fiber/context.go (already 90% done) |
| 88 | Repository Authorization | ✅ DONE | repository/base.go FindByIDAndTeam |
| 89 | Preload Chain Consolidation | ✅ DONE | repository/preloads.go PreloadConfig |
| 90 | Service BaseService Accessor | ✅ DONE | service/module.go ModuleBase |
| 91 | Retry Logic Consolidation | ✅ DONE | retry/retry.go WithBackoff |
| 92 | HTTP Client Factory | ✅ DONE | httpclient/client.go Default() |
| 93 | JobContext Field Duplication | ✅ DONE | server/jobs/context.go cleaned |
| 94 | Double-Check Lock Pattern | ✅ DONE | syncutil/cached.go Cached[T] |
| 95 | NotifierFake Lock Wrapper | ✅ DONE | testing/notifier_fake.go withLock |
| 96 | ServiceRegistry Boilerplate | ✅ SKIP | Idiomatic Go, provides type safety |
| 97 | Soft Delete Standardization | ✅ DONE | models/archivable.go ArchivableModel |
| 98 | Timeout Configuration | ✅ DONE | timeout/config.go |
| 99 | AppError Type Consolidation | ✅ DONE | response/errors.go uses pkg/errors |
| 100 | Validation Field Embedding | ✅ DONE | dto/fields.go embeddable structs |
| 101 | Handler Validation Flow | ✅ SKIP | Same as Item 84 (ParseAndValidate) |
| 102 | Broadcaster Interface | ✅ DONE | broadcast/model_event.go |
| 103 | Model Broadcast Payload | ✅ DONE | broadcast/model_event.go |
| 104 | Logger Field Iteration | ✅ DONE | logger/fields.go |
| 105 | BeforeCreate Status Init | ✅ DONE | models/status.go SetDefaultStatus |
| 107 | Repository Registry Factory | ✅ SKIP | Idiomatic Go, provides type safety |
| 110 | Collection Filter-Map | ✅ DONE | xutil/slice.go FilterTransform |
| 111-114 | Route/Handler/Webhook/Template | ✅ SKIP | Already done or too intrusive |
| 116 | JSONMap Type | ✅ DONE | dbtype/json.go |
| 117 | Enum SQL Scanner | ✅ DONE | enums/scanner.go |
| 118 | SSH Stream Reader | ✅ SKIP | Already in taskrunner/ssh_client.go |
| 119 | Home Directory Helper | ✅ DONE | serverpath/path.go |
| 120 | HMAC Signature Validator | ✅ DONE | signature/hmac.go |
| 121 | Token Generator | ✅ DONE | token/generator.go |
| 122 | Time Expiration Checker | ✅ DONE | timeutil/expiration.go |
| 123-125 | HTTP Client Patterns | ✅ SKIP | Already done in httpclient/ |
| 126 | Pagination Parameter Parser | ✅ DONE | fiberutil/params.go |
| 127 | Dynamic Sort Handler | ✅ DONE | repository/sort.go |
| 128 | Filter Options Pattern | ✅ DONE | repository/filter.go |
| 129 | Retry Strategy | ✅ SKIP | Already done (Item 91) |
| 130 | Metrics Parsing Helper | ✅ DONE | metrics/parser.go |
| 131 | Service Status Checker | ✅ DONE | status/format.go |
| 132 | Daemon Status Task | ✅ SKIP | Module-specific, kept separate |
| 133 | Health Check Aggregator | ✅ DONE | health/health.go |
| 134 | Config Loader Helper | ✅ DONE | configutil/loader.go LoadInto |
| 135 | Webhook Channel Base | ✅ DONE | channels/webhook_base.go |
| 136 | Admin Alert Base | ✅ SKIP | Already done (Item 66) |
| 137 | SSH Client Factory | ✅ SKIP | Already in taskrunner/ssh_client.go |
| 138 | Cron Schedule Constants | ✅ DONE | enums/cron_schedule.go |
| 139-140 | Job Task Builder/Context | ✅ SKIP | Already done (Items 71, 79) |

---

## Table of Contents

*All embedding tasks completed.*

---

## Implementation Phases

### Phase 1: High Priority (Week 1-2)
| Item | Priority | Est. LOC Saved | Complexity |
|------|----------|----------------|------------|
| WebSocket Handler Base | P1 | 75 | Low |
| Webhook Handler Base | P1 | 40 | Low |
| Repository BaseRepository | P1 | 450 | Medium |
| Service Base Enforcement | P1 | 200 | Medium |
| Module Handler Fields | P1 | 150 | Low |

### Phase 2: Medium Priority (Week 3-4)
| Item | Priority | Est. LOC Saved | Complexity |
|------|----------|----------------|------------|
| Job Base Payload | P2 | 100 | Medium |
| Installable Repository Mixin | P2 | 200 | Medium |
| Queue Dispatch Unification | P2 | 100 | Medium |
| Pagination Embedding | P2 | 50 | Low |
| Task Builder Pattern | P2 | 600 | High |

### Phase 3: Lower Priority (Week 5-6)
| Item | Priority | Est. LOC Saved | Complexity |
|------|----------|----------------|------------|
| Model Scoped Fields | P2 | 100 | Low |
| Activity Logging Builder | P2 | 150 | Medium |
| Broadcast Payload Builder | P3 | 30 | Low |
| DTO Timestamp Embedding | P3 | 400 | Medium |

---

## Success Metrics

After implementing these embeddings:

1. **Code Reduction:** ~2500 lines eliminated
2. **Consistency:** All modules follow identical patterns
3. **Type Safety:** Compile-time checks for embedded fields
4. **Maintainability:** Single point of change for common functionality
5. **Discoverability:** IDE autocomplete works with embedded methods
6. **Testing:** Base classes can be unit tested once

---

## Notes

- Always run tests after each embedding change
- Verify GORM handles embedded fields correctly
- Check JSON serialization of embedded structs
- Update CLAUDE.md with new embedding patterns
- Consider backward compatibility for API responses


## 121. Token Generator Consolidation (P2)

**Problem:** Multiple token generation patterns with inconsistent entropy and formatting.

**Files affected:**
- `internal/modules/auth/services/auth_service.go:125-152` - API tokens
- `internal/modules/server/services/provision_service.go:88-108` - Callback tokens
- `internal/modules/site/services/deployment_service.go:165-182` - Deploy keys
- `internal/modules/git/services/webhook_service.go:72-88` - Webhook secrets
- `internal/modules/backup/services/backup_service.go:95-110` - Encryption keys

**Current (repeated 5+ times):**
```go
// Pattern 1: Random hex token
func generateToken() string {
    b := make([]byte, 32)
    rand.Read(b)
    return hex.EncodeToString(b)
}

// Pattern 2: URL-safe base64
func generateURLSafeToken() string {
    b := make([]byte, 32)
    rand.Read(b)
    return base64.URLEncoding.EncodeToString(b)
}

// Pattern 3: Prefixed token (like Stripe)
func generateAPIToken() string {
    b := make([]byte, 24)
    rand.Read(b)
    return "lnch_" + base64.RawURLEncoding.EncodeToString(b)
}

// Pattern 4: Short numeric code
func generateVerificationCode() string {
    n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
    return fmt.Sprintf("%06d", n.Int64())
}
```

**Solution - Unified Token Generator:**
```go
// internal/pkg/token/generator.go
package token

import (
    "crypto/rand"
    "encoding/base64"
    "encoding/hex"
    "fmt"
    "math/big"
    "strings"
)

type Encoding int

const (
    Hex Encoding = iota
    Base64
    Base64URL
    Base64URLRaw
)

type Generator struct {
    length   int
    encoding Encoding
    prefix   string
}

func New(byteLength int) *Generator {
    return &Generator{
        length:   byteLength,
        encoding: Hex,
    }
}

func (g *Generator) WithEncoding(e Encoding) *Generator {
    g.encoding = e
    return g
}

func (g *Generator) WithPrefix(prefix string) *Generator {
    g.prefix = prefix
    return g
}

func (g *Generator) Generate() (string, error) {
    b := make([]byte, g.length)
    if _, err := rand.Read(b); err != nil {
        return "", err
    }

    var encoded string
    switch g.encoding {
    case Hex:
        encoded = hex.EncodeToString(b)
    case Base64:
        encoded = base64.StdEncoding.EncodeToString(b)
    case Base64URL:
        encoded = base64.URLEncoding.EncodeToString(b)
    case Base64URLRaw:
        encoded = base64.RawURLEncoding.EncodeToString(b)
    }

    if g.prefix != "" {
        return g.prefix + encoded, nil
    }
    return encoded, nil
}

func (g *Generator) MustGenerate() string {
    t, err := g.Generate()
    if err != nil {
        panic(err)
    }
    return t
}

// Convenience functions for common token types
func APIToken() string {
    return New(24).WithEncoding(Base64URLRaw).WithPrefix("lnch_").MustGenerate()
}

func WebhookSecret() string {
    return New(32).WithEncoding(Hex).MustGenerate()
}

func CallbackToken() string {
    return New(16).WithEncoding(Base64URLRaw).MustGenerate()
}

func VerificationCode(digits int) string {
    max := int64(1)
    for i := 0; i < digits; i++ {
        max *= 10
    }
    n, _ := rand.Int(rand.Reader, big.NewInt(max))
    return fmt.Sprintf("%0*d", digits, n.Int64())
}

func EncryptionKey() []byte {
    key := make([]byte, 32)
    rand.Read(key)
    return key
}
```

**Impact:** ~150 lines saved, consistent token generation

---

## 122. Time Expiration Checker (P2)

**Problem:** 8+ expiration check patterns repeated across auth, billing, and cache logic.

**Files affected:**
- `internal/modules/auth/services/token_service.go:85-112` - Token expiration
- `internal/modules/auth/services/password_reset_service.go:62-78` - Reset link expiration
- `internal/modules/billing/services/subscription_service.go:145-175` - Subscription status
- `internal/modules/server/services/provision_service.go:182-198` - Callback expiration
- `internal/modules/site/services/ssl_service.go:125-142` - Certificate expiration
- `internal/modules/backup/services/backup_service.go:188-205` - Retention policy
- `internal/pkg/cache/cache.go:75-92` - Cache TTL checks
- `internal/modules/auth/services/session_service.go:55-72` - Session expiration

**Current (repeated 8+ times):**
```go
// Pattern 1: Simple expiration check
func isExpired(expiresAt time.Time) bool {
    return time.Now().After(expiresAt)
}

// Pattern 2: With grace period
func isExpiredWithGrace(expiresAt time.Time, grace time.Duration) bool {
    return time.Now().After(expiresAt.Add(grace))
}

// Pattern 3: Check if expiring soon
func isExpiringSoon(expiresAt time.Time, threshold time.Duration) bool {
    return time.Now().Add(threshold).After(expiresAt)
}

// Pattern 4: Subscription status with trial
func getSubscriptionStatus(sub *Subscription) string {
    now := time.Now()
    if sub.TrialEndsAt != nil && now.Before(*sub.TrialEndsAt) {
        return "trialing"
    }
    if sub.EndsAt != nil && now.After(*sub.EndsAt) {
        return "expired"
    }
    if sub.EndsAt != nil && now.Add(7*24*time.Hour).After(*sub.EndsAt) {
        return "expiring_soon"
    }
    return "active"
}

// Pattern 5: Certificate expiration status
func getCertStatus(cert *Certificate) string {
    daysUntilExpiry := time.Until(cert.ExpiresAt).Hours() / 24
    if daysUntilExpiry < 0 {
        return "expired"
    } else if daysUntilExpiry < 7 {
        return "critical"
    } else if daysUntilExpiry < 30 {
        return "warning"
    }
    return "valid"
}
```

**Solution - Unified Expiration Helper:**
```go
// internal/pkg/timeutil/expiration.go
package timeutil

import "time"

// Expiration provides expiration checking utilities
type Expiration struct {
    at   time.Time
    now  func() time.Time // Allow mocking for tests
}

func ExpiresAt(t time.Time) Expiration {
    return Expiration{at: t, now: time.Now}
}

func ExpiresAtPtr(t *time.Time) *Expiration {
    if t == nil {
        return nil
    }
    e := ExpiresAt(*t)
    return &e
}

func (e Expiration) IsExpired() bool {
    return e.now().After(e.at)
}

func (e Expiration) IsExpiredWithGrace(grace time.Duration) bool {
    return e.now().After(e.at.Add(grace))
}

func (e Expiration) ExpiresWithin(d time.Duration) bool {
    return e.now().Add(d).After(e.at)
}

func (e Expiration) TimeRemaining() time.Duration {
    remaining := e.at.Sub(e.now())
    if remaining < 0 {
        return 0
    }
    return remaining
}

func (e Expiration) DaysRemaining() int {
    return int(e.TimeRemaining().Hours() / 24)
}

// Status returns a status string based on thresholds
type StatusThresholds struct {
    Critical time.Duration
    Warning  time.Duration
}

func DefaultThresholds() StatusThresholds {
    return StatusThresholds{
        Critical: 7 * 24 * time.Hour,
        Warning:  30 * 24 * time.Hour,
    }
}

func (e Expiration) Status(t StatusThresholds) string {
    if e.IsExpired() {
        return "expired"
    }
    if e.ExpiresWithin(t.Critical) {
        return "critical"
    }
    if e.ExpiresWithin(t.Warning) {
        return "warning"
    }
    return "valid"
}

// SubscriptionStatus handles complex subscription state
type SubscriptionStatus struct {
    TrialEndsAt *time.Time
    EndsAt      *time.Time
    GracePeriod time.Duration
}

func (s SubscriptionStatus) Status() string {
    now := time.Now()

    // Check trial
    if s.TrialEndsAt != nil && now.Before(*s.TrialEndsAt) {
        return "trialing"
    }

    // Check main subscription
    if s.EndsAt == nil {
        return "active"
    }

    exp := ExpiresAt(*s.EndsAt)
    if exp.IsExpired() {
        if exp.IsExpiredWithGrace(s.GracePeriod) {
            return "expired"
        }
        return "grace_period"
    }
    if exp.ExpiresWithin(7 * 24 * time.Hour) {
        return "expiring_soon"
    }
    return "active"
}
```

**Refactored Usage:**
```go
// Simple expiration
exp := timeutil.ExpiresAt(token.ExpiresAt)
if exp.IsExpired() {
    return errors.New("token expired")
}

// Certificate status
certExp := timeutil.ExpiresAt(cert.ExpiresAt)
status := certExp.Status(timeutil.DefaultThresholds())

// Subscription status
subStatus := timeutil.SubscriptionStatus{
    TrialEndsAt: sub.TrialEndsAt,
    EndsAt:      sub.EndsAt,
    GracePeriod: 7 * 24 * time.Hour,
}.Status()
```

**Impact:** ~240 lines saved, consistent time handling

---

## Extended Summary (Items 111-122)

| Category | Items | Est. LOC Saved |
|----------|-------|----------------|
| Route Middleware Chain | Item 111 | ~340 lines |
| Handler Module Aggregation | Item 112 | ~200 lines |
| Webhook Route Standardization | Item 113 | ~180 lines |
| Template Engine Consolidation | Item 114 | ~465 lines |
| Nil-Safe Dereference Helpers | Item 115 | ~450 lines |
| JSONMap Type Deduplication | Item 116 | ~110 lines |
| Enum SQL Scanner Generator | Item 117 | ~800 lines |
| SSH Stream Reader Pattern | Item 118 | ~300 lines |
| Home Directory Helper | Item 119 | ~180 lines |
| HMAC Signature Validator | Item 120 | ~200 lines |
| Token Generator Consolidation | Item 121 | ~150 lines |
| Time Expiration Checker | Item 122 | ~240 lines |
| **Round 8 Total** | **12 patterns** | **~3615 lines** |

---

## 123. HTTP Client Base Pattern (P1)

**Problem:** 14 provider files duplicate identical HTTP client creation with 30-second timeouts.

**Files affected:**
- `internal/modules/server/providers/digitalocean.go:35`
- `internal/modules/server/providers/hetzner.go:35`
- `internal/modules/server/providers/linode.go:35`
- `internal/modules/server/providers/vultr.go:35`
- `internal/modules/git/providers/github.go:35-37`
- `internal/modules/git/providers/gitlab.go:31-33`
- `internal/modules/git/providers/bitbucket.go:31-33`
- `internal/modules/dns/providers/cloudflare.go:29-31`
- `internal/modules/dns/providers/digitalocean.go:26-28`
- `internal/modules/backup/storage/dropbox.go:23-25`
- `internal/modules/billing/providers/lemon_squeezy.go:59-61`
- `internal/modules/notification/slack/admin_alert.go:21-23`

**Current (repeated 14 times):**
```go
type DigitalOceanProvider struct {
    BaseProvider
    client *http.Client
}

func NewDigitalOceanProvider() *DigitalOceanProvider {
    return &DigitalOceanProvider{
        client: &http.Client{Timeout: 30 * time.Second},
    }
}
```

**Solution - Unified HTTP Client Factory:**
```go
// internal/pkg/httpclient/client.go
package httpclient

import (
    "net/http"
    "time"
)

type Config struct {
    Timeout       time.Duration
    MaxRetries    int
    RetryWaitMin  time.Duration
    RetryWaitMax  time.Duration
}

func DefaultConfig() Config {
    return Config{
        Timeout:      30 * time.Second,
        MaxRetries:   3,
        RetryWaitMin: 1 * time.Second,
        RetryWaitMax: 5 * time.Second,
    }
}

type Client struct {
    http   *http.Client
    config Config
}

func New(opts ...Option) *Client {
    cfg := DefaultConfig()
    for _, opt := range opts {
        opt(&cfg)
    }
    return &Client{
        http:   &http.Client{Timeout: cfg.Timeout},
        config: cfg,
    }
}

func WithTimeout(d time.Duration) Option {
    return func(c *Config) { c.Timeout = d }
}

// Shared client for providers
var DefaultClient = New()
```

**Refactored Provider:**
```go
type DigitalOceanProvider struct {
    BaseProvider
    client *httpclient.Client
}

func NewDigitalOceanProvider() *DigitalOceanProvider {
    return &DigitalOceanProvider{
        client: httpclient.DefaultClient,
    }
}
```

**Impact:** ~280 lines saved, configurable retry behavior

---

## 124. API Request Builder (P1)

**Problem:** 20+ repetitions of Bearer token auth, 15+ Content-Type header settings.

**Files affected:**
- `internal/modules/git/providers/github.go:96, 134, 178, 233, 283, 375, 410, 493, 585`
- `internal/modules/git/providers/gitlab.go:174, 249, 326, 372`
- `internal/modules/git/providers/bitbucket.go:168, 244`
- `internal/modules/server/providers/digitalocean.go:278-279`
- `internal/modules/server/providers/hetzner.go:234-235`
- `internal/modules/dns/providers/cloudflare.go:520-522`
- `internal/modules/billing/providers/lemon_squeezy.go:84-86`

**Current (repeated 20+ times):**
```go
req, _ := http.NewRequestWithContext(ctx, method, url, body)
req.Header.Set("Authorization", "Bearer "+token)
req.Header.Set("Content-Type", "application/json")
req.Header.Set("Accept", "application/json")
```

**Solution - Fluent Request Builder:**
```go
// internal/pkg/httpclient/request.go
package httpclient

type RequestBuilder struct {
    method  string
    url     string
    headers map[string]string
    body    io.Reader
    ctx     context.Context
}

func NewRequest(ctx context.Context, method, url string) *RequestBuilder {
    return &RequestBuilder{
        ctx:     ctx,
        method:  method,
        url:     url,
        headers: make(map[string]string),
    }
}

func (r *RequestBuilder) BearerAuth(token string) *RequestBuilder {
    r.headers["Authorization"] = "Bearer " + token
    return r
}

func (r *RequestBuilder) BasicAuth(user, pass string) *RequestBuilder {
    auth := base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
    r.headers["Authorization"] = "Basic " + auth
    return r
}

func (r *RequestBuilder) JSON() *RequestBuilder {
    r.headers["Content-Type"] = "application/json"
    r.headers["Accept"] = "application/json"
    return r
}

func (r *RequestBuilder) JSONBody(v any) *RequestBuilder {
    data, _ := json.Marshal(v)
    r.body = bytes.NewReader(data)
    return r.JSON()
}

func (r *RequestBuilder) Build() (*http.Request, error) {
    req, err := http.NewRequestWithContext(r.ctx, r.method, r.url, r.body)
    if err != nil {
        return nil, err
    }
    for k, v := range r.headers {
        req.Header.Set(k, v)
    }
    return req, nil
}
```

**Refactored Usage:**
```go
req, err := httpclient.NewRequest(ctx, "POST", url).
    BearerAuth(token).
    JSONBody(payload).
    Build()
```

**Impact:** ~400 lines saved, type-safe request building

---

## 125. Response Handler Consolidation (P1)

**Problem:** Duplicated status code validation and body parsing across 10+ providers.

**Files affected:**
- `internal/modules/server/providers/digitalocean.go:287-305`
- `internal/modules/server/providers/hetzner.go:239-252`
- `internal/modules/server/providers/linode.go:232-245`
- `internal/modules/server/providers/vultr.go:239-252`
- `internal/modules/git/providers/github.go:105-116, 143-150`
- `internal/modules/billing/providers/lemon_squeezy.go:99-114`
- `internal/modules/dns/providers/cloudflare.go:114-120`

**Current (repeated 7+ times):**
```go
respBody, err := io.ReadAll(resp.Body)
if err != nil {
    return nil, fmt.Errorf("failed to read response: %w", err)
}

if resp.StatusCode < 200 || resp.StatusCode >= 300 {
    return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
}

if len(respBody) == 0 {
    return nil, nil
}

var result map[string]interface{}
if err := json.Unmarshal(respBody, &result); err != nil {
    return nil, fmt.Errorf("failed to parse response: %w", err)
}
```

**Solution - Generic Response Handler:**
```go
// internal/pkg/httpclient/response.go
package httpclient

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
)

type Response struct {
    StatusCode int
    Body       []byte
    Raw        *http.Response
}

func HandleResponse(resp *http.Response) (*Response, error) {
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response: %w", err)
    }

    return &Response{
        StatusCode: resp.StatusCode,
        Body:       body,
        Raw:        resp,
    }, nil
}

func (r *Response) IsSuccess() bool {
    return r.StatusCode >= 200 && r.StatusCode < 300
}

func (r *Response) CheckSuccess() error {
    if !r.IsSuccess() {
        return fmt.Errorf("request failed with status %d: %s", r.StatusCode, string(r.Body))
    }
    return nil
}

func (r *Response) Decode(v any) error {
    if err := r.CheckSuccess(); err != nil {
        return err
    }
    if len(r.Body) == 0 {
        return nil
    }
    return json.Unmarshal(r.Body, v)
}

func (r *Response) DecodeMap() (map[string]any, error) {
    var result map[string]any
    if err := r.Decode(&result); err != nil {
        return nil, err
    }
    return result, nil
}

// Status-specific error handling
type StatusHandler struct {
    handlers map[int]error
}

func NewStatusHandler() *StatusHandler {
    return &StatusHandler{handlers: make(map[int]error)}
}

func (h *StatusHandler) On(code int, err error) *StatusHandler {
    h.handlers[code] = err
    return h
}

func (h *StatusHandler) Check(r *Response) error {
    if err, ok := h.handlers[r.StatusCode]; ok {
        return err
    }
    return r.CheckSuccess()
}
```

**Refactored Usage:**
```go
resp, err := client.Do(req)
if err != nil {
    return nil, err
}

r, err := httpclient.HandleResponse(resp)
if err != nil {
    return nil, err
}

// Simple case
var result ServerResponse
if err := r.Decode(&result); err != nil {
    return nil, err
}

// With specific error mapping
err = httpclient.NewStatusHandler().
    On(http.StatusNotFound, ErrNotFound).
    On(http.StatusUnauthorized, ErrUnauthorized).
    Check(r)
```

**Impact:** ~350 lines saved, consistent error handling

---

## 126. Pagination Parameter Parser (P2)

**Problem:** 4+ handlers manually parse limit parameters with different validation.

**Files affected:**
- `internal/modules/server/handlers/task_handler.go:21-25`
- `internal/modules/server/handlers/metric_handler.go:42-46`
- `internal/modules/websocket/handlers/logs.go:49-56`
- `internal/modules/websocket/handlers/metrics.go:40-49`
- `internal/modules/websocket/handlers/service_status.go:63-67`

**Current (repeated 5+ times with different ranges):**
```go
// task_handler.go - max 100
limit, _ := strconv.Atoi(c.Query("limit", "50"))
if limit < 1 || limit > 100 {
    limit = 50
}

// metric_handler.go - max 1000
limit, _ := strconv.Atoi(c.Query("limit", "100"))
if limit < 1 || limit > 1000 {
    limit = 100
}

// logs.go - different param name
tail, _ := strconv.Atoi(c.Query("tail", "100"))
if tail <= 0 {
    tail = 100
}
```

**Solution - Generic Parameter Parser:**
```go
// internal/pkg/fiber/params.go
package fiber

import (
    "strconv"
    "github.com/gofiber/fiber/v2"
)

type IntParamConfig struct {
    Name    string
    Default int
    Min     int
    Max     int
}

func ParseIntParam(c *fiber.Ctx, cfg IntParamConfig) int {
    val, err := strconv.Atoi(c.Query(cfg.Name, strconv.Itoa(cfg.Default)))
    if err != nil || val < cfg.Min {
        return cfg.Default
    }
    if cfg.Max > 0 && val > cfg.Max {
        return cfg.Max
    }
    return val
}

// Pre-configured parsers
func ParseLimit(c *fiber.Ctx, defaultVal, maxVal int) int {
    return ParseIntParam(c, IntParamConfig{
        Name:    "limit",
        Default: defaultVal,
        Min:     1,
        Max:     maxVal,
    })
}

func ParseTail(c *fiber.Ctx) int {
    return ParseIntParam(c, IntParamConfig{
        Name:    "tail",
        Default: 100,
        Min:     1,
        Max:     10000,
    })
}

func ParseInterval(c *fiber.Ctx, defaultVal int) int {
    return ParseIntParam(c, IntParamConfig{
        Name:    "interval",
        Default: defaultVal,
        Min:     1,
        Max:     60,
    })
}
```

**Refactored Usage:**
```go
limit := fiber.ParseLimit(c, 50, 100)
tail := fiber.ParseTail(c)
interval := fiber.ParseInterval(c, 5)
```

**Impact:** ~100 lines saved, consistent validation

---

## 127. Dynamic Sort Handler (P2)

**Problem:** 25+ hardcoded `.Order()` clauses with no dynamic sorting support.

**Files affected:**
- `internal/modules/server/repositories/server_repository.go:87, 107, 122`
- `internal/modules/site/repositories/site_repository.go:100, 110, 130, 141`
- `internal/modules/site/repositories/deployment_repository.go:58, 69, 85, 101, 154`
- `internal/modules/server/repositories/task_repository.go:47, 62`
- `internal/modules/server/repositories/metric_repository.go:44, 59`
- `internal/modules/backup/repositories/backup_repository.go:58, 80, 102`
- `internal/modules/dns/repositories/domain_repository.go:48, 65`
- Plus 10+ more repository files

**Current (hardcoded everywhere):**
```go
// All repositories hardcode sort
query.Order("created_at DESC")
query.Order("recorded_at DESC")
query.Order("type ASC, name ASC")  // DNS special case
```

**Solution - Dynamic Sort with Validation:**
```go
// internal/pkg/repository/sort.go
package repository

import (
    "fmt"
    "gorm.io/gorm"
)

type SortConfig struct {
    AllowedFields map[string]string  // user field -> DB column
    DefaultField  string
    DefaultDir    string
}

type SortParams struct {
    Field     string
    Direction string
}

func (s SortParams) Apply(db *gorm.DB, cfg SortConfig) *gorm.DB {
    field := s.Field
    dir := s.Direction

    // Use defaults if not specified
    if field == "" {
        field = cfg.DefaultField
    }
    if dir == "" {
        dir = cfg.DefaultDir
    }

    // Validate field
    dbColumn, ok := cfg.AllowedFields[field]
    if !ok {
        dbColumn = cfg.AllowedFields[cfg.DefaultField]
    }

    // Validate direction
    if dir != "asc" && dir != "desc" {
        dir = cfg.DefaultDir
    }

    return db.Order(fmt.Sprintf("%s %s", dbColumn, dir))
}

// Pre-built configs
var ServerSortConfig = SortConfig{
    AllowedFields: map[string]string{
        "created_at": "created_at",
        "name":       "name",
        "status":     "status",
    },
    DefaultField: "created_at",
    DefaultDir:   "desc",
}

var DeploymentSortConfig = SortConfig{
    AllowedFields: map[string]string{
        "created_at": "created_at",
        "status":     "status",
    },
    DefaultField: "created_at",
    DefaultDir:   "desc",
}
```

**Refactored Usage:**
```go
func (r *ServerRepository) FindAll(ctx context.Context, teamID string, sort SortParams) ([]models.Server, error) {
    var servers []models.Server
    query := r.DB().WithContext(ctx).Where("team_id = ?", teamID)
    query = sort.Apply(query, ServerSortConfig)
    return servers, query.Find(&servers).Error
}
```

**Impact:** ~200 lines saved, flexible sorting with security

---

## 128. Filter Options Pattern (P2)

**Problem:** Repeated WHERE clause building with optional filters across repositories.

**Files affected:**
- `internal/modules/server/repositories/metric_repository.go:31-51`
- `internal/modules/git/repositories/source_control_repository.go:236-266`
- `internal/modules/site/repositories/deployment_repository.go:80-94`
- `internal/modules/server/repositories/server_repository.go:80-89`

**Current (repeated conditional WHERE building):**
```go
query := r.DB().WithContext(ctx).Where("server_id = ?", serverID)
if from != nil {
    query = query.Where("recorded_at >= ?", from)
}
if to != nil {
    query = query.Where("recorded_at <= ?", to)
}
if limit > 0 {
    query = query.Limit(limit)
}
```

**Solution - Generic Filter Options:**
```go
// internal/pkg/repository/filter.go
package repository

import (
    "time"
    "gorm.io/gorm"
)

type FilterOption func(*gorm.DB) *gorm.DB

func WithDateRange(column string, from, to *time.Time) FilterOption {
    return func(db *gorm.DB) *gorm.DB {
        if from != nil {
            db = db.Where(column+" >= ?", from)
        }
        if to != nil {
            db = db.Where(column+" <= ?", to)
        }
        return db
    }
}

func WithOptionalLimit(limit int) FilterOption {
    return func(db *gorm.DB) *gorm.DB {
        if limit > 0 {
            return db.Limit(limit)
        }
        return db
    }
}

func WithStatus(status string) FilterOption {
    return func(db *gorm.DB) *gorm.DB {
        if status != "" {
            return db.Where("status = ?", status)
        }
        return db
    }
}

func WithStatuses(statuses []string) FilterOption {
    return func(db *gorm.DB) *gorm.DB {
        if len(statuses) > 0 {
            return db.Where("status IN ?", statuses)
        }
        return db
    }
}

func ApplyFilters(db *gorm.DB, opts ...FilterOption) *gorm.DB {
    for _, opt := range opts {
        db = opt(db)
    }
    return db
}
```

**Refactored Usage:**
```go
func (r *MetricRepository) FindByServer(ctx context.Context, serverID string, from, to *time.Time, limit int) ([]models.Metric, error) {
    var metrics []models.Metric
    query := r.DB().WithContext(ctx).Where("server_id = ?", serverID)

    query = repository.ApplyFilters(query,
        repository.WithDateRange("recorded_at", from, to),
        repository.WithOptionalLimit(limit),
    )

    return metrics, query.Order("recorded_at DESC").Find(&metrics).Error
}
```

**Impact:** ~180 lines saved, composable filters

---

## 129. Retry Strategy Consolidation (P2)

**Problem:** Scattered retry logic with different backoff strategies across modules.

**Files affected:**
- `internal/modules/server/jobs/wait_for_server_to_connect.go:17-23, 132-212`
- `internal/modules/site/jobs/deploy.go:39-50`
- `internal/modules/billing/models/webhook_event.go:18-32`
- `internal/modules/server/jobs/create_on_provider.go` (polling)

**Current (different retry implementations):**
```go
// wait_for_server_to_connect.go - Exponential backoff
const (
    maxConnectionAttempts = 30
    initialRetryDelay     = 10 * time.Second
    maxRetryDelay         = 30 * time.Second
)

delay := initialRetryDelay
for attempt := 1; attempt <= maxConnectionAttempts; attempt++ {
    // ... attempt connection ...
    time.Sleep(delay)
    delay = time.Duration(float64(delay) * 1.2)
    if delay > maxRetryDelay {
        delay = maxRetryDelay
    }
}

// deploy.go - Fixed retry
for i := 0; i < 3; i++ {
    deployment, err = j.ctx.DeploymentRepo.FindByID(ctx, j.Payload.DeploymentID)
    if err == nil {
        break
    }
    if i < 2 {
        time.Sleep(500 * time.Millisecond)
    }
}
```

**Solution - Unified Retry Package:**
```go
// internal/pkg/retry/retry.go
package retry

import (
    "context"
    "time"
)

type Strategy interface {
    NextDelay(attempt int) time.Duration
    MaxAttempts() int
}

type ExponentialBackoff struct {
    Initial    time.Duration
    Max        time.Duration
    Multiplier float64
    Attempts   int
}

func (e ExponentialBackoff) NextDelay(attempt int) time.Duration {
    delay := time.Duration(float64(e.Initial) * pow(e.Multiplier, float64(attempt-1)))
    if delay > e.Max {
        return e.Max
    }
    return delay
}

func (e ExponentialBackoff) MaxAttempts() int { return e.Attempts }

type FixedDelay struct {
    Delay    time.Duration
    Attempts int
}

func (f FixedDelay) NextDelay(_ int) time.Duration { return f.Delay }
func (f FixedDelay) MaxAttempts() int              { return f.Attempts }

func Do[T any](ctx context.Context, s Strategy, fn func() (T, error)) (T, error) {
    var result T
    var err error

    for attempt := 1; attempt <= s.MaxAttempts(); attempt++ {
        result, err = fn()
        if err == nil {
            return result, nil
        }

        if attempt < s.MaxAttempts() {
            select {
            case <-ctx.Done():
                return result, ctx.Err()
            case <-time.After(s.NextDelay(attempt)):
            }
        }
    }
    return result, err
}

// Pre-configured strategies
var (
    ConnectionRetry = ExponentialBackoff{
        Initial:    10 * time.Second,
        Max:        30 * time.Second,
        Multiplier: 1.2,
        Attempts:   30,
    }

    QuickRetry = FixedDelay{
        Delay:    500 * time.Millisecond,
        Attempts: 3,
    }
)
```

**Refactored Usage:**
```go
// Connection with exponential backoff
connected, err := retry.Do(ctx, retry.ConnectionRetry, func() (bool, error) {
    return j.attemptConnection(ctx)
})

// Quick DB read retry
deployment, err := retry.Do(ctx, retry.QuickRetry, func() (*models.Deployment, error) {
    return j.ctx.DeploymentRepo.FindByID(ctx, j.Payload.DeploymentID)
})
```

**Impact:** ~250 lines saved, consistent retry behavior

---

## 130. Metrics Parsing Helper (P2)

**Problem:** 6 identical strconv.ParseFloat patterns in metrics webhook handler.

**Files affected:**
- `internal/modules/server/handlers/metrics_webhook_handler.go:104-127`

**Current (repeated 6 times):**
```go
diskTotal, err := strconv.ParseFloat(req.Data.DiskTotal, 64)
if err != nil {
    h.logger.Warn().Err(err).Str("server_id", serverID).Str("value", req.Data.DiskTotal).Msg("Failed to parse disk_total")
}
diskFree, err := strconv.ParseFloat(req.Data.DiskFree, 64)
if err != nil {
    h.logger.Warn().Err(err).Str("server_id", serverID).Str("value", req.Data.DiskFree).Msg("Failed to parse disk_free")
}
diskUsed, err := strconv.ParseFloat(req.Data.DiskUsed, 64)
// ... repeated for memory_total, memory_free, memory_used
```

**Solution - Metrics Parser Helper:**
```go
// internal/pkg/metrics/parser.go
package metrics

import (
    "strconv"
    "github.com/rs/zerolog"
)

type Parser struct {
    logger   zerolog.Logger
    serverID string
    errors   []string
}

func NewParser(logger zerolog.Logger, serverID string) *Parser {
    return &Parser{logger: logger, serverID: serverID}
}

func (p *Parser) ParseFloat(value, fieldName string) float64 {
    if value == "" {
        return 0
    }
    v, err := strconv.ParseFloat(value, 64)
    if err != nil {
        p.logger.Warn().
            Err(err).
            Str("server_id", p.serverID).
            Str("field", fieldName).
            Str("value", value).
            Msg("Failed to parse metric")
        p.errors = append(p.errors, fieldName)
        return 0
    }
    return v
}

func (p *Parser) ParseInt(value, fieldName string) int64 {
    if value == "" {
        return 0
    }
    v, err := strconv.ParseInt(value, 10, 64)
    if err != nil {
        p.logger.Warn().
            Err(err).
            Str("server_id", p.serverID).
            Str("field", fieldName).
            Str("value", value).
            Msg("Failed to parse metric")
        p.errors = append(p.errors, fieldName)
        return 0
    }
    return v
}

func (p *Parser) HasErrors() bool {
    return len(p.errors) > 0
}
```

**Refactored Usage:**
```go
parser := metrics.NewParser(h.logger, serverID)

metric := &models.Metric{
    DiskTotal:   parser.ParseFloat(req.Data.DiskTotal, "disk_total"),
    DiskFree:    parser.ParseFloat(req.Data.DiskFree, "disk_free"),
    DiskUsed:    parser.ParseFloat(req.Data.DiskUsed, "disk_used"),
    MemoryTotal: parser.ParseFloat(req.Data.MemoryTotal, "memory_total"),
    MemoryFree:  parser.ParseFloat(req.Data.MemoryFree, "memory_free"),
    MemoryUsed:  parser.ParseFloat(req.Data.MemoryUsed, "memory_used"),
}
```

**Impact:** ~80 lines saved, consistent error logging

---

## 131. Service Status Checker (P2)

**Problem:** Duplicated service status parsing and memory formatting in WebSocket and job handlers.

**Files affected:**
- `internal/modules/websocket/handlers/service_status.go:325-405`
- `internal/modules/server/jobs/check_service_status.go:135-234`

**Current (duplicated formatting):**
```go
// WebSocket handler - lines 353-370
func formatMemory(bytes int64) string {
    const unit = 1024
    if bytes < unit {
        return fmt.Sprintf("%d B", bytes)
    }
    div, exp := int64(unit), 0
    for n := bytes / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Job handler - lines 215-234 (similar but different implementation)
func formatBytes(bytesStr string) string {
    // Similar logic...
}
```

**Solution - Unified Status Package:**
```go
// internal/pkg/status/format.go
package status

import (
    "fmt"
    "time"
)

func FormatBytes(bytes int64) string {
    const unit = 1024
    if bytes < unit {
        return fmt.Sprintf("%d B", bytes)
    }
    div, exp := int64(unit), 0
    for n := bytes / unit; n >= unit; n /= unit {
        div *= unit
        exp++
    }
    return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func FormatUptime(seconds int64) string {
    d := time.Duration(seconds) * time.Second
    days := int(d.Hours() / 24)
    hours := int(d.Hours()) % 24
    minutes := int(d.Minutes()) % 60

    if days > 0 {
        return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
    }
    if hours > 0 {
        return fmt.Sprintf("%dh %dm", hours, minutes)
    }
    return fmt.Sprintf("%dm", minutes)
}

// internal/pkg/status/parser.go
type ServiceStatus struct {
    ID       string `json:"id"`
    Software string `json:"software"`
    Name     string `json:"name"`
    Status   string `json:"status"`
    IsActive bool   `json:"is_active"`
    Memory   string `json:"memory,omitempty"`
    Uptime   string `json:"uptime,omitempty"`
    PID      int    `json:"pid,omitempty"`
}

func ParseSystemctlOutput(output string) *ServiceStatus {
    // Unified parsing logic
}
```

**Impact:** ~150 lines saved, consistent formatting

---

## 132. Daemon Status Task Deduplication (P1)

**Problem:** 85+ lines of identical bash script duplicated between server and site modules.

**Files affected:**
- `internal/modules/server/tasks/daemon_status.go:18-85`
- `internal/modules/site/tasks/daemon_status.go:18-85`

**Current (IDENTICAL 85+ lines in both files):**
```go
// server/tasks/daemon_status.go
const CheckServerDaemonStatusTaskType = "server:check_daemon_status"

// site/tasks/daemon_status.go
const CheckDaemonStatusTaskType = "site:check_daemon_status"

// Both contain identical bash script:
script := `#!/bin/bash
set -euo pipefail

# Get supervisorctl status and parse it
supervisorctl status 2>/dev/null | while read -r line; do
    # ... 60+ lines of identical parsing logic ...
done

echo "===STATUS_CHECK_COMPLETE==="`
```

**Solution - Single Shared Task:**
```go
// internal/pkg/tasks/daemon_status.go
package tasks

import (
    "github.com/kkz6/launch-go/internal/pkg/taskrunner"
)

const CheckDaemonStatusTaskType = "common:check_daemon_status"

var daemonStatusScript = `#!/bin/bash
set -euo pipefail

# Get supervisorctl status and parse it
supervisorctl status 2>/dev/null | while read -r line; do
    program=$(echo "$line" | awk '{print $1}')
    status=$(echo "$line" | awk '{print $2}')
    # ... parsing logic ...
done

echo "===STATUS_CHECK_COMPLETE==="`

func CheckDaemonStatus() *taskrunner.BaseTask {
    return taskrunner.NewBaseTask(
        taskrunner.WithName("Check Daemon Status"),
        taskrunner.WithScript(daemonStatusScript),
        taskrunner.WithTimeoutSeconds(30),
    )
}

// Result parser
type DaemonStatus struct {
    DaemonID      string `json:"daemon_id"`
    Status        string `json:"status"`
    PID           string `json:"pid"`
    UptimeSeconds int64  `json:"uptime_seconds"`
    Description   string `json:"description"`
    Error         string `json:"error,omitempty"`
}

func ParseDaemonStatusOutput(output string) []DaemonStatus {
    // Unified parsing logic
}
```

**Refactored Usage:**
```go
// Both modules use the same task
import "github.com/kkz6/launch-go/internal/pkg/tasks"

task := tasks.CheckDaemonStatus()
result, err := runner.RunTask(task).Dispatch(ctx)
statuses := tasks.ParseDaemonStatusOutput(result.GetOutput())
```

**Impact:** ~170 lines saved (85 lines x 2 - shared code)

---

## 133. Health Check Aggregator (P2)

**Problem:** Basic health endpoint with no dependency checks.

**Files affected:**
- `cmd/api/main.go:230-234` - Minimal health check
- `internal/pkg/cache/redis.go:65-68` - Redis Ping exists but unused

**Current (minimal implementation):**
```go
// cmd/api/main.go
func (a *Application) healthCheck(c *fiber.Ctx) error {
    return c.JSON(fiber.Map{
        "status": "ok",
        "time":   time.Now().UTC(),
    })
}
```

**Solution - Comprehensive Health Package:**
```go
// internal/pkg/health/health.go
package health

import (
    "context"
    "sync"
    "time"
)

type Status string

const (
    StatusHealthy   Status = "healthy"
    StatusDegraded  Status = "degraded"
    StatusUnhealthy Status = "unhealthy"
)

type Check struct {
    Name    string        `json:"name"`
    Status  Status        `json:"status"`
    Message string        `json:"message,omitempty"`
    Latency time.Duration `json:"latency_ms"`
}

type Result struct {
    Status    Status    `json:"status"`
    Timestamp time.Time `json:"timestamp"`
    Checks    []Check   `json:"checks"`
}

type Checker interface {
    Name() string
    Check(ctx context.Context) error
}

type Aggregator struct {
    checkers []Checker
}

func NewAggregator() *Aggregator {
    return &Aggregator{}
}

func (a *Aggregator) Add(c Checker) *Aggregator {
    a.checkers = append(a.checkers, c)
    return a
}

func (a *Aggregator) Check(ctx context.Context) Result {
    result := Result{
        Status:    StatusHealthy,
        Timestamp: time.Now().UTC(),
        Checks:    make([]Check, len(a.checkers)),
    }

    var wg sync.WaitGroup
    for i, checker := range a.checkers {
        wg.Add(1)
        go func(idx int, c Checker) {
            defer wg.Done()
            start := time.Now()
            err := c.Check(ctx)
            check := Check{
                Name:    c.Name(),
                Latency: time.Since(start),
                Status:  StatusHealthy,
            }
            if err != nil {
                check.Status = StatusUnhealthy
                check.Message = err.Error()
                result.Status = StatusDegraded
            }
            result.Checks[idx] = check
        }(i, checker)
    }
    wg.Wait()

    return result
}

// Built-in checkers
type DatabaseChecker struct {
    db *gorm.DB
}

func (c *DatabaseChecker) Name() string { return "database" }
func (c *DatabaseChecker) Check(ctx context.Context) error {
    sqlDB, err := c.db.DB()
    if err != nil {
        return err
    }
    return sqlDB.PingContext(ctx)
}

type RedisChecker struct {
    client *redis.Client
}

func (c *RedisChecker) Name() string { return "redis" }
func (c *RedisChecker) Check(ctx context.Context) error {
    return c.client.Ping(ctx).Err()
}
```

**Refactored Usage:**
```go
healthChecker := health.NewAggregator().
    Add(&health.DatabaseChecker{db: db}).
    Add(&health.RedisChecker{client: redis})

func (a *Application) healthCheck(c *fiber.Ctx) error {
    result := a.healthChecker.Check(c.Context())
    status := fiber.StatusOK
    if result.Status != health.StatusHealthy {
        status = fiber.StatusServiceUnavailable
    }
    return c.Status(status).JSON(result)
}
```

**Impact:** ~120 lines saved, proper dependency checking

---

## 134. Config Loader Helper (P2)

**Problem:** Repeating load/setDefaults pattern in all 10 config files.

**Files affected:**
- `internal/config/app.go:21-40`
- `internal/config/database.go:35-57`
- `internal/config/redis.go:12-24`
- `internal/config/jwt.go:11-21`
- `internal/config/cors.go:10-18`
- `internal/config/queue.go:10-18`
- `internal/config/billing.go:20-36`
- `internal/config/git.go:36-68`
- `internal/config/slack.go:17-21` (inconsistent key naming)
- `internal/config/sentry.go:21-45`

**Current (repeated in every config file):**
```go
func loadAppConfig() AppConfig {
    return AppConfig{
        Name:        viper.GetString("APP_NAME"),
        Environment: viper.GetString("APP_ENV"),
        Port:        viper.GetString("APP_PORT"),
        Debug:       viper.GetBool("APP_DEBUG"),
    }
}

func setAppDefaults() {
    viper.SetDefault("APP_NAME", "Launch")
    viper.SetDefault("APP_ENV", "development")
    viper.SetDefault("APP_PORT", "8080")
    viper.SetDefault("APP_DEBUG", true)
}
```

**Solution - Declarative Config Loader:**
```go
// internal/pkg/config/loader.go
package config

import (
    "reflect"
    "github.com/spf13/viper"
)

// Tag-based config loading
// Field tag: `env:"APP_NAME" default:"Launch"`

func Load(cfg interface{}) error {
    v := reflect.ValueOf(cfg).Elem()
    t := v.Type()

    for i := 0; i < t.NumField(); i++ {
        field := t.Field(i)
        envKey := field.Tag.Get("env")
        defaultVal := field.Tag.Get("default")

        if envKey == "" {
            continue
        }

        // Set default if specified
        if defaultVal != "" {
            viper.SetDefault(envKey, defaultVal)
        }

        // Load value based on field type
        switch field.Type.Kind() {
        case reflect.String:
            v.Field(i).SetString(viper.GetString(envKey))
        case reflect.Bool:
            v.Field(i).SetBool(viper.GetBool(envKey))
        case reflect.Int, reflect.Int64:
            v.Field(i).SetInt(viper.GetInt64(envKey))
        }
    }
    return nil
}

// Validation interface
type Validator interface {
    Validate() error
}

func LoadAndValidate(cfg interface{}) error {
    if err := Load(cfg); err != nil {
        return err
    }
    if v, ok := cfg.(Validator); ok {
        return v.Validate()
    }
    return nil
}
```

**Refactored Config:**
```go
type AppConfig struct {
    Name        string `env:"APP_NAME" default:"Launch"`
    Environment string `env:"APP_ENV" default:"development"`
    Port        string `env:"APP_PORT" default:"8080"`
    Debug       bool   `env:"APP_DEBUG" default:"true"`
    URL         string `env:"APP_URL" default:"http://localhost:8080"`
    Key         string `env:"APP_KEY"`
    LocalMode   bool   `env:"APP_LOCAL_MODE" default:"false"`
}

func (c AppConfig) Validate() error {
    if c.Key == "" && c.Environment == "production" {
        return errors.New("APP_KEY is required in production")
    }
    return nil
}

func loadAppConfig() AppConfig {
    var cfg AppConfig
    config.LoadAndValidate(&cfg)
    return cfg
}
```

**Impact:** ~200 lines saved, declarative configuration

---

## Extended Summary (Items 123-134)

| Category | Items | Est. LOC Saved |
|----------|-------|----------------|
| HTTP Client Base Pattern | Item 123 | ~280 lines |
| API Request Builder | Item 124 | ~400 lines |
| Response Handler Consolidation | Item 125 | ~350 lines |
| Pagination Parameter Parser | Item 126 | ~100 lines |
| Dynamic Sort Handler | Item 127 | ~200 lines |
| Filter Options Pattern | Item 128 | ~180 lines |
| Retry Strategy Consolidation | Item 129 | ~250 lines |
| Metrics Parsing Helper | Item 130 | ~80 lines |
| Service Status Checker | Item 131 | ~150 lines |
| Daemon Status Task Deduplication | Item 132 | ~170 lines |
| Health Check Aggregator | Item 133 | ~120 lines |
| Config Loader Helper | Item 134 | ~200 lines |
| **Round 9 Total** | **12 patterns** | **~2480 lines** |

---

## 135. Webhook Channel Base Class (P1)

**Problem:** 3 webhook-based notification channels duplicate identical HTTP posting and validation logic.

**Files affected:**
- `internal/modules/notification/channels/slack.go` (95 lines)
- `internal/modules/notification/channels/discord.go` (95 lines)
- `internal/modules/notification/channels/telegram.go` (123 lines)

**Current (repeated 3 times):**
```go
type SlackChannel struct {
    webhookURL string
    httpClient HTTPClient
}

func (s *SlackChannel) Connect(ctx context.Context) error {
    webhookURL := s.GetWebhookURL()
    if webhookURL == "" {
        return ErrInvalidConfiguration
    }
    if s.httpClient == nil {
        return ErrConnectionFailed
    }

    payload := slackMessage{Text: "*Connected to Launch*..."}
    _, statusCode, err := s.httpClient.Post(ctx, webhookURL, payload)
    if statusCode < 200 || statusCode >= 300 {
        return fmt.Errorf("%w: received status code %d", ErrConnectionFailed, statusCode)
    }
    return nil
}

func (s *SlackChannel) Send(ctx context.Context, n Notification) error {
    // Identical pattern: build message -> POST -> check status
}
```

**Solution - Base Webhook Channel:**
```go
// internal/modules/notification/channels/webhook_base.go
package channels

type WebhookChannel struct {
    httpClient     HTTPClient
    webhookURL     string
    connectMessage string
}

func (w *WebhookChannel) ValidateConfig() error {
    if w.webhookURL == "" {
        return ErrInvalidConfiguration
    }
    if w.httpClient == nil {
        return ErrConnectionFailed
    }
    return nil
}

func (w *WebhookChannel) Post(ctx context.Context, payload any) error {
    if err := w.ValidateConfig(); err != nil {
        return err
    }
    _, statusCode, err := w.httpClient.Post(ctx, w.webhookURL, payload)
    if err != nil {
        return fmt.Errorf("%w: %v", ErrSendFailed, err)
    }
    if statusCode < 200 || statusCode >= 300 {
        return fmt.Errorf("%w: received status code %d", ErrSendFailed, statusCode)
    }
    return nil
}

// Subclass only overrides message formatting
type SlackChannel struct {
    WebhookChannel
}

func (s *SlackChannel) Connect(ctx context.Context) error {
    return s.Post(ctx, slackMessage{Text: "*Connected to Launch*..."})
}

func (s *SlackChannel) Send(ctx context.Context, n Notification) error {
    return s.Post(ctx, slackMessage{Text: n.ToSlack()})
}
```

**Impact:** ~190 lines saved, consistent webhook handling

---

## 136. Admin Alert Base Class (P1)

**Problem:** 4 admin alert types duplicate 500+ lines of identical builder methods and message construction.

**Files affected:**
- `internal/modules/notification/slack/server_provisioning_failed_alert.go` (174 lines)
- `internal/modules/notification/slack/php_installation_failed_alert.go` (143 lines)
- `internal/modules/notification/slack/php_extension_install_failed_alert.go` (141 lines)
- `internal/modules/notification/slack/php_extension_uninstall_failed_alert.go` (~140 lines)

**Current (repeated in all 4 files):**
```go
type ServerProvisioningFailedAdminAlert struct {
    alerter              *AdminAlerter
    server               ServerInfo
    user                 *UserInfo
    output               string
    errorMessage         string
    outputRetrievalError string
}

// These 4 builder methods are IDENTICAL in all files:
func (a *ServerProvisioningFailedAdminAlert) WithUser(user *UserInfo) *ServerProvisioningFailedAdminAlert {
    a.user = user
    return a
}

func (a *ServerProvisioningFailedAdminAlert) WithOutput(output string) *ServerProvisioningFailedAdminAlert {
    a.output = output
    return a
}

func (a *ServerProvisioningFailedAdminAlert) WithErrorMessage(msg string) *ServerProvisioningFailedAdminAlert {
    a.errorMessage = msg
    return a
}

// buildMessage() is 80% identical with minor header/description differences
func (a *ServerProvisioningFailedAdminAlert) buildMessage() *BlockKitMessage {
    message := NewBlockKitMessage(fallbackText)
    message.AddHeader("🚨 Server Provisioning Failed")  // Only this differs
    message.AddSection("Description specific to alert type")  // And this
    message.AddDivider()

    // IDENTICAL: Server details section (25 lines)
    message.AddFieldsSection(
        fmt.Sprintf("*Server Name:*\n%s", a.server.Name),
        fmt.Sprintf("*Team:*\n%s", a.server.TeamName),
    )

    // IDENTICAL: User details section (15 lines)
    userName := "Unknown"
    if a.user != nil && a.user.Name != "" {
        userName = a.user.Name
    }
    message.AddFieldsSection(...)

    // IDENTICAL: Output handling (10 lines)
    if a.output != "" {
        message.AddSection(fmt.Sprintf("```\n%s\n```", TruncateOutput(a.output, 2900)))
    }

    // IDENTICAL: Context footer (5 lines)
    message.AddContext(fmt.Sprintf("Server ID: %s | Team ID: %s | %s", ...))

    return message
}
```

**Solution - Base Admin Alert:**
```go
// internal/modules/notification/slack/base_admin_alert.go
package slack

type BaseAdminAlert struct {
    alerter              *AdminAlerter
    server               ServerInfo
    user                 *UserInfo
    output               string
    errorMessage         string
    outputRetrievalError string
}

func (b *BaseAdminAlert) WithUser(user *UserInfo) *BaseAdminAlert {
    b.user = user
    return b
}

func (b *BaseAdminAlert) WithOutput(output string) *BaseAdminAlert {
    b.output = output
    return b
}

func (b *BaseAdminAlert) WithErrorMessage(msg string) *BaseAdminAlert {
    b.errorMessage = msg
    return b
}

type AlertConfig struct {
    Header       string
    Description  string
    ExtraFields  func(*BlockKitMessage)  // Hook for type-specific fields
}

func (b *BaseAdminAlert) BuildMessage(cfg AlertConfig) *BlockKitMessage {
    message := NewBlockKitMessage(cfg.Header)
    message.AddHeader(cfg.Header)
    message.AddSection(cfg.Description)
    message.AddDivider()

    // Standard server details
    b.addServerDetails(message)

    // Type-specific fields
    if cfg.ExtraFields != nil {
        cfg.ExtraFields(message)
    }

    // Standard user, output, context
    b.addUserDetails(message)
    b.addOutputSection(message)
    b.addContextFooter(message)

    return message
}

// Specific alert only defines config
type ServerProvisioningFailedAdminAlert struct {
    BaseAdminAlert
}

func (a *ServerProvisioningFailedAdminAlert) buildMessage() *BlockKitMessage {
    return a.BuildMessage(AlertConfig{
        Header:      "🚨 Server Provisioning Failed",
        Description: "Server provisioning has failed. Details below.",
    })
}
```

**Impact:** ~400 lines saved, extensible alert system

---

## 137. SSH Client Factory (P1)

**Problem:** 7+ instances of identical SSH client creation with hardcoded 30-second timeout.

**Files affected:**
- `internal/pkg/taskrunner/dispatcher.go:148-154, 336-342`
- `internal/pkg/taskrunner/stream_monitor.go:97-103, 305-311`
- `internal/modules/server/services/server_service.go:262-268`

**Current (repeated 7+ times):**
```go
sshClient, err := taskrunner.NewSSHClient(taskrunner.SSHConfig{
    Host:       conn.Host,
    Port:       conn.Port,
    User:       conn.User,
    PrivateKey: conn.PrivateKey,
    Timeout:    30 * time.Second,  // Hardcoded everywhere
})
if err != nil {
    return nil, fmt.Errorf("failed to create SSH client: %w", err)
}
defer sshClient.Close()

if err := sshClient.Connect(); err != nil {
    return nil, fmt.Errorf("failed to connect: %w", err)
}
```

**Solution - SSH Client Factory:**
```go
// internal/pkg/ssh/factory.go
package ssh

import (
    "context"
    "time"
)

type ClientFactory struct {
    defaultTimeout time.Duration
}

func NewClientFactory(timeout time.Duration) *ClientFactory {
    if timeout == 0 {
        timeout = 30 * time.Second
    }
    return &ClientFactory{defaultTimeout: timeout}
}

var DefaultFactory = NewClientFactory(30 * time.Second)

func (f *ClientFactory) Connect(ctx context.Context, conn *Connection) (*Client, error) {
    client, err := NewSSHClient(SSHConfig{
        Host:       conn.Host,
        Port:       conn.Port,
        User:       conn.User,
        PrivateKey: conn.PrivateKey,
        Timeout:    f.defaultTimeout,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to create SSH client: %w", err)
    }

    if err := client.Connect(); err != nil {
        client.Close()
        return nil, fmt.Errorf("failed to connect: %w", err)
    }

    return client, nil
}

// WithConnection provides a session scoped to a connection
func (f *ClientFactory) WithConnection(ctx context.Context, conn *Connection, fn func(*Client) error) error {
    client, err := f.Connect(ctx, conn)
    if err != nil {
        return err
    }
    defer client.Close()
    return fn(client)
}
```

**Refactored Usage:**
```go
err := ssh.DefaultFactory.WithConnection(ctx, conn, func(client *ssh.Client) error {
    return client.Run(ctx, command)
})
```

**Impact:** ~140 lines saved, configurable timeout

---

## 138. Cron Schedule Constants (P2)

**Problem:** Cron expressions and frequencies duplicated across job files with redundant Frequency field.

**Files affected:**
- `internal/modules/site/jobs/enable_laravel_scheduler.go:75-83`
- `internal/modules/site/jobs/install_wordpress_cron.go:69-77`
- `internal/modules/server/jobs/install_task_cleanup_cron.go:52-59`
- `internal/modules/server/jobs/cleanup_old_metrics.go:67-71`
- `internal/modules/server/models/cron.go:18-19` (redundant fields)

**Current (Expression and Frequency both stored):**
```go
cron := &servermodels.Cron{
    ServerID:   server.ID,
    Expression: "* * * * *",    // Hardcoded
    Frequency:  "every_minute", // Redundant!
    Hidden:     true,
}
```

**Solution - Cron Schedule Enum:**
```go
// internal/modules/server/enums/cron_schedule.go
package enums

type CronSchedule string

const (
    EveryMinute CronSchedule = "* * * * *"
    Hourly      CronSchedule = "0 * * * *"
    Daily2AM    CronSchedule = "0 2 * * *"
    Daily3AM    CronSchedule = "0 3 * * *"
    Weekly      CronSchedule = "0 0 * * 0"
)

func (c CronSchedule) Expression() string {
    return string(c)
}

func (c CronSchedule) Description() string {
    switch c {
    case EveryMinute:
        return "every_minute"
    case Hourly:
        return "hourly"
    case Daily2AM, Daily3AM:
        return "daily"
    case Weekly:
        return "weekly"
    }
    return "custom"
}

// Remove Frequency field from Cron model - derive from Expression
func (c *Cron) GetFrequency() string {
    return CronSchedule(c.Expression).Description()
}
```

**Impact:** ~60 lines saved, single source of truth

---

## 139. Job Task Builder Generator (P1)

**Problem:** 78+ nearly identical NewTaskName() functions wrapping pkgjobs.NewTask().

**Files affected:**
- `internal/modules/site/jobs/*.go` (20+ task builders)
- `internal/modules/server/jobs/*.go` (30+ task builders)
- `internal/modules/database/jobs/*.go` (10+ task builders)
- `internal/modules/backup/jobs/*.go` (5+ task builders)

**Current (repeated 78+ times):**
```go
func NewCreateDeploymentTask(siteID string, userID *string, gitHash *string, branch *string) (*asynq.Task, error) {
    return pkgjobs.NewTask(TypeCreateDeployment, CreateDeploymentPayload{
        SiteID:  siteID,
        UserID:  userID,
        GitHash: gitHash,
        Branch:  branch,
    })
}

func NewInstallQueueTask(siteID, queueID string, userID *string) (*asynq.Task, error) {
    return pkgjobs.NewTask(TypeInstallQueue, InstallQueuePayload{
        SiteID:  siteID,
        QueueID: queueID,
        UserID:  userID,
    })
}
```

**Solution - Generic Task Builder:**
```go
// internal/pkg/jobs/builder.go
package jobs

import (
    "encoding/json"
    "github.com/hibiken/asynq"
)

type TaskBuilder[P any] struct {
    taskType string
    opts     []asynq.Option
}

func NewTaskBuilder[P any](taskType string) *TaskBuilder[P] {
    return &TaskBuilder[P]{taskType: taskType}
}

func (b *TaskBuilder[P]) WithTaskID(id string) *TaskBuilder[P] {
    b.opts = append(b.opts, asynq.TaskID(id))
    return b
}

func (b *TaskBuilder[P]) WithRetry(n int) *TaskBuilder[P] {
    b.opts = append(b.opts, asynq.MaxRetry(n))
    return b
}

func (b *TaskBuilder[P]) Build(payload P) (*asynq.Task, error) {
    data, err := json.Marshal(payload)
    if err != nil {
        return nil, err
    }
    return asynq.NewTask(b.taskType, data, b.opts...), nil
}

// Usage becomes simpler
var CreateDeploymentBuilder = NewTaskBuilder[CreateDeploymentPayload](TypeCreateDeployment)

// Call site
task, err := CreateDeploymentBuilder.Build(CreateDeploymentPayload{
    SiteID: siteID,
    UserID: userID,
})
```

**Impact:** ~400 lines saved, type-safe builders

---

## 140. Job Context Base Consolidation (P1)

**Problem:** 5-6 JobContext implementations with ~70% duplicated code.

**Files affected:**
- `internal/modules/site/jobs/context.go:40-87`
- `internal/modules/server/jobs/context.go:31-64`
- `internal/modules/database/jobs/context.go:37-68`
- `internal/modules/backup/jobs/context.go:25-46`
- `internal/modules/script/jobs/context.go`

**Current (repeated in each module):**
```go
type JobContext struct {
    *pkgjobs.Base
    DB         *gorm.DB
    Logger     *zerolog.Logger
    WS         broadcast.TeamBroadcaster
    Dispatcher taskrunner.TaskDispatcher
    Queue      *queue.Client
    // Module-specific repos...
    SiteRepo   *repositories.SiteRepository
}

func NewJobContext(
    db *gorm.DB,
    logger *zerolog.Logger,
    ws broadcast.TeamBroadcaster,
    dispatcher taskrunner.TaskDispatcher,
    queueClient *queue.Client,
    // 5+ more parameters...
) *JobContext {
    return &JobContext{
        Base: pkgjobs.NewBase(pkgjobs.BaseDeps{
            DB:         db,
            Logger:     logger,
            WS:         ws,
            Dispatcher: dispatcher,
        }),
        DB:         db,
        Logger:     logger,
        // Repeat all assignments...
    }
}
```

**Solution - Composable Job Context:**
```go
// internal/pkg/jobs/context.go
package jobs

type CoreDeps struct {
    DB         *gorm.DB
    Logger     *zerolog.Logger
    WS         broadcast.TeamBroadcaster
    Dispatcher taskrunner.TaskDispatcher
    Queue      *queue.Client
}

type CoreContext struct {
    *Base
    deps CoreDeps
}

func NewCoreContext(deps CoreDeps) *CoreContext {
    return &CoreContext{
        Base: NewBase(BaseDeps{
            DB:         deps.DB,
            Logger:     deps.Logger,
            WS:         deps.WS,
            Dispatcher: deps.Dispatcher,
        }),
        deps: deps,
    }
}

func (c *CoreContext) DB() *gorm.DB           { return c.deps.DB }
func (c *CoreContext) Logger() *zerolog.Logger { return c.deps.Logger }
func (c *CoreContext) Queue() *queue.Client    { return c.deps.Queue }

// Module contexts embed core and add repos
type SiteJobContext struct {
    *CoreContext
    repos *SiteRepositories
}

func NewSiteJobContext(core *CoreContext, repos *SiteRepositories) *SiteJobContext {
    return &SiteJobContext{CoreContext: core, repos: repos}
}
```

**Impact:** ~250 lines saved, consistent initialization

---

## 141. DNS Provider HTTP Base (P1)

**Problem:** Cloudflare (559 lines) and DigitalOcean (494 lines) DNS providers duplicate HTTP handling.

**Files affected:**
- `internal/modules/dns/providers/cloudflare.go:513-540`
- `internal/modules/dns/providers/digitalocean.go:428-456`

**Current (duplicated request/response handling):**
```go
// Both providers have identical HTTP patterns:
func (p *CloudflareProvider) doRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
    req, _ := http.NewRequestWithContext(ctx, method, p.baseURL+path, bodyReader)
    req.Header.Set("Authorization", "Bearer "+p.token)
    req.Header.Set("Content-Type", "application/json")

    resp, err := p.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    respBody, _ := io.ReadAll(resp.Body)

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return nil, p.parseError(respBody)
    }

    return respBody, nil
}
```

**Solution - DNS Provider Base:**
```go
// internal/modules/dns/providers/base.go
package providers

type BaseHTTPProvider struct {
    httpClient *http.Client
    baseURL    string
    token      string
}

func (p *BaseHTTPProvider) DoRequest(ctx context.Context, method, path string, body any) ([]byte, error) {
    var bodyReader io.Reader
    if body != nil {
        data, _ := json.Marshal(body)
        bodyReader = bytes.NewReader(data)
    }

    req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, bodyReader)
    if err != nil {
        return nil, err
    }

    req.Header.Set("Authorization", "Bearer "+p.token)
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Accept", "application/json")

    resp, err := p.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
    }

    return respBody, nil
}

// Cloudflare embeds base
type CloudflareProvider struct {
    BaseHTTPProvider
    accountID string
    zoneCache map[string]string
}
```

**Impact:** ~200 lines saved, consistent HTTP handling

---

## 142. Backup Job Payload Base (P2)

**Problem:** 3 backup jobs have identical payload structures.

**Files affected:**
- `internal/modules/backup/jobs/install_backup.go:14-18`
- `internal/modules/backup/jobs/run_manual_backup.go:14-18`
- `internal/modules/backup/jobs/sync_launch_config.go:15-18`

**Current (identical structures):**
```go
// install_backup.go
type InstallBackupPayload struct {
    ServerID string  `json:"server_id"`
    BackupID string  `json:"backup_id"`
    UserID   *string `json:"user_id,omitempty"`
}

// run_manual_backup.go
type RunManualBackupPayload struct {
    ServerID string  `json:"server_id"`
    BackupID string  `json:"backup_id"`
    UserID   *string `json:"user_id,omitempty"`
}
```

**Solution - Shared Backup Payload:**
```go
// internal/modules/backup/jobs/payloads.go
package jobs

// BackupJobPayload is the common payload for backup-related jobs
type BackupJobPayload struct {
    ServerID string  `json:"server_id"`
    BackupID string  `json:"backup_id"`
    UserID   *string `json:"user_id,omitempty"`
}

// Type aliases for clarity (optional)
type InstallBackupPayload = BackupJobPayload
type RunManualBackupPayload = BackupJobPayload
type SyncLaunchConfigPayload = BackupJobPayload
```

**Impact:** ~30 lines saved, single payload definition

---

## 143. Service Dispatch Helper (P2)

**Problem:** 3 nearly identical job dispatch helper methods in backup service.

**Files affected:**
- `internal/modules/backup/services/backup_service.go:224-291`

**Current (repeated 3 times):**
```go
func (s *BackupService) dispatchInstallBackup(serverID, backupID string) {
    if s.Queue == nil {
        s.Logger.Warn().Msg("Queue not configured, skipping InstallBackup job")
        return
    }
    task, err := jobs.NewInstallBackupTask(serverID, backupID, nil)
    if err != nil {
        s.Logger.Error().Err(err).Msg("Failed to create InstallBackup task")
        return
    }
    if _, err := s.Queue.EnqueueDefault(task); err != nil {
        s.Logger.Error().Err(err).Msg("Failed to enqueue InstallBackup job")
    }
}

// dispatchDeleteBackup and dispatchRunManualBackup are identical patterns
```

**Solution - Generic Dispatch Helper:**
```go
// internal/pkg/service/dispatch.go
package service

type TaskFactory func() (*asynq.Task, error)

func (s *Base) DispatchTask(name string, factory TaskFactory) {
    if s.Queue == nil {
        s.Logger.Warn().Str("task", name).Msg("Queue not configured, skipping task")
        return
    }
    task, err := factory()
    if err != nil {
        s.Logger.Error().Err(err).Str("task", name).Msg("Failed to create task")
        return
    }
    if _, err := s.Queue.EnqueueDefault(task); err != nil {
        s.Logger.Error().Err(err).Str("task", name).Msg("Failed to enqueue task")
    }
}

// Usage
s.DispatchTask("InstallBackup", func() (*asynq.Task, error) {
    return jobs.NewInstallBackupTask(serverID, backupID, nil)
})
```

**Impact:** ~50 lines saved, reusable dispatch

---

## 144. Webhook Signature Verifier (P1)

**Problem:** 4 separate HMAC-SHA256 signature verification implementations.

**Files affected:**
- `internal/modules/git/providers/github.go:319-321`
- `internal/modules/git/providers/gitlab.go:159`
- `internal/modules/git/providers/bitbucket.go:90`
- `internal/modules/billing/handlers/webhook_handler.go:112`

**Current (repeated 4 times):**
```go
// GitHub
mac := hmac.New(sha256.New, []byte(p.config.WebhookSecret))
mac.Write(payload)
expectedSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
    return ErrInvalidSignature
}

// GitLab (slightly different format)
// Bitbucket (different header)
// Billing (Stripe format)
```

**Solution - Unified Signature Verifier:**
```go
// internal/pkg/webhook/signature.go
package webhook

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "strings"
)

type SignatureFormat int

const (
    FormatSHA256Prefix   SignatureFormat = iota // "sha256=..."
    FormatRaw                                    // Raw hex signature
    FormatStripe                                 // "t=timestamp,v1=signature"
)

type Verifier struct {
    secret string
    format SignatureFormat
}

func NewVerifier(secret string, format SignatureFormat) *Verifier {
    return &Verifier{secret: secret, format: format}
}

func (v *Verifier) Verify(payload []byte, signature string) bool {
    expected := v.computeSignature(payload)

    switch v.format {
    case FormatSHA256Prefix:
        return hmac.Equal([]byte("sha256="+expected), []byte(signature))
    case FormatRaw:
        return hmac.Equal([]byte(expected), []byte(signature))
    case FormatStripe:
        return v.verifyStripe(payload, signature)
    }
    return false
}

func (v *Verifier) computeSignature(payload []byte) string {
    mac := hmac.New(sha256.New, []byte(v.secret))
    mac.Write(payload)
    return hex.EncodeToString(mac.Sum(nil))
}

// Pre-configured verifiers
func GitHubVerifier(secret string) *Verifier {
    return NewVerifier(secret, FormatSHA256Prefix)
}

func GitLabVerifier(secret string) *Verifier {
    return NewVerifier(secret, FormatRaw)
}
```

**Impact:** ~80 lines saved, secure verification

---

## 145. Activity Logger Helper (P2)

**Problem:** 18+ files repeat identical activity logging boilerplate with optional user attribution.

**Files affected:**
- `internal/modules/server/jobs/archive_server.go:41-49`
- `internal/modules/server/jobs/unarchive_server.go:41-49`
- `internal/modules/server/jobs/delete_server.go:92-100`
- `internal/modules/database/jobs/install_database.go:79-87`
- Plus 14+ more job files

**Current (repeated 20+ times):**
```go
logger := activity.New(j.ctx.DB).
    WithContext(ctx).
    UseLog("server").
    On(server).
    WithEvent("archived")
if j.Payload.UserID != nil {
    logger.CausedByUser(*j.Payload.UserID)
}
logger.Log("Server has been archived")
```

**Solution - Activity Logger Helper:**
```go
// internal/pkg/activity/helpers.go
package activity

// LogAction is a helper for common activity logging pattern
func LogAction(db *gorm.DB, ctx context.Context, opts LogOptions) {
    logger := New(db).
        WithContext(ctx).
        UseLog(opts.Log).
        On(opts.Subject).
        WithEvent(opts.Event)

    if opts.UserID != nil {
        logger.CausedByUser(*opts.UserID)
    }

    if opts.Properties != nil {
        logger.WithProperties(opts.Properties)
    }

    logger.Log(opts.Description)
}

type LogOptions struct {
    Log         string
    Subject     Subject
    Event       string
    Description string
    UserID      *string
    Properties  map[string]any
}

// Usage becomes one-liner
activity.LogAction(j.ctx.DB, ctx, activity.LogOptions{
    Log:         "server",
    Subject:     server,
    Event:       "archived",
    Description: "Server has been archived",
    UserID:      j.Payload.UserID,
})
```

**Impact:** ~180 lines saved, consistent activity logging

---

## 146. Token Generation Consolidation (P2)

**Problem:** Duplicate token generation in model and utils package.

**Files affected:**
- `internal/pkg/utils/token.go:11-46` (4 functions)
- `internal/modules/auth/models/password_reset_token.go:31-39` (duplicate)

**Current (duplicated):**
```go
// utils/token.go
func GenerateSecureToken(byteLength int) (string, error) {
    bytes := make([]byte, byteLength)
    if _, err := rand.Read(bytes); err != nil {
        return "", err
    }
    return base64.URLEncoding.EncodeToString(bytes), nil
}

// models/password_reset_token.go - DUPLICATE
func GenerateToken(length int) (string, error) {
    bytes := make([]byte, length)
    if _, err := rand.Read(bytes); err != nil {
        return "", err
    }
    return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}
```

**Solution - Single Token Generator:**
```go
// internal/pkg/token/generator.go
package token

import (
    "crypto/rand"
    "encoding/base64"
    "encoding/hex"
)

type Encoding int

const (
    Base64URL Encoding = iota
    Hex
)

func Generate(byteLength int, encoding Encoding) (string, error) {
    bytes := make([]byte, byteLength)
    if _, err := rand.Read(bytes); err != nil {
        return "", err
    }

    switch encoding {
    case Hex:
        return hex.EncodeToString(bytes), nil
    default:
        return base64.URLEncoding.EncodeToString(bytes), nil
    }
}

// Convenience functions
func SecureToken() (string, error)     { return Generate(32, Base64URL) }
func ShortToken() (string, error)      { return Generate(8, Base64URL) }
func HexToken(length int) string       { s, _ := Generate(length, Hex); return s }

// Remove duplicate from models/password_reset_token.go
```

**Impact:** ~40 lines saved, single token source

---

## Extended Summary (Items 135-146)

| Category | Items | Est. LOC Saved |
|----------|-------|----------------|
| Webhook Channel Base Class | Item 135 | ~190 lines |
| Admin Alert Base Class | Item 136 | ~400 lines |
| SSH Client Factory | Item 137 | ~140 lines |
| Cron Schedule Constants | Item 138 | ~60 lines |
| Job Task Builder Generator | Item 139 | ~400 lines |
| Job Context Base Consolidation | Item 140 | ~250 lines |
| DNS Provider HTTP Base | Item 141 | ~200 lines |
| Backup Job Payload Base | Item 142 | ~30 lines |
| Service Dispatch Helper | Item 143 | ~50 lines |
| Webhook Signature Verifier | Item 144 | ~80 lines |
| Activity Logger Helper | Item 145 | ~180 lines |
| Token Generation Consolidation | Item 146 | ~40 lines |
| **Round 10 Total** | **12 patterns** | **~2020 lines** |

---

## Updated Final Summary

| Category | Items | Total LOC Saved |
|----------|-------|-----------------|
| Handler Bases | 5 patterns | ~400 lines |
| Repository Patterns | 5 patterns | ~1200 lines |
| Service Patterns | 3 patterns | ~450 lines |
| Job/Task Patterns | 3 patterns | ~800 lines |
| Model Mixins | 5 patterns | ~270 lines |
| Provider API Clients | 3 patterns | ~450 lines |
| Middleware Patterns | 1 pattern | ~30 lines |
| Config Patterns | 1 pattern | ~86 lines |
| DTO/Request Patterns | 2 patterns | ~550 lines |
| Testing Patterns | 2 patterns | ~250 lines |
| Infrastructure Patterns | 6 patterns | ~1075 lines |
| Validation & Response | 5 patterns | ~610 lines |
| Template & Script | 3 patterns | ~160 lines |
| Interface & Query | 4 patterns | ~500 lines |
| Security & Events | 3 patterns | ~100 lines |
| HTTP & Helpers (Round 3) | 12 patterns | ~1790 lines |
| Error & DI (Round 4) | 12 patterns | ~1320 lines |
| GORM & Jobs (Round 5) | 12 patterns | ~2290 lines |
| Context & Auth (Round 6) | 12 patterns | ~2560 lines |
| Validation & Enums (Round 7) | 12 patterns | ~1805 lines |
| Routes & Crypto (Round 8) | 12 patterns | ~3615 lines |
| HTTP Client & Config (Round 9) | 12 patterns | ~2480 lines |
| Notification & Jobs (Round 10) | 12 patterns | ~2020 lines |
| **Grand Total** | **146 patterns** | **~24,811 lines** |

---

## Round 11: Model Hooks, DTO Mapping, Status Patterns

---

## 147. BeforeCreate Hook Consolidation (P1)

**Problem:** 10+ models have BeforeCreate hooks with identical token/status initialization patterns.

**Files affected:**
- `internal/modules/server/models/server.go:74-88`
- `internal/modules/backup/models/backup_job.go:26-36`
- `internal/modules/site/models/site.go` (status init)
- `internal/modules/site/models/deployment.go` (status init)
- `internal/modules/database/models/database.go` (status init)
- `internal/modules/auth/models/user.go` (token init)
- `internal/modules/notification/models/notification_channel.go` (status init)
- `internal/modules/git/models/source_control.go` (status init)

**Current (repeated in 10+ models):**
```go
func (s *Server) BeforeCreate(tx *gorm.DB) error {
    if err := s.BaseModel.BeforeCreate(tx); err != nil {
        return err
    }
    if s.Status == "" {
        s.Status = enums.ServerStatusNew
    }
    if s.ConnectionToken == "" {
        s.ConnectionToken, _ = token.SecureToken()
    }
    return nil
}
```

**Solution - Embeddable Init Mixins:**
```go
// internal/pkg/models/init_mixins.go
package models

// StatusInitMixin for models with status field
type StatusInitMixin[T ~string] struct {
    defaultStatus T
}

func (m *StatusInitMixin[T]) InitStatus(current *T) {
    if *current == "" {
        *current = m.defaultStatus
    }
}

// TokenInitMixin for models needing secure tokens
type TokenInitMixin struct{}

func (m *TokenInitMixin) InitToken(field *string) {
    if *field == "" {
        *field, _ = token.SecureToken()
    }
}

// Combined mixin for common case
type StatusTokenMixin[T ~string] struct {
    StatusInitMixin[T]
    TokenInitMixin
}

// Model usage becomes:
type Server struct {
    BaseModel
    StatusTokenMixin[enums.ServerStatus]
    // ...
}

func (s *Server) BeforeCreate(tx *gorm.DB) error {
    if err := s.BaseModel.BeforeCreate(tx); err != nil {
        return err
    }
    s.InitStatus(&s.Status)
    s.InitToken(&s.ConnectionToken)
    return nil
}
```

**Impact:** ~80 lines saved, consistent initialization

---

## 148. ✅ COMPLETED - Generic Slice Mapper

**Problem:** Identical slice transformation pattern repeated in 8+ DTO files.

**Files affected:**
- `internal/modules/auth/dto/team.go:45-52`
- `internal/modules/auth/dto/user.go:38-45`
- `internal/modules/database/dto/database.go:35-42`
- `internal/modules/database/dto/database_user.go:28-35`
- `internal/modules/backup/dto/backup.go:40-47`
- `internal/modules/notification/dto/channel.go:30-37`
- `internal/modules/git/dto/source_control.go:25-32`
- `internal/modules/site/dto/site.go:55-62`

**Current (repeated 8+ times):**
```go
func ToTeamsResponse(teams []models.Team) []TeamResponse {
    responses := make([]TeamResponse, len(teams))
    for i, team := range teams {
        responses[i] = ToTeamResponse(team)
    }
    return responses
}

func ToDatabasesResponse(dbs []models.Database) []DatabaseResponse {
    responses := make([]DatabaseResponse, len(dbs))
    for i, db := range dbs {
        responses[i] = ToDatabaseResponse(db)
    }
    return responses
}
```

**Solution - Generic Mapper:**
```go
// internal/pkg/dto/mapper.go
package dto

// MapSlice transforms a slice using a mapping function
func MapSlice[T, R any](items []T, mapper func(T) R) []R {
    if items == nil {
        return nil
    }
    result := make([]R, len(items))
    for i, item := range items {
        result[i] = mapper(item)
    }
    return result
}

// Usage becomes one-liner:
func ToTeamsResponse(teams []models.Team) []TeamResponse {
    return dto.MapSlice(teams, ToTeamResponse)
}

// Or inline:
responses := dto.MapSlice(servers, dto.ToServerResponse)
```

**Impact:** ~120 lines saved, type-safe generic mapper

---

## 149. ✅ COMPLETED - Enum Response Helper

**Problem:** Status enums all have identical String() and Label() boilerplate.

**Files affected:**
- `internal/modules/server/enums/server_status.go:18-45`
- `internal/modules/site/enums/site_status.go:15-42`
- `internal/modules/site/enums/deployment_status.go:12-38`
- `internal/modules/database/enums/database_status.go:10-35`
- `internal/modules/backup/enums/backup_status.go:8-32`
- `internal/modules/billing/enums/subscription_status.go:10-36`

**Current (repeated per enum):**
```go
func (s ServerStatus) String() string {
    return string(s)
}

func (s ServerStatus) Label() string {
    labels := map[ServerStatus]string{
        ServerStatusNew:          "New",
        ServerStatusProvisioning: "Provisioning",
        ServerStatusRunning:      "Running",
        ServerStatusFailed:       "Failed",
    }
    return labels[s]
}

func (s ServerStatus) IsTerminal() bool {
    return s == ServerStatusRunning || s == ServerStatusFailed
}
```

**Solution - Enum Helper Generator:**
```go
// internal/pkg/enums/helper.go
package enums

// StringEnum provides common enum methods
type StringEnum interface {
    ~string
}

// Labels returns human-readable labels for enums
type Labels[T StringEnum] map[T]string

func (l Labels[T]) Label(e T) string {
    if label, ok := l[e]; ok {
        return label
    }
    return string(e)
}

// Usage in enum file:
var serverStatusLabels = enums.Labels[ServerStatus]{
    ServerStatusNew:          "New",
    ServerStatusProvisioning: "Provisioning",
    ServerStatusRunning:      "Running",
    ServerStatusFailed:       "Failed",
}

func (s ServerStatus) Label() string {
    return serverStatusLabels.Label(s)
}

// String() is trivial - just use string(s) inline
```

**Impact:** ~90 lines saved, consistent enum handling

---

## 150. ✅ COMPLETED - Pointer Dereference Utilities

**Problem:** Scattered nil-check pointer dereference patterns throughout handlers and DTOs.

**Files affected:**
- `internal/modules/server/handlers/server_handler.go` (multiple nil checks)
- `internal/modules/site/handlers/site_handler.go` (multiple nil checks)
- `internal/modules/auth/dto/user.go:25-30` (pointer fields)
- `internal/modules/backup/dto/backup.go:20-25` (pointer fields)
- Various job files with nullable payload fields

**Current (scattered):**
```go
// Pattern 1: Inline nil check
var name string
if req.Name != nil {
    name = *req.Name
}

// Pattern 2: Conditional assignment
if user.AvatarURL != nil {
    response.AvatarURL = *user.AvatarURL
}

// Pattern 3: Default value
timezone := "UTC"
if server.Timezone != nil {
    timezone = *server.Timezone
}
```

**Solution - Pointer Helpers:**
```go
// internal/pkg/ptr/deref.go
package ptr

// Deref returns the dereferenced value or zero value
func Deref[T any](p *T) T {
    if p == nil {
        var zero T
        return zero
    }
    return *p
}

// DerefOr returns the dereferenced value or default
func DerefOr[T any](p *T, defaultVal T) T {
    if p == nil {
        return defaultVal
    }
    return *p
}

// Ptr returns a pointer to the value
func Ptr[T any](v T) *T {
    return &v
}

// Usage:
name := ptr.Deref(req.Name)
timezone := ptr.DerefOr(server.Timezone, "UTC")
response.AvatarURL = ptr.Deref(user.AvatarURL)
```

**Impact:** ~60 lines saved, cleaner pointer handling

---

## 151. ✅ COMPLETED - URL Builder Package

**Problem:** API endpoint and webhook URL construction scattered with fmt.Sprintf.

**Files affected:**
- `internal/modules/git/services/webhook_service.go:45-52` (webhook URLs)
- `internal/modules/git/providers/github/client.go:30-45` (API URLs)
- `internal/modules/git/providers/gitlab/client.go:28-42` (API URLs)
- `internal/modules/git/providers/bitbucket/client.go:32-48` (API URLs)
- `internal/modules/server/providers/digitalocean/client.go:25-40` (API URLs)
- `internal/modules/server/providers/hetzner/client.go:22-38` (API URLs)
- `internal/modules/site/services/deployment_service.go:80-95` (callback URLs)

**Current (scattered):**
```go
// In webhook service
url := fmt.Sprintf("%s/api/webhooks/github/%s", config.AppURL, site.ID)

// In GitHub provider
url := fmt.Sprintf("https://api.github.com/repos/%s/hooks", repo)

// In deployment service  
callbackURL := fmt.Sprintf("%s/api/tasks/%s/callback", config.AppURL, taskID)
```

**Solution - URL Builder:**
```go
// internal/pkg/urlbuilder/builder.go
package urlbuilder

import (
    "fmt"
    "net/url"
    "path"
)

type Builder struct {
    base string
}

func New(baseURL string) *Builder {
    return &Builder{base: baseURL}
}

func (b *Builder) Path(segments ...string) string {
    u, _ := url.Parse(b.base)
    u.Path = path.Join(append([]string{u.Path}, segments...)...)
    return u.String()
}

func (b *Builder) Pathf(format string, args ...any) string {
    return b.Path(fmt.Sprintf(format, args...))
}

// API-specific builders
var (
    GitHub    = New("https://api.github.com")
    GitLab    = New("https://gitlab.com/api/v4")
    Bitbucket = New("https://api.bitbucket.org/2.0")
)

// App URL builder (from config)
func App() *Builder {
    return New(config.Get().AppURL)
}

// Usage:
url := urlbuilder.GitHub.Path("repos", repo, "hooks")
callbackURL := urlbuilder.App().Path("api", "tasks", taskID, "callback")
webhookURL := urlbuilder.App().Path("api", "webhooks", "github", site.ID)
```

**Impact:** ~100 lines saved, consistent URL construction

---

## 152. ✅ COMPLETED - Generic UpdateStatus Repository Method

**Problem:** Identical UpdateStatus methods in 5+ repositories.

**Files affected:**
- `internal/modules/server/repositories/server_repository.go:85-92`
- `internal/modules/site/repositories/site_repository.go:78-85`
- `internal/modules/site/repositories/deployment_repository.go:65-72`
- `internal/modules/database/repositories/database_repository.go:55-62`
- `internal/modules/backup/repositories/backup_repository.go:48-55`

**Current (repeated 5+ times):**
```go
func (r *ServerRepository) UpdateStatus(ctx context.Context, id string, status enums.ServerStatus) error {
    return r.DB().WithContext(ctx).
        Model(&models.Server{}).
        Where("id = ?", id).
        Update("status", status).Error
}

func (r *SiteRepository) UpdateStatus(ctx context.Context, id string, status enums.SiteStatus) error {
    return r.DB().WithContext(ctx).
        Model(&models.Site{}).
        Where("id = ?", id).
        Update("status", status).Error
}
```

**Solution - Generic Repository Method:**
```go
// internal/pkg/repository/status.go
package repository

import (
    "context"
    "gorm.io/gorm"
)

// StatusUpdater provides generic status update
type StatusUpdater[M any, S ~string] struct {
    db func() *gorm.DB
}

func NewStatusUpdater[M any, S ~string](dbFn func() *gorm.DB) *StatusUpdater[M, S] {
    return &StatusUpdater[M, S]{db: dbFn}
}

func (u *StatusUpdater[M, S]) UpdateStatus(ctx context.Context, id string, status S) error {
    var model M
    return u.db().WithContext(ctx).
        Model(&model).
        Where("id = ?", id).
        Update("status", status).Error
}

// Embed in repository:
type ServerRepository struct {
    BaseRepository[models.Server]
    *repository.StatusUpdater[models.Server, enums.ServerStatus]
}

func NewServerRepository(db *gorm.DB) *ServerRepository {
    base := NewBaseRepository[models.Server](db)
    return &ServerRepository{
        BaseRepository:  base,
        StatusUpdater:   repository.NewStatusUpdater[models.Server, enums.ServerStatus](base.DB),
    }
}
```

**Impact:** ~80 lines saved, type-safe status updates

---

## 153. Status Enum Base Generator (P2)

**Problem:** All status enums have identical structure with Values(), IsValid(), terminal states.

**Files affected:**
- `internal/modules/server/enums/server_status.go:50-75`
- `internal/modules/site/enums/site_status.go:45-70`
- `internal/modules/site/enums/deployment_status.go:40-65`
- `internal/modules/database/enums/database_status.go:35-58`
- `internal/modules/backup/enums/backup_status.go:30-52`
- `internal/modules/billing/enums/subscription_status.go:38-60`

**Current (repeated per enum):**
```go
func (s ServerStatus) Values() []ServerStatus {
    return []ServerStatus{
        ServerStatusNew,
        ServerStatusProvisioning,
        ServerStatusRunning,
        ServerStatusFailed,
    }
}

func (s ServerStatus) IsValid() bool {
    for _, v := range s.Values() {
        if s == v {
            return true
        }
    }
    return false
}

func (s ServerStatus) IsTerminal() bool {
    return s == ServerStatusRunning || s == ServerStatusFailed
}
```

**Solution - Enum Set Helper:**
```go
// internal/pkg/enums/set.go
package enums

// Set provides common enum operations
type Set[T comparable] struct {
    values   []T
    terminal map[T]bool
}

func NewSet[T comparable](values []T, terminal ...T) *Set[T] {
    s := &Set[T]{
        values:   values,
        terminal: make(map[T]bool),
    }
    for _, t := range terminal {
        s.terminal[t] = true
    }
    return s
}

func (s *Set[T]) Values() []T        { return s.values }
func (s *Set[T]) IsValid(v T) bool   { 
    for _, val := range s.values {
        if v == val { return true }
    }
    return false
}
func (s *Set[T]) IsTerminal(v T) bool { return s.terminal[v] }

// Usage:
var serverStatusSet = enums.NewSet(
    []ServerStatus{ServerStatusNew, ServerStatusProvisioning, ServerStatusRunning, ServerStatusFailed},
    ServerStatusRunning, ServerStatusFailed, // terminal states
)

func (s ServerStatus) IsValid() bool    { return serverStatusSet.IsValid(s) }
func (s ServerStatus) IsTerminal() bool { return serverStatusSet.IsTerminal(s) }
```

**Impact:** ~120 lines saved, declarative enum definitions

---

## 154. ✅ COMPLETED - Preload Scope Helpers

**Problem:** Identical preload chains repeated across repository methods.

**Files affected:**
- `internal/modules/backup/repositories/backup_repository.go:57-61,79-83,101-105` (3 identical)
- `internal/modules/site/repositories/site_repository.go:40-48,65-73` (2 identical)
- `internal/modules/server/repositories/server_repository.go:35-42,58-65` (2 identical)
- `internal/modules/git/repositories/source_control_repository.go:30-38,52-60` (2 identical)

**Current (backup repo - 3 EXACT duplicates):**
```go
// Lines 57-61, 79-83, 101-105 - IDENTICAL
.Preload("Jobs", func(db *gorm.DB) *gorm.DB {
    return db.Order("created_at DESC").Limit(50)
}).
.Preload("StorageProvider").
.Preload("Databases")
```

**Solution - Preload Scopes:**
```go
// internal/pkg/repository/preload.go
package repository

import "gorm.io/gorm"

// PreloadScope is a reusable preload configuration
type PreloadScope func(*gorm.DB) *gorm.DB

// Common preload scopes
func PreloadAll(scopes ...PreloadScope) func(*gorm.DB) *gorm.DB {
    return func(db *gorm.DB) *gorm.DB {
        for _, scope := range scopes {
            db = scope(db)
        }
        return db
    }
}

func PreloadOrdered(relation, orderBy string, limit int) PreloadScope {
    return func(db *gorm.DB) *gorm.DB {
        return db.Preload(relation, func(tx *gorm.DB) *gorm.DB {
            return tx.Order(orderBy).Limit(limit)
        })
    }
}

func Preload(relations ...string) PreloadScope {
    return func(db *gorm.DB) *gorm.DB {
        for _, rel := range relations {
            db = db.Preload(rel)
        }
        return db
    }
}

// Usage in backup repository:
var backupPreloads = repository.PreloadAll(
    repository.PreloadOrdered("Jobs", "created_at DESC", 50),
    repository.Preload("StorageProvider", "Databases"),
)

func (r *BackupRepository) FindByID(ctx context.Context, id string) (*models.Backup, error) {
    var backup models.Backup
    err := r.DB().WithContext(ctx).
        Scopes(backupPreloads).
        First(&backup, "id = ?", id).Error
    return &backup, err
}
```

**Impact:** ~90 lines saved, DRY preload configurations

---

## 155. Soft Delete Scope Unification (P2)

**Problem:** Mixed ArchivedAt and DeletedAt approaches with inconsistent filtering.

**Files affected:**
- `internal/modules/server/models/server.go:45` (ArchivedAt)
- `internal/modules/site/models/site.go:38` (ArchivedAt)
- `internal/modules/auth/models/team.go:28` (DeletedAt - GORM default)
- `internal/modules/backup/models/backup.go:22` (no soft delete)
- `internal/modules/server/repositories/server_repository.go:95-98` (manual WHERE)
- `internal/modules/site/repositories/site_repository.go:88-91` (manual WHERE)

**Current (inconsistent):**
```go
// Some models use ArchivedAt
type Server struct {
    ArchivedAt *time.Time `json:"archived_at,omitempty"`
}

// Manual filtering required
func (r *ServerRepository) FindActive(ctx context.Context) ([]models.Server, error) {
    var servers []models.Server
    err := r.DB().WithContext(ctx).
        Where("archived_at IS NULL").  // Manual!
        Find(&servers).Error
    return servers, err
}

// Other models use GORM's DeletedAt
type Team struct {
    gorm.DeletedAt
}
// GORM auto-filters
```

**Solution - Unified Archive Mixin:**
```go
// internal/pkg/models/archive.go
package models

import (
    "time"
    "gorm.io/gorm"
)

// ArchiveMixin provides consistent soft-delete via archiving
type ArchiveMixin struct {
    ArchivedAt *time.Time `gorm:"index" json:"archived_at,omitempty"`
}

func (m *ArchiveMixin) IsArchived() bool {
    return m.ArchivedAt != nil
}

func (m *ArchiveMixin) Archive() {
    now := time.Now()
    m.ArchivedAt = &now
}

func (m *ArchiveMixin) Restore() {
    m.ArchivedAt = nil
}

// Scope for active records
func ActiveScope(db *gorm.DB) *gorm.DB {
    return db.Where("archived_at IS NULL")
}

// Scope for archived records
func ArchivedScope(db *gorm.DB) *gorm.DB {
    return db.Where("archived_at IS NOT NULL")
}

// Usage in repository:
func (r *ServerRepository) FindActive(ctx context.Context) ([]models.Server, error) {
    var servers []models.Server
    err := r.DB().WithContext(ctx).
        Scopes(models.ActiveScope).
        Find(&servers).Error
    return servers, err
}
```

**Impact:** ~50 lines saved, consistent archiving

---

## 156. Site Type Helper Methods (P2)

**Problem:** Scattered site type checks throughout the codebase.

**Files affected:**
- `internal/modules/site/jobs/deploy_site.go:45-52` (IsLaravel check)
- `internal/modules/site/jobs/enable_laravel_queue.go:30-35` (IsLaravel check)
- `internal/modules/site/services/deployment_service.go:88-95` (multiple type checks)
- `internal/modules/site/handlers/site_handler.go:120-128` (type validation)
- `internal/modules/site/tasks/deploy.go:60-75` (feature-by-type logic)

**Current (scattered):**
```go
// Repeated pattern
if site.Type == enums.SiteTypeLaravel {
    // Laravel-specific logic
}

// Or with multiple checks
if site.Type == enums.SiteTypeLaravel || site.Type == enums.SiteTypeLaravelZeroDowntime {
    // Laravel-like logic
}

// Feature checks
supportsQueue := site.Type == enums.SiteTypeLaravel
supportsScheduler := site.Type == enums.SiteTypeLaravel
supportsHorizon := site.Type == enums.SiteTypeLaravel
```

**Solution - Site Type Helpers:**
```go
// internal/modules/site/enums/site_type_helpers.go
package enums

// Helper methods on SiteType
func (t SiteType) IsLaravel() bool {
    return t == SiteTypeLaravel || t == SiteTypeLaravelZeroDowntime
}

func (t SiteType) IsPHP() bool {
    return t.IsLaravel() || t == SiteTypeWordPress || t == SiteTypePHP
}

func (t SiteType) IsStatic() bool {
    return t == SiteTypeStatic || t == SiteTypeNextJS || t == SiteTypeNuxt
}

func (t SiteType) SupportsZeroDowntime() bool {
    return t == SiteTypeLaravelZeroDowntime
}

// Feature support helpers
func (t SiteType) SupportsQueue() bool      { return t.IsLaravel() }
func (t SiteType) SupportsScheduler() bool  { return t.IsLaravel() }
func (t SiteType) SupportsHorizon() bool    { return t.IsLaravel() }
func (t SiteType) SupportsMigrations() bool { return t.IsLaravel() }

// Usage becomes:
if site.Type.IsLaravel() {
    // Laravel-specific logic
}

if site.Type.SupportsQueue() {
    // Enable queue features
}
```

**Impact:** ~80 lines saved, centralized type logic

---

## 157. Feature Enable Base Job (P1)

**Problem:** 4 Laravel feature enable jobs have 90% identical structure.

**Files affected:**
- `internal/modules/site/jobs/enable_laravel_queue.go` (~150 lines)
- `internal/modules/site/jobs/enable_laravel_scheduler.go` (~150 lines)
- `internal/modules/site/jobs/enable_laravel_horizon.go` (~150 lines)
- `internal/modules/site/jobs/enable_laravel_inertia.go` (~140 lines)

**Current (near-identical jobs):**
```go
// enable_laravel_queue.go
type EnableLaravelQueueJob struct {
    ctx     *jobs.Context
    Payload EnableQueuePayload
}

func (j *EnableLaravelQueueJob) Handle() error {
    site, err := j.loadSite()
    if err != nil {
        return err
    }
    
    server, err := j.loadServer(site.ServerID)
    if err != nil {
        return err
    }
    
    j.broadcastStart(site)
    
    task := tasks.EnableLaravelQueue(site, server)
    result := j.ctx.TaskRunner.Execute(task, server)
    
    if result.Error != nil {
        j.broadcastFailed(site, result.Error)
        return result.Error
    }
    
    j.updateSiteFeature(site, "queue_enabled", true)
    j.broadcastSuccess(site)
    
    return nil
}
```

**Solution - Base Feature Enable Job:**
```go
// internal/modules/site/jobs/base_feature_job.go
package jobs

import (
    "context"
)

// FeatureConfig defines a feature to enable
type FeatureConfig struct {
    Name        string  // "queue", "scheduler", "horizon"
    DBField     string  // "queue_enabled", "scheduler_enabled"
    TaskBuilder func(site *models.Site, server *models.Server) taskrunner.Task
}

// BaseFeatureJob handles common feature enable logic
type BaseFeatureJob struct {
    ctx     *Context
    config  FeatureConfig
    payload BaseFeaturePayload
}

type BaseFeaturePayload struct {
    SiteID string `json:"site_id"`
}

func (j *BaseFeatureJob) Handle() error {
    site, err := j.loadSite()
    if err != nil {
        return err
    }
    
    server, err := j.loadServer(site.ServerID)
    if err != nil {
        return err
    }
    
    j.broadcastStart(site)
    
    task := j.config.TaskBuilder(site, server)
    result := j.ctx.TaskRunner.Execute(task, server)
    
    if result.Error != nil {
        j.broadcastFailed(site, result.Error)
        return result.Error
    }
    
    j.updateFeature(site, j.config.DBField, true)
    j.broadcastSuccess(site)
    
    return nil
}

// Specific jobs become minimal:
func NewEnableLaravelQueueJob(ctx *Context, payload EnableQueuePayload) *BaseFeatureJob {
    return &BaseFeatureJob{
        ctx: ctx,
        config: FeatureConfig{
            Name:        "queue",
            DBField:     "queue_enabled",
            TaskBuilder: tasks.EnableLaravelQueue,
        },
        payload: BaseFeaturePayload{SiteID: payload.SiteID},
    }
}
```

**Impact:** ~400 lines saved, feature jobs become config

---

## 158. Broadcast Event Builder (P2)

**Problem:** Identical nil-check wrapper methods in 3+ files for WebSocket broadcasting.

**Files affected:**
- `internal/modules/site/services/base.go:45-55` (3 wrapper methods)
- `internal/modules/site/jobs/context.go:60-72` (3 wrapper methods)
- `internal/pkg/taskrunner/callback.go:80-92` (3 wrapper methods)

**Current (repeated in 3 files):**
```go
func (s *Base) BroadcastToTeam(teamID, event string, data interface{}) {
    if s.WS == nil {
        return
    }
    s.WS.BroadcastToTeam(teamID, event, data)
}

func (s *Base) BroadcastToUser(userID, event string, data interface{}) {
    if s.WS == nil {
        return
    }
    s.WS.BroadcastToUser(userID, event, data)
}

func (s *Base) BroadcastToChannel(channel, event string, data interface{}) {
    if s.WS == nil {
        return
    }
    s.WS.BroadcastToChannel(channel, event, data)
}
```

**Solution - Broadcast Mixin:**
```go
// internal/pkg/broadcast/mixin.go
package broadcast

import "launch/internal/pkg/websocket"

// Mixin provides nil-safe broadcasting methods
type Mixin struct {
    WS *websocket.Hub
}

func (m *Mixin) BroadcastToTeam(teamID, event string, data interface{}) {
    if m.WS == nil {
        return
    }
    m.WS.BroadcastToTeam(teamID, event, data)
}

func (m *Mixin) BroadcastToUser(userID, event string, data interface{}) {
    if m.WS == nil {
        return
    }
    m.WS.BroadcastToUser(userID, event, data)
}

func (m *Mixin) BroadcastToChannel(channel, event string, data interface{}) {
    if m.WS == nil {
        return
    }
    m.WS.BroadcastToChannel(channel, event, data)
}

// Event builder for common patterns
type Event struct {
    Channel string
    Name    string
    Data    interface{}
}

func (m *Mixin) Broadcast(e Event) {
    m.BroadcastToChannel(e.Channel, e.Name, e.Data)
}

// Usage - embed in services/jobs:
type DeploymentService struct {
    broadcast.Mixin
    // ...
}

// Or with event builder:
s.Broadcast(broadcast.Event{
    Channel: fmt.Sprintf("team.%s", teamID),
    Name:    "deployment.started",
    Data:    map[string]any{"deployment_id": id},
})
```

**Impact:** ~60 lines saved, consistent broadcasting

---

## Extended Summary (Items 147-158)

| Category | Items | Est. LOC Saved |
|----------|-------|----------------|
| BeforeCreate Hook Consolidation | Item 147 | ~80 lines |
| Generic Slice Mapper | Item 148 | ~120 lines |
| Enum Response Helper | Item 149 | ~90 lines |
| Pointer Dereference Utilities | Item 150 | ~60 lines |
| URL Builder Package | Item 151 | ~100 lines |
| Generic UpdateStatus Repository | Item 152 | ~80 lines |
| Status Enum Base Generator | Item 153 | ~120 lines |
| Preload Scope Helpers | Item 154 | ~90 lines |
| Soft Delete Scope Unification | Item 155 | ~50 lines |
| Site Type Helper Methods | Item 156 | ~80 lines |
| Feature Enable Base Job | Item 157 | ~400 lines |
| Broadcast Event Builder | Item 158 | ~60 lines |
| **Round 11 Total** | **12 patterns** | **~1330 lines** |

---

## Updated Final Summary

| Category | Items | Total LOC Saved |
|----------|-------|-----------------|
| Handler Bases | 5 patterns | ~400 lines |
| Repository Patterns | 5 patterns | ~1200 lines |
| Service Patterns | 3 patterns | ~450 lines |
| Job/Task Patterns | 3 patterns | ~800 lines |
| Model Mixins | 5 patterns | ~270 lines |
| Provider API Clients | 3 patterns | ~450 lines |
| Middleware Patterns | 1 pattern | ~30 lines |
| Config Patterns | 1 pattern | ~86 lines |
| DTO/Request Patterns | 2 patterns | ~550 lines |
| Testing Patterns | 2 patterns | ~250 lines |
| Infrastructure Patterns | 6 patterns | ~1075 lines |
| Validation & Response | 5 patterns | ~610 lines |
| Template & Script | 3 patterns | ~160 lines |
| Interface & Query | 4 patterns | ~500 lines |
| Security & Events | 3 patterns | ~100 lines |
| HTTP & Helpers (Round 3) | 12 patterns | ~1790 lines |
| Error & DI (Round 4) | 12 patterns | ~1320 lines |
| GORM & Jobs (Round 5) | 12 patterns | ~2290 lines |
| Context & Auth (Round 6) | 12 patterns | ~2560 lines |
| Validation & Enums (Round 7) | 12 patterns | ~1805 lines |
| Routes & Crypto (Round 8) | 12 patterns | ~3615 lines |
| HTTP Client & Config (Round 9) | 12 patterns | ~2480 lines |
| Notification & Jobs (Round 10) | 12 patterns | ~2020 lines |
| Model Hooks & DTO (Round 11) | 12 patterns | ~1330 lines |
| **Grand Total** | **158 patterns** | **~26,141 lines** |

---

# CONSOLIDATED IMPLEMENTATION PLAN

This section analyzes the 158 patterns above and groups them into **15 actionable work packages** that can be implemented incrementally. Patterns within each package build upon each other and should be done together.

---

## Work Package Overview

| # | Package Name | Items | Est. LOC Saved | Priority | Effort |
|---|--------------|-------|----------------|----------|--------|
| 1 | Handler Infrastructure | 15 patterns | ~1,200 lines | P1 | High |
| 2 | Repository Infrastructure | 14 patterns | ~1,800 lines | P1 | High |
| 3 | Job/Task Infrastructure | 12 patterns | ~1,500 lines | P1 | Medium |
| 4 | Model Mixins | 11 patterns | ~600 lines | P2 | Medium |
| 5 | DTO/Response Infrastructure | 13 patterns | ~1,100 lines | P1 | Medium |
| 6 | HTTP Client Infrastructure | 11 patterns | ~900 lines | P1 | Medium |
| 7 | Template/Script Engine | 9 patterns | ~800 lines | P2 | Medium |
| 8 | Notification/Webhook | 10 patterns | ~600 lines | P2 | Low |
| 9 | Enum/Type Infrastructure | 8 patterns | ~500 lines | P2 | Low |
| 10 | Security/Crypto | 7 patterns | ~350 lines | P1 | Low |
| 11 | Broadcast/WebSocket | 9 patterns | ~500 lines | P2 | Medium |
| 12 | Config/Constants | 6 patterns | ~300 lines | P3 | Low |
| 13 | Error Handling | 5 patterns | ~200 lines | P1 | Low |
| 14 | Testing Infrastructure | 4 patterns | ~300 lines | P3 | Low |
| 15 | Utility Helpers | 24 patterns | ~800 lines | P3 | Low |
| **Total** | | **158 patterns** | **~11,450 lines** | | |

---

## Work Package 1: Handler Infrastructure (P1)

**Goal:** Create a unified handler base with context extraction, validation, and error handling.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 1 | WebSocket Handler Base Embedding | ~60 lines |
| 2 | Webhook Handler Base Embedding | ~35 lines |
| ~~5~~ | ~~Module Handler Field Consolidation~~ | ~~✅ COMPLETED~~ |
| 33 | ParseAndValidate Adoption | ~300 lines |
| 34 | Global Context Extraction Helpers | ~200 lines |
| 52 | Context Extraction Helpers | ~100 lines |
| 77 | WebSocket Handler Base Struct | ~60 lines |
| 84 | ParseAndValidate Adoption (duplicate) | (merged with 33) |
| 87 | Request Context Extraction | ~80 lines |
| 101 | Handler Validation Flow | ~80 lines |
| 111 | Route Middleware Chain | ~40 lines |
| 112 | Handler Module Aggregation | ~45 lines |

### Implementation Order:

**Step 1: Create Core Handler Base** (`internal/pkg/handler/`)
```go
// base.go - Handler base with logging and context
type Base struct {
    Logger *zerolog.Logger
}

// context.go - Safe context extraction
func GetTeamID(c *fiber.Ctx) (string, error)
func GetUserID(c *fiber.Ctx) (string, error)
func MustGetTeamID(c *fiber.Ctx) string

// request.go - Request parsing with validation
func ParseAndValidate[T any](c *fiber.Ctx) (*T, error)
func ParseQuery[T any](c *fiber.Ctx) (*T, error)

// errors.go - Service error to HTTP response mapping
func HandleServiceError(c *fiber.Ctx, err error) error
```

**Step 2: Create WebSocket Handler Base** (`internal/modules/websocket/handlers/base.go`)
```go
type BaseWSHandler struct {
    handler.Base
    DB              *gorm.DB
    JWTSecret       string
    MembershipCache *cache.TeamMembershipCache
}
```

**Step 3: Create Webhook Handler Base** (`internal/pkg/webhook/base.go`)
```go
type BaseWebhookHandler struct {
    handler.Base
    Signer *signedurl.Signer
}

func (h *BaseWebhookHandler) VerifySignature(c *fiber.Ctx) bool
func (h *BaseWebhookHandler) LogWebhookReceived(c *fiber.Ctx, webhookType string)
```

**Step 4: Refactor All Module Handlers**
- Embed `handler.Base` in all HTTP handlers
- Embed `BaseWSHandler` in WebSocket handlers
- Embed `BaseWebhookHandler` in webhook handlers
- Replace raw `c.Locals()` with safe extractors
- Replace BodyParser+Validate with `ParseAndValidate`

### Files to Create:
- [x] `internal/pkg/handler/base.go` (previously created)
- [x] `internal/pkg/handler/context.go` (previously created)
- [x] `internal/pkg/handler/request.go` (previously created)
- [x] `internal/pkg/handler/errors.go` (previously created)
- [x] `internal/pkg/webhook/base.go` ✅ COMPLETED
- [x] `internal/modules/websocket/handlers/base.go` ✅ COMPLETED

### Estimated Effort: 2-3 days

---

## Work Package 2: Repository Infrastructure (P2)

**Goal:** Create unified repository base with generic query builders, scopes, and common methods.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 3 | Repository BaseRepository Enforcement | ~450 lines |
| 7 | Installable Repository Mixin | ~200 lines |
| 39 | Backup Repository Many-to-Many Pattern | ~50 lines |
| 45 | GORM Query Helper Methods | ~300 lines |
| 50 | N+1 Query Fix | ~50 lines |
| 75 | GORM Query Scope - Order By Latest | ~40 lines |
| 88 | Repository Authorization Method | ~80 lines |
| 89 | Preload Chain Consolidation | ~60 lines |
| 90 | Service BaseService Accessor | ~40 lines |
| 96 | ServiceRegistry Boilerplate | ~30 lines |
| 107 | Repository Registry Factory | ~50 lines |
| 152 | Generic UpdateStatus Repository | ~80 lines |
| 154 | Preload Scope Helpers | ~90 lines |
| 155 | Soft Delete Scope Unification | ~50 lines |

### Implementation Order:

**Step 1: Enhance Repository Base** (`internal/pkg/repository/`)
```go
// base.go - Repository base with common methods
type Base[T any] struct {
    db *gorm.DB
}

func (r *Base[T]) DB() *gorm.DB
func (r *Base[T]) FindByID(ctx context.Context, id string) (*T, error)
func (r *Base[T]) Create(ctx context.Context, entity *T) error
func (r *Base[T]) Update(ctx context.Context, entity *T) error
func (r *Base[T]) Delete(ctx context.Context, id string) error

// status.go - Generic status update
func (r *Base[T]) UpdateStatus(ctx context.Context, id string, status any) error

// query.go - Fluent query builder
type Query[T any] struct { ... }
func (q *Query[T]) Where(cond string, args ...any) *Query[T]
func (q *Query[T]) Preload(rel string) *Query[T]
func (q *Query[T]) OrderByLatest() *Query[T]
func (q *Query[T]) First(ctx context.Context) (*T, error)
func (q *Query[T]) All(ctx context.Context) ([]T, error)
```

**Step 2: Create Scope Library** (`internal/pkg/repository/scopes.go`)
```go
// Already exists - extend with:
func PreloadOrdered(rel, order string, limit int) Scope
func WithActive() Scope  // archived_at IS NULL
func WithArchived() Scope
func JoinLatest(rel string) Scope
```

**Step 3: Create Authorization Mixin**
```go
// auth.go - Authorization helpers
type Authorizable[T any] struct {
    Base[T]
}

func (r *Authorizable[T]) FindByIDAndTeam(ctx context.Context, id, teamID string) (*T, error)
func (r *Authorizable[T]) FindByIDAndServer(ctx context.Context, id, serverID string) (*T, error)
```

**Step 4: Create Registry Factory**
```go
// registry.go - Already exists, enhance with:
func NewRegistry[R any](db *gorm.DB, factory func(*gorm.DB) R) *Registry[R]
```

### Files to Modify/Create:
- [x] Enhance `internal/pkg/repository/base.go` - Added FindByIDAndTeam, FindByIDAndTeamOrFail, UpdateStatus, UpdateStatusByServer
- [x] Enhance `internal/pkg/repository/scopes.go` - Added PreloadOrdered, PreloadWithScope, WithActive, WithArchived, OrderByLatest, OrderByOldest
- [x] Existing `internal/pkg/repository/query.go` - Already has fluent Query builder
- [x] `FindByIDAndTeam` added to base.go (no separate auth.go needed)
- [x] `UpdateStatus` added to base.go (no separate status.go needed)

### Refactored Files:
- [x] `internal/modules/server/repositories/server_repository.go` - Uses WithActive(), WithTeamID() scopes
- [x] `internal/modules/dashboard/services/dashboard_service.go` - Uses WithActive(), WithTeamID() scopes

### Estimated Effort: 2-3 days

---

## Work Package 3: Job/Task Infrastructure (P1)

**Goal:** Unified job context, task builders, and callback handling.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 6 | Job Base Payload Generic | ~100 lines |
| 10 | Task Builder Pattern | ~600 lines |
| 71 | JobContext Generic Base | ~80 lines |
| 79 | Job Struct Boilerplate | ~150 lines |
| 93 | JobContext Field Duplication | ~60 lines |
| 139 | Job Task Builder Generator | ~400 lines |
| 140 | Job Context Base Consolidation | ~250 lines |
| 142 | Backup Job Payload Base | ~30 lines |
| 143 | Service Dispatch Helper | ~50 lines |
| 157 | Feature Enable Base Job | ~400 lines |

### Implementation Order:

**Step 1: Create Generic Job Base** (`internal/pkg/jobs/`)
```go
// base.go - Job base with context
type Base struct {
    ctx *Context
}

func (j *Base) Context() *Context
func (j *Base) DB() *gorm.DB
func (j *Base) Queue() *queue.Client
func (j *Base) Dispatcher() *taskrunner.Dispatcher
func (j *Base) LogInfo(msg string, fields ...any)
func (j *Base) LogError(msg string, fields ...any)
func (j *Base) Broadcast(channel, event string, data any)

// payload.go - Generic payload handling
type BasePayload struct {
    TeamID   string `json:"team_id"`
    UserID   string `json:"user_id,omitempty"`
}

// builder.go - Task builder helper
type TaskBuilder struct { ... }
func (b *TaskBuilder) WithScript(script string) *TaskBuilder
func (b *TaskBuilder) WithTimeout(seconds int) *TaskBuilder
func (b *TaskBuilder) Build() taskrunner.Task
```

**Step 2: Create Feature Job Template**
```go
// feature_job.go - Base for feature enable jobs
type FeatureJob[P any] struct {
    Base
    Payload P
    Config  FeatureConfig
}

type FeatureConfig struct {
    Name        string
    DBField     string
    TaskBuilder func(*models.Site, *models.Server) taskrunner.Task
}

func (j *FeatureJob[P]) Handle() error {
    // Standard feature enable flow
}
```

**Step 3: Refactor Module Jobs**
- Embed `jobs.Base` in all job structs
- Use `TaskBuilder` for task creation
- Convert feature enable jobs to `FeatureJob`

### Files to Create:
- [ ] Enhance `internal/pkg/jobs/base.go`
- [ ] Create `internal/pkg/jobs/payload.go`
- [ ] Create `internal/pkg/jobs/builder.go`
- [ ] Create `internal/pkg/jobs/feature_job.go`

### Estimated Effort: 2 days

---

## Work Package 4: Model Mixins (P2)

**Goal:** Composable model mixins for common patterns.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 11 | Model Scoped Fields Adoption | ~100 lines |
| 15 | Model NamedModel Mixin | ~26 lines |
| 16 | Model StateTrackingModel Mixin | ~30 lines |
| 17 | Model ProgressTrackingModel Mixin | ~10 lines |
| 18 | Compound Scoping Mixins | ~100 lines |
| 57 | BeforeCreate Hook Consolidation | ~60 lines |
| 97 | Soft Delete Pattern | ~40 lines |
| 105 | BeforeCreate Hook Status Init | ~50 lines |
| 147 | BeforeCreate Hook Consolidation (R11) | ~80 lines |
| 155 | Soft Delete Scope Unification | ~50 lines |
| 156 | Site Type Helper Methods | ~80 lines |

### Implementation Order:

**Step 1: Create Model Mixins** (`internal/pkg/models/`)
```go
// mixins.go - Composable mixins
type Named struct {
    Name string `gorm:"size:255;not null" json:"name"`
}

type Described struct {
    Description *string `gorm:"type:text" json:"description,omitempty"`
}

type StatusTracking[S ~string] struct {
    Status S `gorm:"size:50;not null" json:"status"`
}

type ProgressTracking struct {
    Progress   int     `gorm:"default:0" json:"progress"`
    ProgressMessage *string `json:"progress_message,omitempty"`
}

type Archivable struct {
    ArchivedAt *time.Time `gorm:"index" json:"archived_at,omitempty"`
}

func (a *Archivable) IsArchived() bool
func (a *Archivable) Archive()
func (a *Archivable) Restore()

// hooks.go - BeforeCreate helpers
type StatusInitializer[S ~string] interface {
    DefaultStatus() S
}

func InitializeStatus[T StatusInitializer[S], S ~string](model *T) {
    // Set default status if empty
}

type TokenInitializer interface {
    TokenField() *string
}

func InitializeToken(model TokenInitializer) {
    // Generate secure token if empty
}
```

**Step 2: Create Scope Mixins**
```go
// scopes.go - Model-aware scopes
type TeamScoped struct {
    TeamID string `gorm:"size:26;not null;index" json:"team_id"`
}

type ServerScoped struct {
    ServerID string `gorm:"size:26;not null;index" json:"server_id"`
}

type SiteScoped struct {
    SiteID string `gorm:"size:26;not null;index" json:"site_id"`
}

// Compound scopes
type TeamServerScoped struct {
    TeamScoped
    ServerScoped
}
```

### Files to Create:
- [ ] Create `internal/pkg/models/mixins.go`
- [ ] Create `internal/pkg/models/hooks.go`
- [ ] Create `internal/pkg/models/scopes.go`

### Estimated Effort: 1-2 days

---

## Work Package 5: DTO/Response Infrastructure (P1)

**Goal:** Generic DTO helpers for timestamps, mapping, and pointer handling.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 14 | DTO Timestamp Embedding | ~400 lines |
| 24 | Request DTO Mixins | ~150 lines |
| 42 | Timestamp Formatting | ~40 lines |
| 58 | JSON Timestamp Helper | ~50 lines |
| 81 | DTO Timestamp Formatter | ~60 lines |
| 82 | DTO List Transformation | ~80 lines |
| 100 | Validation Field Embedding | ~40 lines |
| 103 | Model Broadcast Payload | ~50 lines |
| 108 | Slice Transformation Generic | ~60 lines |
| 115 | Nil-Safe Dereference Helpers | ~80 lines |
| 148 | Generic Slice Mapper | ~120 lines |
| 149 | Enum Response Helper | ~90 lines |
| 150 | Pointer Dereference Utilities | ~60 lines |

### Implementation Order:

**Step 1: Create DTO Helpers** (`internal/pkg/dto/`)
```go
// time.go - Already exists, enhance
func FormatTime(t *time.Time) *string
func FormatTimeValue(t time.Time) string
func ParseTime(s string) (*time.Time, error)

// ptr.go - Pointer utilities
func Deref[T any](p *T) T
func DerefOr[T any](p *T, defaultVal T) T
func Ptr[T any](v T) *T
func NilIfEmpty(s *string) *string

// mapper.go - Slice mapping
func MapSlice[T, R any](items []T, fn func(T) R) []R
func MapSlicePtr[T, R any](items []T, fn func(T) *R) []*R
func FilterSlice[T any](items []T, fn func(T) bool) []T

// enum.go - Enum response helpers
type EnumResponse struct {
    Value string `json:"value"`
    Label string `json:"label"`
}

func EnumToResponse[E ~string](e E, labelFn func(E) string) EnumResponse
func EnumsToResponses[E ~string](values []E, labelFn func(E) string) []EnumResponse
```

**Step 2: Create Response Mixins**
```go
// response.go - Common response fields
type TimestampResponse struct {
    CreatedAt string  `json:"created_at"`
    UpdatedAt string  `json:"updated_at"`
}

type AuditResponse struct {
    TimestampResponse
    CreatedBy *string `json:"created_by,omitempty"`
    UpdatedBy *string `json:"updated_by,omitempty"`
}

func NewTimestampResponse(createdAt, updatedAt time.Time) TimestampResponse
```

**Step 3: Create Request Mixins**
```go
// request.go - Common request fields
type PaginationRequest struct {
    Page    int `query:"page" validate:"min=1"`
    PerPage int `query:"per_page" validate:"min=1,max=100"`
}

func (r *PaginationRequest) Normalize()

type SortRequest struct {
    SortBy    string `query:"sort_by"`
    SortOrder string `query:"sort_order" validate:"omitempty,oneof=asc desc"`
}
```

### Files to Create/Modify:
- [ ] Enhance `internal/pkg/dto/time.go`
- [ ] Create `internal/pkg/dto/ptr.go`
- [ ] Create `internal/pkg/dto/mapper.go`
- [ ] Create `internal/pkg/dto/enum.go`
- [ ] Create `internal/pkg/dto/response.go`
- [ ] Create `internal/pkg/dto/request.go`

### Estimated Effort: 1-2 days

---

## Work Package 6: HTTP Client Infrastructure (P1)

**Goal:** Unified HTTP client base for all API providers.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 19 | Cloud Provider BaseAPIClient | ~200 lines |
| 20 | Git Provider BaseGitAPIClient | ~150 lines |
| 51 | HTTP Client Base with Request Builder | ~200 lines |
| 80 | Cloud Provider HTTP Client | ~100 lines |
| 91 | Retry Logic Consolidation | ~100 lines |
| 92 | HTTP Client Factory | ~50 lines |
| 123 | HTTP Client Base Pattern | ~100 lines |
| 124 | API Request Builder | ~80 lines |
| 125 | Response Handler Consolidation | ~100 lines |
| 129 | Retry Strategy Consolidation | ~100 lines |
| 141 | DNS Provider HTTP Base | ~100 lines |

### Implementation Order:

**Step 1: Create HTTP Client Base** (`internal/pkg/httpclient/`)
```go
// client.go - Base HTTP client
type Client struct {
    baseURL    string
    httpClient *http.Client
    headers    map[string]string
    retryConfig RetryConfig
    logger     *zerolog.Logger
}

func New(baseURL string, opts ...Option) *Client
func (c *Client) Get(ctx context.Context, path string) (*Response, error)
func (c *Client) Post(ctx context.Context, path string, body any) (*Response, error)
func (c *Client) Put(ctx context.Context, path string, body any) (*Response, error)
func (c *Client) Delete(ctx context.Context, path string) (*Response, error)

// options.go - Client options
type Option func(*Client)
func WithTimeout(d time.Duration) Option
func WithHeader(key, value string) Option
func WithBearerToken(token string) Option
func WithRetry(config RetryConfig) Option
func WithLogger(logger *zerolog.Logger) Option

// retry.go - Retry logic
type RetryConfig struct {
    MaxRetries int
    BackoffFunc func(attempt int) time.Duration
    RetryableStatus []int
}

func ExponentialBackoff(base time.Duration) func(int) time.Duration

// request.go - Request builder
type RequestBuilder struct { ... }
func (b *RequestBuilder) WithQuery(key, value string) *RequestBuilder
func (b *RequestBuilder) WithHeader(key, value string) *RequestBuilder
func (b *RequestBuilder) WithBody(body any) *RequestBuilder
func (b *RequestBuilder) Execute(ctx context.Context) (*Response, error)
```

**Step 2: Create Provider-Specific Bases**
```go
// internal/pkg/providers/cloud/base.go
type BaseCloudClient struct {
    *httpclient.Client
}

func (c *BaseCloudClient) HandleAPIError(resp *httpclient.Response) error

// internal/pkg/providers/git/base.go
type BaseGitClient struct {
    *httpclient.Client
}

func (c *BaseGitClient) Paginate(ctx context.Context, path string, handler func([]byte) bool) error
```

### Files to Create:
- [ ] Create `internal/pkg/httpclient/client.go`
- [ ] Create `internal/pkg/httpclient/options.go`
- [ ] Create `internal/pkg/httpclient/retry.go`
- [ ] Create `internal/pkg/httpclient/request.go`
- [ ] Create `internal/pkg/httpclient/response.go`
- [ ] Create `internal/pkg/providers/cloud/base.go`
- [ ] Create `internal/pkg/providers/git/base.go`

### Estimated Effort: 2-3 days

---

## Work Package 7: Template/Script Engine (P2)

**Goal:** Unified template loading and script building.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 10 | Task Builder Pattern | (shared with WP3) |
| 30 | Script Builder Interface | ~500 lines |
| 31 | Shell Defaults Registry | ~60 lines |
| 35 | Template Loading Consolidation | ~80 lines |
| 36 | Systemd Config Script | ~60 lines |
| 37 | Home Directory Helper | ~20 lines |
| 55 | Home Directory Helper (dup) | (merged) |
| 56 | Task File Path Builder | ~40 lines |
| 65 | Template Engine Consolidation | ~100 lines |
| 114 | Template Engine Consolidation (dup) | (merged) |
| 119 | Home Directory Helper (dup) | (merged) |

### Implementation Order:

**Step 1: Create Script Builder** (`internal/pkg/script/`)
```go
// builder.go - Script builder
type Builder struct {
    sections []Section
}

type Section struct {
    Name    string
    Content string
}

func New() *Builder
func (b *Builder) ShellDefaults(strict bool) *Builder
func (b *Builder) Section(name, content string) *Builder
func (b *Builder) Function(name, body string) *Builder
func (b *Builder) Command(cmd string, args ...string) *Builder
func (b *Builder) Conditional(condition string, body *Builder) *Builder
func (b *Builder) Build() string

// paths.go - Path helpers
func HomeDir(user string) string
func SiteDir(user, site string) string
func ReleaseDir(user, site, release string) string
func SharedDir(user, site string) string
```

**Step 2: Create Template Engine**
```go
// internal/pkg/templates/engine.go
type Engine struct {
    templates *template.Template
    funcMap   template.FuncMap
}

func NewEngine() *Engine
func (e *Engine) LoadFromFS(fs embed.FS, pattern string) error
func (e *Engine) LoadFromGlob(pattern string) error
func (e *Engine) Render(name string, data any) (string, error)
func (e *Engine) MustRender(name string, data any) string

// funcs.go - Standard template functions
func CommonFuncMap() template.FuncMap  // Already exists, enhance
```

### Files to Create:
- [ ] Create `internal/pkg/script/builder.go`
- [ ] Create `internal/pkg/script/paths.go`
- [ ] Enhance `internal/pkg/taskrunner/templates/engine.go`

### Estimated Effort: 1-2 days

---

## Work Package 8: Notification/Webhook (P2)

**Goal:** Unified notification channel and webhook handling.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 21 | Notification BaseWebhookChannel | ~100 lines |
| 40 | Webhook Response Standardization | ~30 lines |
| 66 | Slack Admin Alert Base | ~80 lines |
| 67 | Notification Format Helper | ~60 lines |
| 68 | Webhook Signature Verification | ~50 lines |
| 113 | Webhook Route Standardization | ~40 lines |
| 135 | Webhook Channel Base Class | ~90 lines |
| 136 | Admin Alert Base Class | ~100 lines |
| 144 | Webhook Signature Verifier | ~80 lines |

### Implementation Order:

**Step 1: Create Webhook Channel Base** (`internal/pkg/notification/`)
```go
// channel.go - Base webhook channel
type BaseWebhookChannel struct {
    Name       string
    WebhookURL string
    httpClient *httpclient.Client
    logger     *zerolog.Logger
}

func (c *BaseWebhookChannel) Send(ctx context.Context, payload any) error
func (c *BaseWebhookChannel) FormatMessage(template string, data any) string

// slack.go - Slack-specific
type SlackChannel struct {
    BaseWebhookChannel
}

func (c *SlackChannel) SendAlert(ctx context.Context, level, title, message string) error
func (c *SlackChannel) SendBlocks(ctx context.Context, blocks []slack.Block) error

// discord.go - Discord-specific
type DiscordChannel struct {
    BaseWebhookChannel
}
```

**Step 2: Create Admin Alert System**
```go
// alert.go - Admin alerting
type AdminAlerter struct {
    channels []AlertChannel
}

type AlertChannel interface {
    SendAlert(ctx context.Context, alert Alert) error
}

type Alert struct {
    Level   AlertLevel
    Title   string
    Message string
    Fields  map[string]string
}

func (a *AdminAlerter) Alert(ctx context.Context, alert Alert) error
```

### Files to Create:
- [ ] Create `internal/pkg/notification/channel.go`
- [ ] Create `internal/pkg/notification/slack.go`
- [ ] Create `internal/pkg/notification/discord.go`
- [ ] Create `internal/pkg/notification/alert.go`

### Estimated Effort: 1 day

---

## Work Package 9: Enum/Type Infrastructure (P2)

**Goal:** Reduce enum boilerplate with generators and type helpers.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 28 | Enum Scan/Value Boilerplate | ~315 lines |
| 32 | Encrypted Field Types | ~30 lines |
| 106 | Enum Boilerplate Consolidation | ~80 lines |
| 116 | JSONMap Type Deduplication | ~50 lines |
| 117 | Enum SQL Scanner Generator | ~60 lines |
| 149 | Enum Response Helper | (shared with WP5) |
| 153 | Status Enum Base Generator | ~120 lines |

### Implementation Order:

**Step 1: Create Enum Helpers** (`internal/pkg/enums/`)
```go
// base.go - Enum base types
type StringEnum interface {
    ~string
    String() string
}

// scanner.go - GORM scanner for enums
type Scanner[E StringEnum] struct {
    Value E
}

func (s *Scanner[E]) Scan(value any) error
func (s Scanner[E]) Value() (driver.Value, error)

// set.go - Enum set with validation
type Set[E comparable] struct {
    values   []E
    terminal map[E]bool
    labels   map[E]string
}

func NewSet[E comparable](values []E) *Set[E]
func (s *Set[E]) WithTerminal(values ...E) *Set[E]
func (s *Set[E]) WithLabels(labels map[E]string) *Set[E]
func (s *Set[E]) IsValid(v E) bool
func (s *Set[E]) IsTerminal(v E) bool
func (s *Set[E]) Label(v E) string
func (s *Set[E]) Values() []E
```

**Step 2: Create Custom Types**
```go
// types.go - Reusable custom types
type JSONMap map[string]any

func (m *JSONMap) Scan(value any) error
func (m JSONMap) Value() (driver.Value, error)

type EncryptedString struct {
    value     string
    encrypted string
}

func (e *EncryptedString) Set(value string, key []byte) error
func (e *EncryptedString) Get(key []byte) (string, error)
```

### Files to Create:
- [ ] Create `internal/pkg/enums/base.go`
- [ ] Create `internal/pkg/enums/scanner.go`
- [ ] Create `internal/pkg/enums/set.go`
- [ ] Create `internal/pkg/types/json.go`
- [ ] Create `internal/pkg/types/encrypted.go`

### Estimated Effort: 1 day

---

## Work Package 10: Security/Crypto (P1)

**Goal:** Consolidated crypto utilities for tokens, signatures, and encryption.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 46 | GitLab Webhook Timing Attack Fix | ~20 lines |
| 49 | Crypto Package Consolidation | ~50 lines |
| 72 | Token Generation Consolidation | ~40 lines |
| 120 | HMAC Signature Validator | ~80 lines |
| 121 | Token Generator Consolidation | ~40 lines |
| 122 | Time Expiration Checker | ~50 lines |
| 146 | Token Generation Consolidation (dup) | (merged) |

### Implementation Order:

**Step 1: Create Crypto Package** (`internal/pkg/crypto/`)
```go
// token.go - Token generation
func SecureToken(byteLength int) (string, error)
func SecureTokenHex(byteLength int) (string, error)
func URLSafeToken(byteLength int) (string, error)

// hmac.go - HMAC signatures
func SignHMAC(message, secret []byte) []byte
func VerifyHMAC(message, signature, secret []byte) bool
func VerifyHMACConstantTime(message, signature, secret []byte) bool  // Timing-safe

// expiry.go - Expiration checking
func IsExpired(expiresAt *time.Time) bool
func ExpiresIn(duration time.Duration) time.Time
func TimeRemaining(expiresAt time.Time) time.Duration
```

### Files to Create:
- [ ] Create `internal/pkg/crypto/token.go`
- [ ] Create `internal/pkg/crypto/hmac.go`
- [ ] Create `internal/pkg/crypto/expiry.go`

### Estimated Effort: 0.5 days

---

## Work Package 11: Broadcast/WebSocket (P2)

**Goal:** Unified broadcasting and WebSocket message building.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 8 | Queue Dispatch Unification | ~100 lines |
| 12 | Activity Logging Builder | ~150 lines |
| 13 | Broadcast Payload Builder | ~50 lines |
| 47 | Broadcast Event Constants | ~60 lines |
| 61 | Broadcast Channel Builder | ~40 lines |
| 78 | WebSocket Message Builder | ~50 lines |
| 102 | Broadcaster Interface Dedup | ~30 lines |
| 158 | Broadcast Event Builder | ~60 lines |

### Implementation Order:

**Step 1: Create Broadcast Helpers** (`internal/pkg/broadcast/`)
```go
// events.go - Event constants
const (
    EventCreated = "created"
    EventUpdated = "updated"
    EventDeleted = "deleted"
    EventProgress = "progress"
    EventStatus = "status"
)

// channels.go - Channel builders
func TeamChannel(teamID string) string
func UserChannel(userID string) string
func ServerChannel(serverID string) string
func DeploymentChannel(deploymentID string) string

// payload.go - Already exists, enhance
type Payload struct {
    Event string         `json:"event"`
    Model string         `json:"model"`
    ID    string         `json:"id"`
    Data  any            `json:"data"`
    Meta  map[string]any `json:"meta,omitempty"`
}

func NewPayload(model, id, event string, data any) Payload
func (p *Payload) WithMeta(key string, value any) *Payload

// mixin.go - Broadcast mixin for embedding
type Mixin struct {
    WS *websocket.Hub
}

func (m *Mixin) Broadcast(channel string, payload Payload)
func (m *Mixin) BroadcastToTeam(teamID string, payload Payload)
```

### Files to Modify/Create:
- [ ] Create `internal/pkg/broadcast/events.go`
- [ ] Create `internal/pkg/broadcast/channels.go`
- [ ] Enhance `internal/pkg/broadcast/payload.go`
- [ ] Create `internal/pkg/broadcast/mixin.go`

### Estimated Effort: 1 day

---

## Work Package 12: Config/Constants (P3)

**Goal:** Standardized configuration loading and constants.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 23 | Config Builder Pattern | ~86 lines |
| 60 | Config Loading with Struct Tags | ~80 lines |
| 98 | Timeout Configuration | ~50 lines |
| 134 | Config Loader Helper | ~80 lines |
| 138 | Cron Schedule Constants | ~30 lines |

### Implementation: Already largely complete in `internal/pkg/config/loader.go`

**Enhancements needed:**
```go
// internal/pkg/config/loader.go - Add:
func LoadFromEnvWithPrefix[T any](prefix string) (*T, error)
func ValidateConfig(cfg any) error

// internal/pkg/constants/timeouts.go
const (
    DefaultHTTPTimeout     = 30 * time.Second
    DefaultSSHTimeout      = 2 * time.Minute
    DefaultDeployTimeout   = 10 * time.Minute
    DefaultProvisionTimeout = 30 * time.Minute
)

// internal/pkg/constants/cron.go
const (
    CronEveryMinute     = "* * * * *"
    CronEvery5Minutes   = "*/5 * * * *"
    CronEveryHour       = "0 * * * *"
    CronDaily           = "0 0 * * *"
    CronWeekly          = "0 0 * * 0"
)
```

### Files to Create:
- [ ] Create `internal/pkg/constants/timeouts.go`
- [ ] Create `internal/pkg/constants/cron.go`
- [ ] Enhance `internal/pkg/config/loader.go`

### Estimated Effort: 0.5 days

---

## Work Package 13: Error Handling (P1)

**Goal:** Unified error types with HTTP status mapping.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 41 | AppError HTTPStatus Interface | ~30 lines |
| 63 | Error Type Consolidation | ~80 lines |
| 99 | Duplicate AppError Type | ~50 lines |
| 83 | Activity Logging Helper | (shared with WP11) |

### Implementation: Already largely complete in `internal/pkg/errors/`

**Enhancements needed:**
```go
// internal/pkg/errors/app.go - Ensure single AppError
type AppError struct {
    Code       string `json:"code"`
    Message    string `json:"message"`
    StatusCode int    `json:"-"`
    Cause      error  `json:"-"`
}

func (e *AppError) Error() string
func (e *AppError) Unwrap() error
func (e *AppError) HTTPStatus() int  // For fiber error handler

// Constructors
func NewAppError(code, message string, status int) *AppError
func Wrap(err error, message string) *AppError
func WrapWithCode(err error, code, message string, status int) *AppError
```

### Files to Modify:
- [ ] Consolidate to single `internal/pkg/errors/app.go`
- [ ] Remove duplicate AppError definitions

### Estimated Effort: 0.5 days

---

## Work Package 14: Testing Infrastructure (P3)

**Goal:** Test base classes and mocking utilities.

### Patterns Consolidated:
| Item | Pattern Name | LOC Saved |
|------|--------------|-----------|
| 25 | Test Base Classes | ~200 lines |
| 85 | MockRepository Code Generation | ~80 lines |
| 95 | NotifierFake Lock Wrapper | ~20 lines |

### Implementation Order:

**Step 1: Create Test Helpers** (`internal/pkg/testing/`)
```go
// base.go - Test suite base
type Suite struct {
    suite.Suite
    DB     *gorm.DB
    Ctx    context.Context
    Cancel context.CancelFunc
}

func (s *Suite) SetupTest()
func (s *Suite) TearDownTest()
func (s *Suite) CreateTestDB() *gorm.DB
func (s *Suite) TruncateTables(tables ...string)

// mock.go - Mock helpers
type MockRepository[T any] struct {
    mock.Mock
}

func (m *MockRepository[T]) FindByID(ctx context.Context, id string) (*T, error)
// ... other common methods

// fixtures.go - Test data factories
type Factory[T any] struct {
    build func() *T
}

func NewFactory[T any](build func() *T) *Factory[T]
func (f *Factory[T]) Create() *T
func (f *Factory[T]) CreateN(n int) []*T
```

### Files to Create:
- [ ] Create `internal/pkg/testing/base.go`
- [ ] Create `internal/pkg/testing/mock.go`
- [ ] Create `internal/pkg/testing/fixtures.go`

### Estimated Effort: 1 day

---

## Work Package 15: Utility Helpers (P3)

**Goal:** Misc utility helpers that don't fit other packages.

### Patterns Consolidated:
| Item | Pattern Name |
|------|--------------|
| 37, 55, 119 | Home Directory Helper |
| 53 | JWT Token Parser Helper |
| 54 | Redis Client Factory |
| 73 | Git Ref Trimming Helper |
| 74 | Channel Validation Rules |
| 76 | Sync Associations Pattern |
| 86 | Email Normalization |
| 94 | Double-Check Lock Pattern |
| 109 | Safe Map Access Helper |
| 110 | Collection Filter-Map Pattern |
| 126 | Pagination Parameter Parser |
| 127 | Dynamic Sort Handler |
| 128 | Filter Options Pattern |
| 130 | Metrics Parsing Helper |
| 131 | Service Status Checker |
| 132 | Daemon Status Task Dedup |
| 133 | Health Check Aggregator |
| 151 | URL Builder Package |

### Implementation: Create as needed, these are lower priority standalone utilities.

**Key utilities to create:**
```go
// internal/pkg/utils/path.go
func HomeDir(user string) string
func ExpandPath(path string) string

// internal/pkg/utils/string.go
func NormalizeEmail(email string) string
func TrimGitRef(ref string) string
func Truncate(s string, maxLen int) string

// internal/pkg/utils/map.go
func SafeGet[K comparable, V any](m map[K]V, key K) (V, bool)
func SafeGetOr[K comparable, V any](m map[K]V, key K, defaultVal V) V
func Keys[K comparable, V any](m map[K]V) []K
func Values[K comparable, V any](m map[K]V) []V

// internal/pkg/urlbuilder/builder.go
type Builder struct { baseURL string }
func New(baseURL string) *Builder
func (b *Builder) Path(segments ...string) string
func (b *Builder) WithQuery(params map[string]string) string
```

### Estimated Effort: 1-2 days (as needed)

---

## Implementation Phases

### Phase 1: Core Infrastructure (Week 1-2)
Focus on high-impact, foundational packages:

1. **Work Package 1: Handler Infrastructure** - Creates foundation for all handlers
2. **Work Package 5: DTO/Response Infrastructure** - Used by all handlers
3. **Work Package 10: Security/Crypto** - Critical security fixes
4. **Work Package 13: Error Handling** - Consolidate error types

**Expected Outcome:** ~3,000 lines reduced, unified handler pattern

### Phase 2: Data Layer (Week 3-4)
Focus on repository and model patterns:

5. **Work Package 2: Repository Infrastructure** - Unify all repositories
6. **Work Package 4: Model Mixins** - Reduce model boilerplate
7. **Work Package 9: Enum/Type Infrastructure** - Reduce enum boilerplate

**Expected Outcome:** ~3,000 lines reduced, consistent data access

### Phase 3: Service Layer (Week 5-6)
Focus on jobs, tasks, and HTTP clients:

8. **Work Package 3: Job/Task Infrastructure** - Unify all jobs
9. **Work Package 6: HTTP Client Infrastructure** - Unify API providers
10. **Work Package 7: Template/Script Engine** - Unify script building

**Expected Outcome:** ~3,200 lines reduced, consistent service patterns

### Phase 4: Polish (Week 7-8)
Focus on remaining patterns:

11. **Work Package 8: Notification/Webhook** - Unify notifications
12. **Work Package 11: Broadcast/WebSocket** - Unify broadcasting
13. **Work Package 12: Config/Constants** - Standardize config
14. **Work Package 14: Testing Infrastructure** - Improve testability
15. **Work Package 15: Utility Helpers** - Misc utilities

**Expected Outcome:** ~2,250 lines reduced, complete consolidation

---

## Success Metrics

After completing all work packages:

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| Total LOC | ~65,000 | ~53,550 | -17.6% |
| Handler boilerplate | ~1,200 lines | ~200 lines | -83% |
| Repository duplication | ~1,800 lines | ~300 lines | -83% |
| Job boilerplate | ~1,500 lines | ~300 lines | -80% |
| Enum boilerplate | ~500 lines | ~100 lines | -80% |
| Type safety issues | 177 unsafe casts | 0 | -100% |

---

## Dependency Graph

```
Work Package Dependencies:

WP1 (Handler) ─────────────────────────────────────┐
    │                                               │
    ├── WP5 (DTO) ←── Used by handlers             │
    │       │                                       │
    │       └── WP9 (Enum) ←── Used by DTOs        │
    │                                               │
WP2 (Repository) ──────────────────────────────────┤
    │                                               │
    ├── WP4 (Model) ←── Used by repositories       │
    │                                               │
WP3 (Job/Task) ────────────────────────────────────┤
    │                                               │
    ├── WP7 (Template) ←── Used by tasks           │
    │                                               │
WP6 (HTTP Client) ─────────────────────────────────┤
    │                                               │
    ├── WP8 (Notification) ←── Uses HTTP client    │
    │                                               │
WP10 (Crypto) ─────────────────────────────────────┤
    │                                               │
WP11 (Broadcast) ──────────────────────────────────┤
    │                                               │
WP12 (Config) ─────────────────────────────────────┤
    │                                               │
WP13 (Error) ──────────────────────────────────────┘
    │
WP14 (Testing) ←── Can be done anytime
    │
WP15 (Utility) ←── Can be done anytime
```

---

## Quick Wins (< 1 hour each)

These can be done immediately without full work package:

1. **Item 46: GitLab Timing Attack Fix** - Security fix, 10 minutes
2. **Item 151: URL Builder** - Simple utility, 30 minutes
3. **Item 150: Pointer Helpers** - Simple utility, 20 minutes
4. **Item 148: Generic Slice Mapper** - Simple utility, 20 minutes
5. **Item 138: Cron Constants** - Simple constants, 10 minutes

---

## Notes for Implementation

1. **Start with interfaces** - Define interfaces before implementations
2. **Use generics wisely** - Go 1.18+ generics reduce boilerplate significantly
3. **Maintain backwards compatibility** - Keep old methods as deprecated wrappers
4. **Write tests first** - Each new package should have comprehensive tests
5. **Document patterns** - Update CLAUDE.md with new patterns
6. **Gradual adoption** - Don't refactor all at once, do module by module

---

*Last Updated: 2026-01-21*
*Total Patterns: 158*
*Estimated Total LOC Reduction: ~11,450 lines (17.6% of codebase)*
