/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useEffect, useState } from 'react'
import { getToken } from '../auth'

const STORAGE_KEY = 'camera_notifications'
const MAX_NOTIFICATIONS = 100
const POLL_INTERVAL_MS = 30_000

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
  const [cameraIds, setCameraIds] = useState<string[]>([])

  function update(next: Notification[]) {
    setNotifications(next)
    save(next)
  }

  // Fetch camera list with 30s polling
  useEffect(() => {
    async function fetchCameras() {
      const token = getToken()
      if (!token) return

      try {
        const res = await fetch('/api/settings', {
          headers: { Authorization: `Bearer ${token}` },
        })
        if (!res.ok) return
        const data = await res.json()
        const ids: string[] = (data.cameras ?? []).map((c: { id: string }) => c.id)
        setCameraIds(ids)
      } catch {
        // silently ignore network errors
      }
    }

    fetchCameras()
    const timer = setInterval(fetchCameras, POLL_INTERVAL_MS)
    return () => clearInterval(timer)
  }, [])

  // Open one EventSource per camera
  useEffect(() => {
    if (cameraIds.length === 0) return
    const token = getToken()
    if (!token) return

    const sources = cameraIds.map((id) => {
      const url = `/api/cameras/${id}/motion/live?token=${encodeURIComponent(token)}`
      const es = new EventSource(url)

      es.onmessage = (e) => {
        try {
          const payload = JSON.parse(e.data)
          const time: string = payload.time ?? new Date().toISOString()
          const notification: Notification = {
            id: `${id}-${time}`,
            type: 'motion',
            cameraId: id,
            time,
            score: payload.score ?? 0,
            read: false,
          }
          setNotifications((current) => {
            const next = [notification, ...current].slice(0, MAX_NOTIFICATIONS)
            save(next)
            return next
          })
        } catch {
          // ignore malformed events
        }
      }

      return es
    })

    return () => sources.forEach((es) => es.close())
  }, [cameraIds])

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
      value={{ notifications, unreadCount, markRead, markAllRead, markSelectedRead, markAllUnread, remove, removeAll, removeSelected }}
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
