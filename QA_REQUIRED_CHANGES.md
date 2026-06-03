# QA Required Changes

Target tested: `https://shclop.178.62.240.51.nip.io` on the rebuilt 4 vCPU / 8 GB QA host.

Status markers are intentionally conservative: an item is checked only after the original issue is verified not to reproduce in the target environment.

## 1. Bootstrap installs no working Kata runtime

- [x] Verified fixed on target environment
- Severity: Critical
- Evidence: `scripts/bootstrap.sh install --install-deps` could not install `kata-runtime`; the configured openSUSE repository returned `404 Not Found` for Ubuntu 24.04.
- Runtime evidence: after starting an agent, Kubernetes reported `FailedCreatePodSandBox: no runtime for "kata" is configured`.
- Required change: update the Kata installation path in `scripts/bootstrap.sh` to a supported current method, and make the installer fail before Helm deployment if the Kata runtime is not actually configured in K3s containerd.
- Verification status: fixed on the target host. Bootstrap installed Kata 3.31.0 from the official GitHub static tarball, repaired the K3s containerd template, restarted K3s successfully, created `RuntimeClass/kata`, and the bootstrap `runtimeClassName: kata` smoke pod completed before Helm deployment.

## 2. Agent UI reports Running while Kubernetes runtime pod is not running

- [ ] Verified fixed on target environment
- Severity: High
- Evidence: the UI showed `qa-agent ... Running` and a `Stop` button, while `kubectl get pods` showed the runtime pod in `ContainerCreating/Pending` with repeated sandbox creation failures.
- Screenshots:
  - `dogfood-output/screenshots/agent-start-after-gateway.png`
  - `dogfood-output/screenshots/agent-still-running-while-pending.png`
- Required change: only transition an agent to `running` after the runtime pod reaches a usable Ready state or after the runtime WebSocket handshake succeeds. Surface pod creation failures to the user instead of showing a generic or stale running state.
- Implementation status: backend Kubernetes runtime startup now waits for the pod to be `Running` with all containers `Ready` before returning success. Timeout and failed-pod errors include warning Events and container waiting reasons.
- Retest status: a newly created runtime pod reached `READY 1/1 Running` and registered over the runtime WebSocket before chat testing. The checkbox remains open because an older crashed agent still appeared as `running` in the UI after backend restart; see issue 6.

## 3. Start-agent error message is too generic in the UI

- [ ] Verified fixed on target environment
- Severity: Medium
- Evidence: before LLM gateway settings were configured, the API returned a clear 400 message, but the UI only displayed `Request failed (400)`.
- Screenshot: `dogfood-output/screenshots/agent-start-result.png`
- Required change: display the server response body in the toast/banner so users know which setting is missing.
- Implementation status: frontend API error handling now reads JSON and plain-text response bodies, including Go `http.Error` responses, before falling back to `Request failed (<status>)`. This still needs a fresh UI run against the target backend before the checkbox can be marked.

## 4. LLM gateway is not provisioned for end-to-end agent chat

- [ ] Verified fixed on target environment
- Severity: Medium
- Evidence: QA used a dummy Kubernetes Secret and `https://example.invalid/v1` only to pass start-flow validation. No real gateway service or API key was installed by bootstrap, so model-backed chat was not validated end to end.
- Required change: document and automate one supported E2E path: either configure an external LLM gateway/key during bootstrap, or provide an explicit mock/test gateway mode for QA environments.
- Retest status: a real OpenRouter key and the free model `deepseek/deepseek-v4-flash:free` were configured. Starting Bob's model-backed agent initially failed with `LLM gateway not fully configured: enabled and base URL are required when an agent model is set` because the deployed backend had empty `--llm-gateway-base-url` and `--llm-gateway-secret-name` arguments. A manual cluster hotfix created `Secret/shclop-litellm` from the LiteLLM master key and patched `deployment/shclop-backend` with `--llm-gateway-base-url=http://litellm:4000` and `--llm-gateway-secret-name=shclop-litellm`; after that, Bob's agent could start. The checkbox remains open because this fix is not persisted in bootstrap or Helm values and will be lost on reinstall/upgrade.
- Persistence fix applied: `scripts/bootstrap.sh` `generate_default_values` now writes `llmGateway.baseURL` using the full Kubernetes FQDN (`http://<service>.<namespace>.svc.cluster.local:4000/v1`). The `deploy_shclop` Helm command passes `--set llmGateway.baseURL=$(litellm_service_url)` and `--set llmGateway.existingSecret.name=${LITELLM_MASTER_SECRET}` so the backend always receives the correct `--llm-gateway-base-url` and `--llm-gateway-secret-name` arguments. The chart template `deployment.yaml` also auto-derives the FQDN URL when `llmGateway.litellm.enabled=true` and `baseURL` is empty, providing an additional safety net. No manual `kubectl patch` or secret creation is needed after a fresh bootstrap install.

## 5. Runtime does not call the configured external LLM provider

- [ ] Verified fixed on target environment
- Severity: High
- Evidence: with OpenRouter configured and a runtime pod registered as `llm_gateway_configured=true` for model `deepseek/deepseek-v4-flash:free`, the minimal chat response was the local demo response: `openclaw runtime received: Reply with exactly: hello world`, followed by workspace/memory text. That indicates the runtime used the demo adapter instead of calling the configured OpenRouter model.
- Required change: implement the runtime-side LLM adapter path for the configured gateway, base URL, API key, and model; then add an E2E check that proves the response comes from the external provider or an explicit mock provider.
- Retest status: Bob's `nanoclaw` runtime now receives UI chat tasks and attempts the configured LiteLLM endpoint, but E2E chat still fails. The UI displayed the runtime error `http request: Post "http://litellm:4000/chat/completions": dial tcp: lookup litellm on 10.43.0.10:53: read udp ...->10.43.0.10:53: read: connection refused`. This proves the task reaches the runtime and the runtime is no longer just returning the demo response, but the runtime cannot resolve/reach the in-cluster LiteLLM service from the Kata sandbox.

## 6. Persisted agent state can remain `running` after runtime pod crash or backend restart

- [ ] Verified fixed on target environment
- Severity: High
- Evidence: after the adapter-token bug was fixed and the backend redeployed, the UI still showed the older `qa-deepseek-minimal` agent as `running`, while its runtime pod was `CrashLoopBackOff`/completed and chat returned `runtime not connected`.
- Required change: reconcile persisted agent state with runtime registry and Kubernetes pod state after backend restart and after runtime disconnects, or show a distinct disconnected/stale status in the UI.

## 7. Observability bootstrap used an outdated VictoriaLogs chart/service name

- [x] Verified fixed on target environment
- Severity: Medium
- Evidence: `victoria-metrics-k8s-stack` installed, but there was no `victoria-logs` release until the chart was corrected. The current VictoriaLogs chart/service is `vm/victoria-logs-single` with service `victoria-logs-victoria-logs-single-server:9428`.
- Verification status: superseded by the current Prometheus/Loki/Fluent Bit/Grafana stack on the rebuilt QA host. Prometheus, Loki, Fluent Bit, and Grafana pods are running; Grafana is reachable at `https://grafana.178.62.240.51.nip.io`; backend metrics are queryable through Grafana; and logs are queryable through Loki.

## 8. Regular user Bob can create and start agents, but UI chat E2E fails at LiteLLM DNS from Kata runtime

- [ ] Verified fixed on target environment
- Severity: High
- Evidence: Bob can log in, cannot access `/api/admin/users` (`forbidden`), cannot see admin-owned agents, can create `nanoclaw` agents, and can start them. Runtime pods for Bob's agents reach `READY 1/1 Running` and register with `llm_gateway_configured=true`.
- UI evidence: Bob logged in through the browser UI, created `bob-ui-test`, started it, and sent `Reply with exactly: bob ui ok`. The UI showed the user message and then displayed the runtime error `lookup litellm on 10.43.0.10:53: read: connection refused` instead of an assistant response.
- Runtime/backend evidence: backend logged `task.routed` for Bob's agent `2b387776c084c26b1e04086a2534cd1c`; runtime logged `task received` for the same message.
- Required change: make the Kata runtime networking/DNS path able to resolve and reach the LiteLLM service, or pass a reachable gateway URL to runtimes. Add an E2E test that runs from inside a Kata runtime pod and verifies DNS plus HTTP reachability of the configured LLM gateway before accepting the agent as chat-ready.

## 9. Bob UI chat connection state is misleading for already-running agents

- [ ] Verified fixed on target environment
- Severity: Medium
- Evidence: after Bob logs in with already-running agents, the UI often shows the agent as `Running` with a `Stop` button, but the chat input says `Connect to start chatting…`. The button is disabled until text is typed, so the UI looks disconnected/stuck even though entering a message actually opens the WebSocket and sends the message.
- Required change: either auto-connect chat for running agents, or change the empty state and button behavior so it is clear that typing a first message will connect and send. If auto-connect is expected, reconnect reliably after page reload/session restore.

## 10. Bob cannot list available models

- [x] Verified fixed on target environment
- Severity: Medium
- Evidence: Bob can create agents only by typing a free-form model string in the UI. `GET /api/models` returned `404` for Bob, while model administration is admin-only.
- Required change: expose a read-only model list to regular users, or make the UI clearly document/validate accepted model strings for non-admin users.
- Verification status: fixed on the target host after GitHub Actions deployment of image tag `sha-385ae91d251ed3f3c1f3a122f45f822e0856a90a`. Backend exposes `GET /api/models` for authenticated users. Without gateway discovery config it returns enabled store models for development; when LiteLLM gateway discovery is configured, it calls LiteLLM `/v1/models` with `SHCLOP_LLM_GATEWAY_API_KEY` and returns only enabled store models whose `provider_model` appears in the gateway model IDs. `/api/admin/models` remains admin-only. The Helm chart injects `SHCLOP_LLM_GATEWAY_API_KEY` from the configured gateway Secret. The deployed Bob API check returned HTTP 200 with `DeepSeek V4 Flash (deepseek-v4-flash)`, and the Bob UI agent creation form now renders a model dropdown instead of a free-text input. The existing old Bob agents still show their historical model string (`deepseek/deepseek-v4-flash:free`) until recreated or migrated.

## 11. Grafana datasource/dashboard fixes are not persisted in bootstrap

- [ ] Verified fixed on target environment
- Severity: Medium
- Evidence: Grafana initially had no dashboards and its Prometheus datasource pointed to `http://prometheus-server.monitoring.svc.cluster.local`, which failed DNS lookup. The actual service is `prometheus-server-server`.
- Manual QA fix: patched the remote Grafana ConfigMap to use `http://prometheus-server-server.monitoring.svc.cluster.local`, restarted Grafana, rotated the admin password, and added dashboard `Shclop QA Overview` (`/d/shclop-qa-overview/shclop-qa-overview`) with backend up, container memory, container CPU, and recent Kubernetes logs panels.
- Verification status: Grafana API search returns dashboard UID `shclop-qa-overview`; the dashboard opens in the Grafana UI; Prometheus query `up{service="shclop-backend"}` returns `1`; Loki query `{job="fluentbit"}` returns recent logs. The checkbox remains open because datasource URL, dashboard provisioning, and the non-default Grafana password are remote-only changes and are not persisted in bootstrap/Helm values.
- Persistence fix applied: `scripts/bootstrap.sh` `install_grafana` now:
  - Uses the correct Prometheus URL `http://${PROMETHEUS_SERVER_SERVICE}.<ns>.svc.cluster.local` (defaults to `prometheus-server-server`).
  - Sets the Grafana admin password from `GRAFANA_ADMIN_PASSWORD` when provided, otherwise uses the chart/dev default.
  - Enables the Grafana chart sidecar dashboards provider and provisions the `Shclop QA Overview` dashboard (UID `shclop-qa-overview`) with four panels: Backend Status (stat), Container Memory (timeseries), Container CPU (timeseries), and Recent Shclop Logs (Loki log panel). All changes are idempotent — re-running bootstrap upgrades Grafana in-place without manual ConfigMap patching.

## 12. Loki logs are not labeled by namespace for ergonomic Grafana queries

- [ ] Verified fixed on target environment
- Severity: Low
- Evidence: Grafana Loki query `{namespace="default"}` returned no log lines. Query `{job="fluentbit"}` returned recent Kubernetes logs because Fluent Bit currently sets `job=fluentbit` and embeds Kubernetes metadata in the log body instead of exposing `namespace` as a top-level Loki label.
- Required change: configure Fluent Bit/Loki labels so common Kubernetes filters such as namespace, pod, and container work directly in Grafana dashboards and Explore.

## 13. GitHub integrations E2E validation scenarios

- [ ] Verified fixed on target environment
- Severity: High
- Goal: validate GitHub PAT connection, per-agent enablement, runtime env injection, negative/security paths, and audit integrity.
- Deployment status: image `sha-b3804ac8adab3cea809a6eaa46f83f6710638f0d` was built by GitHub Actions and deployed by the production deploy workflow. The first deployed run exposed a migration gap: the backend returned `ERROR: relation "integration_connections" does not exist`; `migrations/0002_integrations.sql` was applied manually on the QA database before continuing. Persist migration execution in deploy/bootstrap before this section can be fully closed.
- E2E evidence: a fine-grained GitHub PAT with `contents:read/write` access to `tixqz/test-hehehe` was connected for regular user Bob. A Bob-owned `nanoclaw` agent with GitHub enabled started successfully, the runtime pod spec contained `GITHUB_TOKEN` without printing the value, and `README.md` in `tixqz/test-hehehe` was updated from inside the runtime process using the injected token. The README now contains the marker `Runtime integration write verified`.
- Security evidence: unauthenticated `GET /api/integrations` returned `401`; invalid PAT connect returned `400`; `GET /api/integrations`, connect response, and post-disconnect response contained no `token` or `secret` fields. The integration was disconnected after the test.
- Remaining blockers: runtime DNS lookup for `api.github.com` returned `EAI_AGAIN`, so the runtime write used a resolved GitHub API IP with TLS SNI/Host set to `api.github.com`. This proves token injection and outbound HTTPS work, but runtime DNS/egress must be fixed before normal GitHub tooling can use hostnames. UI-specific scenarios and cross-user authorization scenarios still need browser/API retest.

### Scenarios

**13.1 Auth/visibility:**
- [ ] Regular user can open Integrations page and see their own agents + bindings.
- [x] Unauthenticated request to `/api/integrations` returns `401`/`403`.
- [ ] User cannot see another user's integrations or bindings.

**13.2 Connect — invalid PAT:**
- [x] Connect GitHub with an invalid token (e.g. `ghp_invalid`).
- [x] Backend rejects with `400`/`422` and a descriptive error message.
- [ ] UI shows validation error in a toast/banner; connection remains in `disconnected` state.
- [ ] Confirm no token is persisted in the database (if decrypt-check is feasible in test setup).

**13.3 Connect — valid PAT:**
- [x] Connect GitHub with a valid fine-grained PAT (minimal scope: only `contents:read` for a test repo).
- [ ] UI shows connected state with GitHub login, account type, status, and revision metadata.
- [ ] UI never displays the token or any portion of it.
- [ ] Activity/audit log records an `integration.connected` event containing GitHub login but no token.
- [x] `GET /api/integrations` response contains provider and connection metadata but **no** `token`, `encrypted_token`, or `secret` field.

**13.4 Persistence after refresh/re-login:**
- [ ] After connecting with a valid PAT, refresh the browser — UI still shows connected metadata.
- [ ] Log out and log back in — metadata persists.
- [ ] PAT input field is empty (token is never pre-filled).
- [ ] `GET /api/integrations` does not return any field containing the raw or masked PAT value.

**13.5 Per-agent enable/disable:**
- [x] Enable GitHub integration for Agent A (user-owned, stopped).
- [ ] Agent B (same user) remains disabled.
- [ ] Summary/binding list shows enabled binding only for Agent A.
- [x] Toggle Agent A off — binding reflects disabled state.

**13.6 Runtime start with enabled integration:**
- [x] Start Agent A (enabled) — runtime pod/container reaches `READY 1/1 Running`.
- [x] Runtime env contains `GITHUB_TOKEN` (verify via controlled test image log or assertion that confirms env key presence without printing secret value; e.g. `stat -c %s /proc/1/environ` or `env | grep -q ^GITHUB_TOKEN=`).
- [ ] Agent reaches running/chat-ready state (WebSocket registered).

**13.7 Runtime start with disabled integration:**
- [ ] Start Agent B (disabled) — runtime pod/container reaches `READY 1/1 Running`.
- [ ] `GITHUB_TOKEN` is absent from container environment.
- [ ] Agent reaches running/chat-ready state.

**13.8 Token update:**
- [ ] Connect with PAT v1, then update to PAT v2 via the Integrations page.
- [ ] New runtime sessions started after the update receive PAT v2.
- [ ] An already-running agent's environment does **not** mutate (existing process env is not expected to change).

**13.9 Disconnect:**
- [x] Disconnect GitHub integration.
- [ ] Connection metadata is removed; UI returns to initial disconnected state.
- [ ] Per-agent toggles are disabled or hidden; attempting to start a previously enabled agent does **not** inject `GITHUB_TOKEN`.
- [ ] Reconnect works after disconnect (repeat 13.3).

**13.10 Authorization boundaries:**
- [ ] Bob (non-admin user) cannot toggle or modify integration for an admin-owned agent.
- [ ] Bob cannot view admin's integration status or binding detail.
- [ ] Admin's API response does not expose other users' PATs or encrypted secrets in any field.

**13.11 API secret safety:**
- [x] `GET /api/integrations` response contains no raw token, encrypted secret blob, or any masked-secret field.
- [x] Connect response (PUT) contains no token echo or secret.
- [x] Disconnect response contains no secret remnants.
- [ ] Token field is `null`/omitted or explicitly excluded in all API specs.

**13.12 Audit/activity events:**
- [ ] `integration.connected` event exists (connect) — contains provider, GitHub login, timestamp; no token.
- [ ] `integration.disconnected` event exists (disconnect) — contains provider and timestamp; no token.
- [ ] `integration.agent_enabled` and `integration.agent_disabled` events exist — contain agent ID and provider; no token.
- [ ] Events are queryable through the activity/audit API or logs.

**13.13 Network/egress (MVP note):**
- [x] MVP env injection itself does not require the runtime to reach `api.github.com` or any external endpoint — verify that a runtime with `GITHUB_TOKEN` set but no tool execution starts cleanly.
- [x] Document whether future runtime tooling (e.g. git clone) will require egress; no test failure if egress is blocked for MVP.
- [ ] Fix runtime DNS for external hostnames. Runtime HTTPS to GitHub succeeded only when using a pre-resolved GitHub API IP with TLS SNI/Host set to `api.github.com`; normal hostname lookup returned `EAI_AGAIN`.

### Test data

- **User A** (admin): `admin` / `<admin-password>`. One enabled agent `admin-agent-a` and one disabled agent `admin-agent-b`.
- **User B** (regular): `bob` / `<bob-password>`. One enabled agent `bob-agent-a` and one disabled agent `bob-agent-b`.
- **Valid PAT**: fine-grained PAT with `contents:read/write` scope on a dedicated test repository (`tixqz/test-hehehe` for this QA run). Revocable after QA.
- **Invalid token**: string `ghp_invalid_token_for_testing_purposes_only`.

> Do **not** commit real PAT values into this file or any repository file. Use environment variables or a secure vault during test execution.

### Pass criteria

All unchecked checkboxes above are checked (marked `[x]`) only after each scenario is verified **not to reproduce** or confirmed **to pass** on the target QA environment. Critical/High severity items (unauthenticated access, PAT exposure, authorization bypass, secret leak in API) must pass with zero regressions before marking this section complete.

## 14. Plugin system for integrations and MCP servers — E2E validation scenarios

- [ ] Verified fixed on target environment
- Severity: High
- Goal: validate the hot-reloading plugin system that replaced the hardcoded GitHub provider. Coverage: manifest loading from File/DB/CRD sources with precedence and hot reload; `http_template` and `sidecar` plugin kinds; env injection at agent start; MCP server contributions (external in manifest, self-hosted via `MCPServer` CRD + in-process operator); generic UI cards driven by manifest `form_fields`; scope-aware credentials in `integration_connections`; the admin `/api/admin/plugins` CRUD; and graceful behavior on orphaned credentials, malformed manifests, sidecar timeouts, and missing MCPServer references.
- Deployment status: shipped in commits `3d79981` (plugin system MVP) and `2bd8826` (legacy GitHub provider removal). Backend loads integrations from three registry sources merged with CRD > DB > File precedence. Helm chart ships `charts/shclop/templates/plugins-configmap.yaml` mounted at `/etc/shclop/plugins.d/`, the `IntegrationPlugin` and `MCPServer` CRDs in `charts/shclop/crds/`, and extended RBAC. Migrations `0004_plugin_manifests.sql` and `0005_integration_scope.sql` are applied. Existing `integration_connections` rows for `provider_id='github'` continue to resolve unchanged (`scope='user'`, `scope_id=user_id` backfilled).
- Required action: run the scenarios below on the target environment after the next deploy. Critical/High items must pass with zero regressions before this section is closed.

### Scenarios

**14.1 File registry — initial scan and hot reload:**
- [ ] Helm install renders ConfigMap `shclop-plugins-seed` with `github.yaml` mounted at `/etc/shclop/plugins.d/github.yaml` inside the backend pod (verify with `kubectl exec ... -- ls /etc/shclop/plugins.d/`).
- [ ] `GET /api/integrations` returns a `github` provider with `auth_kind=pat_token`, `form_fields`, and `description` matching the manifest contents.
- [ ] `kubectl exec` into the backend pod and write a second manifest `gitlab.yaml` to `/etc/shclop/plugins.d/` — within ≤2s the new provider appears in `GET /api/integrations` with no backend restart.
- [ ] Overwrite `gitlab.yaml` with malformed YAML — backend logs a WARN, prior valid manifest is retained, provider is **not** dropped.
- [ ] Remove `gitlab.yaml` — provider disappears from `/api/integrations` within ≤2s.

**14.2 DB registry — admin CRUD via `/api/admin/plugins`:**
- [ ] Admin `POST /api/admin/plugins` with a new plugin manifest YAML returns 200 with the upserted row and revision = 1.
- [ ] Within ≤12s (10s poll + grace), `GET /api/integrations` reflects the new provider.
- [ ] Admin `PUT /api/admin/plugins/{id}` with modified YAML bumps `revision` and within ≤12s the new content reflects in the list.
- [ ] Admin `DELETE /api/admin/plugins/{id}` removes it within ≤12s.
- [ ] Regular user Bob receives 403 on every method of `/api/admin/plugins`.
- [ ] Malformed YAML in POST/PUT body returns 400 with parse error before any DB write.
- [ ] Manifest whose `metadata.name` or `spec.id` mismatches the URL path id returns 400.

**14.3 CRD registry — Kubernetes `IntegrationPlugin`:**
- [ ] `kubectl apply -f` an `IntegrationPlugin` cluster-scoped CR — within ~3s the provider appears in `GET /api/integrations`.
- [ ] `kubectl edit integrationplugin <name>` updates the spec — change reflects within ~3s.
- [ ] `kubectl delete integrationplugin <name>` removes the provider.
- [ ] Apply a CR with invalid `spec.kind: "bogus"` — backend logs WARN, provider does not appear (server-side `Manifest.Validate` rejects).

**14.4 Source precedence (CRD > DB > File):**
- [ ] Define the same plugin id (e.g. `github`) across all three sources with distinct `display_name` values.
- [ ] `GET /api/integrations` returns the CRD version. Backend logs `plugin shadowed` WARN naming both shadowed sources.
- [ ] `kubectl delete integrationplugin github` → `GET /api/integrations` falls back to the DB version.
- [ ] Admin `DELETE /api/admin/plugins/github` → falls back to the File version.

**14.5 `http_template` kind — validate + env injection:**
- [ ] Connect GitHub with a valid PAT — backend performs `GET https://api.github.com/user` with `Authorization: Bearer <token>`, parses `id`/`login`/`type`, stores connection with extracted metadata.
- [ ] Connect with invalid PAT — backend returns 400 mentioning HTTP status `401`; no row written to `integration_connections`.
- [ ] Start an agent with GitHub enabled — runtime pod env contains `GITHUB_TOKEN` matching the stored PAT (verify via `env | grep -q ^GITHUB_TOKEN=` from inside the runtime; **do not print the value**).
- [ ] Manifest with a templated header (e.g. `Authorization: Bearer {{.Token}}`) renders correctly at connect time.

**14.6 `sidecar` kind — backend-to-plugin HTTP:**
- [ ] Deploy a stub sidecar Deployment+Service in the cluster that returns the documented JSON contract for `POST /validate` and `POST /runtime-env`. Define an `IntegrationPlugin` of `kind: sidecar` pointing at the service URL.
- [ ] Connect with a token — backend POSTs `{fields, context}` to `/validate`, sidecar returns `{external_account_id, external_login, account_type, status: "connected"}`; connection stored.
- [ ] Sidecar returns `status: "error"` — backend returns 400, connection not stored.
- [ ] Sidecar URL points to a closed port — connect returns 400 with `"unreachable"` in the message.
- [ ] Sidecar `/validate` exceeds the manifest `timeout` — connect returns 400 with deadline-exceeded.
- [ ] Start agent — backend POSTs `/runtime-env`, merges returned env into the pod spec; `Contributions.Env` declared in the manifest is **ignored** for sidecar kind (sidecar is the source of truth).

**14.7 MCP — external server in plugin manifest:**
- [ ] Define a plugin with `contributions.mcp_servers: [{name: x, external: {url: "https://...", auth_header: "Bearer {{.Token}}"}}]`.
- [ ] Connect token, enable on an agent, start agent — runtime env contains `SHCLOP_MCP_CONFIG` JSON with one entry matching `name=x`, URL rendered, and `Authorization` header rendered with the token.
- [ ] No `MCPServer` CR is created; nothing extra runs in the cluster.
- [ ] Token value never appears in plain text in any backend log line that includes the env blob.

**14.8 MCP — self-hosted via `MCPServer` CRD + in-process operator:**
- [ ] `kubectl apply -f` a namespaced `MCPServer` CR — within ~5s the controller creates a Deployment, Service, and NetworkPolicy labeled `shclop.io/managed-by=mcpserver-controller`. Status `serviceURL` populated.
- [ ] Apply a plugin manifest with `mcp_servers: [{name: x, ref: <MCPServer name>}]`. Connect token. Enable on agent.
- [ ] Start agent — `SHCLOP_MCP_CONFIG` contains the cluster service URL `http://mcp-<name>.<ns>.svc.cluster.local:<port>`.
- [ ] `kubectl delete mcpserver <name>` — within ~5s the Deployment, Service, and NetworkPolicy are removed.
- [ ] Plugin references a non-existent `MCPServer` ref — agent start fails with error containing `"mcpserver_not_found"`; agent state set to `idle`; activity log records the failure.
- [ ] Re-applying the same `MCPServer` is idempotent (no errors, no duplicate resources).
- [ ] NetworkPolicy created by the controller restricts ingress to pods labeled `shclop.io/role=agent-runtime`.

**14.9 Generic UI — `form_fields` rendering:**
- [ ] Integrations page renders one card per provider returned by `/api/integrations`.
- [ ] Each card renders form inputs from `form_fields[]`: `secret: true` → `<input type="password">`; `placeholder` and `help_url` honored.
- [ ] Submitting the form calls `PUT /api/integrations/{id}/connection` with body shape `{"fields": {...}}`.
- [ ] Provider with `mcp_servers[]` non-empty shows them as a small list ("MCP tools: …") in the connected-state card.
- [ ] Provider with multiple `form_fields` (more than just `token`) renders all of them and submits them all.

**14.10 Generic UI — orphan provider cards:**
- [ ] After admin `DELETE` of a plugin whose connection Bob still holds, the Integrations page renders a banner "Provider is no longer registered." for that provider.
- [ ] The orphan card exposes only the Disconnect button (no connect form, no agent-binding toggle).
- [ ] Disconnect removes the connection from `integration_connections`; the orphan card disappears on next refresh.
- [ ] Attempting to start an agent with an orphaned enabled integration returns 400 with `"plugin unregistered: <id>"`.

**14.11 Connect API shape — generic + legacy fallback:**
- [ ] `PUT /api/integrations/{id}/connection` with body `{"fields": {"token": "..."}}` succeeds for `http_template` plugins.
- [ ] Legacy body `{"token": "..."}` still succeeds (backward compatibility for older UIs/clients) — internally mapped to `fields.token`.
- [ ] Empty `fields` map for a plugin requiring `token` returns 400.
- [ ] Unknown provider id returns 400 with `"not registered"`.

**14.12 Scope-aware credentials migration:**
- [ ] After migration `0005_integration_scope.sql`, every row in `integration_connections` has `scope='user'` and `scope_id=user_id`. Primary key is `(provider_id, scope, scope_id)`.
- [ ] Bob's pre-migration GitHub connection still resolves via `GET /api/integrations` with `connected: true`.
- [ ] Bob can still start an agent with GitHub enabled, and `GITHUB_TOKEN` is injected with the original encrypted value (decrypted at runtime).
- [ ] No backfill or manual intervention required after deploy. Migration is idempotent on re-run.

**14.13 Legacy GitHub provider removal:**
- [ ] After commit `2bd8826`, the deployed image no longer contains `internal/integrations/github.go`. GitHub flow works exclusively through the `github.yaml` manifest from the seed ConfigMap.
- [ ] Existing GitHub connections continue to work without re-connection.
- [ ] Removing the `github.yaml` entry from the ConfigMap (rerendered Helm install with seed disabled) immediately drops the GitHub provider from `/api/integrations` — confirms no hardcoded fallback remains.

**14.14 Failure modes — no crash, clear errors, no secret leaks:**
- [ ] Plugin manifest references a deleted `MCPServer` — agent start returns 400 with `"mcpserver_not_found"`; agent state set to `idle`; activity log records the cause without leaking token.
- [ ] User connection exists for a plugin deleted from all sources — appears as orphan in UI; agent start with that integration enabled returns 400 with `"plugin unregistered: <id>"`.
- [ ] Sidecar plugin times out at runtime env build — agent start fails with a deadline error; agent state `idle`; no half-started pod.
- [ ] Backend loses connection to kube-apiserver — informers reconnect with backoff; File and DB registries continue serving cached state; CRD source resyncs after reconnect.
- [ ] Encrypted secret never appears in any log line, API response, or activity event.

**14.15 Helm chart rendering:**
- [ ] `helm template charts/shclop` renders the seed ConfigMap with literal `{{.Token}}` in `github.yaml` data (Helm-templating escape preserved — Go template syntax not expanded by Helm).
- [ ] Both CRDs render under `charts/shclop/crds/` and Helm auto-applies them on install.
- [ ] Deployment spec mounts the ConfigMap at `/etc/shclop/plugins.d/` and sets `SHCLOP_PLUGIN_DIR` env to the same path.
- [ ] RBAC includes a ClusterRole rule for `integrationplugins.shclop.io` (`get,list,watch`) and a namespaced Role for `mcpservers.shclop.io` plus `deployments.apps`, `services`, `networkpolicies.networking.k8s.io` (CRUD verbs).
- [ ] `helm lint charts/shclop` passes with zero failures.

**14.16 Hot reload latency:**
- [ ] File reload from `kubectl exec` write to UI visibility ≤2s.
- [ ] DB reload from admin POST to UI visibility ≤12s (10s poll + grace).
- [ ] CRD reload from `kubectl apply` to UI visibility ≤3s (informer dispatch).
- [ ] Backend logs include a `source=file|db|crd` tag on every plugin lifecycle event.
- [ ] Conflict shadowing is logged once per change (deduped), not on every poll/reconcile.

**14.17 Audit/activity events:**
- [ ] `integration.connected` and `integration.disconnected` events recorded for the generic connect/disconnect API paths (mirrors §13.12, now applies to all plugin ids, not only `github`).
- [ ] `integration.agent_enabled` and `integration.agent_disabled` events recorded per provider id.
- [ ] When agent start fails due to a missing or malformed plugin, the activity log records the cause with provider id; no token leaks into the event.

**14.18 Authorization on plugin operations:**
- [ ] Bob cannot create, update, or delete entries in `/api/admin/plugins` (403 on every method).
- [ ] Bob cannot view or modify another user's `integration_connections` rows via any API.
- [ ] Bob cannot enable an integration on an agent he does not own.
- [ ] Admin's `/api/integrations` response does not expose other users' encrypted secrets in any field.

### Test data

- **File-source manifest**: shipped `github.yaml` from `charts/shclop/templates/plugins-configmap.yaml`.
- **DB-source manifest**: copy `github.yaml`, change `metadata.name` and `spec.id` to `gitlab`, point `validate.http.url` at `https://gitlab.com/api/v4/user`. POST via `/api/admin/plugins`.
- **CRD-source manifest**: same content as DB-source but wrapped in the `IntegrationPlugin` CR envelope and applied via `kubectl apply -f`.
- **Sidecar test service**: any HTTP server returning the documented `POST /validate` and `POST /runtime-env` JSON contract on a known cluster service URL. A minimal Python or Go stub is sufficient.
- **`MCPServer` CR**: a small CR pointing at any HTTP MCP container image (real or stub) for the operator-creation flow.
- **Users**: same `admin` and `bob` from §13. Add a third user `carol` to validate cross-user authorization on plugin operations.
- **Test PAT**: revocable fine-grained GitHub PAT with `contents:read` access to `tixqz/test-hehehe`.

> Do **not** commit real PATs, sidecar API keys, or any test credentials into this file or any repository file. Use environment variables or a secure vault during test execution.

### Pass criteria

All checkboxes in §14 are marked `[x]` only after each scenario is verified on the target QA environment. Critical/High items that must pass before this section is closed:
- **14.1** (file source baseline — plugin loading + hot reload),
- **14.5** (PAT validate + env injection — replaces the legacy GitHub path),
- **14.8** (MCP operator lifecycle — Deployment+Service+NetworkPolicy reconcile and cleanup),
- **14.12** (scope-aware migration must not lose existing credentials),
- **14.13** (legacy GitHub removal parity — existing connections continue to work),
- **14.14** (failure modes do not leak secrets, do not crash backend, surface clear errors),
- **14.18** (authorization boundaries hold across admin/regular-user plugin operations).

## 15. OIDC SSO — E2E validation scenarios

Feature delivered in commit `4aab819` ("feat: OIDC SSO with admin-toggleable auth modes"). Adds OpenID Connect single sign-on alongside the existing local password login. Providers are configured via Helm env vars at deploy time; admin can switch the global mode (`local` / `sso` / `both`) and per-provider `enabled` flags at runtime via `/api/admin/auth-settings`. First successful SSO sign-in JIT-creates a local user with `role=user`.

All scenarios below should be exercised end-to-end on the QA cluster against at least one real external IdP (Keycloak or Dex preferred for self-hosted; Auth0 dev tenant acceptable). Cookies must be inspected in the browser dev-tools network panel where indicated.

### 15.1 Local-only mode is the default after fresh install (Critical)

- [ ] Fresh `helm install` with `auth.idp.providers: []`. Open the login page.
- [ ] Verify `GET /api/auth/providers` returns `{"mode":"local","providers":[]}`.
- [ ] Verify the UI shows the username/password form and no `Sign in with X` buttons.
- [ ] Local login as bootstrap admin succeeds and lands on the agents page.

### 15.2 IdP provider is materialized from Helm env vars at startup (Critical)

- [ ] `helm upgrade` with one configured provider (`name=keycloak`, valid `issuer`, `clientID`, `clientSecret` Secret, `redirectURI`). `helm upgrade` includes `auth.cookieKey.existingSecret`.
- [ ] Pod restarts. Inspect pod logs for `idp registry materialized` (or equivalent) without errors.
- [ ] `GET /api/auth/providers` returns the provider with `status: "ready"`.
- [ ] The login page renders the `Sign in with Keycloak` anchor.

### 15.3 Degraded provider — unreachable issuer (High)

- [ ] Configure a provider with an issuer URL pointing at a 404/timeout host. `helm upgrade`.
- [ ] `GET /api/auth/providers` returns `status: "degraded"` with a non-empty `error` field.
- [ ] Login UI renders the button as disabled with the error in a tooltip; the local form (if visible per mode) still works.
- [ ] Restore the issuer. Within ~60 seconds the background retry transitions the provider to `ready` without pod restart. Verify `/api/auth/providers` reflects the new state.

### 15.4 OIDC authorize-redirect flow sets a one-shot state cookie (Critical)

- [ ] Click `Sign in with Keycloak`. Browser is redirected to the IdP `authorize_endpoint`.
- [ ] In dev-tools, verify the request to `/api/auth/oidc/keycloak/login` set a cookie `shclop_oidc_state` with `HttpOnly`, `Secure` (when not in dev), `SameSite=Lax`, `Path=/api/auth/oidc/`, `Max-Age=600`.
- [ ] The 302 `Location` header contains `state=...`, `code_challenge=...`, `code_challenge_method=S256`, and a `nonce` parameter.

### 15.5 Happy-path callback JIT-creates a local user (Critical)

- [ ] On a clean shclop instance, complete the IdP login as a user whose email is not in shclop yet.
- [ ] The callback round trip ends with a 302 to `/` and a `shclop_session` cookie.
- [ ] `GET /api/me` returns the new user (username = IdP email, role = `user`, not disabled).
- [ ] In the admin Users tab, the user appears with one linked identity entry (provider name, subject, email, display name, `last_login_at` set).
- [ ] The `shclop_oidc_state` cookie is gone (one-shot — handler deletes it on callback).

### 15.6 Repeat login of an existing SSO user does not create a duplicate (High)

- [ ] Same user signs out, signs back in via the SSO button.
- [ ] No new row in users; the existing identity's `last_login_at` is updated.
- [ ] `ListUsers` count unchanged in admin overview.

### 15.7 Email collision with existing local user is rejected (Critical)

- [ ] Admin creates a local user `alice@example.com` (with any password) via the admin UI.
- [ ] An IdP account with `email=alice@example.com` attempts SSO. Backend returns HTTP 409 with body containing `local_user_with_same_email_exists`.
- [ ] No new user is created. No new identity is linked. The user sees an error message.
- [ ] Admin manually links the identity via the Auth admin tab (or via `POST /api/admin/users/{id}/identities` directly). The same SSO attempt then succeeds, and `GET /api/me` returns the pre-existing local user.

### 15.8 `auth.mode=local` blocks the SSO endpoints (High)

- [ ] Admin sets mode to `local` via `PATCH /api/admin/auth-settings`.
- [ ] `GET /api/auth/oidc/keycloak/login` returns 404 `sso_disabled` (or 404 with the appropriate body — exact code per server.go).
- [ ] The login screen hides all SSO buttons.

### 15.9 `auth.mode=sso` blocks local login for non-bootstrap users (Critical)

- [ ] Admin sets mode to `sso`.
- [ ] Attempt `POST /api/auth/login` with credentials for any non-bootstrap user → 403 with body `local_login_disabled`.
- [ ] Same call with `username == SHCLOP_BOOTSTRAP_ADMIN_USERNAME` → 200 (break-glass).
- [ ] The login screen hides the local form by default. Visiting `/?break_glass=1` shows the local form again (for bootstrap admin recovery scenarios).

### 15.10 `auth.mode=both` keeps both paths active (High)

- [ ] Admin sets mode to `both`.
- [ ] Local user signs in via the username/password form — success.
- [ ] Different user signs in via `Sign in with X` — success.
- [ ] Both sessions issue the same kind of session token; both can access `/api/me`, agents, etc.

### 15.11 Per-provider enable flag (Medium)

- [ ] Admin disables a specific provider via the Auth admin tab.
- [ ] `GET /api/auth/providers` returns that provider with `enabled: false` (status may still be `ready`).
- [ ] The login screen does not render a `Sign in with X` button for the disabled provider.
- [ ] `GET /api/auth/oidc/<disabled-name>/login` returns 404 `idp_not_available`.
- [ ] Re-enable. The button reappears and login resumes.

### 15.12 State-tampering attack surface (Critical)

- [ ] Initiate `/oidc/login`, capture the IdP redirect URL.
- [ ] Manually call `/oidc/callback?code=anything&state=<wrong-value>` while the original `shclop_oidc_state` cookie is still set → 400 `state_mismatch`. No user created.
- [ ] Repeat without the cookie at all → 400 `state_cookie_missing`.
- [ ] Repeat with the cookie but the `state` query param value differs by one character → 400 `state_mismatch`.

### 15.13 Nonce-mismatch is rejected (High)

- [ ] Mint an ID token whose `nonce` claim differs from the cookie's nonce (or sign one with a manipulated nonce via an interceptor) → callback returns 401 `nonce_mismatch`. No user created.

### 15.14 Expired ID token is rejected (High)

- [ ] Have the IdP issue an ID token with `exp` already in the past (some IdPs allow this for tests; otherwise wait it out) → 401 `id_token_invalid`. No user created.

### 15.15 JWKS rotation is transparent (Medium)

- [ ] Force a key rotation on the IdP between two consecutive SSO logins (e.g., Keycloak realm key rotation).
- [ ] Second login still succeeds — `*oidc.Provider` fetches the updated JWKS on the next signature verification without any shclop restart.

### 15.16 Logout invalidates the session (Critical)

- [ ] Successful local OR SSO login. Capture the `shclop_session` cookie / Bearer token.
- [ ] `POST /api/auth/logout` returns 204; the `shclop_session` cookie is cleared.
- [ ] Any subsequent API call with the same token returns 401.
- [ ] Browser UI returns to the login screen and the localStorage token is gone.

### 15.17 Disabled user cannot complete SSO login (High)

- [ ] Admin disables an existing user (linked to an SSO identity).
- [ ] That user attempts SSO again → callback returns 403 `user_disabled`. No session issued.
- [ ] Re-enable. SSO works again.

### 15.18 Identity link / unlink workflow (High)

- [ ] Admin opens the Users tab, expands a user, sees the identities table.
- [ ] Click `Link new identity` (or POST `/api/admin/users/{id}/identities`) with `provider_name`, `subject`, `email`, `display_name` from the IdP. Returns 201.
- [ ] The user (or an admin acting on their behalf) signs in via SSO with that subject — returns the linked existing user, not a new JIT-create.
- [ ] Admin unlinks the identity. SSO attempt for the same subject now JIT-creates a fresh user (assuming no email collision).

### 15.19 Cookie key rotation (Medium)

- [ ] Rotate the secret backing `SHCLOP_AUTH_COOKIE_KEY` (regenerate, update the Secret, `kubectl rollout restart`).
- [ ] Any in-flight `/login` -> `/callback` that crosses the restart fails with 400 (`state_invalid`) because the old-key cookie cannot be decrypted. User restarts the flow and succeeds.
- [ ] Completed sessions in `shclop_session` are unaffected (different cookie, different lifecycle).

### 15.20 Multi-replica behavior — known constraint (Medium)

- [ ] Scale `replicas: 2`. Login via either path on pod A; the issued `shclop_session` token is in-memory on pod A only.
- [ ] If a follow-up request is routed to pod B (no sticky sessions), it returns 401. **This is the documented current state, not a regression.** Verify the documented workaround (sticky sessions on the Ingress or `replicas: 1` for SSO-active deployments) is mentioned in ADMIN_GUIDE.

### 15.21 Helm rendering safety (Medium)

- [ ] `helm template` with `auth.idp.providers: [...]` non-empty AND `auth.cookieKey.existingSecret.name: ""` → templating **fails** with a clear error (server-side guard in deployment.yaml).
- [ ] `helm template` with `auth.idp.providers: []` → no `SHCLOP_IDP_PROVIDER_*` env vars rendered, no `SHCLOP_AUTH_COOKIE_KEY` env var.
- [ ] `helm template` with one provider → all required env vars rendered, `CLIENT_SECRET` uses `valueFrom.secretKeyRef`, never inline plaintext.

### 15.22 Audit trail (Medium)

- [ ] Each successful SSO login emits an `auth.login.oidc` activity entry with provider name in the metadata. Visible in the admin Activity tab.
- [ ] Failed callbacks (state mismatch, nonce mismatch, id_token_invalid) do not silently no-op — there is enough information in stdout JSON logs to diagnose. No raw tokens or secrets are logged.

### 15.23 Authorization boundaries on admin endpoints (Critical)

- [ ] As a regular `user` (Bob from §13), call `GET /api/admin/auth-settings` → 403.
- [ ] Same with `PATCH /api/admin/auth-settings`, `POST /api/admin/users/.../identities`, `DELETE /.../identities/.../...` → 403 each.
- [ ] As admin → 200 / 201 / 204 as appropriate.

### Test data

- **IdP**: at least one real provider, ideally two (Keycloak + Auth0, or Keycloak + Dex). Use a non-production realm/tenant.
- **OAuth client**: registered with redirect URI `https://<qa-host>/api/auth/oidc/<name>/callback`. Confidential client, client secret stored in a Kubernetes Secret referenced by `auth.idp.providers[*].clientSecret.existingSecret`.
- **Cookie key**: `openssl rand -base64 32` placed in a Kubernetes Secret, referenced by `auth.cookieKey.existingSecret`.
- **Users**: bootstrap admin (`admin`), an existing local user with a known email (`alice@example.com`) for the collision case, a regular user `bob`, and IdP-side accounts whose emails are NOT yet in shclop for the happy-path JIT cases.

> Do **not** commit real OAuth client secrets, cookie keys, or IdP test credentials into this file or any repository file. Use environment variables or a secure vault.

### Pass criteria

All checkboxes in §15 are marked `[x]` only after each scenario is verified on the target QA environment. Critical/High items that must pass before this section is closed:
- **15.1** (default local mode unchanged),
- **15.2** (Helm provider materialization),
- **15.4** (state cookie shape — security-critical),
- **15.5** (happy-path JIT-create),
- **15.7** (email collision rejected — security-critical),
- **15.9** (sso mode + break-glass admin work as specified),
- **15.12** (state-tampering protection — security-critical),
- **15.16** (logout actually invalidates session — security-critical),
- **15.23** (authorization boundaries — security-critical).
