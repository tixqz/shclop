-- Integration connections: stores encrypted credentials for external providers.
create table if not exists integration_connections (
    provider_id        text not null,
    user_id            text not null references users(id) on delete cascade,
    external_account_id text not null default '',
    external_login     text not null default '',
    account_type       text not null default '',
    status             text not null default 'connected',
    secret             text not null default '',  -- encrypted token; never logged
    revision           bigint not null default 1,
    created_at         timestamptz not null,
    updated_at         timestamptz not null,
    primary key (provider_id, user_id)
);

-- Per-agent integration enablement.
create table if not exists agent_integrations (
    agent_id    text not null references agents(id) on delete cascade,
    provider_id text not null,
    enabled     boolean not null default false,
    revision    bigint not null default 1,
    status      text not null default 'disabled',
    created_at  timestamptz not null,
    updated_at  timestamptz not null,
    primary key (agent_id, provider_id)
);

create index if not exists agent_integrations_agent_idx on agent_integrations (agent_id);
create index if not exists agent_integrations_provider_idx on agent_integrations (provider_id);
