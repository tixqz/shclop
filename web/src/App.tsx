import { useEffect, useMemo, useRef, useState } from 'react';
import { login, streamMockChatWithLifecycle } from './api';

type EventItem = {
  id: string;
  value: unknown;
};

const DEV_USERNAME = 'admin';
const DEV_PASSWORD = 'admin';

export default function App() {
  const [token, setToken] = useState('');
  const [message, setMessage] = useState('Show me the mock stream.');
  const [events, setEvents] = useState<EventItem[]>([]);
  const [status, setStatus] = useState<'idle' | 'authenticating' | 'streaming' | 'ready' | 'error'>('idle');
  const [error, setError] = useState('');
  const closeRef = useRef<null | (() => void)>(null);

  const connectionLabel = useMemo(() => {
    if (status === 'streaming') return 'Streaming';
    if (token) return 'Authenticated';
    return 'Disconnected';
  }, [status, token]);

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

  function handleSend() {
    if (!message.trim()) return;

    closeRef.current?.();
    setStatus('streaming');
    setError('');

    let stop = () => {};
    let completed = false;
    stop = streamMockChatWithLifecycle(message.trim(), (event) => {
      setEvents((current) => [
        ...current,
        { id: `${Date.now()}-${current.length}`, value: event },
      ]);

      if ((event as { type?: string }).type === 'message.done') {
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
        <div className="eyebrow">Shclop</div>
        <h1>Control plane foundation</h1>
        <p>
          Login as the dev admin, send a message, and watch the mock runtime stream back events.
        </p>

        <div className="status-row">
          <span className={`pill ${status}`}>{connectionLabel}</span>
          {token ? <span className="token">Token acquired</span> : <span className="token muted">No token yet</span>}
        </div>
      </section>

      <section className="grid">
        <div className="card controls">
          <div className="section-label">Authentication</div>
          <button className="button secondary" onClick={handleLogin} disabled={status === 'authenticating'}>
            {status === 'authenticating' ? 'Logging in…' : 'Login as dev admin'}
          </button>
          <div className="hint">Uses {DEV_USERNAME}/{DEV_PASSWORD} against <code>/api/auth/login</code>.</div>
        </div>

        <div className="card controls">
          <div className="section-label">Mock chat</div>
          <label className="textarea-wrap">
            <span>Message</span>
            <textarea value={message} onChange={(e) => setMessage(e.target.value)} rows={6} />
          </label>
          <button className="button primary" onClick={handleSend} disabled={status === 'streaming'}>
            {status === 'streaming' ? 'Streaming…' : 'Send'}
          </button>
        </div>
      </section>

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
    </main>
  );
}
