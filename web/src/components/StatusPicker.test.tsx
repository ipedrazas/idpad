import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { STATUS_OPTIONS } from '../lib/status'
import { StatusPicker } from './StatusPicker'

function renderPicker(props: Partial<Parameters<typeof StatusPicker>[0]> = {}) {
  const onChange = vi.fn()
  const view = render(<StatusPicker status="draft" onChange={onChange} {...props} />)
  return { ...view, onChange }
}

describe('StatusPicker', () => {
  it('offers every state and checks the one the idea is in', () => {
    renderPicker({ status: 'in_progress' })

    // All four are on screen: seeing where the idea could go next is half the
    // point of a radio group rather than a menu.
    expect(screen.getAllByRole('radio')).toHaveLength(STATUS_OPTIONS.length)
    expect(screen.getByRole('radio', { name: 'In progress' })).toBeChecked()
    expect(screen.getByRole('radio', { name: 'Draft' })).not.toBeChecked()
  })

  it('reports the state that was picked', async () => {
    const { onChange } = renderPicker({ status: 'draft' })

    await userEvent.click(screen.getByRole('radio', { name: 'Done' }))
    expect(onChange).toHaveBeenCalledExactlyOnceWith('done')
  })

  it('does not re-send the state the idea is already in', async () => {
    const { onChange } = renderPicker({ status: 'done' })

    await userEvent.click(screen.getByRole('radio', { name: 'Done' }))
    expect(onChange).not.toHaveBeenCalled()
  })

  it('refuses further moves while one is in flight', async () => {
    // Two moves racing would leave the idea in whichever state replied last,
    // which is not necessarily the one clicked last.
    const { onChange } = renderPicker({ status: 'draft', pending: true })

    await userEvent.click(screen.getByRole('radio', { name: 'Rejected' }))
    expect(onChange).not.toHaveBeenCalled()
  })
})
