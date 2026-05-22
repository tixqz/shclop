// ── API contract for the simplified OpenClaw self-hosted control plane ──
// Types mirror the backend responses exactly. Bearer token stored in localStorage.

export type User = {
  id: string;
  username: string;
  role: 'admin' | 'user';
  disabled: boolean;
  created_at: string;
  updated_at: string;
};

export type Agent = {
  id: string;
  owner_user_id: string;
  name: string;
  runtime: 'openclaw' | 'nanoclaw';
  model: string;
  state: string;
  created_at: string;
  updated_at: string;
  last_error?: string;
};

export type LLMModel = {
  id: string;
  display_name: string;
  provider_model: string;
  enabled: boolean;
  created_at: string;
  updated_at: string;
};

export type LLMGatewaySettings = {
  enabled: boolean;
  base_url: string;
  secret_name: string;
  secret_key: string;
  updated_at: string;
};

export type AdminOverview = {
  runtime: {
    provider: string;
    namespace: string;
    runtime_class_name: string;
    images: Record<string, string>;
  };
  observability: {
    metrics_enabled: boolean;
    logging_enabled: boolean;
    grafana_url: string;
  };
  health: {
    healthz: string;
    readyz: string;
  };
};

// ── Token helpers ──

const TOKEN_KEY = 'shclop_token';

export function getStoredToken(): string {
  return localStorage.getItem(TOKEN_KEY) ?? '';
}

export function storeToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY);
}

// ── Authenticated fetch wrapper ──

async function apiFetch<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const token = getStoredToken();
  const headers: Record<string, string> = {
    ...(options.headers as Record<string, string> ?? {}),
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  // Only set Content-Type if we have a body that isn't FormData
  if (options.body && typeof options.body === 'string') {
    headers['Content-Type'] = 'application/json';
  }

  const res = await fetch(path, { ...options, headers });

  if (!res.ok) {
    let msg = `Request failed (${res.status})`;
    try {
      const body = await res.json();
      if (body.error) msg = body.error;
      else if (body.message) msg = body.message;
    } catch {
      // ignore parse errors
    }
    throw new Error(msg);
  }

  // 204 No Content
  if (res.status === 204) return undefined as unknown as T;

  return (await res.json()) as T;
}

// ── Auth ──

export async function login(
  username: string,
  password: string,
): Promise<{ user: User; token: string }> {
  const data = await apiFetch<{ user: User; token: string }>('/api/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  });
  storeToken(data.token);
  return data;
}

export async function getMe(): Promise<{ user: User }> {
  return apiFetch<{ user: User }>('/api/me');
}

// ── Agents ──

export async function listAgents(): Promise<Agent[]> {
  return apiFetch<Agent[]>('/api/agents');
}

export async function createAgent(body: {
  name: string;
  runtime: string;
  model: string;
}): Promise<Agent> {
  return apiFetch<Agent>('/api/agents', {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export async function startAgent(id: string): Promise<Agent> {
  return apiFetch<Agent>(`/api/agents/${encodeURIComponent(id)}/start`, {
    method: 'POST',
  });
}

export async function stopAgent(id: string): Promise<Agent> {
  return apiFetch<Agent>(`/api/agents/${encodeURIComponent(id)}/stop`, {
    method: 'POST',
  });
}

// ── Admin: Users ──

export async function adminListUsers(): Promise<User[]> {
  return apiFetch<User[]>('/api/admin/users');
}

export async function adminCreateUser(body: {
  username: string;
  password: string;
  role: string;
}): Promise<User> {
  return apiFetch<User>('/api/admin/users', {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export async function adminPatchUser(
  id: string,
  body: { disabled?: boolean; role?: string },
): Promise<User> {
  return apiFetch<User>(`/api/admin/users/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  });
}

// ── Admin: Models ──

export async function adminListModels(): Promise<LLMModel[]> {
  return apiFetch<LLMModel[]>('/api/admin/models');
}

export async function adminCreateModel(body: {
  display_name: string;
  provider_model: string;
  enabled?: boolean;
}): Promise<LLMModel> {
  return apiFetch<LLMModel>('/api/admin/models', {
    method: 'POST',
    body: JSON.stringify(body),
  });
}

export async function adminPatchModel(
  id: string,
  body: { display_name?: string; provider_model?: string; enabled?: boolean },
): Promise<LLMModel> {
  return apiFetch<LLMModel>(`/api/admin/models/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(body),
  });
}

// ── Admin: LLM Gateway ──

export async function adminGetGateway(): Promise<LLMGatewaySettings> {
  return apiFetch<LLMGatewaySettings>('/api/admin/llm-gateway');
}

export async function adminPatchGateway(
  body: Partial<LLMGatewaySettings>,
): Promise<LLMGatewaySettings> {
  return apiFetch<LLMGatewaySettings>('/api/admin/llm-gateway', {
    method: 'PATCH',
    body: JSON.stringify(body),
  });
}

// ── Admin: Overview ──

export async function adminGetOverview(): Promise<AdminOverview> {
  return apiFetch<AdminOverview>('/api/admin/overview');
}

// ── Health ──

export async function getHealthz(): Promise<string> {
  const res = await fetch('/healthz');
  return res.text();
}

export async function getReadyz(): Promise<string> {
  const res = await fetch('/readyz');
  return res.text();
}

// ── WebSocket chat ──

export type ChatEvent = {
  type: string;
  text?: string;
  error?: string;
  done?: boolean;
  [key: string]: unknown;
};

export function connectChat(
  agentId: string,
  _token: string,
  onEvent: (ev: ChatEvent) => void,
  onError: (err: string) => void,
  onClose: () => void,
): () => void {
  const params = new URLSearchParams({ agent_id: agentId });
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const url = `${protocol}//${window.location.host}/ws?${params}`;

  const ws = new WebSocket(url);

  ws.onopen = () => {
    // ready to send
  };

  ws.onmessage = (msg) => {
    try {
      const data = JSON.parse(msg.data) as ChatEvent;
      onEvent(data);
    } catch {
      onError('Invalid message from server');
    }
  };

  ws.onerror = () => {
    onError('WebSocket connection error');
  };

  ws.onclose = () => {
    onClose();
  };

  return () => {
    if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
      ws.close();
    }
  };
}

export function sendChatMessage(
  ws: WebSocket,
  text: string,
): void {
  ws.send(JSON.stringify({ text }));
}
