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

type HomeEventSeverity = 'normal' | 'warning' | 'urgent';

type HomeEventCategory = 'chat' | 'agent' | 'integration' | 'policy' | 'file' | 'schedule' | 'system';

type HomeEvent = {
  id: string;
  workspaceID: string;
  workspaceName: string;
  subject: string;
  action: string;
  detail: string;
  time: string;
  severity: HomeEventSeverity;
  category: HomeEventCategory;
  sourceOrder: number;
};

type WorkspaceChat = {
  id: string;
  title: string;
  goal: string;
  primaryAgent: string;
  allowedContext: string[];
  safetyPreset: SafetyPreset;
  status: 'ready' | 'draft';
};

type WorkspaceProgressStatus = 'done' | 'in progress' | 'blocked' | 'planned';

type WorkspaceProgressTask = {
  id: string;
  label: string;
  status: WorkspaceProgressStatus;
  percent: number;
  detail: string;
};

type Workspace = {
  id: string;
  name: string;
  description: string;
  health: 'active' | 'review' | 'draft';
  agents: Array<{ name: string; role: string; state: string }>;
  integrations: Array<{ name: string; status: string }>;
  chats: WorkspaceChat[];
  activity: Array<{ time: string; message: string }>;
};

type GlobalAgent = {
  id: string;
  name: string;
  model?: string;
  tags: string[];
  purpose: string;
  state: 'ready' | 'draft' | 'idle' | 'review' | 'scheduled';
  linkedWorkspaceIDs: string[];
};

type GlobalSkill = {
  id: string;
  name: string;
  description: string;
  tags: string[];
  content: string;
  source: 'manual' | 'url';
  sourceLabel?: string;
};

type GlobalAgentDraft = {
  name: string;
  model: string;
  tags: string;
  purpose: string;
  state: GlobalAgent['state'];
};

type GlobalSkillDraft = {
  name: string;
  description: string;
  tags: string;
  content: string;
};

type AgentCatalogMode = 'create' | 'view' | 'edit';
type SkillCatalogMode = 'create' | 'view' | 'edit';

type SafetyPreset = 'Chat only' | 'Read-only research' | 'Draft with approvals';

const WORKSPACE_CONTEXT_OPTIONS = ['Recent activity', 'Integration status', 'Shared notes', 'Workspace notes'] as const;
const SAFETY_PRESETS: Array<{ label: SafetyPreset; description: string }> = [
  { label: 'Chat only', description: 'Fast back-and-forth without workspace reads' },
  { label: 'Read-only research', description: 'Can inspect workspace context, but not edit it' },
  { label: 'Draft with approvals', description: 'Can prepare drafts, pending human approval' },
];

const ADMIN_TABS = ['Integrations', 'MCP', 'Federations', 'Audit logs', 'Policies', 'Runtimes', 'Models'] as const;
type AdminTab = (typeof ADMIN_TABS)[number];

const ADMIN_INTEGRATIONS = [
  { name: 'Slack', status: 'enabled', auth: 'OAuth bot + events', scope: 'Workspace channels and approvals', policy: 'Tenant-scoped only' },
  { name: 'Google Drive', status: 'enabled', auth: 'Service account', scope: 'Shared docs and notes', policy: 'Read-only by default' },
  { name: 'CRM', status: 'restricted', auth: 'Delegated tenant token', scope: 'Account notes and pipeline context', policy: 'Approval required for writes' },
  { name: 'Zendesk', status: 'enabled', auth: 'OAuth / support app', scope: 'Tickets and macros', policy: 'Support-only workspaces' },
  { name: 'GitHub', status: 'restricted', auth: 'App installation', scope: 'Issues, PRs, and release notes', policy: 'No repo writes without approval' },
  { name: 'Statuspage', status: 'disabled', auth: 'API token vault', scope: 'Incident updates and public status', policy: 'Planned for ops workspaces' },
] as const;

const ADMIN_MCP_SERVERS = [
  { name: 'docs-mcp', transport: 'http', tools: 6, scopes: 'Docs search and note extraction', approval: 'auto for read-only', status: 'ready' },
  { name: 'browser-mcp', transport: 'stdio', tools: 9, scopes: 'Web research and page capture', approval: 'human approval on external writes', status: 'ready' },
  { name: 'ticket-mcp', transport: 'sse', tools: 7, scopes: 'Ticket summarization and routing', approval: 'workspace owner approval', status: 'restricted' },
  { name: 'release-mcp', transport: 'http', tools: 5, scopes: 'Release notes and changelog drafting', approval: 'locked behind policy', status: 'planned' },
] as const;

const ADMIN_FEDERATIONS = [
  { name: 'LDAP', status: 'enabled', mapping: 'groups -> tenant roles', notes: 'Primary enterprise directory mapping' },
  { name: 'Keycloak / OIDC', status: 'enabled', mapping: 'claims -> teams', notes: 'Supports multi-tenant SSO' },
  { name: 'Header auth', status: 'restricted', mapping: 'reverse proxy headers', notes: 'Only for trusted internal hops' },
  { name: 'Mock YAML', status: 'enabled', mapping: 'config/identity.mock.yaml', notes: 'Demo identity source' },
  { name: 'SCIM', status: 'planned', mapping: 'HRIS / directory sync', notes: 'User lifecycle automation later' },
] as const;

const ADMIN_POLICIES = [
  { name: 'Network egress', value: 'deny by default', detail: 'Allowlist only for approved destinations' },
  { name: 'Runtime isolation', value: 'Kata / microVM', detail: 'Guest compromise must not reach host' },
  { name: 'Approval gates', value: 'required for writes', detail: 'External side effects need explicit review' },
  { name: 'Resource limits', value: 'CPU/RAM/disk caps', detail: 'Per-agent limits and tenant budgets' },
  { name: 'Workspace size', value: 'bounded', detail: 'Prevent noisy or oversized workspaces' },
  { name: 'Secret boundaries', value: 'broker-only', detail: 'Secrets stay behind typed platform brokers' },
] as const;

const ADMIN_MODELS = [
  { provider: 'OpenAI broker', models: 'GPT-4.1, GPT-4.1-mini', routing: 'high accuracy routes to GPT-4.1', boundary: 'credentials stay in LLM Broker' },
  { provider: 'Anthropic broker', models: 'Claude 3.7, Claude Haiku', routing: 'drafting and summarization fallback', boundary: 'tenant-scoped tokens only' },
  { provider: 'Local gateway', models: 'shclop-embed, shclop-draft', routing: 'private, low-risk tasks', boundary: 'internal-only deployment' },
] as const;

const DEV_USERNAME = 'bob@acme.test';
const DEV_PASSWORD = 'bob';

const INITIAL_WORKSPACES: Workspace[] = [
  {
    id: 'ws-research',
    name: 'Market research hub',
    description: 'Coordinates research agents, CRM context, source rules, and reviewer handoffs for go-to-market work.',
    health: 'active',
    agents: [
      { name: 'Bob Research Agent', role: 'primary analyst', state: 'ready' },
      { name: 'Source Scout', role: 'web evidence', state: 'scheduled' },
      { name: 'Deck Writer', role: 'summary drafts', state: 'idle' },
    ],
    integrations: [
      { name: 'Slack', status: 'connected' },
      { name: 'Google Drive', status: 'connected' },
      { name: 'CRM', status: 'pending' },
    ],
    chats: [
      {
        id: 'chat-competitor-analysis',
        title: 'Competitor analysis',
        goal: 'Compare competitors and surface launch risks for the go-to-market team.',
        primaryAgent: 'Bob Research Agent',
        allowedContext: ['Recent activity', 'Integration status'],
        safetyPreset: 'Read-only research',
        status: 'ready',
      },
      {
        id: 'chat-q2-deck-draft',
        title: 'Q2 deck draft',
        goal: 'Draft a Q2 deck outline from the latest research and internal notes.',
        primaryAgent: 'Deck Writer',
        allowedContext: ['Shared notes', 'Workspace notes'],
        safetyPreset: 'Draft with approvals',
        status: 'draft',
      },
    ],
    activity: [
      { time: '09:42', message: 'Source Scout queued 12 links for review' },
      { time: '10:15', message: 'Bob Research Agent summarized launch risks' },
      { time: '10:28', message: 'Rule check blocked uncited paragraph' },
    ],
  },
  {
    id: 'ws-ops',
    name: 'Support operations',
    description: 'A future workspace for ticket triage agents, Slack routing, runbooks, and SLA audit trails.',
    health: 'review',
    agents: [
      { name: 'Ticket Triage', role: 'classifier', state: 'ready' },
      { name: 'Runbook Helper', role: 'incident copilot', state: 'review' },
    ],
    integrations: [
      { name: 'Zendesk', status: 'connected' },
      { name: 'Slack', status: 'connected' },
      { name: 'Statuspage', status: 'planned' },
    ],
    chats: [
      {
        id: 'chat-ticket-triage',
        title: 'Ticket triage',
        goal: 'Cluster incoming tickets and route likely payments issues to the right responder.',
        primaryAgent: 'Ticket Triage',
        allowedContext: ['Recent activity', 'Integration status'],
        safetyPreset: 'Read-only research',
        status: 'ready',
      },
      {
        id: 'chat-incident-notes',
        title: 'Incident notes',
        goal: 'Turn incident updates into a concise timeline for the on-call lead.',
        primaryAgent: 'Runbook Helper',
        allowedContext: ['Workspace notes', 'Recent activity'],
        safetyPreset: 'Draft with approvals',
        status: 'draft',
      },
    ],
    activity: [
      { time: 'Yesterday', message: 'Runbook Helper drafted incident checklist' },
      { time: 'Today', message: 'Ticket Triage grouped 8 payment issues' },
    ],
  },
];

const INITIAL_GLOBAL_AGENTS: GlobalAgent[] = [
  {
    id: 'agent-bob-research',
    name: 'Bob Research Agent',
    model: 'GPT-4.1',
    tags: ['research', 'evidence', 'analysis'],
    purpose: 'Collect sources, compare competitors, and turn findings into launch-ready notes.',
    state: 'ready',
    linkedWorkspaceIDs: ['ws-research'],
  },
  {
    id: 'agent-source-scout',
    name: 'Source Scout',
    model: 'Claude 3.7',
    tags: ['web', 'sources', 'verification'],
    purpose: 'Find supporting sources, verify claims, and queue links for review.',
    state: 'scheduled',
    linkedWorkspaceIDs: ['ws-research'],
  },
  {
    id: 'agent-deck-writer',
    name: 'Deck Writer',
    model: 'GPT-4.1-mini',
    tags: ['drafting', 'summary', 'slides'],
    purpose: 'Draft concise deck outlines and turn research into presentation-ready copy.',
    state: 'idle',
    linkedWorkspaceIDs: ['ws-research'],
  },
  {
    id: 'agent-ticket-triage',
    name: 'Ticket Triage',
    model: 'Claude Haiku',
    tags: ['support', 'triage', 'routing'],
    purpose: 'Cluster incoming tickets, identify likely owners, and route urgent issues.',
    state: 'ready',
    linkedWorkspaceIDs: ['ws-ops'],
  },
  {
    id: 'agent-runbook-helper',
    name: 'Runbook Helper',
    model: 'GPT-4.1-mini',
    tags: ['incident', 'runbook', 'ops'],
    purpose: 'Turn incident updates into timelines, checklists, and approval-ready drafts.',
    state: 'review',
    linkedWorkspaceIDs: ['ws-ops'],
  },
];

const INITIAL_GLOBAL_SKILLS: GlobalSkill[] = [
  {
    id: 'skill-research-brief',
    name: 'Research brief writer',
    description: 'Turns source notes into decision-ready research briefs.',
    tags: ['research', 'brief', 'summary'],
    content: 'Focus on evidence quality, compare options, and end with a short recommendation.',
    source: 'manual',
    sourceLabel: 'Created in UI',
  },
  {
    id: 'skill-incident-timeline',
    name: 'Incident timeline',
    description: 'Converts rough incident updates into a readable chronology.',
    tags: ['ops', 'timeline', 'incident'],
    content: 'Extract timestamps, note owners, highlight blockers, and preserve follow-up items.',
    source: 'manual',
    sourceLabel: 'Created in UI',
  },
  {
    id: 'skill-workspace-kickoff',
    name: 'Workspace kickoff',
    description: 'Shapes new workspace context into a short starter plan.',
    tags: ['workspace', 'planning', 'handoff'],
    content: 'Summarize goals, identify the first agent to use, and propose the next 3 actions.',
    source: 'manual',
    sourceLabel: 'Created in UI',
  },
];

function makeID(prefix: string) {
  return `${prefix}-${Math.random().toString(36).slice(2, 9)}`;
}

function userInitials(user: User | null) {
  const label = user?.display_name || user?.email || user?.username || 'User';
  return label
    .split(/\s+|@|\./)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase())
    .join('') || 'U';
}

function userDisplayName(user: User | null) {
  return user?.display_name || user?.email || user?.username || 'Workspace user';
}

function userRoleLabel(user: User | null) {
  const roles = user?.roles?.length ? user.roles : ['workspace user'];
  return roles.map((role) => role.replace(/_/g, ' ')).join(' · ');
}

function userOrganizationLabel(user: User | null) {
  if (!user?.tenant_id && !user?.team_ids?.length) return 'No organization assigned';
  const teamPart = user.team_ids?.length ? ` · ${user.team_ids.join(', ')}` : '';
  return `${user.tenant_id ?? 'No tenant'}${teamPart}`;
}

function deriveChatTitle(goal: string) {
  const trimmed = goal.trim();
  if (!trimmed) return 'New chat';
  const firstSentence = trimmed.split(/[.!?]/)[0]?.trim() ?? trimmed;
  return firstSentence.length > 42 ? `${firstSentence.slice(0, 39).trimEnd()}…` : firstSentence;
}

function latestWorkspaceSignal(workspace: Workspace) {
  return workspace.activity[workspace.activity.length - 1]?.message ?? 'No recent activity';
}

function ellipsize(text: string, max = 86) {
  const trimmed = text.trim();
  if (trimmed.length <= max) return trimmed;
  return `${trimmed.slice(0, max - 1).trimEnd()}…`;
}

function statusPercent(status: WorkspaceProgressStatus) {
  switch (status) {
    case 'done': return 100;
    case 'in progress': return 65;
    case 'blocked': return 32;
    case 'planned': return 14;
  }
}

function progressDetailForChat(chat: WorkspaceChat) {
  if (chat.status === 'ready') {
    return `${chat.primaryAgent} has a clean draft ready for review.`;
  }
  return `${chat.primaryAgent} is drafting with ${chat.allowedContext.join(' · ') || 'workspace context'}.`;
}

function deriveWorkspaceProgress(workspace: Workspace): WorkspaceProgressTask[] {
  const tasks: WorkspaceProgressTask[] = [];

  for (const chat of workspace.chats.slice(0, 2)) {
    tasks.push({
      id: `${workspace.id}-${chat.id}`,
      label: chat.title,
      status: chat.status === 'ready' ? 'done' : 'in progress',
      percent: chat.status === 'ready' ? 100 : 65,
      detail: progressDetailForChat(chat),
    });
  }

  const pendingIntegration = workspace.integrations.find((integration) => integration.status !== 'connected');
  if (pendingIntegration) {
    tasks.push({
      id: `${workspace.id}-integration-${pendingIntegration.name}`,
      label: pendingIntegration.name,
      status: pendingIntegration.status === 'planned' ? 'planned' : 'blocked',
      percent: pendingIntegration.status === 'planned' ? 14 : 32,
      detail: pendingIntegration.status === 'planned'
        ? 'Connector is on the roadmap and waiting for enablement.'
        : 'Connector is waiting on configuration or approval.',
    });
  } else if (workspace.activity[0]) {
    tasks.push({
      id: `${workspace.id}-activity`,
      label: 'Workspace follow-up',
      status: 'in progress',
      percent: workspace.health === 'active' ? 72 : 40,
      detail: ellipsize(latestWorkspaceSignal(workspace)),
    });
  }

  if (tasks.length < 3) {
    tasks.push({
      id: `${workspace.id}-stabilize`,
      label: 'Workspace stabilization',
      status: workspace.health === 'draft' ? 'planned' : 'in progress',
      percent: workspace.health === 'draft' ? 10 : 58,
      detail: workspace.health === 'draft'
        ? 'Workspace is still being shaped.'
        : 'Keep the workspace moving with the next handoff.',
    });
  }

  if (tasks.length < 2) {
    tasks.push({
      id: `${workspace.id}-kickoff`,
      label: 'Kickoff plan',
      status: 'planned',
      percent: 8,
      detail: 'Seed the workspace with the first task and owner.',
    });
  }

  return tasks.slice(0, 3);
}

function chatStatusLabel(status: WorkspaceProgressStatus) {
  return status;
}

function describeAuditAction(entry: ActivityEntry) {
  if (entry.type === 'auth.login') return 'Signed in';
  if (entry.type === 'auth.login_failed') return 'Login blocked';
  if (entry.type === 'agent.created') return 'Created agent';
  if (entry.type === 'agent.start_requested') return 'Requested runtime';
  if (entry.type === 'sandbox.started') return 'Sandbox started';
  if (entry.type === 'sandbox.start_failed') return 'Sandbox failed to start';
  if (entry.type === 'runtime.connected') return 'Runtime connected';
  if (entry.type === 'runtime.disconnected') return 'Runtime disconnected';
  if (entry.type === 'task.routed') return 'Routed task';
  return entry.type.replace(/\./g, ' ');
}

function describeAuditTarget(entry: ActivityEntry) {
  const name = entry.details?.name || entry.message;
  if (typeof name === 'string' && name.trim()) return ellipsize(name, 54);
  return 'Platform activity';
}

function auditSeverity(entry: ActivityEntry) {
  if (/failed|blocked|error/i.test(entry.type) || /failed|blocked|error/i.test(entry.message ?? '')) return 'urgent';
  if (/requested|started|routed/i.test(entry.type)) return 'warning';
  return 'normal';
}

function toEventSeverity(text: string): HomeEventSeverity {
  if (/blocked|failed|pending review|needs review|review/i.test(text)) return 'urgent';
  if (/pending|planned|draft|queued|waiting/i.test(text)) return 'warning';
  return 'normal';
}

function splitSubjectAndAction(message: string) {
  const text = message.trim();
  if (!text) return { subject: 'Workspace', action: 'updated' };

  const verbMatch = text.match(/\b(queued|summarized|blocked|drafted|grouped|created|started|pending|planned|waiting|reviewed|routed|synced|connected|disconnected)\b/i);
  if (!verbMatch || verbMatch.index == null) {
    return { subject: 'Workspace', action: text };
  }

  const subject = text.slice(0, verbMatch.index).trim().replace(/[:—-]+$/u, '').replace(/\s+/g, ' ');
  const action = text.slice(verbMatch.index).trim();
  return {
    subject: subject || 'Workspace',
    action,
  };
}

function activityToHomeEvent(entry: ActivityEntry, agents: Agent[]): HomeEvent | null {
  if (entry.type === 'auth.login' || entry.type === 'auth.login_failed' || entry.type === 'runtime.connected' || entry.type === 'runtime.disconnected') {
    return null;
  }

  const agentName = entry.agent_id ? agents.find((agent) => agent.id === entry.agent_id)?.name : undefined;
  const detailFromMessage = entry.message?.trim() || entry.type;

  if (entry.type === 'agent.created') {
    return {
      id: `${entry.time}-${entry.type}`,
      workspaceID: 'backend',
      workspaceName: 'Backend activity',
      subject: String(entry.details?.['name'] ?? agentName ?? 'New agent'),
      action: 'created',
      detail: 'Agent added to the workspace.',
      time: entry.time,
      severity: 'normal',
      category: 'agent',
      sourceOrder: 300,
    };
  }

  if (entry.type === 'agent.start_requested') {
    return {
      id: `${entry.time}-${entry.type}`,
      workspaceID: 'backend',
      workspaceName: 'Backend activity',
      subject: agentName || 'Agent',
      action: 'start queued',
      detail: 'Runtime start is in progress.',
      time: entry.time,
      severity: 'warning',
      category: 'agent',
      sourceOrder: 290,
    };
  }

  if (entry.type === 'sandbox.started') {
    return {
      id: `${entry.time}-${entry.type}`,
      workspaceID: 'backend',
      workspaceName: 'Backend activity',
      subject: agentName || 'Agent runtime',
      action: `started on ${String(entry.details?.runtime ?? 'runtime')}`,
      detail: 'The agent runtime is ready.',
      time: entry.time,
      severity: 'normal',
      category: 'agent',
      sourceOrder: 280,
    };
  }

  if (entry.type === 'sandbox.start_failed') {
    return {
      id: `${entry.time}-${entry.type}`,
      workspaceID: 'backend',
      workspaceName: 'Backend activity',
      subject: agentName || 'Agent runtime',
      action: 'start blocked',
      detail: 'The runtime could not be started.',
      time: entry.time,
      severity: 'urgent',
      category: 'agent',
      sourceOrder: 260,
    };
  }

  if (entry.type === 'task.routed') {
    return {
      id: `${entry.time}-${entry.type}`,
      workspaceID: 'backend',
      workspaceName: 'Backend activity',
      subject: agentName || 'Agent chat',
      action: 'task routed to runtime',
      detail: 'The next chat turn was handed off for execution.',
      time: entry.time,
      severity: 'normal',
      category: 'chat',
      sourceOrder: 270,
    };
  }

  return {
    id: `${entry.time}-${entry.type}`,
    workspaceID: 'backend',
    workspaceName: 'Backend activity',
    subject: 'System',
    action: detailFromMessage,
    detail: 'Background activity updated.',
    time: entry.time,
    severity: 'normal',
    category: 'system',
    sourceOrder: 100,
  };
}

function workspaceActivityToHomeEvents(workspace: Workspace, baseOrder: number): HomeEvent[] {
  const events: HomeEvent[] = [];

  for (const [index, entry] of [...workspace.activity].reverse().entries()) {
    if (/^(today|yesterday|now)$/i.test(entry.message.trim())) {
      continue;
    }

    const { subject, action } = splitSubjectAndAction(entry.message);
    const severity = toEventSeverity(entry.message);
    const category: HomeEventCategory = /policy|blocked/i.test(entry.message)
      ? 'policy'
      : /link|drive|file/i.test(entry.message)
        ? 'file'
        : /schedule|queued/i.test(entry.message)
          ? 'schedule'
          : /integration|sync|connected|pending|planned/i.test(entry.message)
            ? 'integration'
            : /chat|draft/i.test(entry.message)
              ? 'chat'
              : 'agent';

    events.push({
      id: `${workspace.id}-activity-${index}`,
      workspaceID: workspace.id,
      workspaceName: workspace.name,
      subject,
      action,
      detail: severity === 'urgent'
        ? 'Needs review before the next step continues.'
        : severity === 'warning'
          ? 'This is waiting on a follow-up.'
          : 'Recent workspace activity.',
      time: entry.time,
      severity,
      category,
      sourceOrder: baseOrder - index,
    });
  }

  for (const [index, integration] of workspace.integrations.entries()) {
    if (integration.status === 'connected') continue;
    const severity: HomeEventSeverity = integration.status === 'planned' ? 'warning' : 'urgent';
    events.push({
      id: `${workspace.id}-integration-${integration.name}`,
      workspaceID: workspace.id,
      workspaceName: workspace.name,
      subject: integration.name,
      action: integration.status,
      detail: integration.status === 'planned'
        ? 'Planned integration waiting for setup.'
        : 'Integration is waiting for review.',
      time: 'Now',
      severity,
      category: 'integration',
      sourceOrder: baseOrder - 20 - index,
    });
  }

  for (const [index, chat] of workspace.chats.entries()) {
    if (chat.status !== 'draft') continue;
    events.push({
      id: `${workspace.id}-chat-${chat.id}`,
      workspaceID: workspace.id,
      workspaceName: workspace.name,
      subject: chat.title,
      action: 'waiting review',
      detail: 'Draft chat is ready when the workspace owner approves it.',
      time: 'Now',
      severity: 'warning',
      category: 'chat',
      sourceOrder: baseOrder - 40 - index,
    });
  }

  return events;
}

function formatHomeEventAction(event: HomeEvent) {
  return `${event.subject} · ${event.action}`;
}

function normalizeTags(raw: string) {
  return raw
    .split(',')
    .map((tag) => tag.trim())
    .filter(Boolean)
    .filter((tag, index, list) => list.indexOf(tag) === index);
}

function deriveWorkspaceRoleFromAgent(agent: GlobalAgent) {
  const blob = `${agent.purpose} ${agent.tags.join(' ')}`.toLowerCase();
  if (blob.includes('incident') || blob.includes('runbook') || blob.includes('ops')) return 'incident copilot';
  if (blob.includes('support') || blob.includes('ticket') || blob.includes('triage')) return 'triage';
  if (blob.includes('deck') || blob.includes('slides') || blob.includes('summary')) return 'drafting';
  if (blob.includes('research') || blob.includes('evidence') || blob.includes('analysis')) return 'research lead';
  if (agent.tags[0]) return agent.tags[0];
  return 'workspace helper';
}

function resolveWorkspaceNames(workspaces: Workspace[], workspaceIDs: string[]) {
  const map = new Map(workspaces.map((workspace) => [workspace.id, workspace.name]));
  return workspaceIDs.map((workspaceID) => map.get(workspaceID) ?? 'Unknown workspace');
}

function makeSkillFromUrl(url: string): GlobalSkill {
  let label = url.trim();
  try {
    label = new URL(label).hostname || label;
  } catch {
    // keep raw input as the label when URL parsing fails; this is UI-only for now.
  }

  return {
    id: makeID('skill'),
    name: `${label} import`,
    description: `Imported from ${url.trim()}`,
    tags: ['imported', 'external'],
    content: `Imported from ${url.trim()}\n\nTODO: wire a backend skill import endpoint and normalize instructions here.`,
    source: 'url',
    sourceLabel: url.trim(),
  };
}

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
  const [view, setView] = useState<'home' | 'workspaces' | 'agents' | 'skills' | 'admin'>('home');
  const [adminTab, setAdminTab] = useState<AdminTab>('Integrations');
  const [workspaces, setWorkspaces] = useState<Workspace[]>(() => INITIAL_WORKSPACES);
  const [selectedWorkspaceID, setSelectedWorkspaceID] = useState(INITIAL_WORKSPACES[0]?.id ?? '');
  const [globalAgents, setGlobalAgents] = useState<GlobalAgent[]>(() => INITIAL_GLOBAL_AGENTS);
  const [globalSkills, setGlobalSkills] = useState<GlobalSkill[]>(() => INITIAL_GLOBAL_SKILLS);
  const [agentDraftName, setAgentDraftName] = useState('');
  const [agentDraftModel, setAgentDraftModel] = useState('GPT-4.1');
  const [agentDraftTags, setAgentDraftTags] = useState('research, workspace');
  const [agentDraftPurpose, setAgentDraftPurpose] = useState('Coordinate a workspace task and keep the team aligned.');
  const [agentDraftState, setAgentDraftState] = useState<GlobalAgent['state']>('draft');
  const [skillDraftName, setSkillDraftName] = useState('');
  const [skillDraftDescription, setSkillDraftDescription] = useState('');
  const [skillDraftTags, setSkillDraftTags] = useState('');
  const [skillDraftContent, setSkillDraftContent] = useState('');
  const [skillUrl, setSkillUrl] = useState('');
  const [homeFilter, setHomeFilter] = useState<'all' | 'attention'>('all');
  const [status, setStatus] = useState<'idle' | 'authenticating' | 'working' | 'streaming' | 'ready' | 'error'>('idle');
  const [error, setError] = useState('');
  const closeRef = useRef<null | (() => void)>(null);

  const selectedAgent = useMemo(() => agents.find((candidate) => candidate.id === selectedAgentID) ?? null, [agents, selectedAgentID]);
  const selectedWorkspace = useMemo(
    () => workspaces.find((workspace) => workspace.id === selectedWorkspaceID) ?? workspaces[0],
    [workspaces, selectedWorkspaceID],
  );
  const isAdmin = Boolean(user?.roles?.includes('admin'));
  const canUseWorkspaces = Boolean(user);
  const homeWorkspace = selectedWorkspace ?? workspaces[0] ?? null;
  const homeChat = homeWorkspace
    ? homeWorkspace.chats.find((chat) => chat.status === 'draft') ?? homeWorkspace.chats[homeWorkspace.chats.length - 1] ?? null
    : null;

  const homeEvents = useMemo(() => {
    const backendEvents = activity
      .slice()
      .reverse()
      .map((entry, index) => activityToHomeEvent(entry, agents))
      .filter((event): event is HomeEvent => event !== null)
      .map((event, index) => ({ ...event, sourceOrder: 1000 - index }));

    const workspaceEvents = workspaces.flatMap((workspace, index) => workspaceActivityToHomeEvents(workspace, 700 - index * 50));

    return [...backendEvents, ...workspaceEvents].sort((a, b) => b.sourceOrder - a.sourceOrder);
  }, [activity, agents, workspaces]);

  const attentionEvents = useMemo(
    () => homeEvents.filter((event) => event.severity !== 'normal').slice(0, 4),
    [homeEvents],
  );

  const visibleHomeEvents = useMemo(
    () => (homeFilter === 'attention' ? attentionEvents : homeEvents).slice(0, 10),
    [attentionEvents, homeEvents, homeFilter],
  );

  useEffect(() => () => closeRef.current?.(), []);

  useEffect(() => {
    if (selectedWorkspace && selectedWorkspace.id !== selectedWorkspaceID) {
      setSelectedWorkspaceID(selectedWorkspace.id);
    }
  }, [selectedWorkspace, selectedWorkspaceID]);

  async function refreshHomeData(nextToken = token) {
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
      setAgents([]);
      setSelectedAgentID('');
      setActivity([]);
      setView('home');
      setAdminTab('Integrations');
      setGlobalAgents(INITIAL_GLOBAL_AGENTS);
      setGlobalSkills(INITIAL_GLOBAL_SKILLS);
      setAgentDraftName('');
      setAgentDraftModel('GPT-4.1');
      setAgentDraftTags('research, workspace');
      setAgentDraftPurpose('Coordinate a workspace task and keep the team aligned.');
      setAgentDraftState('draft');
      setSkillDraftName('');
      setSkillDraftDescription('');
      setSkillDraftTags('');
      setSkillDraftContent('');
      setSkillUrl('');
      setStatus('ready');
      refreshHomeData(result.token).catch(() => undefined);
      if (result.user.roles?.includes('admin')) {
        refreshAdminData(result.token).catch(() => undefined);
      } else {
        setAdminOverview(null);
      }
    } catch (err) {
      setStatus('error');
      setError(err instanceof Error ? err.message : 'Login failed');
    }
  }

  function handleLogout() {
    closeRef.current?.();
    closeRef.current = null;
    setToken('');
    setUser(null);
    setAgents([]);
    setSelectedAgentID('');
    setRuntimeStart(null);
    setEvents([]);
    setActivity([]);
    setAdminOverview(null);
    setView('home');
    setStatus('idle');
    setError('');
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

  function handleCreateGlobalAgent(draft?: GlobalAgentDraft): boolean {
    const nextDraft = draft ?? {
      name: agentDraftName,
      model: agentDraftModel,
      tags: agentDraftTags,
      purpose: agentDraftPurpose,
      state: agentDraftState,
    };
    const trimmedName = nextDraft.name.trim();
    const trimmedPurpose = nextDraft.purpose.trim();
    if (!trimmedName || !trimmedPurpose) return false;

    const nextAgent: GlobalAgent = {
      id: makeID('agent'),
      name: trimmedName,
      model: nextDraft.model.trim() || undefined,
      tags: normalizeTags(nextDraft.tags),
      purpose: trimmedPurpose,
      state: nextDraft.state,
      linkedWorkspaceIDs: [],
    };

    // TODO: persist global agent catalog entries through backend agent catalog endpoints.
    setGlobalAgents((current) => [nextAgent, ...current]);
    setAgentDraftName('');
    setAgentDraftModel('GPT-4.1');
    setAgentDraftTags('research, workspace');
    setAgentDraftPurpose('Coordinate a workspace task and keep the team aligned.');
    setAgentDraftState('draft');
    setView('agents');
    return true;
  }

  function handleUpdateGlobalAgent(agentID: string, draft: GlobalAgentDraft): boolean {
    const trimmedName = draft.name.trim();
    const trimmedPurpose = draft.purpose.trim();
    if (!trimmedName || !trimmedPurpose) return false;

    const previousAgent = globalAgents.find((agent) => agent.id === agentID);
    const nextAgent: GlobalAgent = {
      id: agentID,
      name: trimmedName,
      model: draft.model.trim() || undefined,
      tags: normalizeTags(draft.tags),
      purpose: trimmedPurpose,
      state: draft.state,
      linkedWorkspaceIDs: previousAgent?.linkedWorkspaceIDs ?? [],
    };

    setGlobalAgents((current) => current.map((existing) => {
      if (existing.id !== agentID) return existing;
      return nextAgent;
    }));

    setWorkspaces((current) => current.map((workspace) => ({
      ...workspace,
      agents: workspace.agents.map((agent) => (
        agent.name === previousAgent?.name
          ? {
              ...agent,
              name: trimmedName,
              role: deriveWorkspaceRoleFromAgent(nextAgent),
              state: draft.state,
            }
          : agent
      )),
    })));

    return true;
  }

  function handleDeleteGlobalAgent(agentID: string): boolean {
    const targetAgent = globalAgents.find((agent) => agent.id === agentID);
    if (!targetAgent) return false;

    setGlobalAgents((current) => current.filter((agent) => agent.id !== agentID));
    setWorkspaces((current) => current.map((workspace) => ({
      ...workspace,
      agents: workspace.agents.filter((agent) => agent.name !== targetAgent.name),
    })));
    return true;
  }

  function handleCreateSkill(draft?: GlobalSkillDraft): boolean {
    const nextDraft = draft ?? {
      name: skillDraftName,
      description: skillDraftDescription,
      tags: skillDraftTags,
      content: skillDraftContent,
    };
    const trimmedName = nextDraft.name.trim();
    const trimmedDescription = nextDraft.description.trim();
    const trimmedContent = nextDraft.content.trim();
    if (!trimmedName || !trimmedDescription || !trimmedContent) return false;

    const nextSkill: GlobalSkill = {
      id: makeID('skill'),
      name: trimmedName,
      description: trimmedDescription,
      tags: normalizeTags(nextDraft.tags),
      content: trimmedContent,
      source: 'manual',
      sourceLabel: 'Created in UI',
    };

    // TODO: persist global skill creation through backend skill catalog endpoints.
    setGlobalSkills((current) => [nextSkill, ...current]);
    setSkillDraftName('');
    setSkillDraftDescription('');
    setSkillDraftTags('');
    setSkillDraftContent('');
    setView('skills');
    return true;
  }

  function handleImportSkillUrl(): boolean {
    const trimmedUrl = skillUrl.trim();
    if (!trimmedUrl) return false;

    const nextSkill = makeSkillFromUrl(trimmedUrl);
    // TODO: persist imported skills from external URLs through backend skill catalog endpoints.
    setGlobalSkills((current) => [nextSkill, ...current]);
    setSkillUrl('');
    setView('skills');
    return true;
  }

  function handleUpdateSkill(skillID: string, draft: GlobalSkillDraft): boolean {
    const trimmedName = draft.name.trim();
    const trimmedDescription = draft.description.trim();
    const trimmedContent = draft.content.trim();
    if (!trimmedName || !trimmedDescription || !trimmedContent) return false;

    setGlobalSkills((current) => current.map((existing) => {
      if (existing.id !== skillID) return existing;
      return {
        ...existing,
        name: trimmedName,
        description: trimmedDescription,
        tags: normalizeTags(draft.tags),
        content: trimmedContent,
        source: existing.source,
        sourceLabel: existing.sourceLabel,
      };
    }));

    return true;
  }

  function handleDeleteSkill(skillID: string): boolean {
    setGlobalSkills((current) => current.filter((skill) => skill.id !== skillID));
    return true;
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
        refreshHomeData().catch(() => undefined);
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
          <div className="hint">Workspace user: bob@acme.test/bob. Admin: alice@acme.test/alice. Local fallback: admin/admin.</div>
          {error ? <span className="error">{error}</span> : null}
        </section>
      ) : null}

      {token ? (
        <>
          <div className="app-topbar">
            <nav className="tabs card">
              <button className={`tab ${view === 'home' ? 'active' : ''}`} onClick={() => setView('home')}>Home</button>
              <button className={`tab ${view === 'workspaces' ? 'active' : ''}`} onClick={() => setView('workspaces')}>Workspaces</button>
              <button className={`tab ${view === 'agents' ? 'active' : ''}`} onClick={() => setView('agents')}>Agents</button>
              <button className={`tab ${view === 'skills' ? 'active' : ''}`} onClick={() => setView('skills')}>Skills</button>
              {isAdmin ? <button className={`tab ${view === 'admin' ? 'active' : ''}`} onClick={() => { setView('admin'); setAdminTab('Integrations'); refreshAdminData().catch(() => undefined); }}>Admin area</button> : null}
            </nav>

            <div className="user-menu card">
              <button className="user-chip" type="button" aria-haspopup="menu">
                <span className="user-avatar" aria-hidden="true">{userInitials(user)}</span>
                <span className="user-chip-copy">
                  <strong>{userDisplayName(user)}</strong>
                  <small>{userRoleLabel(user)}</small>
                </span>
              </button>
              <div className="user-menu-popover" role="menu">
                <div className="user-menu-meta">
                  <span>Organization</span>
                  <strong>{userOrganizationLabel(user)}</strong>
                </div>
                <div className="user-menu-meta">
                  <span>Role</span>
                  <strong>{userRoleLabel(user)}</strong>
                </div>
                <button className="user-menu-item" type="button" disabled>Settings</button>
                <button className="user-menu-item danger" type="button" onClick={handleLogout}>Log out</button>
              </div>
            </div>
          </div>

          {view === 'home' ? (
            <section className="home-layout">
              <section className="card controls home-strip">
                <div className="home-strip-head">
                  <div>
                    <div className="section-label">Home</div>
                    <h2>Continue</h2>
                  </div>
                  {homeWorkspace ? <span className={`pill ${homeWorkspace.health}`}>{homeWorkspace.health}</span> : null}
                </div>

                <div className="home-strip-grid">
                  <div className="home-strip-tile">
                    <span>Workspace</span>
                    <strong>{homeWorkspace?.name ?? 'No workspace selected'}</strong>
                    <small>{homeWorkspace ? latestWorkspaceSignal(homeWorkspace) : 'Pick a workspace to keep going.'}</small>
                  </div>
                  <div className="home-strip-tile">
                    <span>{homeChat?.status === 'draft' ? 'Next chat' : 'Recent chat'}</span>
                    <strong>{homeChat?.title ?? 'No chat yet'}</strong>
                    <small>{homeChat?.goal ?? 'Chats will surface here when a workspace has work ready.'}</small>
                  </div>
                  <div className="home-strip-tile compact">
                    <span>Open</span>
                    <strong>Workspace view</strong>
                    <small>{homeWorkspace ? `${homeWorkspace.chats.length} chats · ${homeWorkspace.agents.length} agents` : 'Jump into Workspaces for details.'}</small>
                  </div>
                </div>

                <div className="home-strip-actions">
                  <button
                    className="button primary"
                    onClick={() => {
                      if (homeWorkspace) setSelectedWorkspaceID(homeWorkspace.id);
                      setView('workspaces');
                    }}
                  >
                    Open workspace
                  </button>
                  <span className="hint">Workspaces stay in the Workspaces tab; Home stays event-first.</span>
                </div>
              </section>

              <section className="card controls home-progress-panel">
                <div className="section-header">
                  <div>
                    <div className="section-label">Workspace progress</div>
                    <h3>By workspace</h3>
                  </div>
                  <span className="token muted">Compact task progress</span>
                </div>
                <div className="workspace-progress-grid">
                  {workspaces.map((workspace) => {
                    const tasks = deriveWorkspaceProgress(workspace);
                    return (
                      <article className="workspace-progress-card" key={workspace.id}>
                        <div className="workspace-progress-card-head">
                          <div>
                            <strong>{workspace.name}</strong>
                            <small>{workspace.description}</small>
                          </div>
                          <span className={`pill ${workspace.health}`}>{workspace.health}</span>
                        </div>
                        <div className="workspace-progress-list">
                          {tasks.map((task) => (
                            <div className="workspace-progress-item" key={task.id}>
                              <div className="workspace-progress-item-top">
                                <span>{task.label}</span>
                                <div className="workspace-progress-item-meta">
                                  <em>{chatStatusLabel(task.status)}</em>
                                  <strong>{task.percent}%</strong>
                                </div>
                              </div>
                              <div className="progress-bar" aria-hidden="true">
                                <i className={task.status} style={{ width: `${statusPercent(task.status)}%` }} />
                              </div>
                              <small>{task.detail}</small>
                            </div>
                          ))}
                        </div>
                      </article>
                    );
                  })}
                </div>
              </section>

              <section className="card controls home-attention-rail">
                <div className="section-header">
                  <div>
                    <div className="section-label">Needs attention</div>
                    <h3>Pinned items</h3>
                  </div>
                  <span className="token muted">{attentionEvents.length} pinned</span>
                </div>
                <div className="attention-rail">
                  {attentionEvents.length === 0 ? <div className="empty">Nothing needs attention.</div> : attentionEvents.map((event) => (
                    <button
                      key={event.id}
                      className={`attention-card ${event.severity}`}
                      onClick={() => {
                        if (event.workspaceID !== 'backend') {
                          setSelectedWorkspaceID(event.workspaceID);
                          setView('workspaces');
                        }
                      }}
                    >
                      <div className="attention-card-top">
                        <strong>{event.workspaceName}</strong>
                        <span className={`pill ${event.severity}`}>{event.severity}</span>
                      </div>
                      <p>{formatHomeEventAction(event)}</p>
                      <small>{event.detail}</small>
                    </button>
                  ))}
                </div>
              </section>

              <section className="card controls home-events-panel">
                <div className="section-header">
                  <div>
                    <div className="section-label">Recent events</div>
                    <h3>Meaningful timeline</h3>
                  </div>
                  <div className="home-filter-tabs" role="tablist" aria-label="Home event filters">
                    <button className={`tab ${homeFilter === 'all' ? 'active' : ''}`} onClick={() => setHomeFilter('all')}>All</button>
                    <button className={`tab ${homeFilter === 'attention' ? 'active' : ''}`} onClick={() => setHomeFilter('attention')}>Needs attention</button>
                  </div>
                </div>

                <div className="event-timeline">
                  {visibleHomeEvents.length === 0 ? <div className="empty">No events yet.</div> : visibleHomeEvents.map((event) => (
                    <article className={`event-card ${event.severity}`} key={event.id}>
                      <div className="event-card-top">
                        <div className="event-card-meta">
                          <span className="event-workspace">{event.workspaceName}</span>
                          <span className="event-category">{event.category}</span>
                        </div>
                        <div className="event-card-status">
                          <span className={`pill ${event.severity}`}>{event.severity}</span>
                          <time>{event.time}</time>
                        </div>
                      </div>
                      <strong>{formatHomeEventAction(event)}</strong>
                      <span>{event.detail}</span>
                    </article>
                  ))}
                </div>
              </section>
            </section>
          ) : null}

          {view === 'workspaces' && canUseWorkspaces ? (
            <WorkspaceArea
              workspaces={workspaces}
              setWorkspaces={setWorkspaces}
              selectedWorkspace={selectedWorkspace}
              onSelectWorkspace={setSelectedWorkspaceID}
              setSelectedWorkspaceID={setSelectedWorkspaceID}
            />
          ) : null}

          {view === 'agents' && canUseWorkspaces ? (
            <AgentsArea
              agents={globalAgents}
              workspaces={workspaces}
              onCreateAgent={handleCreateGlobalAgent}
              onUpdateAgent={handleUpdateGlobalAgent}
              onDeleteAgent={handleDeleteGlobalAgent}
            />
          ) : null}

          {view === 'skills' && canUseWorkspaces ? (
            <SkillsArea
              skills={globalSkills}
              onCreateSkill={handleCreateSkill}
              onImportUrl={handleImportSkillUrl}
              onUpdateSkill={handleUpdateSkill}
              onDeleteSkill={handleDeleteSkill}
              skillUrl={skillUrl}
              setSkillUrl={setSkillUrl}
            />
          ) : null}

          {view === 'admin' && isAdmin ? <AdminArea overview={adminOverview} /> : null}
        </>
      ) : null}
    </main>
  );
}

function WorkspaceArea({
  workspaces,
  selectedWorkspace,
  onSelectWorkspace,
  setWorkspaces,
  setSelectedWorkspaceID,
}: {
  workspaces: Workspace[];
  selectedWorkspace: Workspace;
  onSelectWorkspace: (workspaceID: string) => void;
  setWorkspaces: (next: Workspace[] | ((current: Workspace[]) => Workspace[])) => void;
  setSelectedWorkspaceID: (workspaceID: string) => void;
}) {
  const [selectedSection, setSelectedSection] = useState<'chats' | 'agents' | 'events' | 'integrations'>('chats');
  const [chatSetupOpen, setChatSetupOpen] = useState(false);
  const [newWorkspaceOpen, setNewWorkspaceOpen] = useState(false);
  const [chatAgent, setChatAgent] = useState(selectedWorkspace.agents[0]?.name ?? '');
  const [chatGoal, setChatGoal] = useState(`Review ${selectedWorkspace.name} and draft the next workspace step for the team.`);
  const [allowedContext, setAllowedContext] = useState<string[]>(['Recent activity', 'Integration status']);
  const [safetyPreset, setSafetyPreset] = useState<SafetyPreset>('Read-only research');
  const [workspaceName, setWorkspaceName] = useState('');
  const [workspaceDescription, setWorkspaceDescription] = useState('');
  const hasWorkspaceAgents = selectedWorkspace.agents.length > 0;

  useEffect(() => {
    setSelectedSection('chats');
    setChatSetupOpen(false);
    setChatAgent(selectedWorkspace.agents[0]?.name ?? '');
    setChatGoal(`Review ${selectedWorkspace.name} and draft the next workspace step for the team.`);
    setAllowedContext(['Recent activity', 'Integration status']);
    setSafetyPreset('Read-only research');
  }, [selectedWorkspace.id]);

  function toggleContext(option: string) {
    setAllowedContext((current) => (
      current.includes(option)
        ? current.filter((item) => item !== option)
        : [...current, option]
    ));
  }

  function handleCreateChat() {
    if (!chatAgent.trim() || !chatGoal.trim()) return;
    const nextChat: WorkspaceChat = {
      id: makeID('chat'),
      title: deriveChatTitle(chatGoal),
      goal: chatGoal.trim(),
      primaryAgent: chatAgent.trim(),
      allowedContext: [...allowedContext],
      safetyPreset,
      status: 'ready',
    };

    // TODO: persist workspace chat/session through backend once workspace APIs exist.
    setWorkspaces((current) => current.map((workspace) => (
      workspace.id === selectedWorkspace.id
        ? { ...workspace, chats: [...workspace.chats, nextChat] }
        : workspace
    )));
    setChatSetupOpen(false);
  }

  function handleCreateWorkspaceDraft() {
    const trimmedName = workspaceName.trim();
    if (!trimmedName) return;

    const nextWorkspace: Workspace = {
      id: makeID('ws'),
      name: trimmedName,
      description: workspaceDescription.trim(),
      health: 'draft',
      agents: [],
      integrations: [],
      chats: [],
      activity: [],
    };

    // TODO: persist workspace creation through the backend once workspace APIs exist.
    setWorkspaces((current) => [nextWorkspace, ...current]);
    setSelectedWorkspaceID(nextWorkspace.id);
    setWorkspaceName('');
    setWorkspaceDescription('');
    setNewWorkspaceOpen(false);
  }

  function closeWorkspaceDraft() {
    setWorkspaceName('');
    setWorkspaceDescription('');
    setNewWorkspaceOpen(false);
  }

  return (
    <section className="workspace-layout">
      <aside className="card controls workspace-sidebar">
        <div className="workspace-sidebar-head">
          <div>
            <div className="section-label">Workspaces</div>
            <h2>Workspace library</h2>
          </div>
          <button className="button primary workspace-create-button" onClick={() => setNewWorkspaceOpen((current) => !current)}>
            New workspace
          </button>
        </div>

        {newWorkspaceOpen ? (
          <div className="workspace-create-form">
            <label className="input-wrap">
              <span>Workspace name</span>
              <input value={workspaceName} onChange={(e) => setWorkspaceName(e.target.value)} placeholder="Growth programs" />
            </label>
            <label className="textarea-wrap">
              <span>Description</span>
              <textarea
                value={workspaceDescription}
                onChange={(e) => setWorkspaceDescription(e.target.value)}
                rows={3}
                placeholder="Optional context for the team"
              />
            </label>
            <div className="workspace-create-actions">
              <button className="button primary" onClick={handleCreateWorkspaceDraft}>Create workspace</button>
              <button className="button secondary" onClick={() => { closeWorkspaceDraft(); setNewWorkspaceOpen(false); }}>Cancel</button>
            </div>
          </div>
        ) : null}

        <div className="workspace-list">
          {workspaces.map((workspace) => (
            <button
              key={workspace.id}
              className={`workspace-row ${workspace.id === selectedWorkspace.id ? 'selected' : ''}`}
              onClick={() => onSelectWorkspace(workspace.id)}
            >
              <span>
                <strong>{workspace.name}</strong>
                <small>{workspace.agents.length} agents · {workspace.chats.length} chats · {workspace.integrations.length} integrations</small>
              </span>
            </button>
          ))}
        </div>
      </aside>

      <section className="workspace-main">
        <div className="card workspace-tabs">
          <button className={`workspace-tab ${selectedSection === 'chats' ? 'active' : ''}`} onClick={() => setSelectedSection('chats')}>Chats <span>{selectedWorkspace.chats.length}</span></button>
          <button className={`workspace-tab ${selectedSection === 'agents' ? 'active' : ''}`} onClick={() => setSelectedSection('agents')}>Agents <span>{selectedWorkspace.agents.length}</span></button>
          <button className={`workspace-tab ${selectedSection === 'events' ? 'active' : ''}`} onClick={() => setSelectedSection('events')}>Events <span>{selectedWorkspace.activity.length}</span></button>
          <button className={`workspace-tab ${selectedSection === 'integrations' ? 'active' : ''}`} onClick={() => setSelectedSection('integrations')}>Integrations <span>{selectedWorkspace.integrations.length}</span></button>
        </div>

        {selectedSection === 'chats' ? (
          <section className="card controls workspace-section-card">
            <div className="workspace-chat-head compact">
              <div>
                <div className="section-label">Chat setup</div>
                <h3>Create a chat in this workspace</h3>
              </div>
              <button className="button secondary workspace-chat-cta" onClick={() => setChatSetupOpen((current) => !current)}>{chatSetupOpen ? 'Hide' : 'New chat'}</button>
            </div>

            {!chatSetupOpen ? (
              <p className="panel-copy">Choose one primary agent, define the goal, and the chat lands in this workspace.</p>
            ) : (
              <div className="workspace-chat-form">
                <label className="input-wrap">
                  <span>Agent</span>
                  {hasWorkspaceAgents ? (
                    <select value={chatAgent} onChange={(e) => setChatAgent(e.target.value)}>
                      {selectedWorkspace.agents.map((agent) => <option key={agent.name} value={agent.name}>{agent.name} — {agent.role}</option>)}
                    </select>
                  ) : (
                    <input value={chatAgent} onChange={(e) => setChatAgent(e.target.value)} placeholder="Primary agent name" />
                  )}
                </label>

                <label className="textarea-wrap">
                  <span>Chat goal</span>
                  <textarea value={chatGoal} onChange={(e) => setChatGoal(e.target.value)} rows={4} />
                </label>

                <div className="workspace-chat-section">
                  <div className="section-label">Allowed context</div>
                  <div className="chip-group compact">
                    {WORKSPACE_CONTEXT_OPTIONS.map((option) => (
                      <button
                        key={option}
                        type="button"
                        className={`chip ${allowedContext.includes(option) ? 'selected' : ''}`}
                        aria-pressed={allowedContext.includes(option)}
                        onClick={() => toggleContext(option)}
                      >
                        {option}
                      </button>
                    ))}
                  </div>
                </div>

                <div className="workspace-chat-section">
                  <div className="section-label">Safety preset</div>
                  <div className="safety-grid">
                    {SAFETY_PRESETS.map((preset) => (
                      <button
                        key={preset.label}
                        type="button"
                        className={`safety-card ${safetyPreset === preset.label ? 'selected' : ''}`}
                        onClick={() => setSafetyPreset(preset.label)}
                      >
                        <strong>{preset.label}</strong>
                        <span>{preset.description}</span>
                      </button>
                    ))}
                  </div>
                </div>

                <div className="workspace-chat-actions">
                  <button className="button primary" onClick={handleCreateChat}>Create chat</button>
                  <button className="button secondary" onClick={() => setChatSetupOpen(false)}>Cancel</button>
                </div>
              </div>
            )}

            <div className="chat-list compact">
              {selectedWorkspace.chats.length === 0 ? <div className="empty">No chats yet.</div> : selectedWorkspace.chats.map((chat) => (
                <div className="chat-row compact" key={chat.id}>
                  <div className="chat-row-main">
                    <div className="chat-row-title">
                      <strong>{chat.title}</strong>
                      <span className={`pill ${chat.status === 'ready' ? 'ready' : 'idle'}`}>{chat.status}</span>
                    </div>
                    <p>{chat.goal}</p>
                  </div>
                  <div className="chat-row-meta">
                    <span><small>Primary agent</small><strong>{chat.primaryAgent}</strong></span>
                    <span><small>Safety</small><strong>{chat.safetyPreset}</strong></span>
                    <span><small>Context</small><strong>{chat.allowedContext.join(' · ') || 'none'}</strong></span>
                  </div>
                </div>
              ))}
            </div>
          </section>
        ) : null}

        {selectedSection === 'agents' ? (
          <section className="card controls workspace-section-card">
            <div className="section-label">Agents</div>
            <div className="mini-list compact">
              {selectedWorkspace.agents.length === 0 ? <div className="empty">No agents added yet.</div> : selectedWorkspace.agents.map((agent) => (
                <div className="mini-row compact" key={`${selectedWorkspace.id}-${agent.name}`}>
                  <span><strong>{agent.name}</strong><small>{agent.role}</small></span>
                  <em>{agent.state}</em>
                </div>
              ))}
            </div>
          </section>
        ) : null}

        {selectedSection === 'events' ? (
          <section className="card controls workspace-section-card">
            <div className="section-label">Events</div>
            <div className="timeline-list compact">
              {selectedWorkspace.activity.length === 0 ? <div className="empty">No activity yet.</div> : selectedWorkspace.activity.map((entry) => (
                <div className="timeline-item compact" key={`${entry.time}-${entry.message}`}>
                  <time>{entry.time}</time>
                  <span>{entry.message}</span>
                </div>
              ))}
            </div>
          </section>
        ) : null}

        {selectedSection === 'integrations' ? (
          <section className="card controls workspace-section-card">
            <div className="section-label">Integrations</div>
            <div className="integration-cloud compact">
              {selectedWorkspace.integrations.length === 0 ? <div className="empty">No integrations connected yet.</div> : selectedWorkspace.integrations.map((integration) => (
                <span className="integration-chip" key={`${selectedWorkspace.id}-${integration.name}`}>
                  {integration.name}<small>{integration.status}</small>
                </span>
              ))}
            </div>
          </section>
        ) : null}
      </section>
    </section>
  );
}

function AgentsArea({
  agents,
  workspaces,
  onCreateAgent,
  onUpdateAgent,
  onDeleteAgent,
}: {
  agents: GlobalAgent[];
  workspaces: Workspace[];
  onCreateAgent: (draft?: GlobalAgentDraft) => boolean;
  onUpdateAgent: (agentID: string, draft: GlobalAgentDraft) => boolean;
  onDeleteAgent: (agentID: string) => boolean;
}) {
  const [mode, setMode] = useState<AgentCatalogMode | null>(null);
  const [activeAgentID, setActiveAgentID] = useState('');
  const [draftName, setDraftName] = useState('');
  const [draftModel, setDraftModel] = useState('');
  const [draftTags, setDraftTags] = useState('');
  const [draftPurpose, setDraftPurpose] = useState('');
  const [draftState, setDraftState] = useState<GlobalAgent['state']>('draft');

  const activeAgent = useMemo(
    () => agents.find((agent) => agent.id === activeAgentID) ?? null,
    [activeAgentID, agents],
  );
  const linkedWorkspaces = useMemo(
    () => (activeAgent ? resolveWorkspaceNames(workspaces, activeAgent.linkedWorkspaceIDs) : []),
    [activeAgent, workspaces],
  );

  function resetDrafts() {
    setDraftName('');
    setDraftModel('');
    setDraftTags('');
    setDraftPurpose('');
    setDraftState('draft');
  }

  function openCreate() {
    resetDrafts();
    setActiveAgentID('');
    setMode('create');
  }

  function openView(agent: GlobalAgent) {
    setActiveAgentID(agent.id);
    setMode('view');
  }

  function openEdit() {
    if (!activeAgent) return;
    setDraftName(activeAgent.name);
    setDraftModel(activeAgent.model ?? '');
    setDraftTags(activeAgent.tags.join(', '));
    setDraftPurpose(activeAgent.purpose);
    setDraftState(activeAgent.state);
    setMode('edit');
  }

  function closeModal() {
    setMode(null);
    setActiveAgentID('');
    resetDrafts();
  }

  useEffect(() => {
    if (mode !== 'edit' || !activeAgent) return;
    setDraftName(activeAgent.name);
    setDraftModel(activeAgent.model ?? '');
    setDraftTags(activeAgent.tags.join(', '));
    setDraftPurpose(activeAgent.purpose);
    setDraftState(activeAgent.state);
  }, [activeAgent, mode]);

  useEffect(() => {
    if (mode === 'create') resetDrafts();
  }, [mode]);

  useEffect(() => {
    if (activeAgentID && !activeAgent) {
      closeModal();
    }
  }, [activeAgent, activeAgentID]);

  function submitAgent() {
    const draft = {
      name: draftName,
      model: draftModel,
      tags: draftTags,
      purpose: draftPurpose,
      state: draftState,
    } satisfies GlobalAgentDraft;

    const saved = mode === 'create'
      ? onCreateAgent(draft)
      : activeAgent
        ? onUpdateAgent(activeAgent.id, draft)
        : false;

    if (saved) {
      closeModal();
    }
  }

  const stateClass = activeAgent?.state === 'ready' ? 'ready' : activeAgent?.state === 'review' ? 'warning' : 'idle';

  return (
    <section className="catalog-layout">
      <div className="section-header catalog-header">
        <div>
          <div className="section-label">Agents</div>
          <h2>Global agents</h2>
        </div>
        <div className="catalog-header-actions">
          <span className="pill">{agents.length} total</span>
          <button className="button primary" onClick={openCreate}>Add agent</button>
        </div>
      </div>

      <div className="catalog-list" role="list" aria-label="Agents list">
        {agents.length === 0 ? (
          <div className="empty">No agents yet.</div>
        ) : agents.map((agent) => {
          const workspaceNames = resolveWorkspaceNames(workspaces, agent.linkedWorkspaceIDs);
          return (
            <button className="catalog-row" key={agent.id} type="button" onClick={() => openView(agent)}>
              <span className="catalog-row-main">
                <strong>{agent.name}</strong>
                <small>{agent.purpose}</small>
              </span>
              <span className="catalog-row-cell">{agent.model || '—'}</span>
              <span className="catalog-row-cell">{agent.tags.length ? agent.tags.join(' · ') : 'No tags'}</span>
              <span className="catalog-row-cell">{workspaceNames.length ? `${workspaceNames.length} · ${ellipsize(workspaceNames.join(' · '), 42)}` : 'No workspaces'}</span>
            </button>
          );
        })}
      </div>

      {mode ? (
        <div className="catalog-modal-backdrop" role="presentation" onClick={closeModal}>
          <section className="card controls catalog-modal" role="dialog" aria-modal="true" aria-labelledby="agent-modal-title" onClick={(event) => event.stopPropagation()}>
            <div className="catalog-modal-head">
              <div>
                <div className="section-label">Agents</div>
                <h3 id="agent-modal-title">{mode === 'create' ? 'Add agent' : mode === 'edit' ? 'Edit agent' : activeAgent?.name}</h3>
                <p>{mode === 'view' ? activeAgent?.purpose : 'Name it once, then reuse it from the global agents list.'}</p>
              </div>
              {mode === 'view' && activeAgent ? <span className={`pill ${stateClass}`}>{activeAgent.state}</span> : null}
            </div>

            {mode === 'view' && activeAgent ? (
              <div className="catalog-modal-body">
                <dl className="facts catalog-detail-facts">
                  <dt>Model</dt><dd>{activeAgent.model || 'Not set'}</dd>
                  <dt>Tags</dt><dd>{activeAgent.tags.length ? activeAgent.tags.join(' · ') : 'None'}</dd>
                  <dt>Linked workspaces</dt><dd>{linkedWorkspaces.length ? linkedWorkspaces.join(' · ') : 'None yet'}</dd>
                </dl>

                <div className="catalog-detail-copy">{activeAgent.purpose}</div>

                <div className="catalog-detail-actions">
                  <div className="catalog-modal-buttons">
                    <button className="button secondary" onClick={openEdit}>Edit</button>
                    <button className="button danger" onClick={() => { if (onDeleteAgent(activeAgent.id)) closeModal(); }}>Delete</button>
                  </div>
                </div>
              </div>
            ) : null}

            {mode !== 'view' ? (
              <div className="catalog-modal-body">
                <div className="catalog-form-grid">
                  <label className="input-wrap">
                    <span>Name *</span>
                    <input value={draftName} onChange={(event) => setDraftName(event.target.value)} placeholder="Ops triage agent" />
                  </label>
                  <label className="input-wrap">
                    <span>Model</span>
                    <input value={draftModel} onChange={(event) => setDraftModel(event.target.value)} placeholder="GPT-4.1 / Claude / local" />
                  </label>
                  <label className="input-wrap catalog-wide-input">
                    <span>Tags</span>
                    <input value={draftTags} onChange={(event) => setDraftTags(event.target.value)} placeholder="ops, triage, support" />
                  </label>
                  <label className="textarea-wrap catalog-wide-input">
                    <span>Purpose *</span>
                    <textarea value={draftPurpose} onChange={(event) => setDraftPurpose(event.target.value)} rows={4} placeholder="What this agent will do" />
                  </label>
                </div>
                <div className="catalog-modal-buttons">
                  <button className="button primary" onClick={submitAgent}>{mode === 'create' ? 'Create agent' : 'Save changes'}</button>
                  <button className="button secondary" onClick={closeModal}>Cancel</button>
                </div>
              </div>
            ) : null}
          </section>
        </div>
      ) : null}
    </section>
  );
}

function SkillsArea({
  skills,
  onCreateSkill,
  onImportUrl,
  onUpdateSkill,
  onDeleteSkill,
  skillUrl,
  setSkillUrl,
}: {
  skills: GlobalSkill[];
  onCreateSkill: (draft?: GlobalSkillDraft) => boolean;
  onImportUrl: () => boolean;
  onUpdateSkill: (skillID: string, draft: GlobalSkillDraft) => boolean;
  onDeleteSkill: (skillID: string) => boolean;
  skillUrl: string;
  setSkillUrl: (value: string) => void;
}) {
  const [mode, setMode] = useState<SkillCatalogMode | null>(null);
  const [activeSkillID, setActiveSkillID] = useState('');
  const [draftName, setDraftName] = useState('');
  const [draftDescription, setDraftDescription] = useState('');
  const [draftTags, setDraftTags] = useState('');
  const [draftContent, setDraftContent] = useState('');

  const activeSkill = useMemo(
    () => skills.find((skill) => skill.id === activeSkillID) ?? null,
    [activeSkillID, skills],
  );

  function resetDrafts() {
    setDraftName('');
    setDraftDescription('');
    setDraftTags('');
    setDraftContent('');
    setSkillUrl('');
  }

  function openCreate() {
    resetDrafts();
    setActiveSkillID('');
    setMode('create');
  }

  function openView(skill: GlobalSkill) {
    setActiveSkillID(skill.id);
    setMode('view');
  }

  function openEdit() {
    if (!activeSkill) return;
    setDraftName(activeSkill.name);
    setDraftDescription(activeSkill.description);
    setDraftTags(activeSkill.tags.join(', '));
    setDraftContent(activeSkill.content);
    setMode('edit');
  }

  function closeModal() {
    setMode(null);
    setActiveSkillID('');
    resetDrafts();
  }

  useEffect(() => {
    if (mode !== 'edit' || !activeSkill) return;
    setDraftName(activeSkill.name);
    setDraftDescription(activeSkill.description);
    setDraftTags(activeSkill.tags.join(', '));
    setDraftContent(activeSkill.content);
  }, [activeSkill, mode]);

  useEffect(() => {
    if (mode === 'create') resetDrafts();
  }, [mode]);

  useEffect(() => {
    if (activeSkillID && !activeSkill) {
      closeModal();
    }
  }, [activeSkill, activeSkillID]);

  function submitSkill() {
    const draft = {
      name: draftName,
      description: draftDescription,
      tags: draftTags,
      content: draftContent,
    } satisfies GlobalSkillDraft;

    const saved = mode === 'create'
      ? onCreateSkill(draft)
      : activeSkill
        ? onUpdateSkill(activeSkill.id, draft)
        : false;

    if (saved) {
      closeModal();
    }
  }

  return (
    <section className="catalog-layout">
      <div className="section-header catalog-header">
        <div>
          <div className="section-label">Skills</div>
          <h2>Global skills</h2>
        </div>
        <div className="catalog-header-actions">
          <span className="pill">{skills.length} total</span>
          <button className="button primary" onClick={openCreate}>Add skill</button>
        </div>
      </div>

      <div className="catalog-list" role="list" aria-label="Skills list">
        {skills.length === 0 ? (
          <div className="empty">No skills yet.</div>
        ) : skills.map((skill) => (
          <button className="catalog-row" key={skill.id} type="button" onClick={() => openView(skill)}>
            <span className="catalog-row-main">
              <strong>{skill.name}</strong>
              <small>{skill.description}</small>
            </span>
            <span className="catalog-row-cell">{skill.sourceLabel ?? skill.source}</span>
            <span className="catalog-row-cell">{skill.tags.length ? skill.tags.join(' · ') : 'No tags'}</span>
          </button>
        ))}
      </div>

      {mode ? (
        <div className="catalog-modal-backdrop" role="presentation" onClick={closeModal}>
          <section className="card controls catalog-modal" role="dialog" aria-modal="true" aria-labelledby="skill-modal-title" onClick={(event) => event.stopPropagation()}>
            <div className="catalog-modal-head">
              <div>
                <div className="section-label">Skills</div>
                <h3 id="skill-modal-title">{mode === 'create' ? 'Add skill' : mode === 'edit' ? 'Edit skill' : activeSkill?.name}</h3>
                <p>{mode === 'view' ? activeSkill?.description : 'Store the reusable instruction pack here.'}</p>
              </div>
              {mode === 'view' && activeSkill ? <span className={`pill ${activeSkill.source === 'url' ? 'warning' : 'normal'}`}>{activeSkill.source}</span> : null}
            </div>

            {mode === 'view' && activeSkill ? (
              <div className="catalog-modal-body">
                <dl className="facts catalog-detail-facts">
                  <dt>Source</dt><dd>{activeSkill.sourceLabel ?? 'Created in UI'}</dd>
                  <dt>Tags</dt><dd>{activeSkill.tags.length ? activeSkill.tags.join(' · ') : 'None'}</dd>
                  <dt>Content</dt><dd>{activeSkill.content}</dd>
                </dl>
                <div className="catalog-detail-copy">{activeSkill.content}</div>
                <div className="catalog-modal-buttons">
                  <button className="button secondary" onClick={openEdit}>Edit</button>
                  <button className="button danger" onClick={() => { if (onDeleteSkill(activeSkill.id)) closeModal(); }}>Delete</button>
                </div>
              </div>
            ) : null}

            {mode !== 'view' ? (
              <div className="catalog-modal-body">
                <div className="catalog-form-grid">
                  <label className="input-wrap">
                    <span>Name *</span>
                    <input value={draftName} onChange={(event) => setDraftName(event.target.value)} placeholder="Evidence synthesizer" />
                  </label>
                  <label className="input-wrap catalog-wide-input">
                    <span>Description / purpose *</span>
                    <input value={draftDescription} onChange={(event) => setDraftDescription(event.target.value)} placeholder="Summarize sources into a concise brief" />
                  </label>
                  <label className="input-wrap catalog-wide-input">
                    <span>Tags</span>
                    <input value={draftTags} onChange={(event) => setDraftTags(event.target.value)} placeholder="research, summary, briefs" />
                  </label>
                  <label className="textarea-wrap catalog-wide-input">
                    <span>Content / instructions *</span>
                    <textarea value={draftContent} onChange={(event) => setDraftContent(event.target.value)} rows={4} placeholder="Paste the reusable instructions here" />
                  </label>
                </div>

                {mode === 'create' ? (
                  <div className="catalog-import-panel">
                    <label className="input-wrap">
                      <span>External skill URL</span>
                      <input value={skillUrl} onChange={(event) => setSkillUrl(event.target.value)} placeholder="https://example.com/skill.md" />
                    </label>
                    <div className="catalog-modal-buttons">
                      <button className="button secondary" onClick={() => { if (onImportUrl()) closeModal(); }}>Import URL</button>
                    </div>
                  </div>
                ) : null}

                <div className="catalog-modal-buttons">
                  <button className="button primary" onClick={submitSkill}>{mode === 'create' ? 'Create skill' : 'Save changes'}</button>
                  <button className="button secondary" onClick={closeModal}>Cancel</button>
                </div>
              </div>
            ) : null}
          </section>
        </div>
      ) : null}
    </section>
  );
}

function AdminArea({ overview }: { overview: AdminOverview | null }) {
  const [selectedTab, setSelectedTab] = useState<AdminTab>('Integrations');

  if (!overview) {
    return <section className="card controls"><div className="empty">Loading admin overview…</div></section>;
  }

  return (
    <section className="admin-layout">
      <section className="card controls admin-hero">
        <div className="section-label">Admin area</div>
        <div className="admin-hero-head">
          <div>
            <h2>Control-plane catalog</h2>
            <p>Read-only tabs for integration policy, MCP servers, federations, audit logs, runtimes, and model routing.</p>
          </div>
          <div className="admin-hero-badges">
            <span className="pill">{overview.identity_provider}</span>
            <span className="pill">{overview.sandbox_provider}</span>
          </div>
        </div>
        <div className="admin-hero-metrics">
          <div><strong>{overview.users.length}</strong><span>users exposed</span></div>
          <div><strong>{Object.keys(overview.runtime_images).length}</strong><span>runtime images</span></div>
          <div><strong>{overview.activity.length}</strong><span>audit events</span></div>
        </div>
      </section>

      <section className="card admin-shell">
        <div className="admin-tabs" role="tablist" aria-label="Admin tabs">
          {ADMIN_TABS.map((tab) => (
            <button
              key={tab}
              className={`admin-tab ${selectedTab === tab ? 'active' : ''}`}
              onClick={() => setSelectedTab(tab)}
            >
              {tab}
            </button>
          ))}
        </div>

        {selectedTab === 'Integrations' ? (
          <section className="admin-panel-grid">
            {ADMIN_INTEGRATIONS.map((integration) => (
              <article className="admin-matrix-card" key={integration.name}>
                <div className="admin-card-top">
                  <strong>{integration.name}</strong>
                  <span className={`pill ${integration.status === 'enabled' ? 'ready' : integration.status === 'restricted' ? 'warning' : 'idle'}`}>{integration.status}</span>
                </div>
                <dl className="facts admin-facts">
                  <dt>Auth mode</dt><dd>{integration.auth}</dd>
                  <dt>Allowed scope</dt><dd>{integration.scope}</dd>
                  <dt>Policy</dt><dd>{integration.policy}</dd>
                </dl>
              </article>
            ))}
          </section>
        ) : null}

        {selectedTab === 'MCP' ? (
          <section className="admin-panel-grid">
            {ADMIN_MCP_SERVERS.map((server) => (
              <article className="admin-table-card" key={server.name}>
                <div className="admin-card-top">
                  <strong>{server.name}</strong>
                  <span className={`pill ${server.status === 'ready' ? 'ready' : server.status === 'restricted' ? 'warning' : 'idle'}`}>{server.status}</span>
                </div>
                <div className="admin-kv-row"><span>Transport</span><strong>{server.transport}</strong></div>
                <div className="admin-kv-row"><span>Tools</span><strong>{server.tools}</strong></div>
                <div className="admin-kv-row"><span>Allowed scopes</span><strong>{server.scopes}</strong></div>
                <div className="admin-kv-row"><span>Approval mode</span><strong>{server.approval}</strong></div>
              </article>
            ))}
          </section>
        ) : null}

        {selectedTab === 'Federations' ? (
          <section className="admin-panel-grid">
            {ADMIN_FEDERATIONS.map((connector) => (
              <article className="admin-matrix-card" key={connector.name}>
                <div className="admin-card-top">
                  <strong>{connector.name}</strong>
                  <span className={`pill ${connector.status === 'enabled' ? 'ready' : connector.status === 'restricted' ? 'warning' : 'idle'}`}>{connector.status}</span>
                </div>
                <dl className="facts admin-facts">
                  <dt>Mapping</dt><dd>{connector.mapping}</dd>
                  <dt>Notes</dt><dd>{connector.notes}</dd>
                </dl>
              </article>
            ))}
          </section>
        ) : null}

        {selectedTab === 'Audit logs' ? (
          <section className="admin-audit-list">
            {overview.activity.length === 0 ? <div className="empty">No audit events yet.</div> : overview.activity.slice().reverse().map((entry, index) => (
              <article className="audit-row" key={`${entry.time}-${entry.type}-${index}`}>
                <div className="audit-row-top">
                  <div>
                    <strong>{describeAuditAction(entry)}</strong>
                    <span>{describeAuditTarget(entry)}</span>
                  </div>
                  <span className={`pill ${auditSeverity(entry)}`}>{auditSeverity(entry)}</span>
                </div>
                <div className="audit-row-meta">
                  <span>{entry.time}</span>
                  <span>{entry.actor_id ?? 'system'}</span>
                  <span>{entry.type}</span>
                  <span>{overview.sandbox_provider}</span>
                </div>
              </article>
            ))}
          </section>
        ) : null}

        {selectedTab === 'Policies' ? (
          <section className="admin-policy-grid">
            {ADMIN_POLICIES.map((policy) => (
              <article className="admin-policy-card" key={policy.name}>
                <strong>{policy.name}</strong>
                <span>{policy.value}</span>
                <small>{policy.detail}</small>
              </article>
            ))}
          </section>
        ) : null}

        {selectedTab === 'Runtimes' ? (
          <section className="admin-panel-grid runtime-panel-grid">
            <article className="admin-matrix-card">
              <div className="admin-card-top">
                <strong>{overview.sandbox_provider}</strong>
                <span className="pill ready">ready</span>
              </div>
              <dl className="facts admin-facts">
                <dt>Identity provider</dt><dd>{overview.identity_provider}</dd>
                <dt>Runtime images</dt><dd>{Object.keys(overview.runtime_images).length}</dd>
                <dt>Readiness</dt><dd>Connected to the mock control plane</dd>
              </dl>
            </article>
            {Object.entries(overview.runtime_images).map(([name, image]) => (
              <article className="admin-table-card" key={name}>
                <div className="admin-card-top">
                  <strong>{name}</strong>
                  <span className="pill normal">catalog</span>
                </div>
                <div className="admin-kv-row"><span>Image</span><strong>{image}</strong></div>
                <div className="admin-kv-row"><span>Status</span><strong>Ready for sandbox leasing</strong></div>
              </article>
            ))}
          </section>
        ) : null}

        {selectedTab === 'Models' ? (
          <section className="admin-panel-grid">
            {ADMIN_MODELS.map((provider) => (
              <article className="admin-matrix-card" key={provider.provider}>
                <div className="admin-card-top">
                  <strong>{provider.provider}</strong>
                  <span className="pill ready">catalog</span>
                </div>
                <dl className="facts admin-facts">
                  <dt>Models</dt><dd>{provider.models}</dd>
                  <dt>Routing</dt><dd>{provider.routing}</dd>
                  <dt>Credential boundary</dt><dd>{provider.boundary}</dd>
                </dl>
              </article>
            ))}
          </section>
        ) : null}
      </section>
    </section>
  );
}
