import { useEffect, useMemo, useRef, useState } from 'react';
import {
  type Agent,
  type AdminOverview,
  type ChatEvent,
  type IntegrationProvider,
  type LLMGatewaySettings,
  type LLMModel,
  type User,
  getStoredToken,
  clearToken,
  login,
  getMe,
  listAgents,
  createAgent,
  startAgent,
  stopAgent,
  listIntegrations,
  connectGitHub,
  disconnectGitHub,
  setAgentGitHubIntegration,
  listModels,
  adminListUsers,
  adminCreateUser,
  adminPatchUser,
  adminListModels,
  adminCreateModel,
  adminPatchModel,
  adminGetGateway,
  adminPatchGateway,
  adminGetOverview,
} from './api';

type Page = 'agents' | 'integrations' | 'admin';

type AdminTab = 'overview' | 'users' | 'models' | 'gateway';

/* ── helpers ── */

function timeAgo(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.floor(hrs / 24);
  return `${days}d ago`;
}

/* ── Main App ── */

export default function App() {
  const [token, setToken] = useState(getStoredToken);
  const [user, setUser] = useState<User | null>(null);
  const [page, setPage] = useState<Page>('agents');

  // login form
  const [loginUsername, setLoginUsername] = useState('');
  const [loginPassword, setLoginPassword] = useState('');
  const [loginError, setLoginError] = useState('');
  const [loggingIn, setLoggingIn] = useState(false);

  // agents
  const [agents, setAgents] = useState<Agent[]>([]);
  const [selectedAgentId, setSelectedAgentId] = useState('');
  const [createName, setCreateName] = useState('');
  const [createRuntime, setCreateRuntime] = useState<'openclaw' | 'nanoclaw'>('openclaw');
  const [createModel, setCreateModel] = useState('');
  const [creating, setCreating] = useState(false);
  const [availableModels, setAvailableModels] = useState<LLMModel[]>([]);
  const [modelsLoading, setModelsLoading] = useState(false);
  const [modelsError, setModelsError] = useState('');
  const [actionLoading, setActionLoading] = useState('');

  // integrations
  const [integrationProviders, setIntegrationProviders] = useState<IntegrationProvider[]>([]);
  const [integrationsLoading, setIntegrationsLoading] = useState(false);
  const [integrationToken, setIntegrationToken] = useState('');
  const [integrationSaving, setIntegrationSaving] = useState(false);
  const [integrationAction, setIntegrationAction] = useState('');

  // chat
  const [chatText, setChatText] = useState('');
  const [chatMessages, setChatMessages] = useState<ChatEvent[]>([]);
  const [chatStatus, setChatStatus] = useState<'idle' | 'connecting' | 'connected' | 'error'>('idle');
  const [chatError, setChatError] = useState('');
  const wsRef = useRef<WebSocket | null>(null);
  const chatEndRef = useRef<HTMLDivElement>(null);
  const disconnectRef = useRef<(() => void) | null>(null);
  const chatSessionIdRef = useRef<string>(crypto.randomUUID());

  // admin
  const [adminTab, setAdminTab] = useState<AdminTab>('overview');
  const [adminOverview, setAdminOverview] = useState<AdminOverview | null>(null);
  const [adminUsers, setAdminUsers] = useState<User[]>([]);
  const [adminModels, setAdminModels] = useState<LLMModel[]>([]);
  const [adminGateway, setAdminGateway] = useState<LLMGatewaySettings | null>(null);

  // admin new user form
  const [newUsername, setNewUsername] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [newRole, setNewRole] = useState<'user' | 'admin'>('user');
  const [creatingUser, setCreatingUser] = useState(false);

  // admin new model form
  const [newModelName, setNewModelName] = useState('');
  const [newModelProvider, setNewModelProvider] = useState('');
  const [newModelEnabled, setNewModelEnabled] = useState(true);
  const [creatingModel, setCreatingModel] = useState(false);

  // admin gateway form
  const [gatewayFormEnabled, setGatewayFormEnabled] = useState(false);
  const [gatewayFormBaseUrl, setGatewayFormBaseUrl] = useState('');
  const [gatewayFormSecretName, setGatewayFormSecretName] = useState('');
  const [gatewayFormSecretKey, setGatewayFormSecretKey] = useState('');
  const [gatewaySaving, setGatewaySaving] = useState(false);

  // status
  const [statusError, setStatusError] = useState('');

  const selectedAgent = useMemo(
    () => agents.find((a) => a.id === selectedAgentId) ?? null,
    [agents, selectedAgentId],
  );
  const githubProvider = useMemo(
    () => integrationProviders.find((p) => p.provider_id === 'github') ?? null,
    [integrationProviders],
  );
  const isAdmin = user?.role === 'admin';

  // Chat auto-scroll
  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [chatMessages]);

  // ── Auth session ──

  useEffect(() => {
    if (!token) return;
    getMe()
      .then((d) => setUser(d.user))
      .catch(() => {
        // token expired
        setToken('');
        clearToken();
        setUser(null);
      });
  }, [token]);

  // On mount, if we have a token, load agents and available models
  useEffect(() => {
    if (!token) return;
    loadAgents();
    loadAvailableModels();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token]);

  async function loadAgents() {
    try {
      const list = await listAgents();
      setAgents(list);
      if (list.length > 0 && !selectedAgentId) {
        setSelectedAgentId(list[0].id);
      }
    } catch (err: unknown) {
      setStatusError(err instanceof Error ? err.message : 'Failed to load agents');
    }
  }

  async function loadAvailableModels() {
    setModelsLoading(true);
    setModelsError('');
    try {
      const models = await listModels();
      setAvailableModels(models);
      // If the previously selected model is no longer in the list, reset
      if (models.length > 0 && !models.some(m => m.provider_model === createModel)) {
        setCreateModel(models[0].provider_model);
      }
    } catch (err: unknown) {
      setModelsError(err instanceof Error ? err.message : 'Failed to load models');
      setAvailableModels([]);
    } finally {
      setModelsLoading(false);
    }
  }

  // ── Login ──

  async function handleLogin() {
    if (!loginUsername || !loginPassword) return;
    setLoggingIn(true);
    setLoginError('');
    try {
      const res = await login(loginUsername, loginPassword);
      setToken(res.token);
      setUser(res.user);
      setLoginUsername('');
      setLoginPassword('');
      setPage('agents');
    } catch (err: unknown) {
      setLoginError(err instanceof Error ? err.message : 'Login failed');
    } finally {
      setLoggingIn(false);
    }
  }

  function handleLogout() {
    disconnectRef.current?.();
    disconnectRef.current = null;
    wsRef.current = null;
    clearToken();
    setToken('');
    setUser(null);
    setAgents([]);
    setSelectedAgentId('');
    setChatMessages([]);
    setChatStatus('idle');
    setAdminOverview(null);
    setAdminUsers([]);
    setAdminModels([]);
    setAdminGateway(null);
    setAvailableModels([]);
    setIntegrationProviders([]);
    setIntegrationToken('');
    setModelsError('');
    setStatusError('');
  }

  // ── Agent actions ──

  async function handleCreateAgent() {
    if (!createName.trim()) return;
    setCreating(true);
    setStatusError('');
    try {
      await createAgent({
        name: createName.trim(),
        runtime: createRuntime,
        model: createModel.trim(),
      });
      setCreateName('');
      setCreateModel('');
      await loadAgents();
    } catch (err: unknown) {
      setStatusError(err instanceof Error ? err.message : 'Failed to create agent');
    } finally {
      setCreating(false);
    }
  }

  async function handleStartAgent(id: string) {
    setActionLoading(id);
    setStatusError('');
    try {
      await startAgent(id);
      await loadAgents();
    } catch (err: unknown) {
      setStatusError(err instanceof Error ? err.message : 'Failed to start agent');
    } finally {
      setActionLoading('');
    }
  }

  async function handleStopAgent(id: string) {
    setActionLoading(id);
    setStatusError('');
    try {
      await stopAgent(id);
      await loadAgents();
    } catch (err: unknown) {
      setStatusError(err instanceof Error ? err.message : 'Failed to stop agent');
    } finally {
      setActionLoading('');
    }
  }

  async function loadIntegrations() {
    setIntegrationsLoading(true);
    try {
      const summary = await listIntegrations();
      setIntegrationProviders(summary.providers ?? []);
    } catch (err: unknown) {
      setStatusError(err instanceof Error ? err.message : 'Failed to load integrations');
    } finally {
      setIntegrationsLoading(false);
    }
  }

  useEffect(() => {
    if (token && page === 'integrations') {
      loadIntegrations();
      loadAgents(); // ensure fresh agent list for per-agent bindings
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token, page]);

  async function handleConnectGitHub() {
    if (!integrationToken.trim()) return;
    setIntegrationSaving(true);
    setStatusError('');
    try {
      await connectGitHub(integrationToken.trim());
      setIntegrationToken('');
      await loadIntegrations();
    } catch (err: unknown) {
      setStatusError(err instanceof Error ? err.message : 'Failed to connect GitHub');
    } finally {
      setIntegrationSaving(false);
    }
  }

  async function handleDisconnectGitHub() {
    setIntegrationSaving(true);
    setStatusError('');
    try {
      await disconnectGitHub();
      await loadIntegrations();
    } catch (err: unknown) {
      setStatusError(err instanceof Error ? err.message : 'Failed to disconnect GitHub');
    } finally {
      setIntegrationSaving(false);
    }
  }

  async function handleToggleAgentIntegration(agentId: string, enabled: boolean) {
    setIntegrationAction(agentId);
    setStatusError('');
    try {
      await setAgentGitHubIntegration(agentId, enabled);
      await loadIntegrations();
    } catch (err: unknown) {
      setStatusError(err instanceof Error ? err.message : 'Failed to update agent integration');
    } finally {
      setIntegrationAction('');
    }
  }

  // ── Chat ──

  function handleSendChat() {
    if (!chatText.trim()) return;
    if (chatStatus !== 'connected' && chatStatus !== 'connecting') {
      // connect first, then the text will be sent on open
      handleConnectWithText(chatText.trim());
      setChatText('');
      return;
    }
    if (chatStatus === 'connected' && wsRef.current) {
      wsRef.current.send(JSON.stringify({ text: chatText.trim(), session_id: chatSessionIdRef.current }));
      setChatMessages((prev) => [...prev, { type: 'user', text: chatText.trim() }]);
      setChatText('');
    }
  }

  function handleConnectWithText(text: string) {
    if (!selectedAgent || !token) return;
    disconnectRef.current?.();
    setChatError('');
    setChatStatus('connecting');

    setChatMessages((prev) => [...prev, { type: 'user', text }]);

    const params = new URLSearchParams({ agent_id: selectedAgent.id });
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${protocol}//${window.location.host}/ws?${params}`;
    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      setChatStatus('connected');
      ws.send(JSON.stringify({ text, session_id: chatSessionIdRef.current }));
    };

    ws.onmessage = (msg) => {
      try {
        const data = JSON.parse(msg.data) as ChatEvent;
        if (data.type === 'message.started') {
          setChatMessages((prev) => [...prev, { type: 'message.delta', text: '' }]);
        } else if (data.type === 'message.delta') {
          setChatMessages((prev) => {
            const last = prev[prev.length - 1];
            if (last?.type === 'message.delta') {
              return [...prev.slice(0, -1), { ...last, text: (last.text ?? '') + (data.text ?? '') }];
            }
            return [...prev, { type: 'message.delta', text: data.text ?? '' }];
          });
        } else if (data.type === 'message.done') {
          ws.close();
        } else if (data.type === 'message.error') {
          setChatMessages((prev) => [...prev, data]);
          ws.close();
        } else if (data.done) {
          ws.close();
        }
      } catch {
        setChatError('Invalid message from server');
        setChatStatus('error');
      }
    };

    ws.onerror = () => {
      setChatError('WebSocket connection error');
      setChatStatus('error');
    };

    ws.onclose = () => {
      setChatStatus('idle');
      wsRef.current = null;
    };

    disconnectRef.current = () => {
      if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
        ws.close();
      }
      wsRef.current = null;
    };
  }

  // ── Admin data ──

  function loadAdminData() {
    if (!isAdmin) return;
    adminGetOverview()
      .then(setAdminOverview)
      .catch(() => {});
    adminListUsers()
      .then(setAdminUsers)
      .catch(() => {});
    adminListModels()
      .then(setAdminModels)
      .catch(() => {});
    adminGetGateway()
      .then(setAdminGateway)
      .catch(() => {});
  }

  useEffect(() => {
    if (isAdmin && page === 'admin') {
      loadAdminData();
    }
  }, [isAdmin, page]);

  // Populate gateway form when data loads
  useEffect(() => {
    if (adminGateway) {
      setGatewayFormEnabled(adminGateway.enabled);
      setGatewayFormBaseUrl(adminGateway.base_url);
      setGatewayFormSecretName(adminGateway.secret_name);
      setGatewayFormSecretKey(adminGateway.secret_key);
    }
  }, [adminGateway]);

  async function handleCreateUser() {
    if (!newUsername.trim() || !newPassword.trim()) return;
    setCreatingUser(true);
    try {
      await adminCreateUser({
        username: newUsername.trim(),
        password: newPassword,
        role: newRole,
      });
      setNewUsername('');
      setNewPassword('');
      setNewRole('user');
      const users = await adminListUsers();
      setAdminUsers(users);
    } catch (err: unknown) {
      setStatusError(err instanceof Error ? err.message : 'Failed to create user');
    } finally {
      setCreatingUser(false);
    }
  }

  async function handleToggleUserDisabled(id: string, current: boolean) {
    try {
      await adminPatchUser(id, { disabled: !current });
      setAdminUsers((prev) =>
        prev.map((u) => (u.id === id ? { ...u, disabled: !current } : u)),
      );
    } catch (err: unknown) {
      setStatusError(err instanceof Error ? err.message : 'Failed to update user');
    }
  }

  async function handleChangeUserRole(id: string, role: string) {
    try {
      await adminPatchUser(id, { role });
      setAdminUsers((prev) =>
        prev.map((u) => (u.id === id ? { ...u, role: role as 'admin' | 'user' } : u)),
      );
    } catch (err: unknown) {
      setStatusError(err instanceof Error ? err.message : 'Failed to update user');
    }
  }

  async function handleCreateModel() {
    if (!newModelName.trim() || !newModelProvider.trim()) return;
    setCreatingModel(true);
    try {
      await adminCreateModel({
        display_name: newModelName.trim(),
        provider_model: newModelProvider.trim(),
        enabled: newModelEnabled,
      });
      setNewModelName('');
      setNewModelProvider('');
      setNewModelEnabled(true);
      const models = await adminListModels();
      setAdminModels(models);
    } catch (err: unknown) {
      setStatusError(err instanceof Error ? err.message : 'Failed to create model');
    } finally {
      setCreatingModel(false);
    }
  }

  async function handleToggleModel(id: string, current: boolean) {
    try {
      await adminPatchModel(id, { enabled: !current });
      setAdminModels((prev) =>
        prev.map((m) => (m.id === id ? { ...m, enabled: !current } : m)),
      );
    } catch (err: unknown) {
      setStatusError(err instanceof Error ? err.message : 'Failed to update model');
    }
  }

  async function handleSaveGateway() {
    setGatewaySaving(true);
    setStatusError('');
    try {
      const updated = await adminPatchGateway({
        enabled: gatewayFormEnabled,
        base_url: gatewayFormBaseUrl,
        secret_name: gatewayFormSecretName,
        secret_key: gatewayFormSecretKey,
      });
      setAdminGateway(updated);
    } catch (err: unknown) {
      setStatusError(err instanceof Error ? err.message : 'Failed to save gateway settings');
    } finally {
      setGatewaySaving(false);
    }
  }

  // ── Render ──

  if (!token) {
    return (
      <main className="shell">
        <div className="login-page">
          <div className="login-card">
            <div className="login-header">
              <h1>Shclop</h1>
              <p>Self-hosted agent control plane</p>
            </div>
            <form
              onSubmit={(e) => {
                e.preventDefault();
                handleLogin();
              }}
            >
              <div className="form-group">
                <label htmlFor="username">Username</label>
                <input
                  id="username"
                  type="text"
                  value={loginUsername}
                  onChange={(e) => setLoginUsername(e.target.value)}
                  autoFocus
                  placeholder="admin"
                />
              </div>
              <div className="form-group">
                <label htmlFor="password">Password</label>
                <input
                  id="password"
                  type="password"
                  value={loginPassword}
                  onChange={(e) => setLoginPassword(e.target.value)}
                  placeholder="••••••••"
                />
              </div>
              {loginError ? <div className="form-error">{loginError}</div> : null}
              <button
                type="submit"
                className="btn btn-primary btn-block"
                disabled={loggingIn}
              >
                {loggingIn ? 'Signing in…' : 'Sign in'}
              </button>
            </form>
          </div>
        </div>
      </main>
    );
  }

  return (
    <main className="shell">
      {/* Top bar */}
      <header className="topbar">
        <div className="topbar-left">
          <strong className="brand">Shclop</strong>
          <nav className="topbar-nav">
            <button
              className={`topbar-tab ${page === 'agents' ? 'active' : ''}`}
              onClick={() => setPage('agents')}
            >
              Agents
            </button>
            <button
              className={`topbar-tab ${page === 'integrations' ? 'active' : ''}`}
              onClick={() => setPage('integrations')}
            >
              Integrations
            </button>
            {isAdmin ? (
              <button
                className={`topbar-tab ${page === 'admin' ? 'active' : ''}`}
                onClick={() => setPage('admin')}
              >
                Admin
              </button>
            ) : null}
          </nav>
        </div>
        <div className="topbar-right">
          <span className="topbar-user">
            {user?.username ?? 'User'}
            {user?.role === 'admin' ? <span className="badge badge-admin">admin</span> : null}
          </span>
          <button className="btn btn-ghost btn-sm" onClick={handleLogout}>
            Sign out
          </button>
        </div>
      </header>

      {/* Status error banner */}
      {statusError ? (
        <div className="banner banner-error">
          <span>{statusError}</span>
          <button className="btn btn-ghost btn-sm" onClick={() => setStatusError('')}>
            Dismiss
          </button>
        </div>
      ) : null}

      {/* ── Agents Page ── */}
      {page === 'agents' ? (
        <div className="page-layout">
          {/* Left panel: agent list + create */}
          <aside className="panel panel-sidebar">
            <div className="panel-head">
              <h2>Agents</h2>
              <span className="count-badge">{agents.length}</span>
            </div>

            {/* Create agent form */}
            <div className="card card-form">
              <div className="form-group">
                <label>Name</label>
                <input
                  type="text"
                  value={createName}
                  onChange={(e) => setCreateName(e.target.value)}
                  placeholder="my-agent"
                />
              </div>
              <div className="form-row">
                <div className="form-group">
                  <label>Runtime</label>
                  <select
                    value={createRuntime}
                    onChange={(e) =>
                      setCreateRuntime(e.target.value as 'openclaw' | 'nanoclaw')
                    }
                  >
                    <option value="openclaw">OpenClaw</option>
                    <option value="nanoclaw">NanoClaw</option>
                  </select>
                </div>
                <div className="form-group">
                  <label>Model</label>
                  {modelsError || (!modelsLoading && availableModels.length === 0) ? (
                    <div className="field-error">
                      {modelsError || 'No models available. Contact an admin.'}
                    </div>
                  ) : modelsLoading ? (
                    <div className="field-loading">Loading models…</div>
                  ) : (
                    <select
                      value={createModel}
                      onChange={(e) => setCreateModel(e.target.value)}
                    >
                      {availableModels.length === 0 ? (
                        <option value="">No models available</option>
                      ) : (
                        availableModels.map((m) => (
                          <option key={m.id} value={m.provider_model}>
                            {m.display_name} ({m.provider_model})
                          </option>
                        ))
                      )}
                    </select>
                  )}
                </div>
              </div>
              <button
                className="btn btn-primary btn-block"
                onClick={handleCreateAgent}
                disabled={creating || !createName.trim() || modelsLoading || availableModels.length === 0}
              >
                {creating ? 'Creating…' : 'Create agent'}
              </button>
            </div>

            {/* Agent list */}
            <div className="agent-list">
              {agents.length === 0 ? (
                <div className="empty-state">No agents yet. Create one above.</div>
              ) : (
                agents.map((agent) => (
                  <button
                    key={agent.id}
                    className={`agent-card ${agent.id === selectedAgentId ? 'selected' : ''}`}
                    onClick={() => {
                      setSelectedAgentId(agent.id);
                      disconnectRef.current?.();
                      disconnectRef.current = null;
                      wsRef.current = null;
                      setChatMessages([]);
                      setChatStatus('idle');
                      chatSessionIdRef.current = crypto.randomUUID();
                    }}
                  >
                    <div className="agent-card-head">
                      <strong>{agent.name}</strong>
                      <span className={`state-dot ${agent.state}`} title={agent.state} />
                    </div>
                    <div className="agent-card-meta">
                      <span className="tag">{agent.runtime}</span>
                      <span className="model-tag">{agent.model}</span>
                    </div>
                    <span className="agent-state-label">{agent.state}</span>
                  </button>
                ))
              )}
            </div>
          </aside>

          {/* Right panel: agent detail + chat */}
          <div className="panel panel-main">
            {selectedAgent ? (
              <div className="detail-panel">
                {/* Agent info */}
                <div className="card card-detail">
                  <div className="detail-head">
                    <div>
                      <h2>{selectedAgent.name}</h2>
                      <div className="detail-meta">
                        <span className="tag">{selectedAgent.runtime}</span>
                        <span className="tag tag-model">{selectedAgent.model}</span>
                        <span className={`state-label ${selectedAgent.state}`}>
                          {selectedAgent.state}
                        </span>
                      </div>
                    </div>
                    <div className="detail-actions">
                      {selectedAgent.state === 'running' ? (
                        <button
                          className="btn btn-danger"
                          onClick={() => handleStopAgent(selectedAgent.id)}
                          disabled={actionLoading === selectedAgent.id}
                        >
                          {actionLoading === selectedAgent.id ? '…' : 'Stop'}
                        </button>
                      ) : (
                        <button
                          className="btn btn-primary"
                          onClick={() => handleStartAgent(selectedAgent.id)}
                          disabled={actionLoading === selectedAgent.id}
                        >
                          {actionLoading === selectedAgent.id ? '…' : 'Start'}
                        </button>
                      )}
                    </div>
                  </div>
                  {selectedAgent.last_error ? (
                    <div className="detail-error">
                      Last error: {selectedAgent.last_error}
                    </div>
                  ) : null}
                  <div className="detail-ts">
                    Created {timeAgo(selectedAgent.created_at)}
                    {selectedAgent.updated_at
                      ? ` · Updated ${timeAgo(selectedAgent.updated_at)}`
                      : ''}
                  </div>
                </div>

                {/* Chat */}
                <div className="card card-chat">
                  <div className="chat-head">
                    <h3>Chat</h3>
                    {chatStatus === 'connected' ? (
                      <span className="badge badge-live">Live</span>
                    ) : null}
                  </div>

                  <div className="chat-messages">
                    {chatMessages.length === 0 ? (
                      <div className="empty-state">
                        Send a message to start chatting with {selectedAgent.name}.
                      </div>
                    ) : (
                      chatMessages.map((msg, i) => (
                        <div
                          key={i}
                          className={`chat-msg ${msg.type === 'user' ? 'msg-user' : 'msg-agent'}`}
                        >
                          <div className="msg-label">
                            {msg.type === 'user' ? 'You' : selectedAgent.name}
                          </div>
                          <div className="msg-content">
                            {msg.text || msg.error || (msg.type !== 'message.delta' ? JSON.stringify(msg, null, 2) : '…')}
                          </div>
                        </div>
                      ))
                    )}
                    {chatStatus === 'connecting' ? (
                      <div className="chat-msg msg-agent">
                        <div className="msg-label">System</div>
                        <div className="msg-content connecting-dots">
                          Connecting<span>.</span><span>.</span><span>.</span>
                        </div>
                      </div>
                    ) : null}
                    {chatError ? (
                      <div className="chat-msg msg-error">
                        <div className="msg-label">Error</div>
                        <div className="msg-content">{chatError}</div>
                      </div>
                    ) : null}
                    <div ref={chatEndRef} />
                  </div>

                  <div className="chat-input-row">
                    <input
                      type="text"
                      value={chatText}
                      onChange={(e) => setChatText(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' && !e.shiftKey) {
                          e.preventDefault();
                          handleSendChat();
                        }
                      }}
                      placeholder={
                        chatStatus === 'connected'
                          ? 'Type a message…'
                          : 'Connect to start chatting…'
                      }
                      disabled={
                        chatStatus === 'connecting' || chatStatus === 'error'
                      }
                    />
                    <button
                      className="btn btn-primary"
                      onClick={handleSendChat}
                      disabled={
                        chatStatus === 'connecting' ||
                        !chatText.trim()
                      }
                    >
                      {chatStatus === 'connected' ? 'Send' : chatStatus === 'connecting' ? '…' : 'Connect'}
                    </button>
                  </div>
                </div>
              </div>
            ) : (
              <div className="empty-state empty-state-centered">
                Select an agent to view details and chat.
              </div>
            )}
          </div>
        </div>
      ) : null}

      {/* ── Admin Page ── */}
      {page === 'admin' ? (
        <div className="page-layout page-layout-single">
          <div className="panel panel-main">
            <div className="admin-tabs">
              <button
                className={`admin-tab ${adminTab === 'overview' ? 'active' : ''}`}
                onClick={() => setAdminTab('overview')}
              >
                Overview
              </button>
              <button
                className={`admin-tab ${adminTab === 'users' ? 'active' : ''}`}
                onClick={() => setAdminTab('users')}
              >
                Users
              </button>
              <button
                className={`admin-tab ${adminTab === 'models' ? 'active' : ''}`}
                onClick={() => setAdminTab('models')}
              >
                Models
              </button>
              <button
                className={`admin-tab ${adminTab === 'gateway' ? 'active' : ''}`}
                onClick={() => setAdminTab('gateway')}
              >
                LLM Gateway
              </button>
            </div>

            {/* Overview tab */}
            {adminTab === 'overview' ? (
              <div className="admin-section">
                <h2>System Overview</h2>
                {adminOverview ? (
                  <div className="overview-grid">
                    <div className="card card-stat">
                      <div className="stat-label">Runtime Provider</div>
                      <div className="stat-value">{adminOverview.runtime.provider}</div>
                    </div>
                    <div className="card card-stat">
                      <div className="stat-label">Namespace</div>
                      <div className="stat-value">{adminOverview.runtime.namespace}</div>
                    </div>
                    <div className="card card-stat">
                      <div className="stat-label">Runtime Class</div>
                      <div className="stat-value">{adminOverview.runtime.runtime_class_name}</div>
                    </div>
                    <div className="card card-stat">
                      <div className="stat-label">Metrics</div>
                      <div className="stat-value">
                        {adminOverview.observability.metrics_enabled ? 'Enabled' : 'Disabled'}
                      </div>
                    </div>
                    <div className="card card-stat">
                      <div className="stat-label">Logging</div>
                      <div className="stat-value">
                        {adminOverview.observability.logging_enabled ? 'Enabled' : 'Disabled'}
                      </div>
                    </div>
                    <div className="card card-stat">
                      <div className="stat-label">Grafana</div>
                      <div className="stat-value stat-value-url">
                        {adminOverview.observability.grafana_url || '—'}
                      </div>
                    </div>
                    <div className="card card-stat">
                      <div className="stat-label">Healthz</div>
                      <div className="stat-value">{adminOverview.health.healthz}</div>
                    </div>
                    <div className="card card-stat">
                      <div className="stat-label">Readyz</div>
                      <div className="stat-value">{adminOverview.health.readyz}</div>
                    </div>
                  </div>
                ) : (
                  <div className="empty-state">Loading overview…</div>
                )}

                {adminOverview ? (
                  <div className="card card-table">
                    <h3>Runtime Images</h3>
                    <table>
                      <thead>
                        <tr>
                          <th>Name</th>
                          <th>Image</th>
                        </tr>
                      </thead>
                      <tbody>
                        {Object.entries(adminOverview.runtime.images).map(
                          ([name, image]) => (
                            <tr key={name}>
                              <td>{name}</td>
                              <td className="cell-mono">{image}</td>
                            </tr>
                          ),
                        )}
                      </tbody>
                    </table>
                  </div>
                ) : null}
              </div>
            ) : null}

            {/* Users tab */}
            {adminTab === 'users' ? (
              <div className="admin-section">
                <h2>Users</h2>

                {/* Create user form */}
                <div className="card card-form card-form-inline">
                  <div className="form-row form-row-inline">
                    <div className="form-group">
                      <label>Username</label>
                      <input
                        type="text"
                        value={newUsername}
                        onChange={(e) => setNewUsername(e.target.value)}
                        placeholder="jdoe"
                      />
                    </div>
                    <div className="form-group">
                      <label>Password</label>
                      <input
                        type="password"
                        value={newPassword}
                        onChange={(e) => setNewPassword(e.target.value)}
                        placeholder="min 8 chars"
                      />
                    </div>
                    <div className="form-group">
                      <label>Role</label>
                      <select
                        value={newRole}
                        onChange={(e) =>
                          setNewRole(e.target.value as 'user' | 'admin')
                        }
                      >
                        <option value="user">User</option>
                        <option value="admin">Admin</option>
                      </select>
                    </div>
                    <div className="form-group form-group-submit">
                      <label>&nbsp;</label>
                      <button
                        className="btn btn-primary"
                        onClick={handleCreateUser}
                        disabled={
                          creatingUser ||
                          !newUsername.trim() ||
                          !newPassword.trim()
                        }
                      >
                        {creatingUser ? 'Adding…' : 'Add user'}
                      </button>
                    </div>
                  </div>
                </div>

                {/* Users table */}
                <div className="card card-table">
                  <table>
                    <thead>
                      <tr>
                        <th>Username</th>
                        <th>Role</th>
                        <th>Status</th>
                        <th>Created</th>
                        <th>Actions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {adminUsers.length === 0 ? (
                        <tr>
                          <td colSpan={5} className="cell-empty">
                            No users found.
                          </td>
                        </tr>
                      ) : (
                        adminUsers.map((u) => (
                          <tr key={u.id}>
                            <td>
                              <strong>{u.username}</strong>
                            </td>
                            <td>
                              <select
                                value={u.role}
                                onChange={(e) =>
                                  handleChangeUserRole(u.id, e.target.value)
                                }
                                className="cell-select"
                              >
                                <option value="user">User</option>
                                <option value="admin">Admin</option>
                              </select>
                            </td>
                            <td>
                              {u.disabled ? (
                                <span className="badge badge-disabled">
                                  Disabled
                                </span>
                              ) : (
                                <span className="badge badge-active">
                                  Active
                                </span>
                              )}
                            </td>
                            <td className="cell-ts">
                              {timeAgo(u.created_at)}
                            </td>
                            <td>
                              <button
                                className="btn btn-sm btn-ghost"
                                onClick={() =>
                                  handleToggleUserDisabled(u.id, u.disabled)
                                }
                              >
                                {u.disabled ? 'Enable' : 'Disable'}
                              </button>
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </div>
            ) : null}

            {/* Models tab */}
            {adminTab === 'models' ? (
              <div className="admin-section">
                <h2>LLM Models</h2>

                {/* Create model form */}
                <div className="card card-form card-form-inline">
                  <div className="form-row form-row-inline">
                    <div className="form-group">
                      <label>Display Name</label>
                      <input
                        type="text"
                        value={newModelName}
                        onChange={(e) => setNewModelName(e.target.value)}
                        placeholder="GPT-4o"
                      />
                    </div>
                    <div className="form-group">
                      <label>Provider Model ID</label>
                      <input
                        type="text"
                        value={newModelProvider}
                        onChange={(e) => setNewModelProvider(e.target.value)}
                        placeholder="gpt-4o-2024-08-06"
                      />
                    </div>
                    <div className="form-group form-group-check">
                      <label>
                        <input
                          type="checkbox"
                          checked={newModelEnabled}
                          onChange={(e) => setNewModelEnabled(e.target.checked)}
                        />{' '}
                        Enabled
                      </label>
                    </div>
                    <div className="form-group form-group-submit">
                      <label>&nbsp;</label>
                      <button
                        className="btn btn-primary"
                        onClick={handleCreateModel}
                        disabled={
                          creatingModel ||
                          !newModelName.trim() ||
                          !newModelProvider.trim()
                        }
                      >
                        {creatingModel ? 'Adding…' : 'Add model'}
                      </button>
                    </div>
                  </div>
                </div>

                {/* Models table */}
                <div className="card card-table">
                  <table>
                    <thead>
                      <tr>
                        <th>Display Name</th>
                        <th>Provider Model</th>
                        <th>Status</th>
                        <th>Created</th>
                        <th>Actions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {adminModels.length === 0 ? (
                        <tr>
                          <td colSpan={5} className="cell-empty">
                            No models configured.
                          </td>
                        </tr>
                      ) : (
                        adminModels.map((m) => (
                          <tr key={m.id}>
                            <td>
                              <strong>{m.display_name}</strong>
                            </td>
                            <td className="cell-mono">
                              {m.provider_model}
                            </td>
                            <td>
                              {m.enabled ? (
                                <span className="badge badge-active">
                                  Enabled
                                </span>
                              ) : (
                                <span className="badge badge-disabled">
                                  Disabled
                                </span>
                              )}
                            </td>
                            <td className="cell-ts">
                              {timeAgo(m.created_at)}
                            </td>
                            <td>
                              <button
                                className="btn btn-sm btn-ghost"
                                onClick={() =>
                                  handleToggleModel(m.id, m.enabled)
                                }
                              >
                                {m.enabled ? 'Disable' : 'Enable'}
                              </button>
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </div>
            ) : null}

            {/* Gateway tab */}
            {adminTab === 'gateway' ? (
              <div className="admin-section">
                <h2>LLM Gateway Settings</h2>
                <div className="card card-form">
                  <div className="form-group form-group-check">
                    <label>
                      <input
                        type="checkbox"
                        checked={gatewayFormEnabled}
                        onChange={(e) => setGatewayFormEnabled(e.target.checked)}
                      />{' '}
                      Enabled
                    </label>
                  </div>
                  <div className="form-group">
                    <label>Base URL</label>
                    <input
                      type="text"
                      value={gatewayFormBaseUrl}
                      onChange={(e) => setGatewayFormBaseUrl(e.target.value)}
                      placeholder="https://api.openai.com"
                    />
                  </div>
                  <div className="form-group">
                    <label>Secret Name</label>
                    <input
                      type="text"
                      value={gatewayFormSecretName}
                      onChange={(e) => setGatewayFormSecretName(e.target.value)}
                      placeholder="my-gateway-secret"
                    />
                  </div>
                  <div className="form-group">
                    <label>Secret Key</label>
                    <input
                      type="text"
                      value={gatewayFormSecretKey}
                      onChange={(e) => setGatewayFormSecretKey(e.target.value)}
                      placeholder="sk-..."
                    />
                  </div>
                  <button
                    className="btn btn-primary"
                    onClick={handleSaveGateway}
                    disabled={gatewaySaving}
                  >
                    {gatewaySaving ? 'Saving…' : 'Save settings'}
                  </button>
                  {adminGateway ? (
                    <div className="gateway-ts">
                      Last updated {timeAgo(adminGateway.updated_at)}
                    </div>
                  ) : null}
                </div>
              </div>
            ) : null}
          </div>
        </div>
      ) : null}

      {/* ── Integrations Page ── */}
      {page === 'integrations' ? (
        <div className="page-layout page-layout-single">
          <div className="panel panel-main">

            {/* GitHub integration card */}
            <div className="card card-detail">
              <div className="detail-head">
                <div>
                  <h2>GitHub Integration</h2>
                </div>
                {!integrationsLoading ? (
                  githubProvider?.connected ? (
                    <span className="badge badge-active">Connected</span>
                  ) : (
                    <span className="badge badge-disabled">Disconnected</span>
                  )
                ) : null}
              </div>

              {integrationsLoading ? (
                <div className="field-loading" style={{ marginTop: 12 }}>Loading integrations…</div>
              ) : githubProvider?.connected && githubProvider?.connection ? (
                <>
                  {/* Connection details */}
                  <div className="integration-details">
                    <div className="integration-detail-item">
                      <span className="integration-detail-label">GitHub Login</span>
                      <span className="integration-detail-value">{githubProvider.connection.external_login}</span>
                    </div>
                    <div className="integration-detail-item">
                      <span className="integration-detail-label">Account Type</span>
                      <span className="integration-detail-value">{githubProvider.connection.account_type}</span>
                    </div>
                    <div className="integration-detail-item">
                      <span className="integration-detail-label">Status</span>
                      <span className="integration-detail-value">{githubProvider.connection.status}</span>
                    </div>
                    <div className="integration-detail-item">
                      <span className="integration-detail-label">Revision</span>
                      <span className="integration-detail-value">{githubProvider.connection.revision}</span>
                    </div>
                  </div>

                  {/* Token update / disconnect */}
                  <div className="integration-token-section">
                    <div className="form-group">
                      <label>Update Personal Access Token</label>
                      <input
                        type="password"
                        value={integrationToken}
                        onChange={(e) => setIntegrationToken(e.target.value)}
                        placeholder="New GitHub PAT…"
                      />
                    </div>
                    <div className="integration-actions">
                      <button
                        className="btn btn-primary"
                        onClick={handleConnectGitHub}
                        disabled={!integrationToken.trim() || integrationSaving}
                      >
                        {integrationSaving ? 'Updating…' : 'Update token'}
                      </button>
                      <button
                        className="btn btn-danger"
                        onClick={handleDisconnectGitHub}
                        disabled={integrationSaving}
                      >
                        {integrationSaving ? 'Disconnecting…' : 'Disconnect'}
                      </button>
                    </div>
                    <div className="integration-hint">
                      Use a fine-grained GitHub PAT with minimal repository permissions (contents: read, pull requests: read/write).
                    </div>
                  </div>
                </>
              ) : (
                <>
                  {/* Connect form */}
                  <div className="form-group">
                    <label>Personal Access Token (PAT)</label>
                    <input
                      type="password"
                      value={integrationToken}
                      onChange={(e) => setIntegrationToken(e.target.value)}
                      placeholder="github_pat_…"
                    />
                  </div>
                  <div className="integration-hint">
                    Use a fine-grained GitHub PAT with minimal repository permissions (contents: read, pull requests: read/write).
                  </div>
                  <div className="integration-actions">
                    <button
                      className="btn btn-primary"
                      onClick={handleConnectGitHub}
                      disabled={!integrationToken.trim() || integrationSaving}
                    >
                      {integrationSaving ? 'Connecting…' : 'Connect GitHub'}
                    </button>
                  </div>
                </>
              )}
            </div>

            {/* Per-agent enablement */}
            <h2 style={{ marginTop: 8 }}>Per-Agent GitHub Integration</h2>
            <div className="card card-table">
              {agents.length === 0 ? (
                <div className="empty-state">No agents found. Create an agent first.</div>
              ) : (
                <table>
                  <thead>
                    <tr>
                      <th>Name</th>
                      <th>Runtime</th>
                      <th>Model</th>
                      <th>GitHub</th>
                      <th>Status</th>
                      <th></th>
                    </tr>
                  </thead>
                  <tbody>
                    {agents.map((agent) => {
                      const binding = githubProvider?.agent_bindings.find(
                        (b) => b.agent_id === agent.id,
                      );
                      return (
                        <tr key={agent.id}>
                          <td><strong>{agent.name}</strong></td>
                          <td><span className="tag">{agent.runtime}</span></td>
                          <td><span className="tag tag-model">{agent.model}</span></td>
                          <td>
                            {binding?.enabled ? (
                              <span className="badge badge-active">Enabled</span>
                            ) : (
                              <span className="badge badge-disabled">Disabled</span>
                            )}
                          </td>
                          <td>{binding?.status ?? '—'}</td>
                          <td>
                            <button
                              className="btn btn-sm"
                              onClick={() =>
                                handleToggleAgentIntegration(agent.id, !binding?.enabled)
                              }
                              disabled={
                                !githubProvider?.connected ||
                                integrationAction === agent.id
                              }
                            >
                              {integrationAction === agent.id
                                ? '…'
                                : binding?.enabled
                                ? 'Disable'
                                : 'Enable'}
                            </button>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              )}
            </div>

          </div>
        </div>
      ) : null}
    </main>
  );
}
