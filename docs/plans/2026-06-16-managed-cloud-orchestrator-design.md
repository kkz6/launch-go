# Managed Cloud Orchestrator (Launch v2) — Design & Delivery Plan

- **Status:** Proposed
- **Date:** 2026-06-16
- **Owner:** kkz6
- **Tracking:** GitHub epics labelled `epic` + `managed-orchestrator`, grouped by phase milestones, surfaced on the account-level project **"Launch · Managed Cloud Orchestrator"**.

---

## 1. Summary

Today Launch is a **PaaS that runs on top of VMs you control over SSH** (DigitalOcean,
Hetzner, Linode, Vultr, and AWS-as-EC2). Every provider implements one VM-centric
`Provider` interface: `Create()` a server, get its `PublicIPv4`, SSH in, install
Docker + Traefik, deploy sites.

This document designs a second mode in which **Launch becomes an orchestrator / control
plane that sits above the customer's own cloud account** and provisions **managed
services** (think FlightControl): AWS Fargate/RDS/ElastiCache/Route 53, GCP Cloud
Run/Cloud SQL/Memorystore/Cloud DNS — with no SSH and no Docker host to babysit.

The customer works in **intents** ("I need a database"), and Launch decides the right
managed service, provisions it, tracks its state, shows it on a combined dashboard, and
analyses its cost.

**Verdict:** feasible for **AWS and GCP now**, with **Azure deferrable** by design. The
hard 20% is the substrate (networking, image build, credentials, partial-failure
recovery), not the individual services.

## 2. Goals / Non-goals

### Goals
- A mode chosen at signup: **Managed** (single cloud + API credentials) or **Custom**
  (the existing SSH + Docker on VMs flow, unchanged).
- **Intent-based** resource model: Launch picks the managed service from the customer's
  intent + budget/HA hints, with override.
- A **combined dashboard** across all resources in the account: status, health,
  connection info, provisioning progress, and cost.
- **Pricing analysis**: (a) pre-deploy estimate, (b) live actual spend, (c) optimization
  advice.
- A **provider-agnostic abstraction** so Azure (and others) slot in later as just another
  implementation.

### Non-goals (v1)
- Multi-cloud *per single account* (each managed account targets one cloud).
- Kubernetes / Crossplane control plane (explicitly deferred).
- Cross-cloud cost comparison at signup (later; same estimate engine).

## 3. Architecture

Managed mode is a **second "driver" behind the same product surface**. An account carries
a `mode` (`managed` | `custom`). The Custom path is untouched. Managed mode adds a path
that never touches SSH:

```
Customer & dashboard
        │  expresses intent, sees cost
        ▼
Launch orchestrator (launch-go)
  Intent ──► Planner ──► Pulumi engine
  (desired   (picks the   (provisions +
   state)     service)     tracks state)
  runs on existing: Asynq workers · WebSocket · GORM
        │
        ▼
Customer's cloud account (BYOC)
  RDS/Cloud SQL · ElastiCache/Memorystore · Fargate/Cloud Run · Route 53/Cloud DNS
        │
        ▼
Cost engine  ◄── Pulumi preview (estimate) + cloud billing APIs (spend/optimize)
```

- **Intent** — the typed desired-state record stored via GORM. Source of truth.
- **Planner** — the opinionated brain. Given intent + cloud + budget/HA hints, it picks
  the managed service and surrounding wiring (VPC, subnets, security groups, IAM) and
  emits a concrete plan the customer can preview and override.
- **Pulumi engine** — takes the plan and runs it through Pulumi's **Automation API**
  (Go, inline programs) against the customer's credentials. Owns state, dependency
  ordering, drift, and rollback.

All slow/async/realtime machinery already exists: Pulumi runs execute inside **Asynq**
workers, progress streams over the **WebSocket** hub, desired/actual state lives in
**GORM**.

## 4. Why Pulumi (not raw SDK)

An orchestrator of managed services needs desired-vs-actual **state**, **dependency
ordering**, **drift detection**, and **rollback**. With direct SDK calls these are
hand-built for every service on every cloud — the surface multiplies
`RDS × ElastiCache × EC2 × Route53 × VPC × IAM … × GCP × Azure`.

Pulumi gives all four as built-ins, its **Automation API** is purpose-built for embedding
provisioning inside a control plane, resources are expressed in **real Go** (reuse our
types/tests), and `preview` (dry-run) doubles as the foundation of the **pre-deploy cost
estimate**. One provider model spans AWS/GCP/Azure. FlightControl itself is built on
Pulumi.

**Cost:** adds a runtime dependency + learning curve, and a **state backend** that holds
the keys to customers' clouds and must be KMS-encrypted and per-tenant isolated.

## 5. The abstraction

Our domain concepts already exist — `Database`, `Domain`, `Certificate`, `Application` —
but they are **`ServerScoped`** (bound to a VM). Managed mode lifts that binding: a
resource belongs to an **Environment** (cloud account + region), not a Server. DNS is
already provider-abstracted via `DomainProvider`, so Route 53 and Cloud DNS are just two
more providers. We **extend**, not greenfield.

One Go interface per **primitive**, with an implementation per cloud, registered by
`(kind, cloud)`:

```go
type Primitive interface {
    // Planner: turn an intent + environment into a concrete resource plan
    Plan(intent Intent, env Environment) (ResourcePlan, error)
    // Pulumi engine: declare the real managed resources inside an inline program
    Materialize(ctx *pulumi.Context, plan ResourcePlan, wiring Wiring) (Outputs, error)
    // Cost engine: price the plan before apply
    Estimate(plan ResourcePlan) (CostBreakdown, error)
}
// registry[Database][AWS] = awsDatabase{}   registry[Database][GCP] = gcpDatabase{}
// Azure later: registry[Database][Azure] = azureDatabase{}
```

`Outputs` from one primitive (a DB host) become `Wiring` inputs to another (the app's env
var). The Pulumi inline program walks the resolved plan and instantiates each primitive as
a `ComponentResource`, so ordering, state, and rollback come from Pulumi.

## 6. Per-primitive feasibility (AWS + GCP, now)

| Primitive (intent) | AWS managed | GCP managed | Notes / risk |
|---|---|---|---|
| **Service** (HTTP app) | App Runner, or ECS Fargate + ALB | Cloud Run | Cloud Run trivial; Fargate needs ALB+VPC wiring. Needs an image. |
| **Worker** (background) | ECS Fargate (no LB) | Cloud Run job / no-ingress | — |
| **Database** (relational) | RDS / Aurora | Cloud SQL | private IP + connector wiring; HA/backup flags |
| **Cache** (Redis) | ElastiCache | Memorystore | private-only; compute must reach over VPC |
| **Object storage** | S3 | GCS | easy |
| **DNS / domain** | Route 53 | Cloud DNS | easiest — extend `DomainProvider` |
| **TLS cert** | ACM | Google-managed cert | auto-issued, bound to LB/Run |
| **Static site + CDN** | CloudFront + S3 | Cloud CDN + GCS | — |
| **Secrets** | Secrets Manager / SSM | Secret Manager | inject into compute env |
| **Container registry** | ECR | Artifact Registry | needed for the build path |
| **Network substrate** | VPC, subnets, SG, NAT | VPC, firewall, Serverless VPC connector | the hard part; implicit dependency of nearly everything |
| **Queue / Cron** (phase 2) | SQS / EventBridge | Pub/Sub / Cloud Scheduler | defer to v2 |

Every row is supported by mature `pulumi-aws` / `pulumi-gcp` providers.

## 7. The hard parts (where the real work is)

1. **Networking substrate** — every managed service silently needs a VPC, subnets,
   firewall/SG, and *private connectivity*. Classic trap: GCP Cloud Run → Memorystore /
   Cloud SQL private IP requires a **Serverless VPC Access connector**; Fargate tasks must
   sit in the right subnets with SG rules to RDS. ~50% of the engineering.
2. **Image build** — Cloud Run / Fargate run containers. "Build from the customer's repo →
   push to ECR / Artifact Registry → deploy" is a distinct subsystem (reuse `git` +
   `docker` + `launch-deploy` modules), or v1 accepts a pre-built image.
3. **Credentials & blast radius** — prefer **AWS cross-account IAM role (external ID)** and
   **GCP Workload Identity Federation / short-lived SA** over stored static keys. Pulumi
   state holds cloud access — KMS-encrypt, isolate per customer.
4. **GCP project model** — AWS is one account; GCP wants a **project per customer** (create
   project → enable APIs → link billing). An onboarding primitive AWS doesn't need.
5. **Partial-apply recovery** — RDS / VPC provisioning fails halfway constantly. Pulumi
   state + `refresh`/`up` retry handles most of it, but the worker needs idempotent retries
   and a clear "degraded" state in the UI.

## 8. Cost engine

- **Pre-deploy estimate** — from the `ResourcePlan` (chosen SKUs + sizes) via Infracost
  and/or the cloud Pricing APIs (AWS Price List API, GCP Cloud Billing Catalog API).
- **Live spend** — AWS Cost Explorer / CUR; GCP Billing BigQuery export. Tag/label every
  resource with Launch metadata for per-environment attribution.
- **Optimization** — AWS Compute Optimizer / Trusted Advisor; GCP Recommender API
  (rightsizing, idle resources). Both clouds expose ready-made recommendation APIs to
  surface.

## 9. Delivery plan (phases → epics)

Phases are milestones; each epic is a tracking issue with story sub-issues.

- **P0 — Foundations & Abstraction:** account mode + onboarding, provisioning engine,
  resource abstraction & planner, credentials & security (core).
- **P1 — First Vertical Slice:** App + Postgres + Domain end-to-end on AWS **and** GCP —
  networking substrate, compute & image build, data services, DNS/TLS, GCP project
  lifecycle, dashboard MVP.
- **P2 — Full Resource Primitives:** cache, object storage, static+CDN, secrets, queues,
  remaining service/worker shapes.
- **P3 — Cost Engine:** pre-deploy estimate, live spend, optimization, budgets/alerts, cost
  dashboard.
- **P4 — Hardening, GA & Azure-ready:** drift/reconcile, teardown, quotas, observability,
  Azure provider stub, load tests, runbooks.

The full epic and story breakdown is tracked in GitHub (see the project board). This
document is the source of truth the epics link back to.

## 10. Open questions

- Image build: build-from-repo in v1, or pre-built image only?
- Credential model: enforce role assumption / WIF from day one, or allow static keys for
  the earliest testing?
- State backend: self-managed S3/GCS vs Pulumi Cloud for v1.
- Does the existing `billing` module (subscriptions) stay separate from cloud-cost
  analytics, or share models?
