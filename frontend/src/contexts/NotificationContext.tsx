/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { getToken } from '../auth'
import { useBrowserNotifications } from '../hooks/useBrowserNotifications'

const STORAGE_KEY = 'camera_notifications'
const MAX_NOTIFICATIONS = 100

export interface Notification {
  id: string
  type: 'motion'
  cameraId: string
  time: string
  score: number
  read: boolean
}

interface NotificationContextValue {
  notifications: Notification[]
  unreadCount: number
  markRead(id: string): void
  markAllRead(): void
  markSelectedRead(ids: string[]): void
  markAllUnread(ids: string[]): void
  remove(id: string): void
  removeAll(): void
  removeSelected(ids: string[]): void
  browserSupported: boolean
  browserPermission: NotificationPermission | 'unavailable'
  browserEnabled: boolean
  enableBrowserNotifications(): Promise<void>
  disableBrowserNotifications(): void
}

const NotificationContext = createContext<NotificationContextValue | null>(null)

function load(): Notification[] {
  try {
    return JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]')
  } catch {
    return []
  }
}

function save(notifications: Notification[]) {
  if (notifications.length === 0) {
    localStorage.removeItem(STORAGE_KEY)
  } else {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(notifications))
  }
}

export function NotificationProvider({ children }: { children: React.ReactNode }) {
  const [notifications, setNotifications] = useState<Notification[]>(load)
  const navigate = useNavigate()
  const {
    supported: browserSupported,
    permission: browserPermission,
    enabled: browserEnabled,
    requestAndEnable: enableBrowserNotifications,
    disable: disableBrowserNotifications,
    notify: browserNotify,
  } = useBrowserNotifications()

  const browserNotifyRef = useRef(browserNotify)
  useEffect(() => { browserNotifyRef.current = browserNotify }, [browserNotify])

  function update(next: Notification[]) {
    setNotifications(next)
    save(next)
  }

  // Single SSE connection that receives events from all accessible cameras
  useEffect(() => {
    const token = getToken()
    if (!token) return

    const url = `/api/motion/live?token=${encodeURIComponent(token)}`
    const es = new EventSource(url)

    es.onmessage = (e) => {
      try {
        const payload = JSON.parse(e.data)
        const id: string = payload.camera_id ?? 'unknown'
        const time: string = payload.time ?? new Date().toISOString()
        const score: number = payload.score ?? 0
        const notification: Notification = {
          id: `${id}-${time}`,
          type: 'motion',
          cameraId: id,
          time,
          score,
          read: false,
        }
        setNotifications((current) => {
          const next = [notification, ...current].slice(0, MAX_NOTIFICATIONS)
          save(next)
          return next
        })
        browserNotifyRef.current(id, score, () => {
          navigate(`/cameras/${id}`, { state: { eventTime: time } })
        })
      } catch {
        // ignore malformed events
      }
    }

    return () => es.close()
  }, [navigate])

  function markRead(id: string) {
    update(notifications.map((n) => (n.id === id ? { ...n, read: true } : n)))
  }

  function markAllRead() {
    update(notifications.map((n) => ({ ...n, read: true })))
  }

  function markSelectedRead(ids: string[]) {
    const idSet = new Set(ids)
    update(notifications.map((n) => idSet.has(n.id) ? { ...n, read: true } : n))
  }

  function markAllUnread(ids: string[]) {
    const idSet = new Set(ids)
    update(notifications.map((n) => idSet.has(n.id) ? { ...n, read: false } : n))
  }

  function remove(id: string) {
    update(notifications.filter((n) => n.id !== id))
  }

  function removeAll() {
    update([])
  }

  function removeSelected(ids: string[]) {
    const idSet = new Set(ids)
    update(notifications.filter((n) => !idSet.has(n.id)))
  }

  const unreadCount = notifications.filter((n) => !n.read).length

  return (
    <NotificationContext.Provider
      value={{
        notifications, unreadCount,
        markRead, markAllRead, markSelectedRead, markAllUnread,
        remove, removeAll, removeSelected,
        browserSupported, browserPermission, browserEnabled,
        enableBrowserNotifications, disableBrowserNotifications,
      }}
    >
      {children}
    </NotificationContext.Provider>
  )
}

export function useNotifications(): NotificationContextValue {
  const ctx = useContext(NotificationContext)
  if (!ctx) throw new Error('useNotifications must be used inside NotificationProvider')
  return ctx
}
