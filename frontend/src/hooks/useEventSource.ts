import { useEffect } from 'react'
import { getToken } from '../auth'

export function useEventSource(path: string | null, onMessage: (data: string) => void) {
  useEffect(() => {
    if (!path) return
    const token = getToken()
    if (!token) return

    const sep = path.includes('?') ? '&' : '?'
    const es = new EventSource(`${path}${sep}token=${encodeURIComponent(token)}`)
    es.onmessage = (e) => onMessage(e.data)
    return () => es.close()
  }, [path, onMessage])
}
