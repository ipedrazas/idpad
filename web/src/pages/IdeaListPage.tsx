import { useMemo } from 'react'
import { Link, useSearchParams } from 'react-router-dom'

import { IdeaCard } from '../components/IdeaCard'
import { StatusBadge } from '../components/StatusBadge'
import { TagChip } from '../components/TagChip'
import { Button, EmptyState, ErrorState, Skeleton } from '../components/ui'
import { useIdeaStatuses, useIdeas } from '../hooks/useIdeas'
import { useTags } from '../hooks/useTags'
import { isIdeaStatus, statusLabel } from '../lib/status'

/** The home view: every idea as a card, newest update first. */
export function IdeaListPage() {
  const [searchParams, setSearchParams] = useSearchParams()

  // The filter lives in the URL, so a filtered view is shareable and the back
  // button steps through filters rather than leaving the page.
  const activeTags = useMemo(() => searchParams.getAll('tag'), [searchParams])
  // An unknown status in a hand-edited URL is dropped rather than sent on, so
  // a stale link degrades to a wider list instead of a validation error.
  const activeStatuses = useMemo(
    () => searchParams.getAll('status').filter(isIdeaStatus),
    [searchParams],
  )
  const query = searchParams.get('q') ?? ''
  const filtering = activeTags.length > 0 || activeStatuses.length > 0 || query !== ''

  const { data: ideas, isPending, error, refetch } = useIdeas({
    tags: activeTags,
    statuses: activeStatuses,
    q: query,
  })
  const { data: vocabulary } = useTags()
  const { data: statusCounts } = useIdeaStatuses()

  /** Adds or removes one value from a repeated parameter, keeping the rest. */
  const toggle = (param: 'tag' | 'status', value: string) => {
    const next = new URLSearchParams(searchParams)
    const current = next.getAll(param)
    next.delete(param)
    for (const kept of current) {
      if (kept !== value) next.append(param, kept)
    }
    if (!current.includes(value)) next.append(param, value)
    setSearchParams(next, { replace: true })
  }

  const setQuery = (value: string) => {
    const next = new URLSearchParams(searchParams)
    if (value) next.set('q', value)
    else next.delete('q')
    setSearchParams(next, { replace: true })
  }

  // Only tags actually in use are worth offering, plus any already-selected
  // one, so a filter never silently disappears from the bar it was set in.
  const offeredTags = (vocabulary ?? []).filter(
    (tag) => tag.idea_count > 0 || activeTags.includes(tag.slug),
  )

  return (
    <div className="mx-auto w-full max-w-5xl px-4 py-8">
      <header className="mb-6 flex items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">Ideas</h1>
          <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
            Jot something down, then comment on any part of it.
          </p>
        </div>
        <Link to="/ideas/new">
          <Button variant="primary">New idea</Button>
        </Link>
      </header>

      <div className="mb-6 flex flex-col gap-3">
        <input
          type="search"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder="Search titles…"
          aria-label="Search ideas by title"
          className="w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-slate-500 dark:border-slate-700 dark:bg-slate-900"
        />

        {/*
          Statuses are ORed and tags ANDed, which is why they are two rows:
          picking a second status widens the list, picking a second tag narrows
          it, and putting them in one row would imply they behave alike.
        */}
        <div className="flex flex-wrap items-center gap-1.5" role="group" aria-label="Filter by status">
          {(statusCounts ?? []).map(({ status, idea_count }) => {
            const active = activeStatuses.includes(status)
            return (
              <button
                key={status}
                type="button"
                aria-pressed={active}
                aria-label={`${statusLabel(status)} (${idea_count})`}
                onClick={() => toggle('status', status)}
              >
                <StatusBadge status={status} count={idea_count} selected={active} />
              </button>
            )
          })}
        </div>

        {offeredTags.length > 0 || filtering ? (
          <div className="flex flex-wrap items-center gap-1.5">
            {offeredTags.map((tag) => {
              const active = activeTags.includes(tag.slug)
              return (
                <button key={tag.slug} type="button" onClick={() => toggle('tag', tag.slug)}>
                  <TagChip tag={tag} count={tag.idea_count} active={active} />
                </button>
              )
            })}
            {filtering ? (
              <Button
                variant="ghost"
                size="sm"
                className="ml-1"
                onClick={() => setSearchParams(new URLSearchParams(), { replace: true })}
              >
                Clear
              </Button>
            ) : null}
          </div>
        ) : null}
      </div>

      {isPending ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {[0, 1, 2].map((key) => (
            <Skeleton key={key} className="h-40" />
          ))}
        </div>
      ) : error ? (
        <ErrorState
          title="Could not load your ideas"
          description={error.message}
          action={<Button onClick={() => void refetch()}>Try again</Button>}
        />
      ) : ideas.length === 0 ? (
        filtering ? (
          <EmptyState
            title="Nothing matches that filter"
            description="Try a different status or tag, or clear the filter to see everything."
            action={
              <Button onClick={() => setSearchParams(new URLSearchParams(), { replace: true })}>
                Clear filter
              </Button>
            }
          />
        ) : (
          <EmptyState
            title="No ideas yet"
            description="Everything starts with a rough note. Write the first one."
            action={
              <Link to="/ideas/new">
                <Button variant="primary">New idea</Button>
              </Link>
            }
          />
        )
      ) : (
        <ul className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {ideas.map((idea) => (
            <li key={idea.id}>
              <IdeaCard idea={idea} activeTags={activeTags} />
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
