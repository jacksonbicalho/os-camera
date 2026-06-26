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
  net_mbps: number
  mem_rss_bytes: number
  sys_mem_total_bytes: number
  sys_mem_free_bytes: number
  goroutines: number
}

// POLL_MS: intervalo do poll de /api/stats. FAIL_THRESHOLD: nº de falhas
// consecutivas antes de marcar "desconectado" (tolerância a blip transitório).
const POLL_MS = 10_000
const FAIL_THRESHOLD = 2

// useStats faz poll de /api/stats e devolve { stats, connected }. `connected`
// começa false, vira true no 1º sucesso e só volta a false após FAIL_THRESHOLD
// falhas consecutivas (rejeição de rede ou resposta !ok) — o servidor cair derruba
// o indicador, mas um soluço isolado não. O último `stats` é mantido na falha
// (a UI de dados não pisca; quem sinaliza a queda é `connected`).
export function useStats(): { stats: Stats | null; connected: boolean } {
  const [stats, setStats] = useState<Stats | null>(null)
  const [connected, setConnected] = useState(false)

  useEffect(() => {
    const cancelled = { value: false }
    let failures = 0

    function onSuccess(data: Stats) {
      if (cancelled.value) return
      failures = 0
      setStats(data)
      setConnected(true)
    }
    function onFailure() {
      if (cancelled.value) return
      failures += 1
      if (failures >= FAIL_THRESHOLD) setConnected(false)
    }

    function fetchStats() {
      fetch('/api/stats', { headers: authHeaders() })
        .then(res => {
          if (res.status === 401) { onUnauthorized(); return }
          if (!res.ok) { onFailure(); return }
          res.json().then(onSuccess).catch(onFailure)
        })
        .catch(onFailure)
    }
    fetchStats()
    const interval = setInterval(fetchStats, POLL_MS)
    return () => { cancelled.value = true; clearInterval(interval) }
  }, [])

  return { stats, connected }
}
