# DodoPayments → Polar.sh Migration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans (or subagent-driven-development) to implement this plan task-by-task.

**Goal:** Replace DodoPayments with Polar.sh as the billing provider — clean cutover (no data migration), monthly-only (yearly removed everywhere), Polar hosted checkout (redirect), keeping the 3 plans (Hobby/Compact/Turbo) and their limits unchanged.

**Architecture:** The billing module is already provider-abstracted. We introduce a small `BillingProvider` interface in `internal/modules/billing/providers`, implement it with a new `PolarClient` (Go SDK `github.com/polarsource/polar-go`), switch the service + webhook handler to depend on the interface, wire Polar in `module.go`, then delete all DodoPayments code. Plan product IDs move to env. A console command (`billing:polar-setup`, exposed as `make polar-setup`) creates the 3 products in Polar and prints their IDs. Polar uses the same Standard Webhooks spec as Dodo, so the webhook handler shape barely changes.

**Tech Stack:** Go (Fiber v2, GORM, cobra console), `github.com/polarsource/polar-go`, Nuxt 3/Vue 3 frontend, Standard Webhooks.

**Tracking issue:** kkz6/launch-go#121

---

## Reference: current state (read before starting)

- Provider client: `internal/modules/billing/providers/dodo_payments.go` — methods `CreateCheckout(ctx, productID, productName, teamID, redirectURL, customerEmail, customerName) (string, error)`, `GetSubscription`, `CancelSubscription`, `ResumeSubscription`, `UpdateSubscription`, `GetCustomerPortalURL`, `GetUpdatePaymentMethodURL`, `UnwrapWebhook`, `UnsafeUnwrapWebhook`.
- Service: `internal/modules/billing/services/billing_service.go` — field `dodoPayments *providers.DodoPaymentsClient`; consumes `CreateCheckout`, `CancelSubscription`, `ResumeSubscription`, `GetUpdatePaymentMethodURL`. Builds `plansByProductID`/`plansByVariantID` from `MonthlyID`+`YearlyID`. `GenerateCheckoutURL` picks `plan.YearlyID` when `req.Annual`.
- Webhook handler: `internal/modules/billing/handlers/webhook_handler.go` — verifies via `UnwrapWebhook(body, webhook-id, webhook-signature, webhook-timestamp)`, dispatches events to `WebhookService` (`internal/modules/billing/services/webhook_service.go`).
- Webhook service handlers: `CreateOrUpdateSubscription`, `UpdateSubscription`, `CancelSubscriptionByWebhook`, `ExpireSubscription`, `HandleSubscriptionFailed`, `PauseSubscription`, `HandleSubscriptionRenewed`, `CreateOrder`, `HandlePaymentFailed`, `RefundOrder`, `HandleDisputeOpened`, `HandleDisputeResolved`.
- Routes: `internal/modules/billing/routes.go` — `POST /webhooks/dodo-payments`; team-scoped `/billing/*`.
- Models: `models/plan.go` (in-memory `DefaultPlans()` with hardcoded Dodo product IDs + Yearly fields), `models/subscription.go`, `models/order.go` (provider-agnostic ID columns).
- Config: `internal/config/services.go` — `BillingConfig{SubscriptionsEnabled, WebhookSecret(env DODO_PAYMENTS_WEBHOOK_KEY), DodoPayments}`; `DodoPaymentsConfig{APIKey(env DODO_PAYMENTS_API_KEY), TestMode(env DODO_PAYMENTS_TEST_MODE)}`.
- Module wiring: `internal/modules/billing/module.go` — constructs `DodoPaymentsClient` when `cfg.DodoPayments.APIKey != ""`.
- Frontend: `plugins/dodo-payments.client.ts`, `components/settings/BillingTab.vue`, `components/billing/PricingModal.vue`, landing pricing tables, `package.json` dep `dodopayments-checkout`.

## Polar SDK quick reference (verify exact field names against the SDK while implementing)

```go
import polargo "github.com/polarsource/polar-go"

s := polargo.New(
    polargo.WithServer("sandbox"), // or "production"
    polargo.WithSecurity(token),
)
s.Checkouts.Create(ctx, components.CheckoutCreate{ Products: []string{productID}, SuccessURL: ..., CustomerEmail: ..., Metadata: map[string]...{"team_id": teamID} })
s.Products.Create(ctx, components.ProductCreate{ Name, RecurringInterval: "month", Prices: [...], OrganizationID })
s.Subscriptions.Get(ctx, id) / s.Subscriptions.Update(ctx, id, ...) / s.Subscriptions.Revoke(ctx, id)
s.CustomerSessions.Create(ctx, components.CustomerSessionCreate{ CustomerID })
// Webhook verification: github.com/polarsource/polar-go/webhooks → ValidateEvent(body, headers, secret)
```

Polar webhook events → existing handlers:
| Polar event | Action |
|---|---|
| `subscription.created`, `subscription.active` | `CreateOrUpdateSubscription` (team_id from checkout/subscription metadata) → status `active`/`on_trial` |
| `subscription.updated` | `UpdateSubscription` |
| `subscription.canceled` | `CancelSubscriptionByWebhook` (set EndsAt = current_period_end) |
| `subscription.uncanceled` | clear cancel → status `active`, EndsAt nil |
| `subscription.revoked` | `ExpireSubscription` (status `expired`) |
| `subscription.past_due` | `HandleSubscriptionFailed` / status `past_due` |
| `order.created`, `order.paid` | `CreateOrder` (status `paid`) |
| `order.refunded` | `RefundOrder` |
| `checkout.*`, `customer.*`, `benefit_grant.*`, `product.*`, `organization.*` | store + log, no state change |

---

## Phase 1 — Backend: provider interface + Polar client

### Task 1: Define the `BillingProvider` interface

**Files:**
- Create: `internal/modules/billing/providers/provider.go`

**Step 1:** Extract the methods the service + webhook handler actually use into an interface:
```go
package providers

import "context"

// BillingProvider is the payment-provider contract the billing service and
// webhook handler depend on. Implemented by PolarClient.
type BillingProvider interface {
    CreateCheckout(ctx context.Context, productID, productName, teamID, redirectURL, customerEmail, customerName string) (string, error)
    CancelSubscription(ctx context.Context, providerSubscriptionID string) error
    ResumeSubscription(ctx context.Context, providerSubscriptionID string) error
    UpdateSubscription(ctx context.Context, providerSubscriptionID, productID string) error
    GetUpdatePaymentMethodURL(ctx context.Context, customerID string) (string, error)
}
```
**Step 2:** `go build ./internal/modules/billing/...` — expected PASS (interface only).
**Step 3:** Commit: `git commit -m "Add BillingProvider interface for the billing module"`

### Task 2: Add the Polar Go SDK dependency

**Files:** Modify: `go.mod`, `go.sum`

**Step 1:** `go get github.com/polarsource/polar-go@latest`
**Step 2:** `go build ./...` — PASS.
**Step 3:** Commit: `git commit -m "Add polar-go SDK dependency"`

### Task 3: Implement `PolarClient`

**Files:**
- Create: `internal/modules/billing/providers/polar.go`
- Test: `internal/modules/billing/providers/polar_test.go`

**Step 1:** Implement a `PolarConfig{AccessToken, WebhookSecret, OrganizationID, Sandbox bool}` and `PolarClient` that satisfies `BillingProvider`. Map methods to the SDK:
- `CreateCheckout`: `s.Checkouts.Create` with `Products: []string{productID}`, `SuccessURL: redirectURL`, `CustomerEmail`, `Metadata: {"team_id": teamID}`. Return the hosted `URL`.
- `CancelSubscription`: `s.Subscriptions.Update(id, {CancelAtPeriodEnd: true})`.
- `ResumeSubscription`: `s.Subscriptions.Update(id, {CancelAtPeriodEnd: false})`.
- `UpdateSubscription`: `s.Subscriptions.Update(id, {ProductID: productID})`.
- `GetUpdatePaymentMethodURL`: `s.CustomerSessions.Create({CustomerID})` → return the customer-portal URL.
- Add `ValidateWebhook(payload []byte, headers http.Header) ([]byte, error)` using the polar-go `webhooks` validator with `WebhookSecret` (verifies + returns the validated body for event-type parsing). Add a `CreateProduct(ctx, name string, priceCents int64) (productID string, err error)` helper for the setup command (Task 9).
- Add a `mapError` mirroring the Dodo one (401/404/429 → domain errors).

**Step 2:** Unit test the pure bits only (don't hit the network): construction (`NewPolarClient` returns non-nil, picks sandbox vs production server), and `mapError` mapping. Verify the type satisfies the interface with `var _ BillingProvider = (*PolarClient)(nil)`.
```go
func TestPolarClientImplementsProvider(t *testing.T) {
    var _ providers.BillingProvider = (*providers.PolarClient)(nil)
}
```
**Step 3:** `go test ./internal/modules/billing/providers/...` — PASS.
**Step 4:** `golangci-lint run ./internal/modules/billing/providers/...` — clean.
**Step 5:** Commit: `git commit -m "Implement Polar billing provider"`

---

## Phase 2 — Backend: config, models, service, module wiring

### Task 4: Replace billing config

**Files:** Modify: `internal/config/services.go`, `.env.example`

**Step 1:** Replace `DodoPaymentsConfig` + the Dodo fields on `BillingConfig` with:
```go
type BillingConfig struct {
    SubscriptionsEnabled bool   `env:"BILLING_SUBSCRIPTIONS_ENABLED" default:"false"`
    Polar                PolarConfig
}

type PolarConfig struct {
    AccessToken    string `env:"POLAR_ACCESS_TOKEN" default:""`
    WebhookSecret  string `env:"POLAR_WEBHOOK_SECRET" default:""`
    OrganizationID string `env:"POLAR_ORGANIZATION_ID" default:""`
    Sandbox        bool   `env:"POLAR_SANDBOX" default:"true"`
    ProductHobby   string `env:"POLAR_PRODUCT_HOBBY" default:""`
    ProductCompact string `env:"POLAR_PRODUCT_COMPACT" default:""`
    ProductTurbo   string `env:"POLAR_PRODUCT_TURBO" default:""`
}
```
**Step 2:** `.env.example`: remove the `DODO_PAYMENTS_*` block; add `POLAR_ACCESS_TOKEN`, `POLAR_WEBHOOK_SECRET`, `POLAR_ORGANIZATION_ID`, `POLAR_SANDBOX=true`, `POLAR_PRODUCT_HOBBY/COMPACT/TURBO`, keep `BILLING_SUBSCRIPTIONS_ENABLED=false`.
**Step 3:** `go build ./internal/config/...` — will fail to compile elsewhere (module.go still refs Dodo); that's fixed in Task 6.
**Step 4:** Commit after Task 6 builds (config + model + wiring land together).

### Task 5: Plans — monthly only, IDs from config

**Files:** Modify: `internal/modules/billing/models/plan.go`; Test: `internal/modules/billing/models/plan_test.go`

**Step 1:** Remove `YearlyID` and `YearlyPricing` from `Plan`. Change `DefaultPlans()` → `PlansFromConfig(hobbyID, compactID, turboID string) []Plan` that returns the 3 plans with `MonthlyID` set from the passed IDs (keep names, pricing 199/699/2000, features, limits, `Recommended`). Update `PlanByID` to take the slice or read from a package-level built set — simplest: keep `PlanByID(plans []Plan, id string)`.
**Step 2:** Test `PlansFromConfig` returns 3 plans with the given monthly IDs and no yearly fields; pricing/limits unchanged.
**Step 3:** `go test ./internal/modules/billing/models/...` — PASS.
**Step 4:** Commit with Task 6.

### Task 6: Service + module wiring → Polar

**Files:** Modify: `internal/modules/billing/services/billing_service.go`, `internal/modules/billing/module.go`

**Step 1 (service):** Change field `dodoPayments *providers.DodoPaymentsClient` → `provider providers.BillingProvider`. Update `NewBillingService` signature param. Remove `plansByVariantID` and the `YearlyID` indexing (monthly only). In `GenerateCheckoutURL` drop the `req.Annual` branch — always `plan.MonthlyID`. Replace all `s.dodoPayments.X` calls with `s.provider.X`. Replace `GetDodoPaymentsClient()` with `GetProvider() providers.BillingProvider` (or remove if unused). Guard `s.provider == nil` as today.
**Step 2 (module):** Build `PolarConfig`/`PolarClient` when `cfg.Polar.AccessToken != ""`; pass `models.PlansFromConfig(cfg.Polar.ProductHobby, cfg.Polar.ProductCompact, cfg.Polar.ProductTurbo)` into `services.Config.Plans`; pass the client (as `providers.BillingProvider`) into `NewBillingService`.
**Step 3:** `go build ./...` — PASS (Dodo file still present but now unused by service; that's fine until Task 8).
**Step 4:** `go test ./internal/modules/billing/...` — PASS.
**Step 5:** Commit: `git commit -m "Switch billing service + plans to Polar (monthly-only, config-driven IDs)"`

### Task 7: Webhook handler + route → Polar

**Files:** Modify: `internal/modules/billing/handlers/webhook_handler.go`, `internal/modules/billing/routes.go`; the `DodoWebhookType`/event constants the handler switches on.

**Step 1:** Change verification to `provider.ValidateWebhook(body, c.GetReqHeaders()...)` (Standard Webhooks: `webhook-id`/`webhook-signature`/`webhook-timestamp`). Re-map the event switch to Polar event names per the table above. Reuse all `WebhookService` handlers. Keep storing raw events in `billing_webhook_events`.
**Step 2:** Route: rename `POST /webhooks/dodo-payments` → `POST /webhooks/polar` in `routes.go`.
**Step 3:** Add/adjust a unit test for the event-name→handler mapping (pure switch, no network) if one exists; otherwise a small table test of the mapping function.
**Step 4:** `go build ./... && go test ./internal/modules/billing/...` — PASS.
**Step 5:** Commit: `git commit -m "Handle Polar webhooks at /webhooks/polar"`

### Task 8: Delete DodoPayments

**Files:** Delete: `internal/modules/billing/providers/dodo_payments.go`; Modify: `go.mod`/`go.sum` (drop `github.com/dodopayments/dodopayments-go`).

**Step 1:** Remove the file; `go mod tidy`. Grep both repos for `dodo`/`Dodo` (case-insensitive) → zero hits in launch-go except historical migrations/comments (leave migration history intact; update comments that say "DodoPayments product IDs").
**Step 2:** `go build ./... && go vet ./... && golangci-lint run ./internal/modules/billing/...` — clean.
**Step 3:** Commit: `git commit -m "Remove DodoPayments integration"`

---

## Phase 3 — Backend: Polar product setup command

### Task 9: `billing:polar-setup` console command + `make polar-setup`

**Files:** Create: `cmd/console/polar.go`; Modify: `cmd/console/main.go` (register), `Makefile` (add target).

**Step 1:** Implement a cobra command (mirror `aboutCommand` in `cmd/console/main.go`): `Signature() "billing:polar-setup"`, category `billing`. In `Handle`, read `POLAR_ACCESS_TOKEN` + `POLAR_ORGANIZATION_ID` from config; construct `PolarClient`; for each of the 3 plans (Hobby 199, Compact 699, Turbo 2000) call `CreateProduct(name, priceCents)` (monthly recurring) and collect the returned product IDs. Print a `ctx.Table([]string{"Plan","Env var","Product ID"}, ...)` instructing the operator to set `POLAR_PRODUCT_HOBBY/COMPACT/TURBO`. If a product with that name already exists, list it instead of creating a duplicate (look up via `s.Products.List` filtered by org+name).
**Step 2:** Register `&polarSetupCommand{}` in `cmd/console/main.go`'s `cliApp.Register(...)`.
**Step 3:** `Makefile`: add
```make
polar-setup: ## Create Polar products and print their IDs
	go run ./cmd/console billing:polar-setup
```
**Step 4:** `go build ./cmd/console` — PASS. (Cannot run live without a real token; build-verify only.)
**Step 5:** Commit: `git commit -m "Add billing:polar-setup command to create Polar products"`

---

## Phase 4 — Frontend (launch-nuxt)

### Task 10: Remove the Dodo checkout SDK + plugin

**Files:** Delete: `plugins/dodo-payments.client.ts`; Modify: `package.json` (drop `dodopayments-checkout`), `BillingTab.vue`.

**Step 1:** Remove the plugin file and the dep. In `BillingTab.vue`, replace `DodoPayments.Checkout.open(url)` / `$dodoCheckout.open(url)` with `window.location.href = url` (redirect to Polar hosted checkout). Keep the `POST /billing/generate-checkout-url` call.
**Step 2:** `npm install` to update lockfile.
**Step 3:** `npm run build` — PASS.
**Step 4:** Commit: `git commit -m "Use Polar hosted checkout redirect; remove Dodo checkout SDK"`

### Task 11: Remove yearly from the UI

**Files:** Modify: `components/billing/PricingModal.vue`, the landing pricing tables (`components/.../LandingPricingTables*` / `pages/pricing.vue`), `BillingTab.vue`, and the checkout request type/service (drop the `annual` flag).

**Step 1:** Delete the monthly/yearly toggle, the yearly price column/line, and the "save X%" savings copy. Show monthly price only. Remove `isAnnual` from `selectPlan`/`handleCheckout` and the `annual` field from the `GenerateCheckoutURLRequest` payload in `services/`.
**Step 2:** `npm run build` — PASS.
**Step 3:** Commit: `git commit -m "Remove yearly billing option from the UI"`

### Task 12 (backend): drop `annual` from the checkout request DTO

**Files:** Modify: `internal/modules/billing/dto/requests.go` (`GenerateCheckoutURLRequest` — remove `Annual`), and any handler/test referencing it.

**Step 1:** Remove the field; `go build ./... && go test ./internal/modules/billing/...` — PASS.
**Step 2:** Commit: `git commit -m "Drop annual flag from checkout request"`

---

## Phase 5 — Verify, document, ship

### Task 13: Full verification

**Step 1:** `go build ./... && go vet ./... && golangci-lint run ./...` (launch-go) — all clean.
**Step 2:** `go test ./internal/modules/billing/...` — PASS.
**Step 3:** `npm run build` (launch-nuxt) — PASS.
**Step 4:** Grep both repos case-insensitively for `dodo` → only acceptable hits are historical DB migration filenames/contents (must remain) and the `billing_webhook_events` rows. No code/config/UI references remain.

### Task 14: Docs + rollout notes

**Files:** Modify: `.env.example` (done in Task 4), add a short `docs/` note or README section: how to run `make polar-setup`, set the 3 product-ID env vars, configure the webhook endpoint (`/webhooks/polar`) + secret in the Polar dashboard, and flip `POLAR_SANDBOX=false` for production.

**Step 1:** Write the note. **Step 2:** Commit.

### Task 15: Ship

**Step 1:** Open PRs (launch-go + launch-nuxt, `Refs #121`), code review, admin-merge.
**Step 2:** Backend release (next tag). Frontend deploys via its pipeline.
**Step 3:** In production: set `POLAR_ACCESS_TOKEN` + `POLAR_ORGANIZATION_ID`, run `make polar-setup`, paste the 3 product IDs into env, set `POLAR_WEBHOOK_SECRET` + register the `/webhooks/polar` endpoint in Polar, keep `POLAR_SANDBOX=true` until verified end-to-end, then flip to `false` and set `BILLING_SUBSCRIPTIONS_ENABLED=true`.
**Step 4:** Move #121 to `verification` with the end-to-end test steps (checkout → webhook → active → gating flips → cancel/resume). Do NOT close.

---

## Notes / risks
- Verify exact polar-go request/response field names against the installed SDK version (the snippets above are indicative). The interface insulates the rest of the codebase from SDK specifics.
- `subscriptions`/`orders` columns are provider-agnostic — no destructive migration. A tiny migration may flip the `provider` column default string to `polar`; existing rows are irrelevant (clean cutover).
- Keep the `BILLING_SUBSCRIPTIONS_ENABLED` gate so nothing changes for users until Polar is verified in prod.
- Webhook signature: Polar = Standard Webhooks, same `webhook-id`/`webhook-signature`/`webhook-timestamp` headers as Dodo — reuse the header-extraction code.
