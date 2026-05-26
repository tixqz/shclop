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
- Evidence: Bob (`bob` / `Un6J6N0cKTgr24QU`) can log in, cannot access `/api/admin/users` (`forbidden`), cannot see admin-owned agents, can create `nanoclaw` agents, and can start them. Runtime pods for Bob's agents reach `READY 1/1 Running` and register with `llm_gateway_configured=true`.
- UI evidence: Bob logged in through the browser UI, created `bob-ui-test`, started it, and sent `Reply with exactly: bob ui ok`. The UI showed the user message and then displayed the runtime error `lookup litellm on 10.43.0.10:53: read: connection refused` instead of an assistant response.
- Runtime/backend evidence: backend logged `task.routed` for Bob's agent `2b387776c084c26b1e04086a2534cd1c`; runtime logged `task received` for the same message.
- Required change: make the Kata runtime networking/DNS path able to resolve and reach the LiteLLM service, or pass a reachable gateway URL to runtimes. Add an E2E test that runs from inside a Kata runtime pod and verifies DNS plus HTTP reachability of the configured LLM gateway before accepting the agent as chat-ready.

## 9. Bob UI chat connection state is misleading for already-running agents

- [ ] Verified fixed on target environment
- Severity: Medium
- Evidence: after Bob logs in with already-running agents, the UI often shows the agent as `Running` with a `Stop` button, but the chat input says `Connect to start chatting…`. The button is disabled until text is typed, so the UI looks disconnected/stuck even though entering a message actually opens the WebSocket and sends the message.
- Required change: either auto-connect chat for running agents, or change the empty state and button behavior so it is clear that typing a first message will connect and send. If auto-connect is expected, reconnect reliably after page reload/session restore.

## 10. Bob cannot list available models

- [ ] Verified fixed on target environment
- Severity: Medium
- Evidence: Bob can create agents only by typing a free-form model string in the UI. `GET /api/models` returned `404` for Bob, while model administration is admin-only.
- Required change: expose a read-only model list to regular users, or make the UI clearly document/validate accepted model strings for non-admin users.
- Implementation status: backend now exposes `GET /api/models` for authenticated users. Without gateway discovery config it returns enabled store models for development; when LiteLLM gateway discovery is configured, it calls LiteLLM `/v1/models` with `SHCLOP_LLM_GATEWAY_API_KEY` and returns only enabled store models whose `provider_model` appears in the gateway model IDs. `/api/admin/models` remains admin-only. The Helm chart injects `SHCLOP_LLM_GATEWAY_API_KEY` from the configured gateway Secret. The SPA now loads `/api/models` for all logged-in users and uses a model dropdown in the agent creation form. Backend tests cover regular-user listing, disabled-model filtering, gateway intersection filtering, gateway failure as 502, unauthenticated requests, disallowed mutation methods, and continued admin-route denial for regular users. This still needs deployment through GitHub Actions and a fresh Bob UI retest before the checkbox can be marked.

## 11. Grafana datasource/dashboard fixes are not persisted in bootstrap

- [ ] Verified fixed on target environment
- Severity: Medium
- Evidence: Grafana initially had no dashboards and its Prometheus datasource pointed to `http://prometheus-server.monitoring.svc.cluster.local`, which failed DNS lookup. The actual service is `prometheus-server-server`.
- Manual QA fix: patched the remote Grafana ConfigMap to use `http://prometheus-server-server.monitoring.svc.cluster.local`, restarted Grafana, reset the admin password to `CiHuGao7eVT3bpo6`, and added dashboard `Shclop QA Overview` (`/d/shclop-qa-overview/shclop-qa-overview`) with backend up, container memory, container CPU, and recent Kubernetes logs panels.
- Verification status: Grafana API search returns dashboard UID `shclop-qa-overview`; the dashboard opens in the Grafana UI; Prometheus query `up{service="shclop-backend"}` returns `1`; Loki query `{job="fluentbit"}` returns recent logs. The checkbox remains open because datasource URL, dashboard provisioning, and the non-default Grafana password are remote-only changes and are not persisted in bootstrap/Helm values.
- Persistence fix applied: `scripts/bootstrap.sh` `install_grafana` now:
  - Uses the correct Prometheus URL `http://${PROMETHEUS_SERVER_SERVICE}.<ns>.svc.cluster.local` (defaults to `prometheus-server-server`).
  - Sets the Grafana admin password to `CiHuGao7eVT3bpo6` by default (configurable via `GRAFANA_ADMIN_PASSWORD` env var).
  - Enables the Grafana chart sidecar dashboards provider and provisions the `Shclop QA Overview` dashboard (UID `shclop-qa-overview`) with four panels: Backend Status (stat), Container Memory (timeseries), Container CPU (timeseries), and Recent Shclop Logs (Loki log panel). All changes are idempotent — re-running bootstrap upgrades Grafana in-place without manual ConfigMap patching.

## 12. Loki logs are not labeled by namespace for ergonomic Grafana queries

- [ ] Verified fixed on target environment
- Severity: Low
- Evidence: Grafana Loki query `{namespace="default"}` returned no log lines. Query `{job="fluentbit"}` returned recent Kubernetes logs because Fluent Bit currently sets `job=fluentbit` and embeds Kubernetes metadata in the log body instead of exposing `namespace` as a top-level Loki label.
- Required change: configure Fluent Bit/Loki labels so common Kubernetes filters such as namespace, pod, and container work directly in Grafana dashboards and Explore.
