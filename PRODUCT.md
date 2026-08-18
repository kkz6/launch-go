# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Developers, engineering leads, and infrastructure operators who use launchctl to provision servers, deploy applications, monitor operational health, and respond to failures.

## Product Purpose

launchctl is a zero-downtime deployment and server-management platform. It centralizes deployment, infrastructure, databases, backups, certificates, monitoring, and operational automation so teams can ship and operate applications without assembling those workflows themselves.

## Positioning

The product combines a focused web control plane with an `lctl` command-line workflow. Its notifications are operational handoffs: they should make the event, affected resource, next action, and supporting evidence immediately understandable.

## Operating Context

Users move between the launchctl application, deployment output, source-control metadata, servers, sites, databases, backups, and notification channels. Email is the asynchronous entry point for account actions, invitations, lifecycle receipts, threshold alerts, security results, and operational failures.

## Capabilities and Constraints

- The email system covers all transactional emails, including authentication, invitations, team lifecycle, provisioning, deployment, monitoring, security, backup, and failure notifications.
- Email markup must remain reliable in major webmail, mobile, and Outlook clients, including clients with limited CSS support.
- Dynamic values and logs must remain escaped and safe.
- Plain-text alternatives remain required.
- Existing business facts, destination URLs, and user actions must not change as a side effect of the redesign.
- Deployment failure is the first high-information reference template and stress test for the system.

## Brand Commitments

- Product name: `launchctl`.
- The application and `lctl` CLI are the visual and interaction authority for email—not generic SaaS notification conventions.
- The application uses a precise monochrome system, Plus Jakarta Sans for interface text, JetBrains Mono for operational data, and status color only where it conveys meaning.

## Evidence on Hand

- Application tokens and typography: `../launch-nuxt/assets/css/main.css` and `../launch-nuxt/tailwind.config.ts`.
- Product navigation and wordmark: `../launch-nuxt/components/layout/Navbar.vue`.
- Current transactional templates: `internal/pkg/mail/templates/`.
- Current notification catalog: `internal/modules/notification/` and `internal/modules/docker/notifications/`.
- Existing deployment-failure redesign and tests provide compatibility and content requirements, but not final visual authority.

## Product Principles

- Lead with the operational truth, not decorative framing.
- Make the next useful action obvious without hiding the evidence.
- Preserve density where experts need it and remove repetition everywhere else.
- Treat every email as part of the same product, while allowing event severity and information shape to change the composition.
- Prefer durable email-client behavior over browser-only effects.

## Accessibility & Inclusion

Maintain semantic reading order, sufficient contrast, meaningful link text, keyboard-accessible destinations, non-color status cues, readable mobile layouts, and complete plain-text alternatives.
