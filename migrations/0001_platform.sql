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
    name text not null,
    state text not null,
    created_at timestamptz not null
);

create index if not exists agents_owner_created_idx on agents (owner_id, created_at, id);
