# Kubernetes Management (Launch v3) — Design & Delivery Plan

- **Status:** Proposed
- **Date:** 2026-06-16
- **Owner:** kkz6
- **Tracking:** GitHub epics labelled `epic` + `kubernetes`, grouped by phase milestones
  (K0–K3), surfaced on the account-level project **"Launch · Kubernetes Management"**.

---

## 1. Summary

The third Launch mode: **manage Kubernetes clusters and their workloads from the Launch
web dashboard** — browse any resource (incl. CRDs), read/edit YAML, follow pod logs, open
a shell into a pod or node, and see a cluster-health cockpit. This is the capability set
of tools like **k9s / Lens**, delivered in Launch's web UI instead of a terminal.

The idea was seeded by **[bjarneo/kli](https://github.com/bjarneo/kli)**, an excellent Go
TUI whose `internal/k8s` package is a clean blueprint for the client-go interactions we
need. See the licensing note below — we build this **clean-room**.

## 2. Relationship to v1 / v2

- **v1 — Custom:** PaaS on VMs you control over SSH.
- **v2 — Managed Cloud Orchestrator:** provision *managed* cloud services (Pulumi).
- **v3 — Kubernetes Management:** operate clusters and the workloads inside them.

**Synergy:** v2 can provision the cluster (EKS/GKE); v3 manages what runs in it. They share
the **per-environment credential vault** (KMS-encrypted), the **WebSocket hub**, and the
**dashboard shell**. v3 also stands alone for customers who bring an existing cluster
(kubeconfig).

## 3. Licensing note (important)

`kli` currently ships **no LICENSE file** — by default that is *all rights reserved*, and
`go install` does not grant a reuse license. Therefore:

- **We do not copy kli source into Launch.** `internal/k8s` is studied as a reference only.
- We implement our own `kubernetes` module against **client-go**, whose API usage patterns
  are standard and not themselves copyrightable.
- Optionally, contact the author ([x.com/iamdothash](https://x.com/iamdothash)) to request a
  permissive license; only then may the package be vendored directly.

This policy is a hard constraint on every story in this plan.

## 4. What kli shows is reusable (as a reference)

kli is a Bubble Tea **terminal TUI**; its `internal/ui` (terminal rendering) does **not**
port to a web dashboard. Its `internal/k8s` layer, built on
`client-go` (`clientset` + `dynamic` + `discovery`), demonstrates exactly the backend
surface we need:

| Capability | Web-port path in Launch |
|---|---|
| Resource discovery incl. CRDs; resolve by plural/kind/short | REST list |
| Server-side tables (same columns as `kubectl get`) | REST → data grid |
| Get YAML / object (secret-decode gated) | REST detail |
| Edit (apply), delete, scale, rollout restart, CronJob trigger | REST actions + RBAC |
| Pod logs (follow) | WebSocket (existing hub) |
| Exec shell into pod/node (SPDY/pty) | WebSocket + xterm.js |
| Cluster cockpit, node CPU/mem metrics | REST → dashboard |
| Per-cluster connection from kubeconfig (context + path) | per-environment, KMS-encrypted |

## 5. Architecture

```
launch-nuxt (web)
  resource browser · YAML editor · log viewer · web terminal (xterm.js) · cockpit
        │  REST (CRUD)            │  WebSocket (logs, exec)
        ▼                         ▼
launch-go · kubernetes module (clean-room, client-go)
  connection registry  ·  resource registry (+CRDs)  ·  server-side tables
  ops (apply/scale/restart/delete/cron)  ·  logs  ·  exec(SPDY)  ·  cockpit/metrics
        │  per-cluster rest.Config from the KMS-encrypted credential vault (shared with v2)
        ▼
Customer / v2-provisioned clusters (kubeconfig or EKS/GKE)
```

- **Connection registry** — one client-go `rest.Config`/clientset per cluster, rebuilt on
  switch; credentials pulled from the shared vault (kubeconfig for BYO clusters, or fetched
  from the v2 environment for EKS/GKE).
- **AuthZ** — Launch team roles map to allowed actions; per-cluster scoped service-account
  tokens enforce least privilege; destructive ops gated + audited.
- **Streaming** — logs and exec ride the existing WebSocket hub; exec bridges client-go's
  SPDY stream to a browser `xterm.js` terminal.

## 6. The hard parts

1. **Web terminal (exec)** — bridging client-go's SPDY/WebSocket exec stream to a browser
   terminal with correct TTY/resize handling and session lifecycle.
2. **Multi-cluster + RBAC** — many clusters per team, least-privilege per cluster, mapping
   Launch roles to Kubernetes permissions safely.
3. **Performance on large clusters** — use informers/watch caching rather than naive list
   spam (kli tunes QPS/Burst for a single user; a multi-tenant server needs caching).
4. **Secret exposure** — gating secret decode behind permissions and audit.

## 7. Delivery plan (phases → epics)

- **K0 — Foundations:** cluster connection & credentials, clean-room client-go module,
  AuthZ/RBAC & multi-tenant safety.
- **K1 — Read & observe:** resource browser & detail, cluster cockpit & metrics, log
  streaming.
- **K2 — Operate:** resource operations (edit/scale/restart/delete/cron), web terminal
  (exec into pods/nodes).
- **K3 — Multi-cluster, integration & GA:** multi-cluster + namespace management, v2↔v3
  integration (auto-register provisioned clusters), GA hardening.

The full epic + story breakdown is tracked in GitHub. This document is the source of truth
the epics link back to.

## 8. Open questions

- BYO-cluster kubeconfig upload vs only v2-provisioned clusters in v1 of this mode?
- Watch/informer caching from day one, or start with on-demand lists?
- Web terminal: ship pod exec first and defer node-debug (privileged) to later?
