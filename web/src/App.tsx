import { useEffect, useMemo, useRef, useState } from 'react';
import { createAgent, login, startAgent, streamAgentChat, type Agent, type StartAgentResponse, type StreamEnvelope } from './api';

type EventItem = {
  id: string;
  value: unknown;
};

const DEV_USERNAME = 'admin';
const DEV_PASSWORD = 'admin';

export default function App() {
  const [token, setToken] = useState('');
  const [agent, setAgent] = useState<Agent | null>(null);
  const [agentName, setAgentName] = useState('Demo OpenClaw');
  const [runtime, setRuntime] = useState('openclaw');
  const [runtimeStart, setRuntimeStart] = useState<StartAgentResponse | null>(null);
  const [message, setMessage] = useState('Hello from browser. Show me the demo runtime stream.');
  const [events, setEvents] = useState<EventItem[]>([]);
  const [status, setStatus] = useState<'idle' | 'authenticating' | 'working' | 'streaming' | 'ready' | 'error'>('idle');
  const [error, setError] = useState('');
  const closeRef = useRef<null | (() => void)>(null);

  const connectionLabel = useMemo(() => {
    if (status === 'streaming') return 'Streaming';
    if (status === 'working') return 'Working';
    if (token && agent && runtimeStart) return 'Runtime token issued';
    if (token && agent) return 'Agent created';
    if (token) return 'Authenticated';
    return 'Disconnected';
  }, [agent, runtimeStart, status, token]);

  const runtimeCommand = useMemo(() => {
    if (!agent || !runtimeStart) return '';
    const url = `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}${runtimeStart.runtime_url}`;
    return `go run ./cmd/shclop-runtime --gateway ${url} --agent-id ${agent.id} --token ${runtimeStart.runtime_token} --runtime ${runtimeStart.runtime}`;
  }, [agent, runtimeStart]);

  useEffect(() => () => closeRef.current?.(), []);

  async function handleLogin() {
    setStatus('authenticating');
    setError('');
    try {
      const nextToken = await login(DEV_USERNAME, DEV_PASSWORD);
      setToken(nextToken);
      setStatus('ready');
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
      setAgent(created);
      setStatus('ready');
    } catch (err) {
      setStatus('error');
      setError(err instanceof Error ? err.message : 'Create agent failed');
    }
  }

  async function handleStartAgent() {
    if (!token || !agent) return;
    setStatus('working');
    setError('');
    try {
      const started = await startAgent(token, agent.id, runtime);
      setAgent(started.agent);
      setRuntimeStart(started);
      setStatus('ready');
    } catch (err) {
      setStatus('error');
      setError(err instanceof Error ? err.message : 'Start agent failed');
    }
  }

  function handleSend() {
    if (!agent || !runtimeStart || !message.trim()) return;

    closeRef.current?.();
    setStatus('streaming');
    setError('');

    let stop = () => {};
    let completed = false;
    stop = streamAgentChat(agent.id, message.trim(), (event: StreamEnvelope) => {
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
        <div className="eyebrow">Shclop functional demo</div>
        <h1>Browser → Gateway → Runtime</h1>
        <p>
          Create an agent, start it, run the printed runtime command, then send a browser task and watch the runtime stream back events.
        </p>

        <div className="status-row">
          <span className={`pill ${status}`}>{connectionLabel}</span>
          {token ? <span className="token">Session ready</span> : <span className="token muted">No session yet</span>}
          {agent ? <span className="token">Agent {agent.id}</span> : <span className="token muted">No agent</span>}
        </div>
      </section>

      <section className="grid three">
        <div className="card controls">
          <div className="section-label">1. Login</div>
          <button className="button secondary" onClick={handleLogin} disabled={status === 'authenticating'}>
            {status === 'authenticating' ? 'Logging in…' : 'Login as dev admin'}
          </button>
          <div className="hint">Uses {DEV_USERNAME}/{DEV_PASSWORD} against <code>/api/auth/login</code>.</div>
        </div>

        <div className="card controls">
          <div className="section-label">2. Agent</div>
          <label className="input-wrap">
            <span>Name</span>
            <input value={agentName} onChange={(e) => setAgentName(e.target.value)} />
          </label>
          <button className="button secondary" onClick={handleCreateAgent} disabled={!token || status === 'working'}>
            Create agent
          </button>
          {agent ? <div className="hint">State: <code>{agent.state}</code></div> : <div className="hint">Creates platform state through <code>/api/agents</code>.</div>}
        </div>

        <div className="card controls">
          <div className="section-label">3. Runtime</div>
          <label className="input-wrap">
            <span>Runtime image flavor</span>
            <select value={runtime} onChange={(e) => setRuntime(e.target.value)}>
              <option value="openclaw">openclaw</option>
              <option value="nanoclaw">nanoclaw</option>
              <option value="nemoclaw">nemoclaw</option>
            </select>
          </label>
          <button className="button secondary" onClick={handleStartAgent} disabled={!agent || status === 'working'}>
            Start agent
          </button>
          <div className="hint">Issues a short demo runtime token and waits for <code>/runtime/ws</code>.</div>
        </div>
      </section>

      {runtimeCommand ? (
        <section className="card command-card">
          <div className="section-label">4. Run runtime process</div>
          <p>Run this in another terminal from the repository root, then send a chat message below.</p>
          <pre className="command">{runtimeCommand}</pre>
        </section>
      ) : null}

      <section className="grid">
        <div className="card controls">
          <div className="section-label">5. Chat task</div>
          <label className="textarea-wrap">
            <span>Message</span>
            <textarea value={message} onChange={(e) => setMessage(e.target.value)} rows={6} />
          </label>
          <button className="button primary" onClick={handleSend} disabled={!runtimeStart || status === 'streaming'}>
            {status === 'streaming' ? 'Streaming…' : 'Send to runtime'}
          </button>
        </div>

        <section className="card feed">
          <div className="section-header">
            <div>
              <div className="section-label">Streamed events</div>
              <h2>Runtime output</h2>
            </div>
            {error ? <span className="error">{error}</span> : null}
          </div>

          <div className="events">
            {events.length === 0 ? (
              <div className="empty">No events yet.</div>
            ) : (
              events.map((event) => (
                <pre key={event.id} className="event">
                  {JSON.stringify(event.value, null, 2)}
                </pre>
              ))
            )}
          </div>
        </section>
      </section>
    </main>
  );
}
