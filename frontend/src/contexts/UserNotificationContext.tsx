/* eslint-disable react-refresh/only-export-components */
import { createContext, useCallback, useContext, useEffect, useState } from 'react'
import { authHeaders, getToken, onUnauthorized } from '../auth'

export type UserNotificationType = 'success' | 'error' | 'warning' | 'info'

export interface UserNotification {
  id: number
  type: UserNotificationType
  title?: string
  message: string
  link?: string
  created_at: string
  read: boolean
}

interface UserNotificationContextValue {
  notifications: UserNotification[]
  unreadCount: number
  reload: () => void
  markRead: (id: number) => void
  markAllRead: () => void
  remove: (id: number) => void
  removeAll: () => void
}

const Ctx = createContext<UserNotificationContextValue | null>(null)
const POLL_MS = 30000

export function UserNotificationProvider({ children }: { children: React.ReactNode }) {
  const [notifications, setNotifications] = useState<UserNotification[]>([])
  const [unreadCount, setUnreadCount] = useState(0)

  // reload only mutates state asynchronously (inside the fetch .then) so it is
  // safe to call from an effect without cascading synchronous renders.
  const reload = useCallback(() => {
    if (!getToken()) return
    fetch('/api/notifications', { headers: authHeaders() })
      .then(res => {
        if (res.status === 401) { onUnauthorized(); return null }
        return res.json()
      })
      .then(data => {
        if (data) {
          setNotifications(data.notifications ?? [])
          setUnreadCount(data.unread_count ?? 0)
        }
      })
      .catch(() => {})
  }, [])

  useEffect(() => {
    reload()
    const t = setInterval(reload, POLL_MS)
    const onToken = () => {
      if (getToken()) {
        reload()
      } else {
        // logged out: clear so a new login doesn't briefly show the previous user's badge
        setNotifications([])
        setUnreadCount(0)
      }
    }
    window.addEventListener('camera:token-changed', onToken)
    return () => {
      clearInterval(t)
      window.removeEventListener('camera:token-changed', onToken)
    }
  }, [reload])

  const mutate = useCallback(
    (url: string, method: string) =>
      fetch(url, { method, headers: authHeaders() }).then(() => reload()).catch(() => {}),
    [reload]
  )

  const markRead = useCallback((id: number) => mutate(`/api/notifications/${id}/read`, 'POST'), [mutate])
  const markAllRead = useCallback(() => mutate('/api/notifications/read-all', 'POST'), [mutate])
  const remove = useCallback((id: number) => mutate(`/api/notifications/${id}`, 'DELETE'), [mutate])
  const removeAll = useCallback(() => mutate('/api/notifications', 'DELETE'), [mutate])

  return (
    <Ctx.Provider value={{ notifications, unreadCount, reload, markRead, markAllRead, remove, removeAll }}>
      {children}
    </Ctx.Provider>
  )
}

export function useUserNotifications(): UserNotificationContextValue {
  const ctx = useContext(Ctx)
  if (!ctx) throw new Error('useUserNotifications must be used inside UserNotificationProvider')
  return ctx
}
