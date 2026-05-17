import { useEffect, useMemo, useRef, useState } from 'react';
import {
  createAgent,
  getActivity,
  getAdminOverview,
  listAgents,
  login,
  startAgent,
  streamAgentChat,
  type ActivityEntry,
  type AdminOverview,
  type Agent,
  type StartAgentResponse,
  type StreamEnvelope,
  type User,
} from './api';

type EventItem = {
  id: string;
  value: unknown;
};

const DEV_USERNAME = 'bob@acme.test';
const DEV_PASSWORD = 'bob';

export default function App() {
  const [token, setToken] = useState('');
  const [user, setUser] = useState<User | null>(null);
  const [username, setUsername] = useState(DEV_USERNAME);
  const [password, setPassword] = useState(DEV_PASSWORD);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [selectedAgentID, setSelectedAgentID] = useState('');
  const [agentName, setAgentName] = useState('Bob Research Agent');
  const [runtime, setRuntime] = useState('openclaw');
  const [runtimeStart, setRuntimeStart] = useState<StartAgentResponse | null>(null);
  const [message, setMessage] = useState('Hello from Bob. Show me the demo runtime stream.');
  const [events, setEvents] = useState<EventItem[]>([]);
  const [activity, setActivity] = useState<ActivityEntry[]>([]);
  const [adminOverview, setAdminOverview] = useState<AdminOverview | null>(null);
  const [view, setView] = useState<'member' | 'admin'>('member');
  const [status, setStatus] = useState<'idle' | 'authenticating' | 'working' | 'streaming' | 'ready' | 'error'>('idle');
  const [error, setError] = useState('');
  const closeRef = useRef<null | (() => void)>(null);

  const selectedAgent = useMemo(() => agents.find((candidate) => candidate.id === selectedAgentID) ?? null, [agents, selectedAgentID]);
  const isAdmin = Boolean(user?.roles?.includes('admin'));
  const connectionLabel = useMemo(() => {
    if (status === 'streaming') return 'Streaming';
    if (status === 'working') return 'Working';
    if (token && selectedAgent && runtimeStart) return 'Runtime started';
    if (token && selectedAgent) return 'Agent selected';
    if (token) return 'Authenticated';
    return 'Disconnected';
  }, [runtimeStart, selectedAgent, status, token]);

  useEffect(() => () => closeRef.current?.(), []);

  async function refreshMemberData(nextToken = token) {
    if (!nextToken) return;
    const [nextAgents, nextActivity] = await Promise.all([listAgents(nextToken), getActivity(nextToken)]);
    setAgents(nextAgents);
    setActivity(nextActivity);
    if (!selectedAgentID && nextAgents[0]) {
      setSelectedAgentID(nextAgents[0].id);
    }
  }

  async function refreshAdminData(nextToken = token) {
    if (!nextToken || !isAdmin) return;
    setAdminOverview(await getAdminOverview(nextToken));
  }

  async function handleLogin() {
    setStatus('authenticating');
    setError('');
    setAdminOverview(null);
    try {
      const result = await login(username, password);
      setToken(result.token);
      setUser(result.user);
      setView(result.user.roles?.includes('admin') ? 'admin' : 'member');
      setStatus('ready');
      if (result.user.roles?.includes('member')) {
        const nextAgents = await listAgents(result.token);
        setAgents(nextAgents);
        setSelectedAgentID(nextAgents[0]?.id ?? '');
        setActivity(await getActivity(result.token));
      } else {
        setAgents([]);
        setSelectedAgentID('');
        setActivity([]);
      }
      if (result.user.roles?.includes('admin')) {
        setAdminOverview(await getAdminOverview(result.token));
      }
    } catch (err) {
      setStatus('error');
      setError(err instanceof Error ? err.message : 'Login failed');
    }
  }

  async function handleCreateAgent() {
    if (!token || !agentName.trim()) return;
    setStatus('working');
    setError('');
    setEvents([]);
    setRuntimeStart(null);
    try {
      const created = await createAgent(token, agentName.trim());
      const nextAgents = await listAgents(token);
      setAgents(nextAgents);
      setSelectedAgentID(created.id);
      setActivity(await getActivity(token));
      setStatus('ready');
    } catch (err) {
      setStatus('error');
      setError(err instanceof Error ? err.message : 'Create agent failed');
    }
  }

  async function handleStartAgent() {
    if (!token || !selectedAgent) return;
    setStatus('working');
    setError('');
    try {
      const started = await startAgent(token, selectedAgent.id, runtime);
      setRuntimeStart(started);
      const nextAgents = await listAgents(token);
      setAgents(nextAgents);
      setActivity(await getActivity(token));
      setStatus('ready');
    } catch (err) {
      setStatus('error');
      setError(err instanceof Error ? err.message : 'Start agent failed');
    }
  }

  function handleSend() {
    if (!selectedAgent || !runtimeStart || !message.trim()) return;

    closeRef.current?.();
    setStatus('streaming');
    setError('');

    let stop = () => {};
    let completed = false;
    stop = streamAgentChat(selectedAgent.id, message.trim(), (event: StreamEnvelope) => {
      setEvents((current) => [
        ...current,
        { id: `${Date.now()}-${current.length}`, value: event },
      ]);

      if (event.type === 'message.error') {
        completed = true;
        setStatus('error');
        setError(String(event.payload?.text ?? 'Runtime error'));
        stop();
        closeRef.current = null;
      }

      if (event.type === 'message.done') {
        completed = true;
        setStatus('ready');
        setError('');
        refreshMemberData().catch(() => undefined);
        stop();
        closeRef.current = null;
      }
    }, {
      onError: (message) => {
        completed = true;
        setStatus('error');
        setError(message);
        stop();
        closeRef.current = null;
      },
      onClose: () => {
        if (!completed) {
          setStatus('error');
          setError('Stream closed unexpectedly');
        }
        closeRef.current = null;
      },
    });

    closeRef.current = stop;
  }

  return (
    <main className="shell">
      <section className="hero card">
        <div className="eyebrow">Shclop control plane</div>
        <h1>{user ? `Welcome, ${user.display_name || user.username}` : 'Login to manage agents'}</h1>
        <p>
          Members manage their own agents and sessions. Admins inspect the platform environment in a separate read-only area.
        </p>

        <div className="status-row">
          <span className={`pill ${status}`}>{connectionLabel}</span>
          {user ? <span className="token">Tenant {user.tenant_id || 'local'}</span> : <span className="token muted">No session yet</span>}
          {user?.roles?.map((role) => <span key={role} className="token">{role}</span>)}
          {selectedAgent ? <span className="token">Agent {selectedAgent.id}</span> : <span className="token muted">No agent selected</span>}
        </div>
      </section>

      {!token ? (
        <section className="card controls login-card">
          <div className="section-label">Login</div>
          <label className="input-wrap">
            <span>Username</span>
            <input value={username} onChange={(e) => setUsername(e.target.value)} />
          </label>
          <label className="input-wrap">
            <span>Password</span>
            <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} />
          </label>
          <button className="button primary" onClick={handleLogin} disabled={status === 'authenticating'}>
            {status === 'authenticating' ? 'Logging in…' : 'Login'}
          </button>
          <div className="hint">Member: bob@acme.test/bob. Admin: alice@acme.test/alice. Local fallback: admin/admin.</div>
          {error ? <span className="error">{error}</span> : null}
        </section>
      ) : null}

      {token ? (
        <>
          <nav className="tabs card">
            {user?.roles?.includes('member') ? <button className={`tab ${view === 'member' ? 'active' : ''}`} onClick={() => setView('member')}>Member dashboard</button> : null}
            {isAdmin ? <button className={`tab ${view === 'admin' ? 'active' : ''}`} onClick={() => { setView('admin'); refreshAdminData().catch(() => undefined); }}>Admin area</button> : null}
          </nav>

          {view === 'member' && user?.roles?.includes('member') ? (
            <section className="grid dashboard-grid">
              <div className="card controls">
                <div className="section-label">Profile</div>
                <dl className="facts">
                  <dt>User</dt><dd>{user?.email || user?.username}</dd>
                  <dt>Tenant</dt><dd>{user?.tenant_id || 'local'}</dd>
                  <dt>Teams</dt><dd>{user?.team_ids?.join(', ') || 'none'}</dd>
                  <dt>Roles</dt><dd>{user?.roles?.join(', ') || 'member'}</dd>
                </dl>
              </div>

              <div className="card controls">
                <div className="section-label">My agents</div>
                <label className="input-wrap">
                  <span>New agent name</span>
                  <input value={agentName} onChange={(e) => setAgentName(e.target.value)} />
                </label>
                <button className="button secondary" onClick={handleCreateAgent} disabled={status === 'working'}>Create agent</button>
                <div className="agent-list">
                  {agents.length === 0 ? <div className="empty">No agents yet.</div> : agents.map((candidate) => (
                    <button key={candidate.id} className={`agent-row ${candidate.id === selectedAgentID ? 'selected' : ''}`} onClick={() => { setSelectedAgentID(candidate.id); setRuntimeStart(null); setEvents([]); }}>
                      <strong>{candidate.name}</strong>
                      <span>{candidate.state}</span>
                    </button>
                  ))}
                </div>
              </div>

              <div className="card controls">
                <div className="section-label">Runtime</div>
                <label className="input-wrap">
                  <span>Runtime image flavor</span>
                  <select value={runtime} onChange={(e) => setRuntime(e.target.value)}>
                    <option value="openclaw">openclaw</option>
                    <option value="nanoclaw">nanoclaw</option>
                    <option value="nemoclaw">nemoclaw</option>
                  </select>
                </label>
                <button className="button secondary" onClick={handleStartAgent} disabled={!selectedAgent || status === 'working'}>Start selected agent</button>
                {runtimeStart ? <div className="hint">Provider: <code>{runtimeStart.provider ?? 'mock'}</code>{runtimeStart.runtime_id ? <> · Runtime id: <code>{runtimeStart.runtime_id}</code></> : null}</div> : <div className="hint">Docker demo starts containers from the backend. Mock mode only issues a runtime lease.</div>}
              </div>

              <div className="card controls">
                <div className="section-label">Chat task</div>
                <label className="textarea-wrap">
                  <span>Message</span>
                  <textarea value={message} onChange={(e) => setMessage(e.target.value)} rows={6} />
                </label>
                <button className="button primary" onClick={handleSend} disabled={!runtimeStart || status === 'streaming'}>{status === 'streaming' ? 'Streaming…' : 'Send to runtime'}</button>
                {error ? <span className="error">{error}</span> : null}
              </div>

              <section className="card feed">
                <div className="section-label">Current session events</div>
                <div className="events">
                  {events.length === 0 ? <div className="empty">No runtime events yet.</div> : events.map((event) => <pre key={event.id} className="event">{JSON.stringify(event.value, null, 2)}</pre>)}
                </div>
              </section>

              <ActivityCard title="My activity" entries={activity} />
            </section>
          ) : null}

          {view === 'admin' && isAdmin ? <AdminArea overview={adminOverview} /> : null}
        </>
      ) : null}
    </main>
  );
}

function AdminArea({ overview }: { overview: AdminOverview | null }) {
  if (!overview) {
    return <section className="card controls"><div className="empty">Loading admin overview…</div></section>;
  }
  return (
    <section className="grid dashboard-grid">
      <div className="card controls">
        <div className="section-label">Environment</div>
        <dl className="facts">
          <dt>Identity provider</dt><dd>{overview.identity_provider}</dd>
          <dt>Sandbox provider</dt><dd>{overview.sandbox_provider}</dd>
        </dl>
      </div>

      <div className="card controls">
        <div className="section-label">Runtime catalog</div>
        <dl className="facts">
          {Object.entries(overview.runtime_images).map(([name, image]) => <><dt key={`${name}-dt`}>{name}</dt><dd key={`${name}-dd`}>{image}</dd></>)}
        </dl>
      </div>

      <div className="card controls wide">
        <div className="section-label">Users / tenants / teams</div>
        <div className="table-list">
          {overview.users.length === 0 ? <div className="empty">No mock users exposed.</div> : overview.users.map((adminUser) => (
            <div className="table-row" key={adminUser.subject}>
              <strong>{adminUser.display_name || adminUser.email}</strong>
              <span>{adminUser.email}</span>
              <span>tenant: {adminUser.tenant_id}</span>
              <span>teams: {adminUser.team_ids?.join(', ') || 'none'}</span>
              <span>roles: {adminUser.roles?.join(', ') || 'none'}</span>
            </div>
          ))}
        </div>
      </div>

      <ActivityCard title="System activity" entries={overview.activity} />
    </section>
  );
}

function ActivityCard({ title, entries }: { title: string; entries: ActivityEntry[] }) {
  return (
    <section className="card feed">
      <div className="section-label">{title}</div>
      <div className="events">
        {entries.length === 0 ? <div className="empty">No activity yet.</div> : entries.slice().reverse().map((entry, index) => (
          <pre className="event" key={`${entry.time}-${entry.type}-${index}`}>{JSON.stringify(entry, null, 2)}</pre>
        ))}
      </div>
    </section>
  );
}
