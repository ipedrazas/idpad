import { Link } from 'react-router-dom'

import type { IdeaSummary } from '../api/types'
import { absoluteTime, bodyPreview, relativeTime } from '../lib/format'
import { TagChip } from './TagChip'

/** How many tag chips a card shows before collapsing the rest into a count. */
const VISIBLE_TAGS = 3

/** One idea in the list view. */
export function IdeaCard({ idea, activeTags = [] }: { idea: IdeaSummary; activeTags?: string[] }) {
  const preview = bodyPreview(idea.body)

  // Tags matching the active filter come first, so the reason this card is in
  // a filtered list is always among the chips that survive the cut.
  const ordered = [...idea.tags].sort((a, b) => {
    const rank = Number(activeTags.includes(b.slug)) - Number(activeTags.includes(a.slug))
    return rank !== 0 ? rank : a.slug.localeCompare(b.slug)
  })
  const shown = ordered.slice(0, VISIBLE_TAGS)
  const hidden = ordered.length - shown.length

  return (
    <Link
      to={`/ideas/${idea.id}`}
      className="block h-full rounded-xl border border-slate-200 bg-white p-4 shadow-sm transition-all hover:-translate-y-0.5 hover:border-slate-300 hover:shadow-md dark:border-slate-800 dark:bg-slate-900 dark:hover:border-slate-700"
    >
      <h2 className="line-clamp-2 text-base font-semibold tracking-tight text-slate-900 dark:text-slate-100">
        {idea.title}
      </h2>

      <p className="mt-2 line-clamp-3 min-h-[3.5rem] text-sm text-slate-500 dark:text-slate-400">
        {preview || <span className="italic text-slate-400">No content yet</span>}
      </p>

      {ordered.length > 0 ? (
        <div className="mt-3 flex flex-wrap items-center gap-1">
          {shown.map((tag) => (
            <TagChip key={tag.slug} tag={tag} active={activeTags.includes(tag.slug)} />
          ))}
          {hidden > 0 ? <span className="text-xs text-slate-400">+{hidden}</span> : null}
        </div>
      ) : null}

      <div className="mt-3 flex items-center gap-3 text-xs text-slate-400">
        <span title={`${idea.comment_count} comments`}>💬 {idea.comment_count}</span>
        <span title={`${idea.resource_count} resources`}>📎 {idea.resource_count}</span>
        <span title={`${idea.link_count} connections`}>🔗 {idea.link_count}</span>
        <time className="ml-auto" dateTime={idea.updated_at} title={absoluteTime(idea.updated_at)}>
          {relativeTime(idea.updated_at)}
        </time>
      </div>
    </Link>
  )
}
