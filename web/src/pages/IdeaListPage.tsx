import { Link } from 'react-router-dom'

import { IdeaCard } from '../components/IdeaCard'
import { Button, EmptyState, ErrorState, Skeleton } from '../components/ui'
import { useIdeas } from '../hooks/useIdeas'

/** The home view: every idea as a card, newest update first. */
export function IdeaListPage() {
  const { data: ideas, isPending, error, refetch } = useIdeas()

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
        <EmptyState
          title="No ideas yet"
          description="Everything starts with a rough note. Write the first one."
          action={
            <Link to="/ideas/new">
              <Button variant="primary">New idea</Button>
            </Link>
          }
        />
      ) : (
        <ul className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {ideas.map((idea) => (
            <li key={idea.id}>
              <IdeaCard idea={idea} />
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
