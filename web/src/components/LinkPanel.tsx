import { useMemo, useState } from 'react'
import { Link as RouterLink } from 'react-router-dom'

import type { CreateLinkInput } from '../api/client'
import type { IdeaSummary, Link, Relation } from '../api/types'
import { cx } from '../lib/format'
import { groupLinks, RELATION_OPTIONS, RELATIONS, relationArrow, relationLabel } from '../lib/relations'
import { Button, EmptyState, ErrorState, FieldError, Skeleton, Spinner } from './ui'

export interface LinkPanelProps {
  links: Link[]
  loading: boolean
  error: Error | null
  /** Candidate ideas to link to, already excluding this one. */
  candidates: IdeaSummary[]
  candidatesLoading: boolean
  /** Drives the candidate search, which the API runs against idea titles. */
  search: string
  onSearchChange: (search: string) => void
  onAdd: (input: CreateLinkInput) => Promise<void>
  onRemove: (id: string) => Promise<void>
}

/**
 * The connections panel. A link is stored once and shown from both ends, so
 * this splits what the idea declared from what points back at it, and phrases
 * each row from this idea's point of view.
 */
export function LinkPanel({
  links,
  loading,
  error,
  candidates,
  candidatesLoading,
  search,
  onSearchChange,
  onAdd,
  onRemove,
}: LinkPanelProps) {
  const [adding, setAdding] = useState(false)
  const [relation, setRelation] = useState<Relation>('references')
  const [note, setNote] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  const { outgoing, incoming } = useMemo(() => groupLinks(links), [links])

  const submit = async (targetId: string) => {
    setSubmitting(true)
    setFormError(null)
    try {
      await onAdd({
        target_idea_id: targetId,
        relation,
        ...(note.trim() ? { note: note.trim() } : {}),
      })
      setAdding(false)
      setNote('')
      onSearchChange('')
    } catch (cause) {
      setFormError(cause instanceof Error ? cause.message : 'Could not connect the ideas.')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <section aria-label="Connections" className="flex flex-col gap-3">
      <header className="flex items-baseline justify-between">
        <h2 className="text-sm font-semibold tracking-tight text-slate-900 dark:text-slate-100">Connections</h2>
        <span className="text-xs text-slate-500 dark:text-slate-400">{links.length}</span>
      </header>

      {adding ? (
        <div className="rounded-xl border border-slate-200 bg-white p-3 dark:border-slate-800 dark:bg-slate-900">
          <label
            htmlFor="link-relation"
            className="block text-xs font-medium text-slate-600 dark:text-slate-300"
          >
            This idea…
          </label>
          <select
            id="link-relation"
            value={relation}
            onChange={(event) => setRelation(event.target.value as Relation)}
            className="mt-1 w-full rounded-lg border border-slate-300 bg-white px-2 py-1.5 text-sm outline-none focus:border-slate-500 dark:border-slate-700 dark:bg-slate-900"
          >
            {RELATION_OPTIONS.map((option) => (
              <option key={option} value={option}>
                {RELATIONS[option].active} — {RELATIONS[option].hint.toLowerCase()}
              </option>
            ))}
          </select>

          <input
            value={search}
            onChange={(event) => onSearchChange(event.target.value)}
            placeholder="Search ideas by title…"
            aria-label="Search ideas to link"
            className="mt-2 w-full rounded-lg border border-slate-300 bg-white px-2 py-1.5 text-sm outline-none focus:border-slate-500 dark:border-slate-700 dark:bg-slate-900"
          />

          <div className="mt-2 max-h-48 overflow-y-auto rounded-lg border border-slate-100 dark:border-slate-800">
            {candidatesLoading ? (
              <div className="p-2">
                <Skeleton className="h-6" />
              </div>
            ) : candidates.length === 0 ? (
              <p className="px-2 py-3 text-center text-xs text-slate-400">
                {search ? 'No idea matches that title.' : 'No other ideas to link to yet.'}
              </p>
            ) : (
              <ul>
                {candidates.map((candidate) => (
                  <li key={candidate.id}>
                    <button
                      type="button"
                      disabled={submitting}
                      onClick={() => void submit(candidate.id)}
                      className="w-full truncate px-2 py-1.5 text-left text-sm transition-colors hover:bg-slate-100 disabled:opacity-50 dark:hover:bg-slate-800"
                    >
                      {candidate.title}
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>

          <input
            value={note}
            onChange={(event) => setNote(event.target.value)}
            maxLength={280}
            placeholder="Why? (optional)"
            aria-label="Note about this connection"
            className="mt-2 w-full rounded-lg border border-slate-300 bg-white px-2 py-1.5 text-sm outline-none focus:border-slate-500 dark:border-slate-700 dark:bg-slate-900"
          />

          {formError ? <FieldError>{formError}</FieldError> : null}

          <div className="mt-2 flex items-center gap-2">
            {submitting ? <Spinner className="size-3 text-slate-400" /> : null}
            <Button
              variant="ghost"
              size="sm"
              className="ml-auto"
              onClick={() => {
                setAdding(false)
                setFormError(null)
                onSearchChange('')
              }}
            >
              Cancel
            </Button>
          </div>
        </div>
      ) : (
        <Button size="sm" onClick={() => setAdding(true)}>
          Connect an idea
        </Button>
      )}

      {loading ? (
        <Skeleton className="h-20" />
      ) : error ? (
        <ErrorState title="Could not load connections" description={error.message} />
      ) : links.length === 0 ? (
        <EmptyState
          title="Not connected yet"
          description="Link this idea to the ones it builds on, expands or echoes."
          className="px-4 py-6"
        />
      ) : (
        <div className="flex flex-col gap-3">
          <LinkGroup links={outgoing} onRemove={onRemove} />
          {incoming.length > 0 ? (
            <div>
              <h3 className="mb-1.5 text-xs font-medium uppercase tracking-wide text-slate-400">
                Pointing here
              </h3>
              <LinkGroup links={incoming} onRemove={onRemove} />
            </div>
          ) : null}
        </div>
      )}
    </section>
  )
}

/** A list of link rows, each phrased from the current idea's point of view. */
function LinkGroup({ links, onRemove }: { links: Link[]; onRemove: (id: string) => Promise<void> }) {
  if (links.length === 0) return null

  return (
    <ul className="flex flex-col gap-1.5">
      {links.map((link) => (
        <li
          key={link.id}
          className={cx(
            'group flex items-start gap-2 rounded-lg border border-slate-200 bg-white px-2.5 py-2',
            'dark:border-slate-800 dark:bg-slate-900',
          )}
        >
          <span
            aria-hidden="true"
            className="mt-0.5 select-none text-sm text-slate-400"
            title={relationLabel(link.relation, link.direction)}
          >
            {relationArrow(link.relation, link.direction)}
          </span>

          <div className="min-w-0 flex-1">
            <p className="text-xs text-slate-400">{relationLabel(link.relation, link.direction)}</p>
            <RouterLink
              to={`/ideas/${link.other_idea_id}`}
              className="block truncate text-sm font-medium text-slate-800 hover:underline dark:text-slate-100"
            >
              {link.other_title}
            </RouterLink>
            {link.note ? (
              <p className="mt-0.5 text-xs text-slate-500 dark:text-slate-400">{link.note}</p>
            ) : null}
          </div>

          <button
            type="button"
            onClick={() => void onRemove(link.id)}
            aria-label={`Remove connection to ${link.other_title}`}
            className="rounded px-1 text-slate-300 opacity-0 transition-opacity group-hover:opacity-100 focus-visible:opacity-100 hover:text-rose-600 dark:hover:text-rose-400"
          >
            ×
          </button>
        </li>
      ))}
    </ul>
  )
}
