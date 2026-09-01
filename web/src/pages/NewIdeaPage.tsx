import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'

import type { Doc } from '../api/types'
import { Button, FieldError, Spinner } from '../components/ui'
import { useCreateIdea } from '../hooks/useIdeas'

/** An idea starts with a title; the body is written in the detail view. */
const EMPTY_BODY: Doc = { type: 'doc', content: [{ type: 'paragraph' }] }

export function NewIdeaPage() {
  const [title, setTitle] = useState('')
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()
  const createIdea = useCreateIdea()

  const submit = async () => {
    const trimmed = title.trim()
    if (!trimmed) {
      setError('Give the idea a title.')
      return
    }

    setError(null)
    try {
      const idea = await createIdea.mutateAsync({ title: trimmed, body: EMPTY_BODY })
      void navigate(`/ideas/${idea.id}`)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : 'Could not create the idea.')
    }
  }

  return (
    <div className="mx-auto w-full max-w-2xl px-4 py-10">
      <Link to="/" className="text-sm text-slate-500 hover:text-slate-900 dark:hover:text-slate-100">
        ← All ideas
      </Link>

      <h1 className="mt-4 text-2xl font-semibold tracking-tight">New idea</h1>

      <form
        className="mt-6"
        onSubmit={(event) => {
          event.preventDefault()
          void submit()
        }}
      >
        <label htmlFor="idea-title" className="block text-sm font-medium text-slate-700 dark:text-slate-200">
          Title
        </label>
        <input
          id="idea-title"
          autoFocus
          value={title}
          maxLength={200}
          onChange={(event) => setTitle(event.target.value)}
          placeholder="A notebook that comments back"
          className="mt-1.5 w-full rounded-lg border border-slate-300 bg-white px-3 py-2 text-base outline-none focus:border-slate-500 dark:border-slate-700 dark:bg-slate-900"
        />
        <p className="mt-1 text-xs text-slate-400">{title.length}/200</p>
        {error ? <FieldError>{error}</FieldError> : null}

        <div className="mt-5 flex items-center gap-2">
          <Button type="submit" variant="primary" disabled={createIdea.isPending}>
            {createIdea.isPending ? <Spinner className="size-3" /> : null}
            Create idea
          </Button>
          <Link to="/">
            <Button variant="ghost">Cancel</Button>
          </Link>
        </div>
      </form>
    </div>
  )
}
