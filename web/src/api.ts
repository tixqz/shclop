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
  tenant_id?: string;
  name: string;
  model?: string;
  purpose?: string;
  tags?: string[];
  state: string;
  latest_revision_id?: string;
  active_revision_id?: string;
  security_status?: string;
  created_at: string;
};

export type Workspace = {
  id: string;
  owner_id: string;
  name: string;
  description?: string;
  created_at: string;
  updated_at: string;
};

export type Skill = {
  id: string;
  owner_id: string;
  tenant_id?: string;
  name: string;
  source_url?: string;
  tags?: string[];
  latest_revision_id: string;
  active_revision_id: string;
  security_status?: string;
  created_at: string;
  updated_at: string;
};

export type CreateSkillInput = {
  name: string;
  source_url?: string;
  content?: string;
  tags?: string[];
};

export type SecurityPolicy = {
  mode: string;
  version: number;
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

export async function listWorkspaces(token: string): Promise<Workspace[]> {
  const response = await fetch('/api/workspaces', { headers: { Authorization: `Bearer ${token}` } });
  if (!response.ok) throw new Error(`List workspaces failed (${response.status})`);
  return (await response.json()) as Workspace[];
}

export async function createWorkspace(token: string, name: string, description?: string): Promise<Workspace> {
  const response = await fetch('/api/workspaces', {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, description }),
  });
  if (!response.ok) throw new Error(`Create workspace failed (${response.status})`);
  return (await response.json()) as Workspace;
}

export async function listSkills(token: string): Promise<Skill[]> {
  const response = await fetch('/api/skills', { headers: { Authorization: `Bearer ${token}` } });
  if (!response.ok) throw new Error(`List skills failed (${response.status})`);
  return (await response.json()) as Skill[];
}

export async function createSkill(token: string, input: CreateSkillInput): Promise<Skill> {
  const response = await fetch('/api/skills', {
    method: 'POST',
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  });
  if (!response.ok) throw new Error(`Create skill failed (${response.status})`);
  return (await response.json()) as Skill;
}

export async function getSecurityPolicy(token: string): Promise<SecurityPolicy> {
  const response = await fetch('/api/security/policy', { headers: { Authorization: `Bearer ${token}` } });
  if (!response.ok) throw new Error(`Get security policy failed (${response.status})`);
  return (await response.json()) as SecurityPolicy;
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
