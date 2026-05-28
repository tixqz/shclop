-- Add description and system_prompt fields to agents.
alter table agents add column if not exists description   text not null default '';
alter table agents add column if not exists system_prompt text not null default '';
