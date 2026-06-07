import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import NotificationsPage from './NotificationsPage'
import type { UserNotification } from '../contexts/UserNotificationContext'

afterEach(cleanup)

vi.mock('../components/AppLayout', () => ({
  default: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
}))

const markRead = vi.fn()
const remove = vi.fn()
const markAllRead = vi.fn()
const removeAll = vi.fn()

let mockNotifications: UserNotification[]
let mockUnread: number

vi.mock('../contexts/UserNotificationContext', () => ({
  useUserNotifications: () => ({
    notifications: mockNotifications,
    unreadCount: mockUnread,
    reload: vi.fn(),
    markRead,
    markAllRead,
    remove,
    removeAll,
  }),
}))

function renderPage() {
  return render(
    <MemoryRouter>
      <NotificationsPage />
    </MemoryRouter>
  )
}

describe('NotificationsPage', () => {
  it('shows empty state when there are no notifications', () => {
    mockNotifications = []
    mockUnread = 0
    renderPage()
    expect(screen.getByText('Nenhuma notificação.')).toBeTruthy()
  })

  it('lists notifications and triggers mark-read / delete', () => {
    mockNotifications = [
      { id: 1, type: 'warning', message: 'Disco quase cheio', created_at: '2026-06-07T12:00:00Z', read: false },
      { id: 2, type: 'info', message: 'Já lida', created_at: '2026-06-06T10:00:00Z', read: true },
    ]
    mockUnread = 1
    renderPage()

    expect(screen.getByText('Disco quase cheio')).toBeTruthy()
    expect(screen.getByText('Já lida')).toBeTruthy()
    expect(screen.getByText('1 não lida(s)')).toBeTruthy()

    // unread item has a mark-read button; read item does not.
    const markButtons = screen.getAllByLabelText('Marcar como lida')
    expect(markButtons).toHaveLength(1)
    fireEvent.click(markButtons[0])
    expect(markRead).toHaveBeenCalledWith(1)

    const deleteButtons = screen.getAllByLabelText('Excluir')
    expect(deleteButtons).toHaveLength(2)
    fireEvent.click(deleteButtons[0])
    expect(remove).toHaveBeenCalledWith(1)
  })

  it('mark-all and clear-all call their actions', () => {
    mockNotifications = [
      { id: 1, type: 'error', message: 'x', created_at: '2026-06-07T12:00:00Z', read: false },
    ]
    mockUnread = 1
    renderPage()

    fireEvent.click(screen.getByText('Marcar todas como lidas'))
    expect(markAllRead).toHaveBeenCalled()
    fireEvent.click(screen.getByText('Limpar tudo'))
    expect(removeAll).toHaveBeenCalled()
  })
})
