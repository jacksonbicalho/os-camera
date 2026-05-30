import { useState, useCallback, useRef } from 'react'

const PREF_KEY = 'camera_browser_notifications_enabled'

function isSupported(): boolean {
  return typeof window !== 'undefined' && typeof Notification !== 'undefined' && !!Notification
}

function loadPref(): boolean {
  return localStorage.getItem(PREF_KEY) === 'true'
}

export interface BrowserNotificationsHook {
  supported: boolean
  permission: NotificationPermission | 'unavailable'
  enabled: boolean
  requestAndEnable(): Promise<void>
  disable(): void
  notify(cameraId: string, score: number, label?: string, onClick?: () => void, cameraName?: string): void
  closeBrowserNotification(cameraId: string): void
  closeAllBrowserNotifications(): void
}

export function useBrowserNotifications(): BrowserNotificationsHook {
  const supported = isSupported()
  const [permission, setPermission] = useState<NotificationPermission | 'unavailable'>(
    () => (supported ? Notification.permission : 'unavailable'),
  )
  const [enabled, setEnabled] = useState<boolean>(() => supported && loadPref())
  const activeRef = useRef<Map<string, Notification>>(new Map())

  const requestAndEnable = useCallback(async () => {
    if (!supported) return
    const result = await Notification.requestPermission()
    setPermission(result)
    if (result === 'granted') {
      setEnabled(true)
      localStorage.setItem(PREF_KEY, 'true')
    }
  }, [supported])

  const disable = useCallback(() => {
    setEnabled(false)
    localStorage.setItem(PREF_KEY, 'false')
  }, [])

  const notify = useCallback(
    (cameraId: string, score: number, label?: string, onClick?: () => void, cameraName?: string) => {
      if (!supported || !enabled || permission !== 'granted') return
      const body = label
        ? `Zona: ${label} · ${(score * 100).toFixed(1)}%`
        : `${(score * 100).toFixed(1)}%`
      const n = new Notification(`Movimento — ${cameraName ?? cameraId}`, {
        body,
        tag: `motion-${cameraId}`,
        icon: '/icon-192.png',
      })
      activeRef.current.set(cameraId, n)
      n.onclose = () => activeRef.current.delete(cameraId)
      n.onclick = () => {
        window.focus()
        onClick?.()
        n.close()
      }
    },
    [supported, enabled, permission],
  )

  const closeBrowserNotification = useCallback((cameraId: string) => {
    const n = activeRef.current.get(cameraId)
    if (n) { n.close(); activeRef.current.delete(cameraId) }
  }, [])

  const closeAllBrowserNotifications = useCallback(() => {
    activeRef.current.forEach(n => n.close())
    activeRef.current.clear()
  }, [])

  return { supported, permission, enabled, requestAndEnable, disable, notify, closeBrowserNotification, closeAllBrowserNotifications }
}
