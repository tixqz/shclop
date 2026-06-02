alter table integration_connections
    add column if not exists scope    text not null default 'user',
    add column if not exists scope_id text not null default '';

update integration_connections set scope_id = user_id where scope_id = '';

do $$ begin
    if exists (
        select 1 from pg_constraint
        where conname = 'integration_connections_pkey'
          and conrelid = 'integration_connections'::regclass
    ) then
        execute 'alter table integration_connections drop constraint integration_connections_pkey';
    end if;
end $$;

do $$ begin
    if not exists (
        select 1 from pg_constraint
        where conname = 'integration_connections_pkey'
          and conrelid = 'integration_connections'::regclass
    ) then
        execute 'alter table integration_connections add primary key (provider_id, scope, scope_id)';
    end if;
end $$;
