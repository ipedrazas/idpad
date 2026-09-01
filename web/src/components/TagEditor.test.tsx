import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'

import type { Tag } from '../api/types'
import { TagEditor } from './TagEditor'

function tag(name: string, slug: string): Tag {
  return { id: `id-${slug}`, name, slug, created_at: '2026-01-01T00:00:00Z' }
}

function renderEditor(props: Partial<Parameters<typeof TagEditor>[0]> = {}) {
  const onChange = vi.fn().mockResolvedValue(undefined)
  const view = render(
    <MemoryRouter>
      <TagEditor tags={[]} loading={false} error={null} onChange={onChange} {...props} />
    </MemoryRouter>,
  )
  return { ...view, onChange }
}

describe('TagEditor suggest button', () => {
  it('is absent when the deployment has no tagging service', () => {
    // onSuggest is omitted exactly when /features reports auto_tagging false.
    renderEditor()
    expect(screen.queryByRole('button', { name: /suggest tags/i })).not.toBeInTheDocument()
  })

  it('is shown and calls the handler when a service is configured', async () => {
    const onSuggest = vi.fn().mockResolvedValue(undefined)
    renderEditor({ onSuggest })

    await userEvent.click(screen.getByRole('button', { name: /suggest tags/i }))
    expect(onSuggest).toHaveBeenCalledTimes(1)
  })

  it('says what it is doing while the service is thinking', async () => {
    // The service can take tens of seconds on a cold start, so a silent
    // button would read as broken.
    let release: () => void = () => {}
    const onSuggest = vi.fn().mockReturnValue(new Promise<void>((resolve) => (release = resolve)))
    renderEditor({ onSuggest })

    await userEvent.click(screen.getByRole('button', { name: /suggest tags/i }))

    const pending = await screen.findByRole('button', { name: /reading the idea/i })
    expect(pending).toBeDisabled()

    release()
    await waitFor(() => expect(screen.getByRole('button', { name: /suggest tags/i })).toBeEnabled())
  })

  it('reports a failure without losing the existing tags', async () => {
    const onSuggest = vi.fn().mockRejectedValue(new Error('the tagging service could not be reached'))
    renderEditor({ tags: [tag('Hand Picked', 'hand-picked')], onSuggest })

    await userEvent.click(screen.getByRole('button', { name: /suggest tags/i }))

    expect(await screen.findByText(/could not be reached/i)).toBeInTheDocument()
    expect(screen.getByText('Hand Picked')).toBeInTheDocument()
  })
})

describe('TagEditor chips', () => {
  it('adds a tag on Enter and sends the whole resulting set', async () => {
    const { onChange } = renderEditor({ tags: [tag('Go', 'go')] })

    await userEvent.type(screen.getByLabelText('Add a tag'), 'Postgres{Enter}')

    // The API replaces the set, so the existing tag has to be sent back too.
    expect(onChange).toHaveBeenCalledWith(['Go', 'Postgres'])
  })

  it('ignores a name that folds onto a tag already present', async () => {
    const { onChange } = renderEditor({ tags: [tag('Machine Learning', 'machine-learning')] })

    await userEvent.type(screen.getByLabelText('Add a tag'), 'machine-learning{Enter}')

    expect(onChange).not.toHaveBeenCalled()
  })

  it('removes a tag from the set', async () => {
    const { onChange } = renderEditor({ tags: [tag('Go', 'go'), tag('Postgres', 'postgres')] })

    await userEvent.click(screen.getByRole('button', { name: 'Remove tag Go' }))

    expect(onChange).toHaveBeenCalledWith(['Postgres'])
  })
})
