import React, { useEffect, useMemo, useRef, useState, useCallback } from 'react';
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
  deleteAgent,
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
  registerAuthErrorHandler,
} from './api';

type Page = 'agents' | 'integrations' | 'admin';

type AdminTab = 'overview' | 'users' | 'models' | 'gateway';

type IntegrationsView = 'list' | 'add' | 'detail';
type AddStep = 'picker' | 'form';

/* ── helpers ── */

// eslint-disable-next-line no-control-regex
const ANSI_RE = /\x1b\[[0-9;]*[a-zA-Z]/g;
const AGENT_INIT_RE = /^🤖 Agent/;
const LOG_START_RE = /^\[\d{2}:\d{2}:\d{2}\]/;

function renderAgentText(raw: string): React.ReactNode {
  const lines = raw.replace(ANSI_RE, '').split('\n');
  const nodes: React.ReactNode[] = [];
  let lastWasLog = false;
  for (let i = 0; i < lines.length; i++) {
    // strip stderr prefix if subprocess added it
    const line = lines[i].startsWith('stderr: ') ? lines[i].slice(8) : lines[i];
    if (line === '') { lastWasLog = false; continue; }
    if (AGENT_INIT_RE.test(line)) continue;
    const isLog: boolean = LOG_START_RE.test(line) || (lastWasLog && /^\s/.test(line));
    lastWasLog = isLog;
    nodes.push(<div key={i} className={isLog ? 'chat-log-line' : 'chat-text-line'}>{line}</div>);
  }
  if (nodes.length === 0) return <span className="chat-streaming">…</span>;
  return <>{nodes}</>;
}

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
  const [availableModels, setAvailableModels] = useState<LLMModel[]>([]);
  const [modelsLoading, setModelsLoading] = useState(false);
  const [modelsError, setModelsError] = useState('');
  const [actionLoading, setActionLoading] = useState('');
  const [hoveredAgentId, setHoveredAgentId] = useState('');
  const [showIntegrationsPanel, setShowIntegrationsPanel] = useState(false);

  // create agent modal
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createName, setCreateName] = useState('');
  const [createDescription, setCreateDescription] = useState('');
  const [createRuntime, setCreateRuntime] = useState<'openclaw' | 'nanoclaw'>('nanoclaw');
  const [createModel, setCreateModel] = useState('');
  const [createSystemPrompt, setCreateSystemPrompt] = useState('');
  const [createIntegrations, setCreateIntegrations] = useState<Set<string>>(new Set());
  const [creating, setCreating] = useState(false);
  const integrationsPanelRef = useRef<HTMLDivElement>(null);
  const [archivedIds, setArchivedIds] = useState<Set<string>>(() => {
    try {
      return new Set(JSON.parse(localStorage.getItem('shclop_archived_agents') ?? '[]') as string[]);
    } catch {
      return new Set();
    }
  });

  // integrations
  const [integrationProviders, setIntegrationProviders] = useState<IntegrationProvider[]>([]);
  const [integrationsLoading, setIntegrationsLoading] = useState(false);
  const [integrationToken, setIntegrationToken] = useState('');
  const [integrationSaving, setIntegrationSaving] = useState(false);
  const [integrationAction, setIntegrationAction] = useState('');
  const [integrationsView, setIntegrationsView] = useState<IntegrationsView>('list');
  const [addStep, setAddStep] = useState<AddStep>('picker');
  const [detailProviderId, setDetailProviderId] = useState('');

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
  const githubConnected = githubProvider?.connected ?? false;
  const isAdmin = user?.role === 'admin';

  const sortedVisibleAgents = useMemo(
    () =>
      [...agents]
        .filter((a) => !archivedIds.has(a.id))
        .sort((a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()),
    [agents, archivedIds],
  );

  const chatStorageKey = useCallback((id: string) => `shclop_chat_${id}`, []);

  // Load chat history from localStorage when agent changes
  useEffect(() => {
    if (!selectedAgentId) { setChatMessages([]); return; }
    try {
      const saved = localStorage.getItem(chatStorageKey(selectedAgentId));
      setChatMessages(saved ? (JSON.parse(saved) as ChatEvent[]) : []);
    } catch {
      setChatMessages([]);
    }
  }, [selectedAgentId, chatStorageKey]);

  // Keep a ref in sync so the save effect never fires on agent switch
  const selectedAgentIdSaveRef = useRef<string>('');
  useEffect(() => {
    selectedAgentIdSaveRef.current = selectedAgentId;
  }, [selectedAgentId]);

  // Save chat history to localStorage — only fires when messages change
  useEffect(() => {
    const agentId = selectedAgentIdSaveRef.current;
    if (!agentId || chatMessages.length === 0) return;
    localStorage.setItem(chatStorageKey(agentId), JSON.stringify(chatMessages));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [chatMessages, chatStorageKey]);

  // Close integrations panel on outside click
  useEffect(() => {
    if (!showIntegrationsPanel) return;
    function handleClick(e: MouseEvent) {
      if (integrationsPanelRef.current && !integrationsPanelRef.current.contains(e.target as Node)) {
        setShowIntegrationsPanel(false);
      }
    }
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [showIntegrationsPanel]);

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

  // Register global 401 handler once — clears session and goes to login
  useEffect(() => {
    registerAuthErrorHandler(handleLogout);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

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
      const agent = await createAgent({
        name: createName.trim(),
        description: createDescription.trim(),
        runtime: createRuntime,
        model: createModel.trim(),
        system_prompt: createSystemPrompt.trim(),
      });
      // Enable any toggled integrations on the new agent
      for (const providerId of createIntegrations) {
        try {
          await setAgentGitHubIntegration(agent.id, true);
        } catch {
          // non-fatal — user can toggle later
        }
      }
      setCreateName('');
      setCreateDescription('');
      setCreateSystemPrompt('');
      setCreateIntegrations(new Set());
      setShowCreateModal(false);
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

  async function handleDeleteAgent(id: string) {
    setStatusError('');
    try {
      await deleteAgent(id);
      setAgents((prev) => prev.filter((a) => a.id !== id));
      if (selectedAgentId === id) {
        setSelectedAgentId('');
        disconnectRef.current?.();
        disconnectRef.current = null;
        wsRef.current = null;
        setChatMessages([]);
        setChatStatus('idle');
      }
    } catch (err: unknown) {
      setStatusError(err instanceof Error ? err.message : 'Failed to delete agent');
    }
  }

  async function handleArchiveAgent(id: string) {
    const agent = agents.find((a) => a.id === id);
    if (agent?.state === 'running') {
      try { await stopAgent(id); } catch { /* best effort */ }
    }
    setArchivedIds((prev) => {
      const next = new Set(prev);
      next.add(id);
      localStorage.setItem('shclop_archived_agents', JSON.stringify([...next]));
      return next;
    });
    if (selectedAgentId === id) {
      setSelectedAgentId('');
      disconnectRef.current?.();
      disconnectRef.current = null;
      wsRef.current = null;
      setChatMessages([]);
      setChatStatus('idle');
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
      setIntegrationsView('list');
      loadIntegrations();
      loadAgents();
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
              <button
                className="btn btn-primary btn-sm"
                onClick={() => {
                  if (integrationProviders.length === 0) loadIntegrations();
                  setShowCreateModal(true);
                }}
              >
                + New
              </button>
            </div>

            {/* Agent list */}
            <div className="agent-list">
              {sortedVisibleAgents.length === 0 ? (
                <div className="empty-state">No agents yet.</div>
              ) : (
                sortedVisibleAgents.map((agent) => (
                  <div
                    key={agent.id}
                    className="agent-card-wrapper"
                    onMouseEnter={() => setHoveredAgentId(agent.id)}
                    onMouseLeave={() => setHoveredAgentId('')}
                  >
                    <button
                      className={`agent-card ${agent.id === selectedAgentId ? 'selected' : ''}`}
                      onClick={() => {
                        setSelectedAgentId(agent.id);
                        disconnectRef.current?.();
                        disconnectRef.current = null;
                        wsRef.current = null;
                        setChatStatus('idle');
                        chatSessionIdRef.current = crypto.randomUUID();
                        setShowIntegrationsPanel(false);
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
                    {hoveredAgentId === agent.id ? (
                      <div className="agent-card-actions">
                        <button
                          className="agent-action-btn archive-btn"
                          title="Archive"
                          onClick={(e) => { e.stopPropagation(); handleArchiveAgent(agent.id); }}
                        >
                          <svg width="13" height="13" viewBox="0 0 16 16" fill="currentColor">
                            <path d="M0 2a1 1 0 0 1 1-1h14a1 1 0 0 1 1 1v2a1 1 0 0 1-1 1v7.5a2.5 2.5 0 0 1-2.5 2.5h-9A2.5 2.5 0 0 1 1 12.5V5a1 1 0 0 1-1-1zm2 3v7.5A1.5 1.5 0 0 0 3.5 14h9a1.5 1.5 0 0 0 1.5-1.5V5zm13-3H1v2h14zM5 7.5a.5.5 0 0 1 .5-.5h5a.5.5 0 0 1 0 1h-5a.5.5 0 0 1-.5-.5"/>
                          </svg>
                        </button>
                        <button
                          className="agent-action-btn delete-btn"
                          title="Delete"
                          onClick={(e) => { e.stopPropagation(); handleDeleteAgent(agent.id); }}
                        >
                          <svg width="13" height="13" viewBox="0 0 16 16" fill="currentColor">
                            <path d="M11 1.5v1h3.5a.5.5 0 0 1 0 1h-.538l-.853 10.66A2 2 0 0 1 11.115 16h-6.23a2 2 0 0 1-1.994-1.84L2.038 3.5H1.5a.5.5 0 0 1 0-1H5v-1A1.5 1.5 0 0 1 6.5 0h3A1.5 1.5 0 0 1 11 1.5m-5 0v1h4v-1a.5.5 0 0 0-.5-.5h-3a.5.5 0 0 0-.5.5M4.5 5.029l.5 8.5a.5.5 0 1 0 .998-.06l-.5-8.5a.5.5 0 1 0-.998.06m6.53-.528a.5.5 0 0 0-.528.47l-.5 8.5a.5.5 0 0 0 .998.058l.5-8.5a.5.5 0 0 0-.47-.528M8 4.5a.5.5 0 0 0-.5.5v8.5a.5.5 0 0 0 1 0V5a.5.5 0 0 0-.5-.5"/>
                          </svg>
                        </button>
                      </div>
                    ) : null}
                  </div>
                ))
              )}
            </div>
          </aside>

          {/* Right panel: chat */}
          <div className="panel panel-main">
            {selectedAgent ? (
              <div className="detail-panel">
                {/* Chat */}
                <div className="card card-chat">
                  <div className="chat-head">
                    <div className="chat-head-left">
                      <span className={`state-dot ${selectedAgent.state}`} title={selectedAgent.state} />
                      <h3>{selectedAgent.name}</h3>
                      <span className="tag tag-model">{selectedAgent.model}</span>
                    </div>
                    <div className="chat-head-right" ref={integrationsPanelRef}>
                      {chatStatus === 'connected' ? (
                        <span className="badge badge-live">Live</span>
                      ) : null}
                      {selectedAgent.state === 'running' ? (
                        <button
                          className="btn btn-sm btn-danger"
                          onClick={() => handleStopAgent(selectedAgent.id)}
                          disabled={actionLoading === selectedAgent.id}
                        >
                          {actionLoading === selectedAgent.id ? '…' : 'Stop'}
                        </button>
                      ) : (
                        <button
                          className="btn btn-sm btn-primary"
                          onClick={() => handleStartAgent(selectedAgent.id)}
                          disabled={actionLoading === selectedAgent.id}
                        >
                          {actionLoading === selectedAgent.id ? '…' : 'Start'}
                        </button>
                      )}
                      <button
                        className="btn btn-sm btn-ghost"
                        onClick={() => {
                          const opening = !showIntegrationsPanel;
                          setShowIntegrationsPanel(opening);
                          if (opening && integrationProviders.length === 0) loadIntegrations();
                        }}
                      >
                        Integrations
                      </button>
                      {showIntegrationsPanel ? (
                        <div className="integrations-panel">
                          <div className="integrations-panel-title">Integrations</div>
                          {integrationProviders.length === 0 ? (
                            <div className="integration-panel-empty">No integrations available.</div>
                          ) : (
                            integrationProviders.map((provider) => {
                              const binding = provider.agent_bindings.find(
                                (b) => b.agent_id === selectedAgent.id,
                              );
                              return (
                                <div key={provider.provider_id} className="integration-panel-row">
                                  <div className="integration-panel-info">
                                    <span className="integration-panel-name">{provider.name}</span>
                                    {!provider.connected ? (
                                      <span className="integration-panel-hint">Not connected — go to Integrations page</span>
                                    ) : null}
                                  </div>
                                  <label className="toggle">
                                    <input
                                      type="checkbox"
                                      checked={binding?.enabled ?? false}
                                      disabled={!provider.connected || integrationAction === selectedAgent.id}
                                      onChange={(e) =>
                                        handleToggleAgentIntegration(selectedAgent.id, e.target.checked)
                                      }
                                    />
                                    <span className="toggle-slider" />
                                  </label>
                                </div>
                              );
                            })
                          )}
                          {selectedAgent.state === 'running' ? (
                            <div className="integration-panel-note">
                              Restart the agent to apply changes
                            </div>
                          ) : null}
                        </div>
                      ) : null}
                    </div>
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
                          className={`chat-msg ${msg.type === 'user' ? 'msg-user' : msg.type === 'message.error' ? 'msg-error' : 'msg-agent'}`}
                        >
                          <div className="msg-content">
                            {msg.type === 'user'
                              ? msg.text
                              : msg.type === 'message.error'
                              ? (msg.error ?? msg.text ?? 'Error')
                              : renderAgentText(msg.text ?? '')}
                          </div>
                        </div>
                      ))
                    )}
                    {chatStatus === 'connecting' ? (
                      <div className="chat-msg msg-agent">
                        <div className="msg-content connecting-dots">
                          Connecting<span>.</span><span>.</span><span>.</span>
                        </div>
                      </div>
                    ) : null}
                    {chatError ? (
                      <div className="chat-msg msg-error">
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

      {/* ── Create Agent Modal ── */}
      {showCreateModal ? (
        <div className="modal-overlay" onClick={(e) => { if (e.target === e.currentTarget) setShowCreateModal(false); }}>
          <div className="modal">
            <div className="modal-head">
              <h2>New Agent</h2>
              <button className="btn btn-ghost btn-sm modal-close" onClick={() => setShowCreateModal(false)}>✕</button>
            </div>
            <div className="modal-body">
              <div className="form-group">
                <label>Name <span className="required">*</span></label>
                <input
                  type="text"
                  value={createName}
                  onChange={(e) => setCreateName(e.target.value)}
                  placeholder="my-agent"
                  autoFocus
                />
              </div>
              <div className="form-group">
                <label>Description</label>
                <input
                  type="text"
                  value={createDescription}
                  onChange={(e) => setCreateDescription(e.target.value)}
                  placeholder="What does this agent do?"
                />
              </div>
              <div className="form-row form-row-inline">
                <div className="form-group">
                  <label>Runtime</label>
                  <select value={createRuntime} onChange={(e) => setCreateRuntime(e.target.value as 'openclaw' | 'nanoclaw')}>
                    <option value="nanoclaw">nanoclaw</option>
                    <option value="openclaw">openclaw</option>
                  </select>
                </div>
                <div className="form-group">
                  <label>Model</label>
                  {modelsLoading ? (
                    <select disabled><option>Loading…</option></select>
                  ) : modelsError ? (
                    <input
                      type="text"
                      value={createModel}
                      onChange={(e) => setCreateModel(e.target.value)}
                      placeholder="e.g. gpt-4o"
                    />
                  ) : availableModels.length > 0 ? (
                    <select value={createModel} onChange={(e) => setCreateModel(e.target.value)}>
                      {availableModels.map((m) => (
                        <option key={m.id} value={m.provider_model}>{m.display_name}</option>
                      ))}
                    </select>
                  ) : (
                    <input
                      type="text"
                      value={createModel}
                      onChange={(e) => setCreateModel(e.target.value)}
                      placeholder="e.g. gpt-4o"
                    />
                  )}
                </div>
              </div>
              <div className="form-group">
                <label>System Prompt</label>
                <textarea
                  value={createSystemPrompt}
                  onChange={(e) => setCreateSystemPrompt(e.target.value)}
                  placeholder="Optional instructions for the agent…"
                  rows={4}
                />
              </div>
              {integrationProviders.length > 0 ? (
                <div className="form-group">
                  <label>Integrations</label>
                  <div className="modal-integrations">
                    {integrationProviders.map((provider) => (
                      <label key={provider.provider_id} className={`modal-integration-row ${!provider.connected ? 'disabled' : ''}`}>
                        <span className="modal-integration-name">{provider.name}</span>
                        {!provider.connected ? (
                          <span className="modal-integration-hint">not connected</span>
                        ) : null}
                        <label className="toggle">
                          <input
                            type="checkbox"
                            checked={createIntegrations.has(provider.provider_id)}
                            disabled={!provider.connected}
                            onChange={(e) => {
                              setCreateIntegrations((prev) => {
                                const next = new Set(prev);
                                if (e.target.checked) next.add(provider.provider_id);
                                else next.delete(provider.provider_id);
                                return next;
                              });
                            }}
                          />
                          <span className="toggle-slider" />
                        </label>
                      </label>
                    ))}
                  </div>
                </div>
              ) : null}
            </div>
            <div className="modal-footer">
              <button className="btn btn-ghost" onClick={() => setShowCreateModal(false)}>Cancel</button>
              <button
                className="btn btn-primary"
                onClick={handleCreateAgent}
                disabled={creating || !createName.trim()}
              >
                {creating ? 'Creating…' : 'Create agent'}
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {/* ── Integrations Page ── */}
      {page === 'integrations' ? (
        <div className="page-layout page-layout-single">
          <div className="panel panel-main">

            {/* ── List view ── */}
            {integrationsView === 'list' ? (
              <>
                <div className="integrations-page-head">
                  <h2>Integrations</h2>
                  <button
                    className="btn btn-primary btn-sm"
                    onClick={() => { setIntegrationsView('add'); setAddStep('picker'); }}
                  >
                    Add integration
                  </button>
                </div>

                {integrationsLoading ? (
                  <div className="field-loading" style={{ padding: '20px 0' }}>Loading integrations…</div>
                ) : integrationProviders.filter((p) => p.connected).length === 0 ? (
                  <div className="card card-detail">
                    <div className="integrations-empty-state">
                      <div className="integrations-empty-icon">
                        <svg width="32" height="32" viewBox="0 0 16 16" fill="currentColor" opacity="0.3">
                          <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/>
                        </svg>
                      </div>
                      <p>No integrations connected yet.</p>
                      <button
                        className="btn btn-primary"
                        onClick={() => { setIntegrationsView('add'); setAddStep('picker'); }}
                      >
                        Connect your first integration
                      </button>
                    </div>
                  </div>
                ) : (
                  <div className="integration-cards-list">
                    {integrationProviders.filter((p) => p.connected).map((provider) => (
                      <div
                        key={provider.provider_id}
                        className="integration-card"
                        onClick={() => { setDetailProviderId(provider.provider_id); setIntegrationsView('detail'); }}
                      >
                        <div className="integration-card-icon integration-card-icon-github">
                          <svg width="24" height="24" viewBox="0 0 16 16" fill="currentColor">
                            <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/>
                          </svg>
                        </div>
                        <div className="integration-card-body">
                          <div className="integration-card-name">{provider.name}</div>
                          {provider.connection ? (
                            <div className="integration-card-sub">{provider.connection.external_login}</div>
                          ) : null}
                        </div>
                        <span className="badge badge-active">Connected</span>
                        <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor" style={{ color: 'var(--text-muted)', flexShrink: 0 }}>
                          <path d="M4.646 1.646a.5.5 0 0 1 .708 0l6 6a.5.5 0 0 1 0 .708l-6 6a.5.5 0 0 1-.708-.708L10.293 8 4.646 2.354a.5.5 0 0 1 0-.708z"/>
                        </svg>
                      </div>
                    ))}
                  </div>
                )}
              </>
            ) : null}

            {/* ── Add view ── */}
            {integrationsView === 'add' ? (
              <>
                <div className="integrations-page-head">
                  <button className="btn btn-ghost btn-sm" onClick={() => setIntegrationsView('list')}>
                    ← Back
                  </button>
                  <h2>{addStep === 'picker' ? 'Choose a provider' : 'Connect GitHub'}</h2>
                  <div />
                </div>

                {addStep === 'picker' ? (
                  <div className="provider-grid">
                    <div className="provider-option" onClick={() => setAddStep('form')}>
                      <div className="provider-option-icon provider-option-icon-github">
                        <svg width="38" height="38" viewBox="0 0 16 16" fill="currentColor">
                          <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/>
                        </svg>
                      </div>
                      <div className="provider-option-name">GitHub</div>
                      <div className="provider-option-desc">Connect your GitHub account via a Personal Access Token</div>
                    </div>
                  </div>
                ) : (
                  <div className="card card-form" style={{ maxWidth: 480 }}>
                    <div className="form-group">
                      <label>Personal Access Token (PAT)</label>
                      <input
                        type="password"
                        value={integrationToken}
                        onChange={(e) => setIntegrationToken(e.target.value)}
                        placeholder="github_pat_…"
                        autoFocus
                      />
                    </div>
                    <div className="integration-hint">
                      Use a fine-grained PAT with minimal permissions: Contents (read) and Pull requests (read/write).
                    </div>
                    <div className="integration-actions">
                      <button
                        className="btn btn-primary"
                        disabled={!integrationToken.trim() || integrationSaving}
                        onClick={async () => {
                          if (!integrationToken.trim()) return;
                          setIntegrationSaving(true);
                          setStatusError('');
                          try {
                            await connectGitHub(integrationToken.trim());
                            setIntegrationToken('');
                            await loadIntegrations();
                            setIntegrationsView('list');
                          } catch (err: unknown) {
                            setStatusError(err instanceof Error ? err.message : 'Failed to connect GitHub');
                          } finally {
                            setIntegrationSaving(false);
                          }
                        }}
                      >
                        {integrationSaving ? 'Connecting…' : 'Connect GitHub'}
                      </button>
                      <button className="btn btn-ghost" onClick={() => setAddStep('picker')}>
                        Back
                      </button>
                    </div>
                  </div>
                )}
              </>
            ) : null}

            {/* ── Detail view ── */}
            {integrationsView === 'detail' ? (() => {
              const provider = integrationProviders.find((p) => p.provider_id === detailProviderId);
              if (!provider) return null;
              return (
                <>
                  <div className="integrations-page-head">
                    <button
                      className="btn btn-ghost btn-sm"
                      onClick={() => { setDetailProviderId(''); setIntegrationsView('list'); }}
                    >
                      ← Back
                    </button>
                    <div className="integration-detail-title">
                      <div className="integration-detail-icon integration-detail-icon-github">
                        <svg width="20" height="20" viewBox="0 0 16 16" fill="currentColor">
                          <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/>
                        </svg>
                      </div>
                      <h2>{provider.name}</h2>
                    </div>
                    <div />
                  </div>

                  {/* Connection info */}
                  {provider.connection ? (
                    <div className="card card-detail">
                      <h3 style={{ fontWeight: 600, marginBottom: 12 }}>Connection</h3>
                      <div className="integration-details">
                        <div className="integration-detail-item">
                          <span className="integration-detail-label">GitHub Login</span>
                          <span className="integration-detail-value">{provider.connection.external_login}</span>
                        </div>
                        <div className="integration-detail-item">
                          <span className="integration-detail-label">Account Type</span>
                          <span className="integration-detail-value">{provider.connection.account_type}</span>
                        </div>
                        <div className="integration-detail-item">
                          <span className="integration-detail-label">Status</span>
                          <span className="integration-detail-value">{provider.connection.status}</span>
                        </div>
                        <div className="integration-detail-item">
                          <span className="integration-detail-label">Revision</span>
                          <span className="integration-detail-value">{provider.connection.revision}</span>
                        </div>
                      </div>
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
                        <div className="integration-hint">
                          Use a fine-grained PAT with minimal permissions: Contents (read) and Pull requests (read/write).
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
                            onClick={async () => {
                              setIntegrationSaving(true);
                              setStatusError('');
                              try {
                                await disconnectGitHub();
                                await loadIntegrations();
                                setDetailProviderId('');
                                setIntegrationsView('list');
                              } catch (err: unknown) {
                                setStatusError(err instanceof Error ? err.message : 'Failed to disconnect GitHub');
                              } finally {
                                setIntegrationSaving(false);
                              }
                            }}
                            disabled={integrationSaving}
                          >
                            {integrationSaving ? 'Disconnecting…' : 'Disconnect'}
                          </button>
                        </div>
                      </div>
                    </div>
                  ) : null}

                  {/* Agents */}
                  <div className="card card-detail">
                    <h3 style={{ fontWeight: 600, marginBottom: 4 }}>Agents</h3>
                    <p style={{ fontSize: '0.84rem', color: 'var(--text-muted)', marginBottom: 12 }}>
                      Enable this integration per agent. Restart the agent after toggling for changes to take effect.
                    </p>
                    {agents.length === 0 ? (
                      <div className="empty-state" style={{ padding: '8px 0 0' }}>No agents found.</div>
                    ) : (
                      <div className="integration-agents-list">
                        {agents.map((agent) => {
                          const binding = provider.agent_bindings.find((b) => b.agent_id === agent.id);
                          return (
                            <div key={agent.id} className="integration-agent-row">
                              <div className="integration-agent-info">
                                <div className="integration-agent-name">{agent.name}</div>
                                <div className="integration-agent-meta">
                                  <span className="tag">{agent.runtime}</span>
                                  <span className={`state-dot ${agent.state}`} title={agent.state} />
                                  {agent.state === 'running' && binding?.enabled ? (
                                    <span style={{ fontSize: '0.72rem', color: 'var(--warning)' }}>restart to apply changes</span>
                                  ) : null}
                                </div>
                              </div>
                              <label className="toggle">
                                <input
                                  type="checkbox"
                                  checked={binding?.enabled ?? false}
                                  disabled={!provider.connected || integrationAction === agent.id}
                                  onChange={(e) => handleToggleAgentIntegration(agent.id, e.target.checked)}
                                />
                                <span className="toggle-slider" />
                              </label>
                            </div>
                          );
                        })}
                      </div>
                    )}
                  </div>
                </>
              );
            })() : null}

          </div>
        </div>
      ) : null}
    </main>
  );
}
