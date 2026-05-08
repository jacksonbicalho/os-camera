import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { authHeaders, clearToken } from '../auth'

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
}

export function useStats(redirectTo: string) {
  const navigate = useNavigate()
  const [stats, setStats] = useState<Stats | null>(null)

  useEffect(() => {
    const cancelled = { value: false }
    function fetchStats() {
      fetch('/api/stats', { headers: authHeaders() })
        .then(res => {
          if (res.status === 401) { clearToken(); navigate('/login', { state: { from: redirectTo }, replace: true }); return null }
          return res.json()
        })
        .then(data => { if (!cancelled.value && data) setStats(data) })
        .catch(() => {})
    }
    fetchStats()
    const interval = setInterval(fetchStats, 30_000)
    return () => { cancelled.value = true; clearInterval(interval) }
  }, [navigate, redirectTo])

  return stats
}
