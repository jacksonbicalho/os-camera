/* eslint-disable react-refresh/only-export-components */
import { createContext, useCallback, useContext, useEffect, useState } from 'react'
import { authHeaders, getToken, onUnauthorized } from '../auth'

export type Theme = 'dark' | 'moderno' | 'system'
type Resolved = 'dark' | 'moderno'

interface ThemeContextValue {
  theme: Theme
  setTheme: (t: Theme) => void
}

const Ctx = createContext<ThemeContextValue>({ theme: 'dark', setTheme: () => {} })

function prefersDark(): boolean {
  return typeof window !== 'undefined' && !!window.matchMedia?.('(prefers-color-scheme: dark)').matches
}

// Resolve a stored preference to a concrete theme. "system" follows the OS:
// dark scheme → our dark, light scheme → our light ("moderno").
function resolve(t: Theme): Resolved {
  if (t === 'system') return prefersDark() ? 'dark' : 'moderno'
  return t
}

function applyTheme(t: Theme) {
  document.documentElement.setAttribute('data-theme', resolve(t))
}

function isTheme(v: unknown): v is Theme {
  return v === 'dark' || v === 'moderno' || v === 'system'
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [theme, setThemeState] = useState<Theme>('dark')

  const load = useCallback(() => {
    if (!getToken()) {
      applyTheme('dark')
      return
    }
    fetch('/api/me/preferences', { headers: authHeaders() })
      .then(res => {
        if (res.status === 401) { onUnauthorized(); return null }
        return res.json()
      })
      .then(data => {
        if (data && isTheme(data.theme)) {
          setThemeState(data.theme)
          applyTheme(data.theme)
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
    if (theme !== 'system' || !window.matchMedia) return
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    const onChange = () => applyTheme('system')
    mq.addEventListener('change', onChange)
    return () => mq.removeEventListener('change', onChange)
  }, [theme])

  const setTheme = useCallback((t: Theme) => {
    setThemeState(t)
    applyTheme(t)
    if (!getToken()) return
    fetch('/api/me/preferences', {
      method: 'PUT',
      headers: { ...authHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify({ theme: t }),
    }).catch(() => {})
  }, [])

  return <Ctx.Provider value={{ theme, setTheme }}>{children}</Ctx.Provider>
}

export function useTheme(): ThemeContextValue {
  return useContext(Ctx)
}
