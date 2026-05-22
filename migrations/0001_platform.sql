create table if not exists users (
    id text primary key,
    username text not null unique,
    password_hash text not null,
    role text not null default 'user',
    disabled boolean not null default false,
    created_at timestamptz not null,
    updated_at timestamptz not null
);

create index if not exists users_username_idx on users (username);

create table if not exists agents (
    id text primary key,
    owner_user_id text not null references users(id) on delete cascade,
    name text not null,
    runtime text not null,
    model text not null,
    state text not null default 'idle',
    last_error text not null default '',
    created_at timestamptz not null,
    updated_at timestamptz not null
);

create index if not exists agents_owner_idx on agents (owner_user_id);

create table if not exists llm_models (
    id text primary key,
    display_name text not null,
    provider_model text not null,
    enabled boolean not null default false,
    created_at timestamptz not null,
    updated_at timestamptz not null
);

create table if not exists llm_gateway_settings (
    id integer primary key default 1,
    enabled boolean not null default false,
    base_url text not null default '',
    secret_name text not null default '',
    secret_key text not null default '',
    updated_at timestamptz not null,
    constraint single_row check (id = 1)
);
