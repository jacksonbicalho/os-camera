import { useEffect, useState } from 'react'
import { authHeaders, getRole, onUnauthorized } from '../auth'

export interface UpdateStatus {
  current: string
  latest: string
  notes_md: string
  image: string
  min_supported: string
  update_available: boolean
  apply_mode: string
  checked_at: string
  error: string
}

export interface ApplyResult {
  ok: boolean
  error?: string
}

const POLL_MS = 60_000

export function useUpdates() {
  const [status, setStatus] = useState<UpdateStatus | null>(null)
  const [loading, setLoading] = useState(false)
  const [key, setKey] = useState(0)

  const isAdmin = getRole() === 'admin'

  useEffect(() => {
    if (!isAdmin) return

    let active = true
    const load = () => {
      setLoading(true)
      fetch('/api/updates', { headers: authHeaders() })
        .then(res => {
          if (res.status === 401) { onUnauthorized(); return null }
          return res.json()
        })
        .then(data => { if (active && data) setStatus(data) })
        .catch(() => {})
        .finally(() => { if (active) setLoading(false) })
    }

    load()
    const id = setInterval(load, POLL_MS)
    return () => { active = false; clearInterval(id) }
  }, [isAdmin, key])

  const reload = () => setKey(k => k + 1)

  const applyUpdate = async (): Promise<ApplyResult> => {
    const res = await fetch('/api/updates/apply', { method: 'POST', headers: authHeaders() })
    if (res.status === 401) { onUnauthorized(); return { ok: false, error: 'não autorizado' } }
    if (res.status === 202) return { ok: true }
    let error = 'falha ao iniciar a atualização'
    try {
      const data = await res.json()
      if (data && typeof data.error === 'string') error = data.error
    } catch { /* corpo vazio */ }
    return { ok: false, error }
  }

  return { status, loading, reload, applyUpdate }
}
