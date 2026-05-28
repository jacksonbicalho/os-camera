import { useEffect, useState } from 'react'
import { authHeaders, onUnauthorized } from '../auth'

export interface CameraHealth {
  id: string
  top_motion_score: number
  min_motion_score: number
  online: boolean
  last_recording_at: string | null
  motion_enabled: boolean
}

export interface Stats {
  recordings_bytes: number
  recordings_count: number
  recordings_duration_seconds: number
  forecast_seconds: number
  disk_total_bytes: number
  disk_free_bytes: number
  camera_count: number
  connected_clients: number
  max_size_bytes: number
  warn_percent: number
  cameras: CameraHealth[]
  os: string
  pid: number
  cpu_percent: number
  mem_rss_bytes: number
  sys_mem_total_bytes: number
  sys_mem_free_bytes: number
  goroutines: number
}

export function useStats() {
  const [stats, setStats] = useState<Stats | null>(null)

  useEffect(() => {
    const cancelled = { value: false }
    function fetchStats() {
      fetch('/api/stats', { headers: authHeaders() })
        .then(res => {
          if (res.status === 401) { onUnauthorized(); return null }
          return res.json()
        })
        .then(data => { if (!cancelled.value && data) setStats(data) })
        .catch(() => {})
    }
    fetchStats()
    const interval = setInterval(fetchStats, 30_000)
    return () => { cancelled.value = true; clearInterval(interval) }
  }, [])

  return stats
}
