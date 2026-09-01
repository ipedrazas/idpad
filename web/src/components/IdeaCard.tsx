import { Link } from 'react-router-dom'

import type { IdeaSummary } from '../api/types'
import { absoluteTime, bodyPreview, relativeTime } from '../lib/format'

/** One idea in the list view. */
export function IdeaCard({ idea }: { idea: IdeaSummary }) {
  const preview = bodyPreview(idea.body)

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

      <div className="mt-3 flex items-center gap-3 text-xs text-slate-400">
        <span title={`${idea.comment_count} comments`}>💬 {idea.comment_count}</span>
        <span title={`${idea.resource_count} resources`}>📎 {idea.resource_count}</span>
        <time className="ml-auto" dateTime={idea.updated_at} title={absoluteTime(idea.updated_at)}>
          {relativeTime(idea.updated_at)}
        </time>
      </div>
    </Link>
  )
}
