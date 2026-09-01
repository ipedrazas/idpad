import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import type { Thread } from '../api/types'
import { CommentThread } from './CommentThread'

const thread: Thread = {
  id: 'thread-1',
  idea_id: 'idea-1',
  parent_id: null,
  anchor: { block_index: 0, start_offset: 10, end_offset: 20, snippet: 'definitely' },
  body: 'Do we have the capacity?',
  status: 'open',
  detached: false,
  created_at: new Date().toISOString(),
  updated_at: new Date().toISOString(),
  replies: [
    {
      id: 'reply-1',
      idea_id: 'idea-1',
      parent_id: 'thread-1',
      anchor: null,
      body: 'Yes, next sprint.',
      status: 'open',
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
  ],
}

function renderThread(overrides: Partial<Thread> = {}, handlers: Partial<Parameters<typeof CommentThread>[0]> = {}) {
  const props = {
    thread: { ...thread, ...overrides },
    index: 0,
    active: false,
    onActivate: vi.fn(),
    onReply: vi.fn().mockResolvedValue(undefined),
    onEdit: vi.fn().mockResolvedValue(undefined),
    onDelete: vi.fn().mockResolvedValue(undefined),
    onToggleResolved: vi.fn().mockResolvedValue(undefined),
    ...handlers,
  }
  render(<CommentThread {...props} />)
  return props
}

describe('CommentThread', () => {
  it('shows the anchored snippet, the comment and its replies', () => {
    renderThread()

    expect(screen.getByText(/definitely/)).toBeInTheDocument()
    expect(screen.getByText('Do we have the capacity?')).toBeInTheDocument()
    expect(screen.getByText('Yes, next sprint.')).toBeInTheDocument()
    expect(screen.queryByText(/no longer attached/i)).not.toBeInTheDocument()
  })

  it('explains a detached comment instead of dropping it', () => {
    renderThread({ detached: true })

    expect(screen.getByText(/no longer attached/i)).toBeInTheDocument()
    // The thread is still fully readable and actionable.
    expect(screen.getByText('Do we have the capacity?')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Resolve' })).toBeInTheDocument()
  })

  it('offers Reopen once resolved', () => {
    renderThread({ status: 'resolved' })
    expect(screen.getByRole('button', { name: 'Reopen' })).toBeInTheDocument()
  })

  it('sends a reply through the composer', async () => {
    const user = userEvent.setup()
    const props = renderThread()

    await user.click(screen.getByRole('button', { name: 'Reply' }))
    await user.type(screen.getByPlaceholderText('Reply…'), 'Let us scope it first.')
    await user.click(screen.getByRole('button', { name: /^Reply$/ }))

    expect(props.onReply).toHaveBeenCalledWith('Let us scope it first.')
  })

  it('requires a confirmation before deleting', async () => {
    const user = userEvent.setup()
    const props = renderThread()

    await user.click(screen.getAllByRole('button', { name: 'Delete' })[0]!)
    expect(props.onDelete).not.toHaveBeenCalled()

    await user.click(screen.getByRole('button', { name: 'Confirm' }))
    expect(props.onDelete).toHaveBeenCalledWith('thread-1')
  })
})
