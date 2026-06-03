create table if not exists user_identities (
    provider_name  text not null,
    subject        text not null,
    user_id        text not null references users(id) on delete cascade,
    email          text not null default '',
    display_name   text not null default '',
    linked_at      timestamptz not null default now(),
    last_login_at  timestamptz,
    primary key (provider_name, subject)
);

create index if not exists user_identities_user_idx on user_identities (user_id);
