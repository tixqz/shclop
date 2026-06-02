create table if not exists plugin_manifests (
    id          text primary key,
    yaml        text not null,
    enabled     boolean not null default true,
    revision    bigint not null default 1,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);

create index if not exists plugin_manifests_enabled_updated_idx
    on plugin_manifests (enabled, updated_at);
