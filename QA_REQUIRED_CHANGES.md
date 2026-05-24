# QA Required Changes

Target tested: `http://shclop.137.184.55.182.nip.io` via local tunnel to the deployed backend.

Status markers are intentionally conservative: an item is checked only after the original issue is verified not to reproduce in the target environment.

## 1. Bootstrap installs no working Kata runtime

- [ ] Verified fixed on target environment
- Severity: Critical
- Evidence: `scripts/bootstrap.sh install --install-deps` could not install `kata-runtime`; the configured openSUSE repository returned `404 Not Found` for Ubuntu 24.04.
- Runtime evidence: after starting an agent, Kubernetes reported `FailedCreatePodSandBox: no runtime for "kata" is configured`.
- Required change: update the Kata installation path in `scripts/bootstrap.sh` to a supported current method, and make the installer fail before Helm deployment if the Kata runtime is not actually configured in K3s containerd.
- Implementation status: code now installs Kata from the official GitHub static tarball, configures K3s containerd for the Kata shim, runs a `runtimeClassName: kata` smoke pod before Helm deployment, and fails before deployment if the smoke test fails. This still needs a fresh install/upgrade run on the target host before the checkbox can be marked.

## 2. Agent UI reports Running while Kubernetes runtime pod is not running

- [ ] Verified fixed on target environment
- Severity: High
- Evidence: the UI showed `qa-agent ... Running` and a `Stop` button, while `kubectl get pods` showed the runtime pod in `ContainerCreating/Pending` with repeated sandbox creation failures.
- Screenshots:
  - `dogfood-output/screenshots/agent-start-after-gateway.png`
  - `dogfood-output/screenshots/agent-still-running-while-pending.png`
- Required change: only transition an agent to `running` after the runtime pod reaches a usable Ready state or after the runtime WebSocket handshake succeeds. Surface pod creation failures to the user instead of showing a generic or stale running state.
- Implementation status: backend Kubernetes runtime startup now waits for the pod to be `Running` with all containers `Ready` before returning success. Timeout and failed-pod errors include warning Events and container waiting reasons. This still needs a fresh UI/runtime E2E run on the target host before the checkbox can be marked.

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
