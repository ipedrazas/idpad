-- 0003_idea_status: where an idea sits in its lifecycle.
--
-- Status is a closed set with exactly one value per idea, which is why it is a
-- column with a check constraint rather than a tag: nothing can make an idea
-- both done and rejected, and the tag vocabulary stays about subject matter.

alter table ideas
    add column if not exists status text not null default 'draft'
        check (status in ('draft', 'in_progress', 'done', 'rejected')),
    -- Kept apart from updated_at so that one keeps meaning "the body was
    -- edited"; moving an idea to done is not an edit of what it says.
    add column if not exists status_changed_at timestamptz not null default now();

-- Existing ideas have never changed status, so the transition they are on is
-- as old as they are.
update ideas set status_changed_at = created_at where status = 'draft';

-- The list view filters on status, and the status index counts by it.
create index if not exists ideas_status_idx on ideas (status);
