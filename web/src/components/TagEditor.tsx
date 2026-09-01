import { useMemo, useRef, useState, type KeyboardEvent } from 'react'

import type { Tag, TagSummary } from '../api/types'
import { cx } from '../lib/format'
import { slugify } from '../lib/slug'
import { Button, FieldError, Skeleton, Spinner } from './ui'
import { TagChip } from './TagChip'

/** Mirrors the API's per-idea cap, so the limit is felt before the request. */
const MAX_TAGS = 25

export interface TagEditorProps {
  tags: Tag[]
  /** The whole vocabulary, used for suggestions. Optional while it loads. */
  vocabulary?: TagSummary[]
  loading: boolean
  error: Error | null
  /** Persists the full replacement set. */
  onChange: (names: string[]) => Promise<void>
  /**
   * Asks the server to read the idea and merge in suggested tags. Omitted when
   * the deployment has no tagging service, in which case no button is shown.
   */
  onSuggest?: () => Promise<void>
}

/**
 * The chip editor on the idea page. It sends the whole tag set on every
 * change, matching the API's PUT semantics, and shows what the server stored
 * rather than what was typed — the server folds spellings onto existing tags.
 */
export function TagEditor({ tags, vocabulary, loading, error, onChange, onSuggest }: TagEditorProps) {
  const [draft, setDraft] = useState('')
  const [saving, setSaving] = useState(false)
  const [suggesting, setSuggesting] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  const currentSlugs = useMemo(() => new Set(tags.map((tag) => tag.slug)), [tags])

  // Suggestions are the vocabulary minus what the idea already carries,
  // matched on the slug so a partial typed name still finds "Machine Learning".
  const suggestions = useMemo(() => {
    const needle = slugify(draft)
    if (!needle || !vocabulary) return []
    return vocabulary
      .filter((tag) => !currentSlugs.has(tag.slug) && tag.slug.includes(needle))
      .slice(0, 6)
  }, [draft, vocabulary, currentSlugs])

  const commit = async (names: string[]) => {
    setSaving(true)
    setFormError(null)
    try {
      await onChange(names)
    } catch (cause) {
      setFormError(cause instanceof Error ? cause.message : 'Could not save the tags.')
    } finally {
      setSaving(false)
    }
  }

  const addTag = async (rawName: string) => {
    const name = rawName.trim()
    if (!name) return
    if (!slugify(name)) {
      setFormError('A tag needs at least one letter or digit.')
      return
    }
    // Adding a tag the idea already has is a no-op, not an error: the user
    // just typed a name that folds onto one of the chips already showing.
    if (currentSlugs.has(slugify(name))) {
      setDraft('')
      return
    }
    if (tags.length >= MAX_TAGS) {
      setFormError(`An idea can carry at most ${MAX_TAGS} tags.`)
      return
    }
    setDraft('')
    await commit([...tags.map((tag) => tag.name), name])
  }

  const removeTag = async (slug: string) => {
    await commit(tags.filter((tag) => tag.slug !== slug).map((tag) => tag.name))
  }

  const suggest = async () => {
    if (!onSuggest) return
    setSuggesting(true)
    setFormError(null)
    try {
      await onSuggest()
    } catch (cause) {
      setFormError(cause instanceof Error ? cause.message : 'Could not reach the tagging service.')
    } finally {
      setSuggesting(false)
    }
  }

  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    // Enter and comma both commit, because both are how people end a tag.
    if (event.key === 'Enter' || event.key === ',') {
      event.preventDefault()
      void addTag(draft)
      return
    }
    // Backspace on an empty field removes the last chip, the usual shortcut.
    if (event.key === 'Backspace' && draft === '' && tags.length > 0) {
      event.preventDefault()
      void removeTag(tags[tags.length - 1].slug)
    }
  }

  return (
    <section aria-label="Tags" className="flex flex-col gap-2">
      <header className="flex items-center justify-between gap-2">
        <h2 className="text-sm font-semibold tracking-tight text-slate-900 dark:text-slate-100">Tags</h2>
        <div className="flex items-center gap-2">
          {saving ? <Spinner className="size-3 text-slate-400" /> : null}
          {onSuggest ? (
            <Button
              size="sm"
              disabled={suggesting || saving}
              onClick={() => void suggest()}
              // The service can take tens of seconds on a cold start, so the
              // button says what it is doing rather than just going quiet.
              title="Read this idea and suggest tags"
            >
              {suggesting ? <Spinner className="size-3" /> : <span aria-hidden="true">✨</span>}
              {suggesting ? 'Reading the idea…' : 'Suggest tags'}
            </Button>
          ) : null}
        </div>
      </header>

      {loading ? (
        <Skeleton className="h-9" />
      ) : (
        <div
          className={cx(
            'rounded-xl border border-slate-200 bg-white p-2',
            'dark:border-slate-800 dark:bg-slate-900',
          )}
          onClick={() => inputRef.current?.focus()}
        >
          <div className="flex flex-wrap items-center gap-1.5">
            {tags.map((tag) => (
              <TagChip key={tag.slug} tag={tag} onRemove={() => void removeTag(tag.slug)} />
            ))}
            <input
              ref={inputRef}
              value={draft}
              disabled={saving}
              onChange={(event) => {
                setDraft(event.target.value)
                setFormError(null)
              }}
              onKeyDown={onKeyDown}
              onBlur={() => void addTag(draft)}
              maxLength={60}
              aria-label="Add a tag"
              placeholder={tags.length === 0 ? 'Add a tag…' : ''}
              className="min-w-24 flex-1 bg-transparent px-1 py-0.5 text-sm outline-none placeholder:text-slate-400"
            />
          </div>

          {suggestions.length > 0 ? (
            <div className="mt-2 flex flex-wrap gap-1.5 border-t border-slate-100 pt-2 dark:border-slate-800">
              <span className="self-center text-xs text-slate-400">Existing:</span>
              {suggestions.map((tag) => (
                <button
                  key={tag.slug}
                  type="button"
                  // onMouseDown, not onClick: the input's blur handler would
                  // otherwise commit the draft and re-render before the click.
                  onMouseDown={(event) => {
                    event.preventDefault()
                    void addTag(tag.name)
                  }}
                  className="rounded-full border border-dashed border-slate-300 px-2 py-0.5 text-xs text-slate-500 transition-colors hover:border-slate-400 hover:text-slate-900 dark:border-slate-700 dark:hover:text-slate-100"
                >
                  {tag.name}
                  <span className="ml-1 text-slate-400 tabular-nums">{tag.idea_count}</span>
                </button>
              ))}
            </div>
          ) : null}
        </div>
      )}

      {error ? <FieldError>{error.message}</FieldError> : null}
      {formError ? <FieldError>{formError}</FieldError> : null}
    </section>
  )
}
