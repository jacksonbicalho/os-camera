import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import EventsPanelHeader from './EventsPanelHeader'

afterEach(cleanup)

interface Overrides {
  allSelected?: boolean
  someSelected?: boolean
  canMarkRead?: boolean
}

function setup(overrides: Overrides = {}) {
  const onToggleAll = vi.fn()
  const onMarkRead = vi.fn()
  const onDelete = vi.fn()
  render(
    <EventsPanelHeader
      allSelected={overrides.allSelected ?? false}
      someSelected={overrides.someSelected ?? false}
      canMarkRead={overrides.canMarkRead ?? false}
      onToggleAll={onToggleAll}
      onMarkRead={onMarkRead}
      onDelete={onDelete}
    />,
  )
  return { onToggleAll, onMarkRead, onDelete }
}

describe('EventsPanelHeader', () => {
  it('shows the panel title "Eventos" (not "Notificações")', () => {
    setup()
    expect(screen.getByText('Eventos')).toBeTruthy()
    expect(screen.queryByText('Notificações')).toBeNull()
  })

  it('does not show the action row when nothing is selected', () => {
    setup({ someSelected: false, canMarkRead: true })
    expect(screen.queryByText('Excluir')).toBeNull()
    expect(screen.queryByText('Marcar como lido')).toBeNull()
  })

  it('shows inline mark-read + delete actions when something is selected', () => {
    const { onMarkRead, onDelete } = setup({ someSelected: true, canMarkRead: true })
    fireEvent.click(screen.getByText('Marcar como lido'))
    fireEvent.click(screen.getByText('Excluir'))
    expect(onMarkRead).toHaveBeenCalledTimes(1)
    expect(onDelete).toHaveBeenCalledTimes(1)
  })

  it('hides "Marcar como lido" when nothing unread is selected, still shows Excluir', () => {
    setup({ someSelected: true, canMarkRead: false })
    expect(screen.queryByText('Marcar como lido')).toBeNull()
    expect(screen.getByText('Excluir')).toBeTruthy()
  })

  it('never renders a "Marcar como não lido" action', () => {
    setup({ someSelected: true, canMarkRead: true })
    expect(screen.queryByText(/marcar como n[ãa]o lid/i)).toBeNull()
  })

  it('calls onToggleAll when the select-all checkbox changes', () => {
    const { onToggleAll } = setup()
    fireEvent.click(screen.getByLabelText('Selecionar todos'))
    expect(onToggleAll).toHaveBeenCalled()
  })

  it('gives unique ids to the title and the action controls', () => {
    setup({ someSelected: true, canMarkRead: true })
    expect(document.getElementById('events-panel-title')?.textContent).toBe('Eventos')
    expect(document.getElementById('events-select-all')).not.toBeNull()
    expect(document.getElementById('events-action-mark-read')).not.toBeNull()
    expect(document.getElementById('events-action-delete')).not.toBeNull()
  })

  it('as ações usam o Button padronizado (buttonVariants)', () => {
    setup({ someSelected: true, canMarkRead: true })
    // buttonVariants injeta a base "inline-flex … rounded-md" — ausente nos <button> crus
    expect(document.getElementById('events-action-mark-read')!.className).toContain('rounded-md')
    expect(document.getElementById('events-action-delete')!.className).toContain('rounded-md')
  })
})
