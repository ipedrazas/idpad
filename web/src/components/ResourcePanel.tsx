import { useState } from 'react'

import type { CreateResourceInput } from '../api/client'
import type { Resource, ResourceType } from '../api/types'
import { cx, urlHost } from '../lib/format'
import { Button, EmptyState, ErrorState, FieldError, Skeleton, Spinner } from './ui'

export interface ResourcePanelProps {
  resources: Resource[]
  loading: boolean
  error: Error | null
  onAdd: (input: CreateResourceInput) => Promise<void>
  onRemove: (id: string) => Promise<void>
}

/** The add-form plus thumbnail grid shown under the comment sidebar. */
export function ResourcePanel({ resources, loading, error, onAdd, onRemove }: ResourcePanelProps) {
  const [type, setType] = useState<ResourceType>('link')
  const [url, setUrl] = useState('')
  const [label, setLabel] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  const submit = async () => {
    const trimmedUrl = url.trim()
    if (!trimmedUrl) {
      setFormError('A URL is required.')
      return
    }

    setSubmitting(true)
    setFormError(null)
    try {
      await onAdd({ type, url: trimmedUrl, ...(label.trim() ? { label: label.trim() } : {}) })
      setUrl('')
      setLabel('')
    } catch (cause) {
      setFormError(cause instanceof Error ? cause.message : 'Could not add the resource.')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <section aria-label="Resources" className="flex flex-col gap-3">
      <header className="flex items-baseline justify-between">
        <h2 className="text-sm font-semibold tracking-tight text-slate-900 dark:text-slate-100">Resources</h2>
        <span className="text-xs text-slate-500 dark:text-slate-400">{resources.length}</span>
      </header>

      <form
        className="rounded-xl border border-slate-200 bg-white p-3 dark:border-slate-800 dark:bg-slate-900"
        onSubmit={(event) => {
          event.preventDefault()
          void submit()
        }}
      >
        <div className="mb-2 flex gap-1" role="group" aria-label="Resource type">
          {(['link', 'image'] as const).map((option) => (
            <button
              key={option}
              type="button"
              aria-pressed={type === option}
              onClick={() => setType(option)}
              className={cx(
                'rounded-md px-2.5 py-1 text-xs font-medium capitalize transition-colors',
                type === option
                  ? 'bg-slate-900 text-white dark:bg-slate-100 dark:text-slate-900'
                  : 'text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800',
              )}
            >
              {option}
            </button>
          ))}
        </div>

        <input
          type="url"
          value={url}
          onChange={(event) => setUrl(event.target.value)}
          placeholder="https://…"
          aria-label="Resource URL"
          className="w-full rounded-lg border border-slate-300 bg-white px-2.5 py-1.5 text-sm outline-none focus:border-slate-500 dark:border-slate-700 dark:bg-slate-950"
        />
        <input
          type="text"
          value={label}
          onChange={(event) => setLabel(event.target.value)}
          placeholder="Label (optional)"
          aria-label="Resource label"
          className="mt-1.5 w-full rounded-lg border border-slate-300 bg-white px-2.5 py-1.5 text-sm outline-none focus:border-slate-500 dark:border-slate-700 dark:bg-slate-950"
        />

        {formError ? <FieldError>{formError}</FieldError> : null}

        <div className="mt-2 flex justify-end">
          <Button type="submit" size="sm" variant="primary" disabled={submitting}>
            {submitting ? <Spinner className="size-3" /> : null}
            Add resource
          </Button>
        </div>
      </form>

      {loading ? (
        <div className="grid grid-cols-2 gap-2">
          <Skeleton className="h-24" />
          <Skeleton className="h-24" />
        </div>
      ) : error ? (
        <ErrorState title="Could not load resources" description={error.message} />
      ) : resources.length === 0 ? (
        <EmptyState title="Nothing attached yet" description="Add an image or a link to keep it with the idea." />
      ) : (
        <ul className="grid grid-cols-2 gap-2">
          {resources.map((resource) => (
            <li key={resource.id}>
              <ResourceCard resource={resource} onRemove={() => onRemove(resource.id)} />
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}

interface ResourceCardProps {
  resource: Resource
  onRemove: () => Promise<void>
}

/** An image thumbnail or a link card, both showing the host. */
function ResourceCard({ resource, onRemove }: ResourceCardProps) {
  const [confirming, setConfirming] = useState(false)
  const [imageFailed, setImageFailed] = useState(false)
  const host = urlHost(resource.url)
  const title = resource.label ?? host

  return (
    <div className="group relative overflow-hidden rounded-lg border border-slate-200 bg-white dark:border-slate-800 dark:bg-slate-900">
      <a href={resource.url} target="_blank" rel="noreferrer noopener" className="block">
        {resource.type === 'image' && !imageFailed ? (
          <img
            src={resource.url}
            alt={title}
            loading="lazy"
            onError={() => setImageFailed(true)}
            className="h-24 w-full bg-slate-100 object-cover dark:bg-slate-800"
          />
        ) : (
          <div className="flex h-24 items-center justify-center bg-slate-100 text-2xl dark:bg-slate-800">
            {resource.type === 'image' ? '🖼️' : '🔗'}
          </div>
        )}
        <div className="p-2">
          <p className="truncate text-xs font-medium text-slate-800 dark:text-slate-100">{title}</p>
          <p className="truncate text-[11px] text-slate-400">{host}</p>
        </div>
      </a>

      {confirming ? (
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-1.5 bg-white/95 p-2 text-center dark:bg-slate-900/95">
          <p className="text-xs text-slate-600 dark:text-slate-300">Remove this resource?</p>
          <div className="flex gap-1.5">
            <Button size="sm" variant="danger" onClick={() => void onRemove()}>
              Remove
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setConfirming(false)}>
              Cancel
            </Button>
          </div>
        </div>
      ) : (
        <button
          type="button"
          aria-label={`Remove ${title}`}
          onClick={() => setConfirming(true)}
          className="absolute top-1 right-1 rounded-md bg-white/90 px-1.5 py-0.5 text-xs text-slate-500 opacity-0 transition-opacity group-hover:opacity-100 focus-visible:opacity-100 hover:text-rose-600 dark:bg-slate-900/90"
        >
          ✕
        </button>
      )}
    </div>
  )
}
