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
