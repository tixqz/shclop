type LoginResponse = {
  token?: string;
  user?: unknown;
};

export type Agent = {
  id: string;
  owner_id: string;
  name: string;
  state: string;
  created_at: string;
};

export type StartAgentResponse = {
  agent: Agent;
  runtime: string;
  runtime_token: string;
  runtime_url: string;
};

export type StreamEnvelope = {
  type?: string;
  agent_id?: string;
  session_id?: string;
  message_id?: string;
  seq?: number;
  payload?: Record<string, unknown>;
  [key: string]: unknown;
};

function wsUrl(path: string): string {
  const url = new URL(path, window.location.href);
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:';
  return url.toString();
}

export async function login(username: string, password: string): Promise<string> {
  const response = await fetch('/api/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });

  if (!response.ok) {
    throw new Error(`Login failed (${response.status})`);
  }

  const data = (await response.json()) as LoginResponse;
  if (!data.token) {
    throw new Error('Login response missing token');
  }
  return data.token;
}

export async function createAgent(token: string, name: string): Promise<Agent> {
  const response = await fetch('/api/agents', {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ name }),
  });
  if (!response.ok) {
    throw new Error(`Create agent failed (${response.status})`);
  }
  return (await response.json()) as Agent;
}

export async function startAgent(token: string, agentID: string, runtime: string): Promise<StartAgentResponse> {
  const response = await fetch(`/api/agents/${encodeURIComponent(agentID)}/start`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ runtime }),
  });
  if (!response.ok) {
    throw new Error(`Start agent failed (${response.status})`);
  }
  return (await response.json()) as StartAgentResponse;
}

type StreamLifecycle = {
  onOpen?: () => void;
  onError?: (message: string) => void;
  onClose?: () => void;
};

export function streamAgentChat(
  agentID: string,
  text: string,
  onEvent: (event: StreamEnvelope) => void,
  lifecycle: StreamLifecycle = {},
) {
  const socket = new WebSocket(wsUrl('/ws'));
  const envelope: StreamEnvelope = {
    type: 'user.message',
    agent_id: agentID,
    session_id: `session-${crypto.randomUUID?.() ?? fallbackId()}`,
    message_id: `message-${crypto.randomUUID?.() ?? fallbackId()}`,
    payload: { text },
  };

  socket.addEventListener('open', () => {
    lifecycle.onOpen?.();
    socket.send(JSON.stringify(envelope));
  });

  socket.addEventListener('message', (event) => {
    try {
      onEvent(JSON.parse(event.data as string));
    } catch {
      onEvent({ type: 'stream.error', payload: { text: 'Invalid JSON event' } });
    }
  });

  socket.addEventListener('error', () => {
    lifecycle.onError?.('WebSocket connection error');
  });

  socket.addEventListener('close', () => {
    lifecycle.onClose?.();
  });

  return () => {
    if (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING) {
      socket.close();
    }
  };
}

function fallbackId(): string {
  return Math.random().toString(36).slice(2, 10);
}
