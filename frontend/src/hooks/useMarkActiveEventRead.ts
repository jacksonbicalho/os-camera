import { useEffect, useRef } from 'react'
import { useNotifications } from '../contexts/NotificationContext'

/**
 * Marks the motion-event notification matching the active event as read whenever
 * the active event changes. This is the single funnel for "the event is being
 * played", so it covers every path — manual click, continuous sequential advance
 * and deep-link — keeping the sidebar-notifications unread badge truthful.
 *
 * The notification id mirrors NotificationContext: `${cameraId}-${time}`.
 * markRead is read through a ref so the effect depends only on the active event,
 * avoiding a re-render loop (markRead identity changes on every provider render).
 */
export function useMarkActiveEventRead(cameraId: string, activeEventTime: string | null) {
  const { markRead } = useNotifications()
  const markReadRef = useRef(markRead)
  useEffect(() => { markReadRef.current = markRead }, [markRead])

  useEffect(() => {
    if (activeEventTime === null) return
    markReadRef.current(`${cameraId}-${activeEventTime}`)
  }, [cameraId, activeEventTime])
}
