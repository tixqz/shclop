create table if not exists workspaces (
    id text primary key,
    owner_id text not null,
    name text not null,
    description text not null default '',
    created_at timestamptz not null,
    updated_at timestamptz not null
);

create index if not exists workspaces_owner_updated_idx on workspaces (owner_id, updated_at desc, id);

create table if not exists agents (
    id text primary key,
    owner_id text not null,
    tenant_id text not null default '',
    name text not null,
    model text not null default '',
    purpose text not null default '',
    tags text not null default '[]',
    state text not null,
    latest_revision_id text not null default '',
    active_revision_id text not null default '',
    security_status text not null default '',
    created_at timestamptz not null
);

create index if not exists agents_owner_created_idx on agents (owner_id, created_at, id);

create table if not exists agent_revisions (
    id text primary key,
    agent_id text not null references agents(id) on delete cascade,
    revision_number integer not null,
    name text not null,
    model text not null default '',
    purpose text not null default '',
    tags text not null default '[]',
    content_digest text not null,
    security_status text not null default '',
    created_by text not null,
    created_at timestamptz not null
);

create index if not exists agent_revisions_agent_idx on agent_revisions (agent_id, revision_number, id);

create table if not exists skills (
    id text primary key,
    tenant_id text not null default '',
    owner_id text not null,
    name text not null,
    source_url text not null default '',
    tags text not null default '[]',
    latest_revision_id text not null default '',
    active_revision_id text not null default '',
    security_status text not null default '',
    created_at timestamptz not null,
    updated_at timestamptz not null
);

create index if not exists skills_owner_updated_idx on skills (owner_id, updated_at desc, id);

create table if not exists skill_revisions (
    id text primary key,
    skill_id text not null references skills(id) on delete cascade,
    revision_number integer not null,
    name text not null,
    description text not null default '',
    content text not null default '',
    tags text not null default '[]',
    source text not null default '',
    source_url text not null default '',
    content_digest text not null,
    security_status text not null default '',
    created_by text not null,
    created_at timestamptz not null
);

create index if not exists skill_revisions_skill_idx on skill_revisions (skill_id, revision_number, id);

create table if not exists audit_runs (
    id text primary key,
    target_type text not null,
    target_revision_id text not null,
    content_digest text not null,
    policy_version integer not null,
    scanner_version text not null,
    risk_level text not null,
    decision text not null,
    findings_json text not null,
    created_by text not null,
    created_at timestamptz not null
);

create index if not exists audit_runs_target_idx on audit_runs (target_type, target_revision_id, created_at desc, id);

create table if not exists approvals (
    id text primary key,
    target_type text not null,
    target_id text not null,
    actor_id text not null,
    decision text not null,
    created_at timestamptz not null
);

create index if not exists approvals_target_idx on approvals (target_type, target_id, created_at desc, id);
