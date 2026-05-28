import { useCallback, useEffect, useState } from 'react'
import { authHeaders, onUnauthorized } from '../auth'
import { useEventSource } from './useEventSource'

export interface MotionDailyPeak {
  camera_id: string
  peak_raw_score: number
  date: string
}

export function useMotionPeak(cameraId: string | undefined) {
  const [peak, setPeak] = useState<MotionDailyPeak | null>(null)

  useEffect(() => {
    if (!cameraId) return

    const load = () => {
      fetch(`/api/cameras/${cameraId}/motion/daily-peak`, { headers: authHeaders() })
        .then(res => {
          if (res.status === 401) { onUnauthorized(); return null }
          if (!res.ok) return null
          return res.json()
        })
        .then(data => { if (data) setPeak(data) })
        .catch(() => {})
    }

    load()
    const id = setInterval(load, 30_000)
    return () => clearInterval(id)
  }, [cameraId])

  // Atualização em tempo real: se o score recebido for maior que o pico atual, atualiza
  const handleScore = useCallback((data: string) => {
    const ev = JSON.parse(data) as { score: number }
    const today = new Date().toISOString().slice(0, 10)
    setPeak(prev => {
      if (!prev || prev.date !== today) return prev
      if (ev.score <= prev.peak_raw_score) return prev
      return { ...prev, peak_raw_score: ev.score }
    })
  }, [])

  useEventSource(cameraId ? `/api/cameras/${cameraId}/motion/scores` : null, handleScore)

  return peak
}
