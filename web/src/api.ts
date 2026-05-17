type LoginResponse = {
  token?: string;
  user?: User;
};

export type User = {
  id: string;
  username: string;
  email?: string;
  display_name?: string;
  tenant_id?: string;
  team_ids?: string[];
  roles?: string[];
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
  provider?: string;
  runtime_id?: string;
  runtime_url: string;
};

export type ActivityEntry = {
  time: string;
  type: string;
  actor_id?: string;
  agent_id?: string;
  message?: string;
  details?: Record<string, unknown>;
};

export type AdminOverview = {
  identity_provider: string;
  sandbox_provider: string;
  runtime_images: Record<string, string>;
  users: Array<{
    email: string;
    subject: string;
    display_name?: string;
    tenant_id: string;
    team_ids?: string[];
    roles?: string[];
    groups?: string[];
  }>;
  activity: ActivityEntry[];
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

export async function login(username: string, password: string): Promise<{ token: string; user: User }> {
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
  if (!data.user) {
    throw new Error('Login response missing user');
  }
  return { token: data.token, user: data.user };
}

export async function listAgents(token: string): Promise<Agent[]> {
  const response = await fetch('/api/agents', { headers: { Authorization: `Bearer ${token}` } });
  if (!response.ok) {
    throw new Error(`List agents failed (${response.status})`);
  }
  return (await response.json()) as Agent[];
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

export async function getActivity(token: string): Promise<ActivityEntry[]> {
  const response = await fetch('/api/activity', { headers: { Authorization: `Bearer ${token}` } });
  if (!response.ok) {
    throw new Error(`Activity failed (${response.status})`);
  }
  const data = (await response.json()) as { activity?: ActivityEntry[] };
  return data.activity ?? [];
}

export async function getAdminOverview(token: string): Promise<AdminOverview> {
  const response = await fetch('/api/admin/overview', { headers: { Authorization: `Bearer ${token}` } });
  if (!response.ok) {
    throw new Error(`Admin overview failed (${response.status})`);
  }
  return (await response.json()) as AdminOverview;
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
