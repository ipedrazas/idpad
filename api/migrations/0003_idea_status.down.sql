drop index if exists ideas_status_idx;

alter table ideas
    drop column if exists status_changed_at,
    drop column if exists status;
