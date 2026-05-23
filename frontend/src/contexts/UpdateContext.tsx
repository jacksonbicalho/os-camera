import { createContext, useContext, useEffect, useState, useCallback } from 'react'
import type { ReactNode } from 'react'
import { getRole, authHeaders } from '../auth'

interface UpdateContextValue {
  updateAvailable: boolean
  clearUpdate: () => void
}

const UpdateContext = createContext<UpdateContextValue>({ updateAvailable: false, clearUpdate: () => {} })

export function UpdateProvider({ children }: { children: ReactNode }) {
  const [updateAvailable, setUpdateAvailable] = useState(false)

  useEffect(() => {
    if (getRole() !== 'admin') return
    fetch('/api/update/check', { headers: authHeaders() })
      .then(r => r.ok ? r.json() : null)
      .then(d => { if (d?.update_available) setUpdateAvailable(true) })
      .catch(() => {})
  }, [])

  const clearUpdate = useCallback(() => setUpdateAvailable(false), [])

  return (
    <UpdateContext.Provider value={{ updateAvailable, clearUpdate }}>
      {children}
    </UpdateContext.Provider>
  )
}

export function useUpdateAvailable() {
  return useContext(UpdateContext)
}
