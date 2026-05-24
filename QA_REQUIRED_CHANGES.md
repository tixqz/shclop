# QA Required Changes

Target tested: `http://shclop.137.184.55.182.nip.io` via local tunnel to the deployed backend.

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
- Retest status: a real OpenRouter key and the free model `deepseek/deepseek-v4-flash:free` were configured manually through Kubernetes Secret and admin API. Bootstrap still does not provision or document this path.

## 5. Runtime does not call the configured external LLM provider

- [ ] Verified fixed on target environment
- Severity: High
- Evidence: with OpenRouter configured and a runtime pod registered as `llm_gateway_configured=true` for model `deepseek/deepseek-v4-flash:free`, the minimal chat response was the local demo response: `openclaw runtime received: Reply with exactly: hello world`, followed by workspace/memory text. That indicates the runtime used the demo adapter instead of calling the configured OpenRouter model.
- Required change: implement the runtime-side LLM adapter path for the configured gateway, base URL, API key, and model; then add an E2E check that proves the response comes from the external provider or an explicit mock provider.

## 6. Persisted agent state can remain `running` after runtime pod crash or backend restart

- [ ] Verified fixed on target environment
- Severity: High
- Evidence: after the adapter-token bug was fixed and the backend redeployed, the UI still showed the older `qa-deepseek-minimal` agent as `running`, while its runtime pod was `CrashLoopBackOff`/completed and chat returned `runtime not connected`.
- Required change: reconcile persisted agent state with runtime registry and Kubernetes pod state after backend restart and after runtime disconnects, or show a distinct disconnected/stale status in the UI.

## 7. Observability bootstrap used an outdated VictoriaLogs chart/service name

- [x] Verified fixed on target environment
- Severity: Medium
- Evidence: `victoria-metrics-k8s-stack` installed, but there was no `victoria-logs` release until the chart was corrected. The current VictoriaLogs chart/service is `vm/victoria-logs-single` with service `victoria-logs-victoria-logs-single-server:9428`.
- Verification status: fixed on the target host. VictoriaMetrics stack pods are running, VictoriaLogs single pod is running, Grafana ingress returns HTTP 200, the backend `/metrics` endpoint is reachable in-cluster, and VictoriaLogs `/health` returns HTTP 200.
