# GitHub account connections and repository isolation

Status: Implemented and automated verification passed. Date: 17 November 2026. Production rollout and live private GitHub/GHCR verification remain pending.

Keep the GitHub App and add explicit repository permissions for each launchctl connection. Two separate launchctl logins must be able to authorize the same GitHub identity and reuse an existing installation. Each account receives only the repositories selected for its own connection. Connecting the second account must not require revoking the first account's GitHub authorization or reinstalling the app.

GitHub installation access is the outer limit of what launchctl can access. It does not express which launchctl account may use each repository. That second permission boundary belongs in launchctl and must apply to every API, deployment, background job, and credential.

## Required account behavior

| GitHub app can access | Launchctl account A selects | Launchctl account B selects | Result |
| --- | --- | --- | --- |
| frontend, backend, internal-tools | frontend | backend | A can use frontend. B can use backend. Neither receives internal-tools. |
| The same repositories plus a newly approved reports repository | frontend | backend | Existing selections remain unchanged. An authorized account must explicitly select reports before using it. |
| frontend, backend | frontend | No repositories | B has no repository access even though it connected the same GitHub installation. |

Repository permissions belong to the destination launchctl workspace/team. For the requested account isolation, account A and account B connect in their own separate workspaces and save independent repository selections. Two logins intentionally using the same team share that team's selected repositories according to their roles. Separate login IDs alone do not establish isolation inside a shared team; this feature does not add per-member repository restrictions there.

From the account connection flow, default to the signed-in user's owned private workspace only when it is unambiguous. Check actual membership and pending invitations: `personal_team=true` alone does not establish privacy. If there is no suitable workspace, use the existing workspace selection/creation flow; do not silently choose a shared team or create a billable workspace. Shared destinations must be labeled “Repositories for [team]” and explain before Save that authorized team members can use the selection. Bind this destination and its membership version to the connection attempt; a membership change during a private setup requires a new review. Later invitations intentionally share the team's selected repositories, which the invitation flow must make clear.

The requirement is explicit selection per account, without automatic inheritance. It does not require a global rule that a repository can belong to only one launchctl account. If the same repository is explicitly selected twice, each connection still requires independent authorization. GitHub Actions use of a repository shared across accounts needs the additional safeguards described below.

### Mandatory isolation rules

- **I1 — Explicit destination:** connecting the same GitHub identity never merges launchctl accounts, transfers permissions, or changes the destination workspace.
- **I2 — Explicit selection:** a new connection has zero grants. GitHub installation access, a candidate in the setup screen, an existing workload, a sync, or a webhook cannot create a grant. Only a verified selection commit can add repository access.
- **I3 — Complete authorization:** each repository-use operation must match the current team, connection, immutable repository ID, active grant, approved capability, and current authorization versions. A selected repository on another account is insufficient. The only post-revocation exception is the separate cleanup authorization defined below, which can remove recorded owned resources but cannot authorize repository use.
- **I4 — Restricted credentials:** credentials supplied to account workloads include only the operation's required repositories already selected for that account. Dependency discovery, a failed clone, or a failed image pull never broadens access. Internal catalog credentials and narrowly bounded cleanup credentials never reach account workloads.
- **I5 — Durable revocation:** removing a selection or disconnecting blocks new work under the old authorization version. Cached tokens, retries, reconnects, or delayed cleanup cannot revive it.
- **I6 — No implicit transfer:** changing workspace, transferring resources, migration, or rollback cannot union selections or move authorization proof to another account.

These rules govern access supplied through the launchctl GitHub integration. They cannot prevent access to public repositories or access using credentials independently supplied outside the integration. Minimal candidate metadata is visible only to the verified connecting person for making a selection; it does not authorize workloads or ordinary repository APIs.

## Findings in the current implementation

Paths in this table are relative to launch-go unless prefixed with launch-nuxt. Line numbers refer to the reviewed source and may move during implementation.

| Finding | Code evidence | Consequence |
| --- | --- | --- |
| Another team is prohibited from reusing an installation. | `internal/modules/git/services/source_control_service.go:90` | The explicit `ensureInstallationNotClaimed` check explains the connection conflict. The database already permits `(provider, provider_id, team_id)` through migration `0015_02_09_000000_add_source_control_unique_constraints.go:19`. |
| All installation repositories become connection repositories. | `internal/modules/git/services/source_control_service.go:236`, `:268`, `:309` | GitHub's `repository_selection` value describes the installation, not a local account selection. There is no separate grant. |
| Installation IDs are accepted without proof of the connecting GitHub identity's access. | `internal/modules/git/handlers/source_control_handler.go:153`, `internal/modules/git/services/source_control_service.go:106`, `internal/modules/git/providers/github.go:47` | App credentials can look up an installation but do not prove the launchctl user may connect it. Neither connection entry point performs GitHub user authorization or binds a one-time state. |
| Callbacks use the currently selected login/team and conceal failures. | `launch-nuxt/server/routes/settings/git-providers/[provider]/callback.get.ts:30` | Switching accounts or teams during setup can change the destination; every backend error is swallowed before redirecting to Connections. |
| Listing and importing can bypass a future picker filter. | `internal/modules/git/handlers/source_control_handler.go:87`, `internal/modules/git/services/source_control_service.go:566` | Raw listing returns the whole installation catalog, and `SaveRepository` imports a repository based on installation access alone. |
| Installation credentials are unrestricted. | `internal/modules/git/providers/github.go:79`, `internal/modules/site/jobs/deploy.go:263`, `internal/modules/docker/jobs/git_auth.go:74` | A deployment receives credentials usable for every repository available to that installation. This would defeat account separation even with filtered lists. |
| Docker and GitHub Actions paths resolve connection IDs without the workload's team. | `internal/modules/docker/services/application_service.go:1207`, `internal/modules/docker/services/compose_service.go:824`, `internal/modules/docker/jobs/git_auth.go:59`, `internal/modules/docker/services/gha_deploy_dispatch.go:195` | Caller-supplied connection references need ownership and repository authorization before storage and again during use. |
| Site validation checks team ownership separately for the connection and repository. | `internal/modules/site/services/site_service.go:641` | It must also prove the repository belongs to that exact connection. |
| Docker credential injection accepts a user-supplied HTTPS host. | `internal/modules/docker/jobs/git_auth.go:162` | An authenticated clone URL must be derived from verified GitHub repository metadata; it cannot be an arbitrary submitted URL. |
| Webhook dispatch loses event type and delivery identity. | `internal/modules/git/handlers/webhook_handler.go:71`, `internal/modules/git/jobs/process_webhook.go:19`, `:78` | Routing only on `action` can treat an unrelated delivered `deleted` event as installation deletion. Repository changes expect `repositories_added`/`repositories_removed` instead of actions `added`/`removed`. |
| Push deployment matching uses repository names rather than installation and repository identity. | `internal/modules/git/jobs/process_webhook.go:286` | Shared-installation fanout must also require the correct provider, installation, stable repository ID, active connection, and active grant. |
| Sync deletes missing repositories, skips metadata updates for known IDs, and hides partial write failures. | `internal/modules/git/services/source_control_service.go:284`, `:313`, `:320` | Repository references can be lost; renames/default branches can stay stale; counts can imply a successful sync despite failed writes. |
| Disconnect and revocation coverage is incomplete. | `internal/modules/git/services/source_control_service.go:184`, `internal/modules/git/repositories/source_control_repository.go:105`, `internal/modules/docker/handlers/gha_webhook_handler.go:148` | Disconnect checks sites but not Docker/Compose. GHA callbacks do not require an active repository grant. Uninstall deletes connection records and history rather than preserving recovery information. |
| The UI exposes installation-wide configuration only. | `launch-nuxt/components/settings/ConnectionsTab.vue:246`, `:294`, `:315`, `:741` | There is no account repository checklist. Docker and Compose submit URL plus connection ID instead of the selected repository identity. Settings and picker requests also need account/team change invalidation. |
| Concurrent deployments share credential configuration. | `internal/modules/site/tasks/templates/deployment/update_repository.sh:32`, `:41`, `internal/modules/docker/tasks/deploy_application.go:419`, `internal/modules/docker/tasks/deploy_compose.go:328` | Global Git helper changes and shared Docker configuration can overwrite or remove another deployment's credentials. |
| Private GHCR pulls use a different credential path. | `internal/modules/docker/handlers/gha_webhook_handler.go:185`, `internal/modules/docker/templates/gha_application.yml.tmpl:111` | Restricting repository tokens does not establish private package access. The package bearer path needs its own scope, expiry, and workload binding. |
| Generic team transfer rewrites resource ownership. | `internal/modules/auth/services/team_service.go:190` | Dynamically changing `team_id` can move connections or future authorization rows without new destination consent and can collide with an existing connection. |
| Remote task scripts can contain credential bytes. | `internal/pkg/taskrunner/dispatcher.go:249`, `internal/modules/site/tasks/templates/deployment/update_repository.sh:42` | Scripts are uploaded with mode `0755`; protecting only queue payloads does not protect secrets embedded in generated scripts or persisted task content. |

These findings are source-level defects and design gaps, not evidence that a particular production account has been accessed improperly.

## GitHub behavior the design relies on

An installation belongs to a GitHub personal or organization account. A GitHub user token can enumerate installations and repositories the user can access; its effective permissions are limited by both the user and the app. Use that intersection to establish eligible selections. [GitHub user authorization](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-user-access-token-for-a-github-app), [installation APIs](https://docs.github.com/en/rest/apps/installations).

The setup URL's `installation_id` is untrusted input. GitHub explicitly requires checking its association with an authorized user. [GitHub setup URL guidance](https://docs.github.com/en/apps/creating-github-apps/registering-a-github-app/about-the-setup-url).

Installation tokens can be restricted using `repository_ids` and `permissions`. Omitting those restrictions grants the installation's full corresponding access. Tokens normally expire after one hour. [GitHub installation token documentation](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app).

Consequently, changing GitHub's selected-repository list changes installation-wide availability. Account A and B cannot use that one list as two separate account selections. The launchctl selection must remain independent.

## Permission model

Use four distinct concepts:

1. **GitHub authorization:** proof that the connecting person controls a particular GitHub identity. The same identity may be authorized independently by separate launchctl logins. This is an integration credential, not a launchctl login identity or an account-merging mechanism.
2. **GitHub installation:** the shared external installation, its lifecycle state, and the repositories GitHub makes available to it.
3. **Launchctl connection:** a team-owned link to that installation, created from an authenticated launchctl account. Multiple teams can link the same installation without sharing their local connections.
4. **Repository grant:** an explicit permission for one repository on one connection, including allowed operations and authorization provenance.

An authenticated repository operation is allowed only when all of the following hold:

- The acting user or workload belongs to the requested team and may perform the launchctl operation.
- The connection belongs to that team and is active.
- The repository is explicitly selected on that connection and has a valid authorization basis.
- GitHub still makes that repository available through the installation.
- The intended operation fits both the local grant and the installation permissions.

No selection means no authenticated repository use by workloads or ordinary repository APIs. The verified connection manager may browse minimal eligible candidate metadata in the setup flow; candidate visibility is not a grant. GitHub's all-repositories setting never means all repositories are selected for a launchctl account. A future “select all currently available” button must store an explicit snapshot of IDs, without subscribing to future repositories. Reconnecting or editing an existing connection preserves its saved selection until a reviewed change; the empty default applies only to new connections.

### Who may grant access

Recommended initial policy: a launchctl team admin/owner with freshly verified GitHub repository administration rights can create or expand a grant. Existing editors may continue deploying resources their team already has permission to use. Reading a repository or being listed on an installation alone must not permit delegating the app's broader write permissions to another account.

Record the authorizing GitHub identity, launchctl actor, time, repository, and approved capabilities. Initially, grants use a verified sponsor who is still a manager in the destination team and retains the required GitHub repository authority. Another verified manager can assume that sponsorship before the original manager leaves. This is a product policy recommendation; it should be explicit in the connection flow.

Revalidate sponsorship at privileged changes and on every resolver use, including operations that reuse a cached installation token. A valid token is not proof of current authority. Check current local connection/grant/authorization versions against authoritative state for each new operation; local selection revocation must not wait for a cache TTL. GitHub-side sponsor and availability checks may use a cache with a maximum age of five minutes, invalidated when relevant events are processed. A privilege-expanding selection commit requires fresh GitHub proof. After the freshness limit, failure to revalidate blocks new authenticated operations with a recoverable status. User authorization revocation, lost repository authority, or sponsor removal suspends affected grants until another eligible manager reauthorizes them. Logging out of launchctl does not revoke a stored authorization.

Refresh expiring user tokens before use under a lock for that local authorization, and atomically replace both access and refresh credentials. A concurrent refresh loser rereads the new version rather than revoking the connection. Refresh must also compare the authorization generation so it cannot overwrite a concurrent revocation. Invalid/expired refresh credentials or changed organization SSO requirements produce a reconnect state while retaining selected repository IDs. Temporary provider failures remain retryable and cannot extend authority past its freshness limit. A reconnect may restore only the previously reviewed selection after verification; repository additions require a separate selection commit. Regular token rotation must work without user action; expired refresh authorization or revoked consent can still require reauthorization. [GitHub refresh behavior](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/refreshing-user-access-tokens).

An installation-owner-managed delegation policy could later remove dependence on a particular employee's authorization, but requires a separate explicit consent and lifecycle design. Do not silently continue using app-only authority after a sponsor loses access in the initial model.

## Proposed storage changes

Preserve existing local connection and repository IDs so sites retain their references. Implement GitHub-specific authorization and installation support within the existing git module; avoid a general rewrite of GitLab and Bitbucket.

| Record | Purpose and constraints |
| --- | --- |
| GitHub authorizations | Encrypted access/refresh credentials with expiry, remote GitHub user ID, launchctl user ID, revocation status, and refresh version/lock. Uniqueness applies to `(GitHub host, app ID, launchctl user ID, remote GitHub user ID)`, not the GitHub identity globally. Scope refresh coordination and revocation lookup by host/app as well. Rotate refresh credentials atomically and never expose them to the browser. |
| Connection attempts | Server-side random state hash, originating launchctl user/session/team, intended operation, PKCE verifier, expiry, and one-time consumption. Proposed expiry: ten minutes. Use the existing durable expiring store or a small table; never trust a browser-submitted team at completion. |
| GitHub installations | One record per GitHub host, app ID, and remote installation ID; owner ID/type, permissions, repository selection mode, suspension/deletion status, successful sync time, and reconciliation version. |
| Installation repository catalog | Installation reference and immutable remote repository ID, current name/owner/default branch/clone URL, availability status, and observed version. This catalog is internal provider inventory, not a tenant authorization list. |
| Existing `source_controls` | Retain the local ID and team ownership; add the installation reference, connection lifecycle, and monotonically increasing generation. Disconnect/reconnect advances the generation and rotates callback authorization. Enforce one connection per team and installation. Audit `user_id` deletion cascades: team connections must not disappear merely because a former creator is deleted; preserve audit attribution and enforce sponsor removal explicitly. |
| Existing `source_control_repositories` | Become explicit grants for GitHub connections. Retain local IDs; add an indexed immutable GitHub repository ID, selection/revocation state, capability set, sponsor reference, and grant version. Enforce uniqueness by connection and remote repository ID instead of mutable name. Never create these rows merely because catalog sync discovers a repository. |
| Webhook receipts | Provider/app and delivery ID uniqueness, event/action, processing status, attempts, and error. Record successful completion separately from receipt so retries can recover failed work. |
| Deployment dispatch intents and outbox | Durable unique `(host, app, delivery, workload, action)` intent linked to one local deployment ID, pinned repository and authorization generations, and a retryable queue publication record. Reuse existing deployment/queue facilities where they provide these guarantees. |
| Credential issuance and leases | Register pending issuance before provider calls. Store released credentials encrypted with actual expiry and bindings to team, connection generation, grant/version, authorization generation, workload/deployment, resource set, and purpose. Queue messages contain opaque references, never credentials. Use existing encrypted storage with expiry and durable revocation work. |
| Revocation cleanup authorization | Created atomically with an explicit revoke/disconnect operation; binds exact retired-generation resource identities, permitted disable/delete actions, destination team, current connection generation, current GitHub authorization, and a maximum fifteen-minute expiry. It cannot grant repository use and is cancelled by reconnect or loss of authority. |
| Workload repository dependencies | Explicit references to already selected grants for private Git dependencies/submodules, including the primary repository. A dependency cannot create a grant or refer to a different team. |

Keep GitHub repository IDs lossless through Go JSON decoding, storage, and frontend DTOs. Use strings at the frontend boundary if necessary. Do not derive identity from `owner/name` or floating-point conversion of arbitrary JSON numbers.

Enforce relationship consistency with database constraints where representable and transactional validation everywhere: a grant belongs to the exact team connection, its repository belongs to the intended installation, and workload grant references belong to the workload's team. Selection changes use a version guard. Background inventory writes must be unable to set grant selection or connection authorization fields. Grant versions and connection generations never reset when a previously selected repository is re-enabled.

## Connection and selection flow

1. In the destination launchctl account, open Connections. Resolve a private or explicitly acknowledged shared workspace as specified above, display “GitHub repositories for [team name]”, and create a connection attempt tied to that login/session/team and membership version.
2. Authorize the GitHub identity using the GitHub App web authorization flow. Validate state, use PKCE, exchange the code on the backend, and obtain the GitHub user identity. Add GitHub client ID, client secret, and registered callback URL configuration; current configuration contains only app ID, private key, webhook secret, and slug.
3. Enumerate the installations accessible to that authorized identity. Offer **Use existing installation** and **Install on another GitHub account**. Never enumerate all app installations to the browser using the app's credentials.
4. If GitHub access must be expanded, open installation settings, preserving existing selections. Explain that those settings determine app availability for every account using that installation. On return, verify the installation again; install/update/request callbacks never directly create grants.
5. Show minimal eligible repository metadata from the authorized user's intersection with the installation. A new connection starts with nothing selected; editing or reconnecting an existing connection retains its reviewed selection. Cache candidates by authenticated launchctl actor, GitHub authorization/version, installation, and destination/attempt, never by installation alone. Do not display another launchctl account's selections, name, or connection metadata.
6. Commit selected repository IDs and capabilities in one transaction after fresh server-side verification. Recheck the actor's current launchctl role, destination team, sponsor authority, installation state, and every submitted repository. An invalid item rejects the change; it must not partially broaden access.
7. Return a structured connection result. The frontend displays success, cancellation, pending organization approval, expired attempt, missing access, or retryable failure. Switching launchctl logins during the attempt requires restarting or resuming as the original login; it must never bind to the new login. A team switch must never silently change the stored destination.

Prefer backend-owned authorization and callback endpoints with the Nuxt route acting only as a controlled forwarding layer. Persist a safe result code for the frontend; do not put tokens or sensitive provider errors in redirect URLs.

Separate the APIs by intent:

- Connection attempts and authorized installation discovery.
- Candidate repositories, available only to connection managers through their own verified GitHub authorization.
- Account repository grants, exposed through the existing team-scoped connection listing and repository picker APIs.
- Explicit grant replacement/removal with a version guard, affected-workload preview, and audit event.
- Reconciliation/status and local disconnect.

Close or replace the old POST that accepts only `installation_id`. Apply the same restrictions to settings, integration, API, and CLI routes; an older route cannot remain an alternate authorization path. Browser redirects may collect proof, but only the verified completion flow commits grants.

## Enforce access at every consumer

Introduce one git-module resolver that accepts trusted team context, local connection ID, local repository/grant ID, and intended operation. It returns an authorized repository identity and a credential capability, not an unrestricted installation ID to arbitrary callers.

For background jobs, derive the team from the persisted site/application/Compose record and its parent server/project, not from an unverified queue field. At scheduling, pin the workload/repository identity, connection generation, grant versions, and authorization generation; at execution, compare them with authoritative state and recheck access before credentials and external side effects. Never substitute the latest active grant for an old job's revoked version. The same rule applies to callback authorization, retries, and workflow bootstrap. Post-revocation cleanup uses its separate authorization and current expected generation, never the revoked repository-use grant. Legacy jobs without the required context must be rejected or deliberately recreated after current authorization, not silently assigned the newest generation.

| Consumer | Required change |
| --- | --- |
| Site create/update/deploy, deployment status writes | Verify the team, connection, repository relationship, grant, and operation together. Reject invalid field combinations and recheck when jobs execute. |
| Docker application and Compose create/update/build | Submit and store the selected grant/repository identity. Validate it against the workload team. Remove credentialed URL fallback that can bypass selection. |
| GitHub Actions bootstrap, dispatch, logs/polling, artifacts, run deletion, secret/variable writes | Use the same resolver and operation-specific permissions in service and job paths. Cover `gha_bootstrap_workflow.go`, `gha_deploy_dispatch.go`, `poll_gha_steps.go`, and provider reads/writes. |
| GitHub Actions deployment callbacks | Require an active workload connection/grant in addition to the existing workload bearer. Disconnect invalidates callback authorization; a supplied registry token must not bypass the grant check. |
| Push webhooks | Match app/provider/installation and immutable repository ID, then find only active grants and enabled workloads for the matching branch. Recheck at execution. |
| Direct repository listing/import APIs | Ordinary APIs return selected active grants only. `SaveRepository` cannot create authorization from arbitrary full names. |
| Team deletion/transfer, account deletion, workload moves | Never bulk-rewrite connection/attempt/grant ownership. Block GitHub-bound team transfers in the initial release until an explicit destination reauthorization flow exists. Cancel source-context work and preflight connection collisions. |

Mint credentials with exactly the allowed `repository_ids` and minimum permissions for the requested operation. Cloning needs content read access; deployment status, Actions, workflow, secret, and variable operations each need an explicitly tested capability mapping. Keep credential minting inside the authorization service. An unscoped token is permitted only inside the trusted installation-inventory worker and must never reach a tenant server, browser, or ordinary consumer.

Derive authenticated clone URLs from verified provider metadata and enforce the configured GitHub host. Reject mismatched repositories, userinfo, arbitrary hosts, and unexpected redirects. Public URL deployments can remain available as an unauthenticated path; never attach GitHub credentials to that path. Avoid storing credentials in Git remotes, generated scripts, queue payloads, or logs; audit cleanup of existing deployment scripts as part of this change.

If credentials are cached, key them by host/app/installation, team, connection/generation, grant/version, authorization/generation, repository set, and operation permissions. Recheck authorization before each reuse. Do not share cached tokens across account connections. On revocation, invalidate the cache and durably schedule revocation of released installation tokens using GitHub's [installation-token revocation endpoint](https://docs.github.com/en/rest/apps/installations#revoke-an-installation-access-token).

### Credential issuance and revocation races

Register an issuance intent with the current authorization generations before calling GitHub. When GitHub returns a token, compare those generations and current state under the same serialization mechanism used by local revocation. Only an unchanged, authorized request may record an encrypted expiring lease and mark it released for use. A changed generation requires discarding the token and attempting provider revocation; never cache or publish it. Track pending issuance so a disconnect cannot miss a request still waiting on GitHub. Queue publication, credential-reference redemption, and new operation admission perform the same version checks.

The committed local revocation is the boundary: no new repository-use operation or credential release under that old generation is admitted afterward. Cleanup can proceed only under the separate revocation-created capability below, checked against the current generation. An operation admitted before revocation may already be in flight; it cannot be undone by a database change. Revoke released installation tokens where supported and retry failures durably. An already delivered credential may remain usable until its actual expiry if provider revocation fails, and a registry bearer may have no remote revocation mechanism. Copied source and credentials cannot be erased from an account's server by this feature. Keep their repository/package scope narrow throughout that window; do not promise instantaneous erasure or exactly-once external execution.

### Dependencies and deployment credential isolation

Private submodules and Git dependencies require explicit grants selected for the same account and explicitly bound to the workload. Validate each dependency independently, including nested dependencies and changed `.gitmodules` URLs. Use separate credentials per repository/installation where possible, or the smallest explicitly approved set for that operation. An undeclared or unselected dependency fails with an actionable selection message; discovering it never creates access. Do not fall back to unrestricted installation tokens, ambient global credentials, or an old SSH key when a GitHub connection/grant is denied. Independently configured SSH/public deployment modes must be explicit and must not be selected automatically after an authorization failure.

Use process-scoped Git configuration and a private temporary `DOCKER_CONFIG` per deployment. Do not change global `credential.helper` or shared Docker login state. Directories use `0700`; credential files use `0600`. Ensure cleanup on success, failure, timeout, and cancellation, with expiry cleanup for interrupted tasks. Deliver secrets separately from generated scripts; scripts reference protected files rather than embedding token bytes. Audit persisted task data and logs as well as queues. Separate directories prevent accidental credential collisions between concurrent deployments; they are not an isolation boundary against processes sharing the same operating-system user or root access.

### Private container images and protected credential references

Keep registry authorization distinct from repository authorization. GitHub package permissions may differ from repository permissions; repository-scoped installation tokens are not a replacement for the existing private GHCR package-bearer path. Bind the expected registry, package, image/digest, workload, deployment, and connection generation to trusted persisted configuration and the authorized build. Validate the active repository grant when accepting its callback and again before pull. Accept only credentials limited to the expected package and pull operation; never accept an arbitrary package merely because its name shares a prefix. [GitHub Container registry documentation](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry).

Store relayed registry credentials encrypted in expiring records, and queue only opaque references bound to that exact deployment, account/team, authorization generations, package, and purpose. A reference is not authority by itself: workers validate every binding and current permission before redemption. Keep references only for valid retries and remove them at completion, expiry, or revocation; never extend their life beyond the actual provider credential expiry. Establish package scope and expiry from a supported, verified registry exchange/credential mechanism, not merely a callback's claimed `minted_at` or an unsigned token decode. Preserve a recoverable expiry state for delayed pulls and rollbacks. Acquire fresh package credentials through a supported, verified path or require the user to retry/rebuild; never substitute an unrestricted installation token for a missing or expired private-package bearer. Prove the existing bearer relay and expiry behavior against a real private GHCR package before release; mock repository-token tests cannot validate it.

## Synchronization and lifecycle

Catalog synchronization updates GitHub availability and metadata. Grant management updates account permissions. These are separate write paths: sync must never expand selections.

| Event | Required result |
| --- | --- |
| Repository added to GitHub installation | Update internal availability. Existing account selections remain unchanged. |
| Repository removed from installation | When a verified event or revalidation detects removal, commit a revocation and block affected grants/new work. Preserve grants and workload references for diagnosis/recovery. |
| Repository re-added | Keep the historical selection, but require manager confirmation before reactivating a grant previously blocked by explicit GitHub removal. |
| Repository renamed | Update names/URLs by stable repository ID without changing account selection. |
| Repository transferred | Block access pending verification of destination owner/installation and explicit grant reconfirmation. Never redirect old authority to a new owner by name. |
| Installation suspended | Pause new authenticated work for all affected connections; retain their selections and running applications. |
| Installation unsuspended | Reconcile and revalidate grants before resuming. It cannot restore a locally revoked grant. |
| Installation uninstalled/reinstalled | Mark old installation and grants unavailable. A new installation ID requires a verified reconnect and selection confirmation; never silently rebind by owner login. |
| GitHub user authorization revoked | Invalidate every local authorization for the same GitHub host/app/user identity, including authorizations from both launchctl logins. Suspend grants relying on that identity; another eligible manager can reauthorize them. Do not delete unrelated connections. |
| Sponsor loses repository authority or destination-team membership | Suspend the grants relying on that lost authority or membership; another eligible manager can reauthorize them. |
| Repository deselected locally | Revoke only that connection's grant, block future credential issuance/callbacks/jobs, and show affected workloads. Do not remove GitHub installation access. |
| Disconnect from one launchctl account | Disable that connection and its grants only. Preserve other accounts' links and the shared GitHub installation. Do not remotely revoke the GitHub user's entire app authorization. |
| Reconnect an existing connection | Advance its generation, rotate callbacks, reverify the retained selection, and never replay old deployment/bootstrap/cleanup jobs. Adding repositories remains a separate selection commit. |
| Team transfer or ownership move | Never union source and destination selections or transfer proof through a generic `team_id` update. Initially reject transfers containing GitHub connections/grants/workloads; a later supported move requires explicit destination selections, fresh proof, collision handling, and new operation generations. |

Keep running workloads available when access is removed; stop future GitHub-dependent actions and deployments with a clear recoverable reason. Removal previews include sites, Docker applications, Compose, GitHub Actions, scheduled work, and queued work. Allow deliberate revocation even when workloads depend on the grant, after showing that impact; dependent records must remain recoverable.

Local disconnect is isolated to the selected launchctl connection. Revoking the shared GitHub identity's app authorization on GitHub affects that identity's tokens across launchctl logins; independently storing credentials does not change GitHub's revocation boundary. Explain this distinction in recovery guidance. [GitHub token revocation](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/token-expiration-and-revocation).

GitHub Actions workflows already committed to a repository can still run independently on GitHub, and build secrets already copied there remain until removed. Track the exact remote files, secrets, variables, environments, and runs owned by each connection/workload. An explicit revoke/disconnect transaction blocks repository-use operations and callbacks and may atomically create a separate cleanup authorization for its recorded owned resources. That authorization is bound to the revocation operation, the retired resource identities, the expected current connection generation, specific disable/delete actions, and a maximum fifteen-minute expiry. Require fresh GitHub authority and resource ownership checks; never derive cleanup authority from a revoked grant alone. Preserve user-modified files and resources owned by other connections. Do not delete ambiguous shared legacy secrets automatically. If GitHub access is already revoked or cleanup fails, show the remaining remote resources and manual cleanup steps; local disconnect still succeeds. Never promise that blocking launchctl callbacks cancels all GitHub-side runs or removes remote secrets.

Cleanup runs through a dedicated capability check, not the normal repository-use resolver. Its credentials stay in the trusted cleanup worker, restricted to the relevant repositories and required permissions; they never reach a browser or tenant server. It may inspect only the named owned resources and metadata needed to verify safe deletion/disablement; it cannot browse/clone application source, deploy, discover additional repositories, change selection, or grant general access. The issuance/version checks apply to this capability and its purpose. Reconnect, authority loss, or expiry cancels it; retries cannot extend it. A later cleanup retry requires a newly reviewed authorization. Serialize cleanup and bootstrap for the same connection/workload and verify remote resource ownership/version before each mutation. Prefer generation-specific resource names. A delayed cleanup must not remove a workflow or secret created by a subsequent reconnect. If old ownership cannot be proved, retain the resource with a visible cleanup-required state rather than deleting it.

Route webhooks by `X-GitHub-Event` plus action. Preserve `X-GitHub-Delivery`, signature validation, and processing status. Handle `installation`, `installation_repositories` with `added`/`removed`, `github_app_authorization`, relevant `repository` changes, and `push` explicitly. Unrelated events with `action=deleted` must never delete a connection. GitHub documents the [event/action payloads](https://docs.github.com/en/webhooks/webhook-events-and-payloads#installation_repositories) and [delivery handling](https://docs.github.com/en/webhooks/using-webhooks/best-practices-for-using-webhooks).

Receipt deduplication alone is insufficient after partial fanout or a crash. Persist a unique dispatch intent and local deployment for `(host, app, delivery, workload, action)` together with an outbox entry in one transaction. Mark the receipt complete only when fanout is durably recorded. Publish/retry with stable identifiers; a repeated consumer execution reuses the existing deployment rather than creating another. Enforce this at deployment creation as well as webhook receipt. For large fanouts, persist a resumable cursor with the unique intents; a crash must neither lose the remaining workloads nor duplicate completed ones. This guarantees at most one local intent/deployment per key, not exactly-once GitHub or SSH effects. Reconcile uncertain external outcomes using persisted task/run/operation identity before retrying; if reconciliation is impossible, surface outcome unknown instead of blindly repeating a mutation.

Use durable reconciliation jobs with retries, per-installation serialization, and a version check so an older in-flight sync cannot overwrite a later revocation. Fetch all pages successfully before marking absent repositories unavailable. Commit metadata and the successful-sync timestamp transactionally; propagate partial write failures. Reconcile periodically to repair missed deliveries. Treat authorization errors as access failures and rate limits/timeouts as retryable failures, not empty catalogs. Surface both installation availability and account selection status without exposing other accounts' inventory.

Specify the periodic inventory reconciliation interval and alert threshold before production rollout; it repairs missed notifications but is not an alternative to the maximum five-minute authority freshness check on each new operation. Denial starts at the committed local revocation, not at an unknowable instant when GitHub changes remote access. After freshness expires, provider unavailability blocks new authenticated work. Delayed positive events or stale API results cannot clear local revocation or authorize a newer generation. The remaining lifetime of an already released credential is a separate bound.

## Frontend and documentation changes

Add **Manage repositories for this account**, **Manage GitHub app access**, and **Disconnect from this account** as distinct actions. Display the team receiving access and the number of selected usable repositories. The eligible candidate count belongs only in the authorized manager's selection view, not in ordinary member views.

Reuse the existing Connections settings and site/application/Compose pickers. Every workload picker lists only granted repositories. Clear and cancel connection/repository requests on logout, login, team switch, or source-control change; guard late responses so they cannot restore data from the previous context.

Scope ordinary repository and connection caches by the destination team, connection, and current selection/generation; scope candidate caches additionally by the authenticated actor and GitHub authorization. Never reuse a repository list solely because the GitHub identity or installation matches. Show primary and explicitly selected dependency repositories together in the workload's access summary.

Replace fixed-delay sync success messages with a returned job/status and observed completion. Show no repositories selected, pending GitHub approval, access removed, suspended, reconnect required, syncing, and failed states. Include useful retry or reconnect actions.

Update account/API documentation and English/Japanese strings. Explain that running applications remain running after disconnection, while future authenticated deployments stop. Remove documentation that advertises unavailable connection controls or providers.

### Reusing the same repository in multiple accounts

Repository selection and repository exclusivity are separate product rules. Explicitly granting the same repository twice must never happen automatically or disclose either account's private configuration to the other.

GitHub Actions currently has additional shared-repository hazards: Compose can write `LAUNCH_DEPLOY_TOKEN`, and build secrets use `LAUNCH_BUILD_<NAME>` across workloads (`internal/modules/docker/jobs/gha_bootstrap_workflow.go:962`). Namespace workflow files, callback secrets, build secrets, variables, environments, and cleanup by connection and workload. One account's cleanup must never remove another account's resources.

Namespacing prevents accidental overwrites; it does not isolate secrets from another actor who can edit workflows in the same GitHub repository. Until that shared trust is explicitly accepted and tested, allow only one workspace to own launchctl-managed GitHub Actions resources for a repository. Claim that ownership atomically by immutable repository ID before issuing automation credentials or writing remote resources. Another account may still explicitly select and clone the repository; that selection must never interrupt the existing owner's automation. Competing Actions setup is blocked with a generic message, and ownership remains reserved until every recorded cleanup has completed. Disjoint repositories on the same installation remain the primary release scenario.

## Migration and rollout

Ship focused changes in this order. Do not enable installation sharing by merely removing `ensureInstallationNotClaimed`.

1. **Contain current authorization gaps.** Add regression coverage and fixes for cross-team connection injection, mismatched site connections/repositories, credential host checks, webhook event/action routing, and repository-restricted workload tokens. Keep sharing disabled. Block unverified installation-ID linking until the verified connection flow in phase 2 is ready; removing the existing claim check is not a containment fix.
2. **Add storage and verified authorization.** Add installation inventory, authorization/attempt records and configuration, grant identity/state, connection generations, credential leases, webhook receipts, and dispatch/outbox uniqueness. Implement account-bound authorization/callbacks and installation verification together with those prerequisites. Exclude authorization records from generic team transfer and add the GitHub-bound transfer guard. Backfill stable GitHub repository IDs with verified API metadata; preserve existing local IDs. Resolve duplicate legacy rows by redirecting references in a transaction before changing uniqueness. Quarantine unresolved records rather than guessing by a renamed slug.
3. **Integrate the resolver across consumers.** Prepare sites, Docker/Compose source configs, dependencies, all GHA service/job/callback paths, registry credential references, raw APIs, and push routing for the enforcement cutover in phase 5. Add per-deployment secret delivery/configuration and generation checks. Old payload compatibility may resolve a URL only within an already selected grant for the same team; it cannot create a grant or mint unrestricted credentials. Once cut over, enforce capability and grant state before every external effect.
4. **Collect explicit selections for existing accounts.** The old schema contains an automatically imported catalog, so it cannot reveal which repositories a person intended to authorize. Show existing workload repositories prechecked in a migration review, with other currently eligible repositories unselected. The manager verifies GitHub identity and saves the selection for cutover; this phase collects proof and consent and does not enable sharing. Freeze automatic expansion immediately. Obtain fresh GitHub-side proof for the exact workload repository grants and revalidate it at the phase-5 cutover; an old team connection, cached repository row, or successful app-token lookup is not proof. Compatibility is limited to old request formats and always requires that new authorization. Define the cutover deadline before production rollout; unconfirmed accounts retain their running applications but new GitHub-dependent operations pause at cutover until verified. No shared-installation connection is permitted for an unverified legacy account.
5. **Retire old execution paths and credentials, then release reuse.** Ensure every API instance, worker, and scheduled process runs the new authorization implementation. Stop old consumers and finish/cancel their in-flight sync/deployment work before cutover; an old binary cannot be allowed to ignore grant state. Quarantine or deliberately recreate legacy queue payloads under current proof. Stop all unrestricted tenant-token issuance, revoke tracked broad credentials, and for untracked credentials wait their maximum remaining lifetime (normally one hour for installation tokens) plus a defined clock/operational margin. Record when issuance stopped and when this expiry barrier is satisfied. Audit/clean legacy credential-bearing remotes, task scripts, and shared config. Only then enable installation sharing or expand installation access for a second account. Ship frontend selection, status, disconnect, request invalidation, and documentation with enforcement active. All new connections start empty; candidate discovery alone never selects repositories.
6. **Remove legacy authorization paths.** After migration verification, retire direct installation-ID linking, full-catalog tenant responses, name-only credential lookup, and compatibility flags. Retain stable local IDs and audit history.

Use feature gates for rollout, not for bypassing authorization. If a new flow fails, disable new linking/selection mutations and preserve existing grant enforcement; rollback must never restore installation-wide credentials or automatic grant expansion. Rehearse migrations against a copy with missing IDs, renamed repositories, null teams, multiple workloads, and interrupted backfills.

The old-token expiry barrier is a release condition, not a promise to undo past access: source already cloned or credentials independently retained on a customer's server cannot be erased retroactively. Deployment artifacts that require an expired external credential need their documented recovery path. Do not roll back to an old worker/API version that can bypass the new authorization checks.

## Acceptance criteria

The release must demonstrate these outcomes with two separate launchctl users and separate teams sharing one GitHub identity/installation:

| Scenario | Expected result |
| --- | --- |
| A selects frontend; B selects backend | Each account lists, clones, builds, and deploys only its selection. Neither needs to revoke or reinstall GitHub access. |
| B connects an installation already used by A | B starts with no repositories selected. A's connection, grants, tokens, and workloads remain unchanged. |
| GitHub app receives another repository | Neither existing account gains access until explicit selection. |
| Same GitHub identity authorized from another launchctl login | No account merging, inherited authorization, or global identity uniqueness conflict. |
| Private workspace selection, invited user with no private workspace, or shared personal team | Destination is reviewed explicitly; the app never silently targets a shared team. The personal-team flag alone does not establish privacy. |
| Two logins intentionally use one shared team | The UI explains shared access; both use only that team's selected repositories according to their roles. Private workspace selections are not imported. |
| Membership changes during a private connection attempt | Completion requires a new destination/sharing review; the old attempt cannot silently grant access to added members. |
| Empty connection browses eligible candidates | Only the verified manager sees minimal candidate metadata; no clone, build, deployment, or ordinary repository API access is granted. |
| Foreign installation ID, repository ID, connection ID, or cross-connection pair submitted directly | Rejected without metadata disclosure, grant creation, queueing unauthorized work, or credential issuance. |
| A read-only GitHub collaborator attempts to delegate app write authority | Rejected by the sponsor/capability policy. |
| Callback state is absent, expired, replayed, denied, or opened after switching logins | No connection is created under the wrong login; the user gets an actionable result. |
| Repository deselected after a deployment was queued | Job revalidation blocks GitHub access; an old GHA callback bearer cannot restart it. |
| Disconnect commits while GitHub is issuing a token | The response is discarded/revoked without publication under the old generation; pending issuance cannot escape revocation tracking. |
| A valid cached token remains after grant revocation or stale sponsor authority | A new operation is rejected by current local versions or the authority freshness limit despite the token still being valid externally. |
| Revoke, reconnect, then receive an old job/callback/cleanup task | New generation and rotated callback authorization reject old work; stale cleanup cannot remove new resources. |
| Simultaneous refresh, normal expiry, expired refresh authorization, or SSO reauthorization | Refresh is atomic and cannot resurrect revoked authorization; recoverable reconnect preserves selected IDs without changing either account's selections. Shared upstream revocation follows its separate lifecycle rule. |
| Account A disconnects | B continues working; shared GitHub authorization and installation are not revoked remotely. |
| Shared GitHub identity revokes the app authorization on GitHub | Both launchctl logins' matching authorizations and dependent grants stop working; unrelated identities remain unaffected. |
| Account disconnects with existing GHA workflows/secrets | Local access stops immediately; only provably owned remote resources are disabled/cleaned up, and failures or retained resources are visible without affecting another account. |
| Cleanup executes after local grant revocation | A separate unexpired revocation-created capability permits only named owned-resource disable/delete actions with current GitHub authority; it cannot clone, deploy, create grants, or survive reconnect. |
| Migration encounters only an old connection/catalog record | That record cannot establish GitHub authority or receive compatibility credentials without fresh proof. |
| Generic team transfer includes GitHub-bound resources or an existing destination installation connection | Initial release blocks the transfer before mutation; no selections/proofs move or merge and no collision causes partial reassignment. |
| Repository/installation permission removed, suspended, or uninstalled | New authenticated operations stop, references remain recoverable, and running workloads stay running. |
| Grant sponsor leaves or loses GitHub authority | New work pauses within the defined freshness limit unless another verified manager assumes sponsorship. |
| Repo renamed, transferred, re-added, or installation reinstalled | Stable IDs preserve history; authority is reverified according to lifecycle rules. |
| Token minted for a clone | Request contains only the primary repository or the explicitly approved, independently authorized dependency set for that operation and required read permissions; no installation-wide credential reaches the server. |
| Selected/unselected nested dependency or changed `.gitmodules` host/repository | Only declared, selected, authorized dependency repositories are accessible; missing access fails without broad-token or ambient-SSH fallback. |
| Concurrent deployments use different repositories/packages on the same server | Process-scoped Git and separate `DOCKER_CONFIG` directories prevent credential replacement or cleanup interference. |
| Pull fails, a task times out, or deployment is cancelled | Private credential files are cleaned; no token bytes appear in queue records, generated/persisted task scripts, or logs. |
| Private GHCR pull is delayed or a rollback needs fresh credentials | Package-scoped credentials and actual expiry are respected; a supported retry/rebuild path works without unrestricted installation-token fallback. |
| Credential reference is forged, replayed for another deployment, or redeemed after revocation | Team/deployment/generation/package/purpose checks deny access; references are not transferable authority. |
| Arbitrary URL supplied with a GitHub connection | No credential is attached; authenticated operations use verified canonical metadata. |
| Duplicate/out-of-order webhook or concurrent sync/selection update | No duplicate deployment, grant expansion, lost revocation, or success timestamp for an incomplete sync. |
| Worker crashes after partial webhook fanout or after queue publication | Unique durable intents/outbox and stable deployment IDs recover missing work without creating another local deployment for the same key. |
| External deployment result is unknown after a crash | Reconcile the existing task/run before retrying; otherwise surface unknown outcome instead of promising exactly-once execution. |
| Old binary, legacy payload, or unexpired broad credential exists during rollout | Installation sharing remains disabled until old consumers retire, payloads are handled explicitly, and tracked revocation or the expiry barrier completes. |
| Delivered repository/comment deletion | The launchctl installation connection is not deleted. |
| Slow picker response returns after account/team change | Prior account repository data cannot repopulate the current screen. |
| Concurrent selection saves | Version checks reject stale changes; they do not restore revoked access. |
| Two account connections select the same repository | Both explicit read/clone grants remain independent; ordinary selection changes do not disrupt existing CI. Competing GHA setup is blocked until the previous workspace's recorded cleanup completes. |

Use service/handler tests for authorization, PostgreSQL integration tests for migrations/constraints/transactions, fake GitHub HTTP tests for exact credential scopes and pagination failures, worker tests for revocation races/crash recovery, and frontend tests for callback/status/context changes. Finish with an end-to-end rehearsal against a dedicated GitHub test installation using two launchctl accounts in separate private workspaces, private primary/dependency repositories, and a private GHCR package. Attempt direct API and actual clone access to each other account's repository and an unselected third repository, including with captured account-scoped credentials; every attempt must fail. Then change GitHub availability, disconnect/reconnect one account, run overlapping deployments, and repeat. A filtered dropdown or passing mocked-token test is insufficient evidence.

## Conditions before implementation and release

The design must preserve I1–I6 across selection, every consumer, revocation, reconnect, team transfer, migration, and rollback. The workspace-sharing boundary and the sponsor policy are explicit product behavior, not an assumption that any two login IDs are isolated. No endpoint, cache, job, or fallback may authorize a repository merely because the shared GitHub installation can access it.

Before production rollout, verify the actual GitHub App callback/user-authorization configuration, required operation permissions, webhook subscriptions, refresh/SSO behavior, supported GHCR credential path, inventory reconciliation interval, migration deadline, and old-token expiry margin. These are release checks; unknown values cannot be replaced by permissive defaults. Release requires the acceptance scenarios above and recorded negative access tests with the real test installation. Design review does not establish that the current code implements these guarantees.

## Validation baseline

On 17 November 2026, `go test ./internal/modules/git/...` passed in the reviewed working tree. At that point the Git service package had no tests, and the passing suite did not establish account isolation. The baseline preceded implementation; the completed changes and later verification are recorded below.

The revised design was rechecked against I1–I6, private/shared workspace selection, dependency access, credential reuse and revocation races, reconnect/cleanup, private registry pulls, crash recovery, team transfer, and rollout ordering. No remaining material contradictions were identified within those reviewed flows. The review required negative-access tests, migration rehearsal, and live GitHub/GHCR checks before release. Automated coverage is recorded below; live acceptance remains a release requirement.


## Implementation and rollout notes

The implementation separates an account's saved repository selection from GitHub installation inventory. Selection edits have their own revision: saving an unchanged selection or adding another repository preserves the authorization of retained repositories and existing Actions bindings. Reconnects and sponsor replacements change the connection generation. Unavailable repositories retain their saved selection for review but cannot authorize operations until explicitly verified again.

The verified connection flow uses `GITHUB_CLIENT_ID`, `GITHUB_CLIENT_SECRET`, and `GITHUB_CALLBACK_URL` in addition to the existing GitHub App settings. Apply migrations 0071–0074 before starting updated API and worker processes. Existing GitHub connections require verified reconnect and explicit selection; legacy synchronized rows do not become grants. Keep all APIs and workers on the updated authorization boundary, retire old installation-wide tokens, and do not roll back to the previous permissive token path.

A reconciliation job runs every minute to refresh stale selected-repository authority, retry pending credential revocations and cleanup, and dispatch undelivered webhook intents. New operations require authority no older than five minutes. Cleanup grants expire after fifteen minutes, are restricted to recorded resources, and cannot remove a modified workflow or act after a replacement connection generation. Failed or expired cleanup remains visible for manual attention; it never restores workload access.

GitHub Actions reserves automation ownership before the first remote write because GitHub's repository secrets are shared within the repository. Selecting the same repository in another private workspace does not interfere with the existing owner's CI, but competing automation is denied until the owner's recorded cleanup completes. Failed or expired cleanup does not release ownership. Server builds support explicitly selected private dependencies, including dependencies on another installation in the same workspace. Git fetches inside Dockerfile build containers require explicit build-secret mounts; host Git credentials are not copied into image layers. Private dependencies in GitHub Actions remain unavailable until a scoped credential broker can renew their credentials safely; the interface explains that server builds are required for this case.

Verification uses simulated GitHub responses, isolated PostgreSQL migration/concurrency tests, worker and callback regressions, secret transport tests, and frontend flow tests. A dedicated live GitHub/GHCR test environment was not available. The live acceptance rehearsal and production App configuration checks above remain release requirements; automated results do not claim live private-repository or GHCR validation.


### Recorded automated verification

- Full Go package suite passed with GitHub, Docker bootstrap, and full migration PostgreSQL integration enabled against disposable databases.
- Focused Go race-detector checks passed for authorization, selection and refresh concurrency, account lifecycle, site grants, and credential transport.
- The Nuxt suite passed: 88 test files, 671 tests. Type checking and targeted lint passed.
- Site deployment tests additionally verify durable webhook redelivery, stale dependency changes, and rejection of a previously authorized deployment whose repository binding was removed.
- Database migrations were exercised both on existing connection/history fixtures and as a clean complete migration run. Former-creator deletion preserves surviving workspace connections; old inventory does not acquire access.

Production still requires the configured GitHub OAuth callback, all updated workers, legacy connection review, and the private GitHub/GHCR acceptance rehearsal described above. No production rollout was performed as part of this implementation.

### Isolation review, 18 November 2026

The follow-up review added regressions for two logins sharing a GitHub identity and installation. Connecting, adding/removing all or some selections, disconnecting, and reconnecting account A preserve account B's authorization, repository grants, and issued credentials. Delayed GitHub responses cannot restore revoked availability or invalidate a newer selection. Inventory removal revokes only the affected repository's credentials.

Automation ownership is claimed atomically before credentials or remote writes. Existing CI continues when another workspace selects the same repository. Concurrent competing claims have one owner; failed, expired, manual-attention, and unrecognized cleanup states retain that ownership until cleanup completes. Local disconnect and cleanup revocation requests are scoped to their own connection or manifest, and an unrelated revocation failure cannot delay push delivery or block completed cleanup.

Membership removal/demotion and sponsored connection invalidation now commit together. Stale sponsor checks cannot revoke a replacement connection or repository grant. Rollbacks validate primary and dependency grants again before execution. Git tasks ignore ambient credential helpers and global URL rewrites, preventing credentials from another deployment from overriding the selected repository credentials.

Frontend regressions cover delayed authentication/team responses, independent token refreshes, cached team lists, and account switches while disconnect or repository-removal confirmations are open. Existing help distinguishes local workspace selection from installation-wide changes made directly on GitHub.

Final verification passed: the complete Go suite (116 packages with tests), including isolated PostgreSQL GitHub/Docker concurrency and migration tests; focused race-detector suites; and the frontend tests/type checks/lint recorded above. Live private GitHub and GHCR validation remains pending.

### Final pre-commit verification

The final review closed two additional connection races. A separate installation repository revision now fences in-flight selections, including repositories with no prior grant in that account. Lifecycle events persist installation metadata even before the first local connection exists, so an initial save cannot lose a concurrent removal or suspension. The revision does not change unrelated workload snapshots. Concurrent first connections from separate accounts reuse one shared installation row and retain separate selections. Migration 0071 adds the revision idempotently while preserving existing installation generations.

Webhook fanout now commits and dispatches successful recipients while failed recipients remain retryable. Redelivery skips existing intents, preventing duplicate deployments. Permission recovery checks the installation before consulting an account's GitHub authorization, so an unrelated account cannot block recovery.

Git execution rejects ambient SSH, local-file, and external-helper paths for GitHub-backed work. Public URL builds require credential-free HTTPS. Existing Site checkouts replace the remote with the selected canonical repository and remove legacy local authentication and URL rewrites. A real Git/TLS regression proves that an ambient `.netrc` credential works in the unprotected baseline, is denied with the deployment settings, and the explicitly selected credential still works. The underlying precedence is documented in [libcurl's NETRC behavior](https://curl.se/libcurl/c/CURLOPT_NETRC.html), and Git's empty-auth setting supplies explicit empty credentials before the selected helper is consulted.

Passkey and two-factor login interactions now retain their original session across browser prompts and challenge submission. Delayed completion cannot replace an account that signed in in the meantime.

Repository counts now reflect the selected, available repositories of each connection. The pickers refresh that count from their account-scoped repository response, independently of search filtering. Regression coverage checks stale installation totals, separate account selections, removed access, disconnects, and empty refresh results. The Site, application, and Compose repository fields use one rounded focus boundary with visible keyboard focus on their buttons. English and Japanese repository-count messages have matching plural forms.

Final verification of the release changes passed: 116 Go test packages with PostgreSQL integration and full migration coverage; 17 focused Go packages under the race detector; 671 frontend tests in 88 files; frontend type checking, targeted lint, and whitespace checks. These are automated results. No deployment was performed, and live private GitHub/GHCR acceptance remains pending.
