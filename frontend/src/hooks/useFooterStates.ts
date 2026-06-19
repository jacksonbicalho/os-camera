import { useEffect, useRef, useState } from 'react'
import { authHeaders, onUnauthorized } from '../auth'

export interface FooterState {
  classifier_id: number
  camera_id: string
  name: string
  state: string
}

// useFooterStates busca os classificadores que o usuário marcou pra ver no rodapé
// (footer_enabled + destinatário) e o estado atual de cada, em poll. Devolve também
// o conjunto que acabou de mudar de estado, para piscar por ~1 s.
export function useFooterStates(pollMs = 5000): { states: FooterState[]; flashing: Set<number> } {
  const [states, setStates] = useState<FooterState[]>([])
  const [flashing, setFlashing] = useState<Set<number>>(new Set())
  const prev = useRef<Record<number, string>>({})
  const timers = useRef<Record<number, ReturnType<typeof setTimeout>>>({})

  useEffect(() => {
    const cancelled = { value: false }
    function load() {
      fetch('/api/me/footer-states', { headers: authHeaders() })
        .then(res => {
          if (res.status === 401) { onUnauthorized(); return null }
          return res.ok ? res.json() : null
        })
        .then((data: FooterState[] | null) => {
          if (cancelled.value || !Array.isArray(data)) return
          const changed: number[] = []
          for (const s of data) {
            const last = prev.current[s.classifier_id]
            if (last !== undefined && last !== s.state) changed.push(s.classifier_id)
            prev.current[s.classifier_id] = s.state
          }
          setStates(data)
          if (changed.length === 0) return
          setFlashing(f => {
            const next = new Set(f)
            changed.forEach(id => next.add(id))
            return next
          })
          changed.forEach(id => {
            clearTimeout(timers.current[id])
            timers.current[id] = setTimeout(() => {
              setFlashing(f => {
                const next = new Set(f)
                next.delete(id)
                return next
              })
            }, 1000)
          })
        })
        .catch(() => {})
    }
    load()
    const interval = setInterval(load, pollMs)
    const pending = timers.current
    return () => {
      cancelled.value = true
      clearInterval(interval)
      Object.values(pending).forEach(clearTimeout)
    }
  }, [pollMs])

  return { states, flashing }
}
