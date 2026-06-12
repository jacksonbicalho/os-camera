/* eslint-disable react-refresh/only-export-components */
import { createContext, useCallback, useContext, useEffect, useState } from 'react'
import { authHeaders, getToken, onUnauthorized } from '../auth'

// dark/light/system are COLOR MODES (the light/dark rendering). The theme
// identity (palette + typography) is a separate concept; for now there is a
// single "default" theme, so only the mode is user-selectable/persisted.
export type Mode = 'dark' | 'light' | 'system'
export type Theme = 'default'
type Resolved = 'dark' | 'light'

interface ThemeContextValue {
  mode: Mode
  setMode: (m: Mode) => void
  theme: Theme
}

const Ctx = createContext<ThemeContextValue>({ mode: 'dark', setMode: () => {}, theme: 'default' })

function prefersDark(): boolean {
  return typeof window !== 'undefined' && !!window.matchMedia?.('(prefers-color-scheme: dark)').matches
}

// Resolve a color mode to a concrete dark/light. "system" follows the OS.
export function resolveMode(m: Mode): Resolved {
  if (m === 'system') return prefersDark() ? 'dark' : 'light'
  return m
}

function applyMode(m: Mode) {
  document.documentElement.setAttribute('data-mode', resolveMode(m))
}

function isMode(v: unknown): v is Mode {
  return v === 'dark' || v === 'light' || v === 'system'
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [mode, setModeState] = useState<Mode>('dark')

  const load = useCallback(() => {
    if (!getToken()) {
      applyMode('dark')
      return
    }
    // The persisted preference (users.theme) currently holds the color mode.
    fetch('/api/me/preferences', { headers: authHeaders() })
      .then(res => {
        if (res.status === 401) { onUnauthorized(); return null }
        return res.json()
      })
      .then(data => {
        if (data && isMode(data.theme)) {
          setModeState(data.theme)
          applyMode(data.theme)
        }
      })
      .catch(() => {})
  }, [])

  useEffect(() => {
    load()
    const onToken = () => load()
    window.addEventListener('camera:token-changed', onToken)
    return () => window.removeEventListener('camera:token-changed', onToken)
  }, [load])

  // While in "system" mode, re-apply when the OS color scheme changes.
  useEffect(() => {
    if (mode !== 'system' || !window.matchMedia) return
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    const onChange = () => applyMode('system')
    mq.addEventListener('change', onChange)
    return () => mq.removeEventListener('change', onChange)
  }, [mode])

  const setMode = useCallback((m: Mode) => {
    setModeState(m)
    applyMode(m)
    if (!getToken()) return
    fetch('/api/me/preferences', {
      method: 'PUT',
      headers: { ...authHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify({ theme: m }),
    }).catch(() => {})
  }, [])

  return <Ctx.Provider value={{ mode, setMode, theme: 'default' }}>{children}</Ctx.Provider>
}

export function useTheme(): ThemeContextValue {
  return useContext(Ctx)
}
