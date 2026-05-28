# GitHub App permissions required by Launch

This is the **operator-facing** reference for the GitHub App permissions
Launch needs. Use it when:

- Creating a new Launch GitHub App from scratch.
- Updating an existing App that pre-dates the **Build with GitHub
  Actions** feature (launched May 2026).
- Diagnosing a 403 from the GitHub API in the worker logs (e.g.
  `Resource not accessible by integration`).

> ⚠️ Existing installations don't auto-pick-up new permissions.
> Whenever you change the App's permissions on github.com, every
> account/org that has the App installed will receive an email and
> **must accept the new permissions** at
> `https://github.com/settings/installations/<installation_id>` (org
> admins can also accept on behalf of an org). Until each install
> accepts, API calls that need the new perms will return 403 for that
> install only.

## Repository permissions

Set on the App settings page → **Permissions & events** → **Repository
permissions**.

| Permission | Access level | What in Launch uses it |
|---|---|---|
| **Metadata** | Read | Default — listing installs, basic repo info. Always required. |
| **Contents** | **Read and write** | Reading the repo tree for the in-server git build path, and committing files (the GHA workflow YAML, see below). |
| **Workflows** | **Read and write** | GitHub gates writes to anything under `.github/workflows/*` behind this *separate* scope, on top of Contents. The GHA-builds bootstrap commits `.github/workflows/launch-deploy.yml`, so this is required even though Contents is also write. |
| **Actions** | **Read and write** | Triggering / observing `workflow_dispatch` runs, used by the GHA-builds resync action and by deploy-now from the UI. |
| **Secrets** | **Read and write** | Fetching the repo's libsodium public key (`GET /actions/secrets/public-key`) and pushing `LAUNCH_DEPLOY_TOKEN`. |
| **Variables** | **Read and write** | Pushing `LAUNCH_APP_ID` / `LAUNCH_COMPOSE_ID` and `LAUNCH_WEBHOOK_URL` so the workflow can call us back. |
| **Pull requests** | Read and write | (Optional, only if you want PR-comment-based deploy previews — not required by GHA-builds.) |

### Why so many separate Actions scopes?

GitHub split what used to be a single "Actions" permission into four
distinct ones (Actions, Secrets, Variables, Workflows). They are *not*
implied by each other. An App that has `Actions: read & write` but not
`Secrets: write` cannot push a secret, and you will see the exact 403
that opened this doc.

## Account permissions

None required.

## User permissions

None required.

## Subscribe to events

The GHA-builds feature **does not require any new event subscriptions**.

The workflow → Launch callback is a plain authenticated HTTPS POST from
the GitHub Actions runner, not a GitHub webhook delivery, so no
subscription is in that critical path.

Keep whatever events the App was already subscribed to for the rest of
Launch (typically: `installation`, `installation_repositories`, and
`push` if you use auto-deploy on git push).

### Optional events (future polish — not wired up today)

- **`workflow_run`** — would let Launch hear about builds that *fail
  mid-workflow* (before the success-callback step runs) and update the
  deployment row accordingly. There is no handler for this event in
  the current codebase; subscribing today is harmless but inert.

## Setting the App up for the first time

If you're spinning up a new GitHub App for Launch:

1. Go to https://github.com/settings/apps/new (or
   `https://github.com/organizations/<org>/settings/apps/new` for an
   org App).
2. **GitHub App name**, **Homepage URL**, **Callback URL** — your
   normal values; Callback URL is the `/auth/github/callback` route on
   your Launch deployment.
3. **Webhook URL** — `https://<your-launch>/webhooks/git/github`.
   Set a strong webhook secret and put it in `GITHUB_WEBHOOK_SECRET`.
4. **Permissions** — set everything in the table above.
5. **Subscribe to events** — `Installation`, `Installation
   repositories`, `Push`.
6. **Where can this GitHub App be installed?** — your call. "Only on
   this account" for SaaS-style single-tenant; "Any account" for
   distribution to customers.
7. Save, generate a private key (`.pem` file), and put the values into
   the Launch `.env`:
   ```
   GITHUB_APP_ID=<numeric id>
   GITHUB_APP_SLUG=<the slug, e.g. launchctl-bot>
   GITHUB_PRIVATE_KEY="-----BEGIN RSA PRIVATE KEY-----
   ...full PEM contents, newlines preserved...
   -----END RSA PRIVATE KEY-----"
   GITHUB_WEBHOOK_SECRET=<what you set on github.com>
   ```

## Updating an existing App

For an App that pre-dates the GHA-builds feature:

1. Visit `https://github.com/settings/apps/<app-slug>` (use the app
   slug from your `GITHUB_APP_SLUG` env var).
2. Open **Permissions & events**.
3. Bring **Contents**, **Workflows**, **Actions**, **Secrets**, and
   **Variables** up to **Read and write** if they aren't already.
4. Click **Save changes** at the bottom.
5. GitHub now emails every account that has the App installed. Visit
   each installation page (`/settings/installations/<id>`) and click
   **Review request** → **Accept new permissions**. Org installs can
   only be accepted by an org owner.
6. Restart the Launch API + worker so cached installation tokens
   include the new permission set (tokens themselves auto-expire after
   an hour, but a restart guarantees a clean state).

After step 5 finishes for every installation, re-trigger any
GHA-backed application or compose stack via the **Resync** action in
its settings tab. The bootstrap job will now reach all four GitHub API
calls successfully and commit the workflow file.

## Verifying the permissions are correct

After the App and one install both have the new perms, you can
spot-check from the launch-go host:

```bash
# Adjust the IDs to a real workload you control.
APP_ID=01ksqm54avbxtnamy8znzw4ywf
SERVER_ID=01kspn4cp9t5edj2bf80rjgny6
PROJECT_ID=01ksqkpj8p93prezqnf1ah5max
BEARER=...        # an access token for a team admin
TEAM_ID=...       # the team that owns the workload

curl -sS -X POST \
  -H "Authorization: Bearer $BEARER" \
  -H "X-Team-ID: $TEAM_ID" \
  "http://localhost:6124/servers/$SERVER_ID/docker/projects/$PROJECT_ID/applications/$APP_ID/gha/resync"
```

Then tail the worker log:

```bash
tail -f /tmp/launch-go-worker.log | grep -iE "bootstrap|gha|github"
```

You should see the bootstrap walk through:

1. resolve installation token,
2. `GET /actions/secrets/public-key` → 200,
3. `PUT /actions/secrets/LAUNCH_DEPLOY_TOKEN` → 201,
4. `PUT /actions/variables/LAUNCH_APP_ID` → 201,
5. `PUT /actions/variables/LAUNCH_WEBHOOK_URL` → 201,
6. `PUT /repos/{owner}/{repo}/contents/.github/workflows/launch-deploy.yml` → 200/201,
7. `source_config.gha_workflow_sha` populated on the DB row,
8. `docker.application.gha_bootstrap_complete` broadcast on `team.<teamID>`.

A 403 on any of those steps means that specific permission either
wasn't set on the App or wasn't accepted on the installation. The
error body from GitHub names which resource the integration couldn't
access, e.g. `secrets`, `actions/variables`, or
`repos/.github/workflows/...`.

## Troubleshooting

| Symptom | Likely cause |
|---|---|
| `provider not configured` in service logs | `GITHUB_APP_ID` / `GITHUB_PRIVATE_KEY` not set in `.env`. Restart API + worker after fixing. |
| `GetActionsPublicKey: status 403 ... Resource not accessible by integration` | `Secrets: read & write` is missing on the App, or the install hasn't accepted the new perms yet. |
| `PutContents .github/workflows/...: status 403` | `Workflows: read & write` is missing — note this is *separate* from Contents. |
| `PutActionsVariable: status 403` | `Variables: read & write` is missing. |
| Workflow runs in GHA but Launch never deploys | The runner couldn't reach `LAUNCH_WEBHOOK_URL`. Verify the variable is set on the repo and your Launch instance is publicly reachable on that URL. |
| Webhook arrives but returns 401 | Token mismatch — rotate the deploy token via the **Rotate token** action in the application's GHA subtab; that mints a fresh token and re-pushes the secret. |
