import { useState, useCallback } from 'react'

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
  notify(cameraId: string, score: number, onClick?: () => void): void
}

export function useBrowserNotifications(): BrowserNotificationsHook {
  const supported = isSupported()
  const [permission, setPermission] = useState<NotificationPermission | 'unavailable'>(
    () => (supported ? Notification.permission : 'unavailable'),
  )
  const [enabled, setEnabled] = useState<boolean>(() => supported && loadPref())

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
    (cameraId: string, score: number, onClick?: () => void) => {
      if (!supported || !enabled || permission !== 'granted') return
      const n = new Notification(`Movimento detectado — ${cameraId}`, {
        body: `Score: ${score.toFixed(3)}`,
        tag: `motion-${cameraId}`,
      })
      if (onClick) {
        n.onclick = () => {
          window.focus()
          onClick()
        }
      }
    },
    [supported, enabled, permission],
  )

  return { supported, permission, enabled, requestAndEnable, disable, notify }
}
