/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useState } from 'react'

export type DisplayMode = 'icons-only' | 'icons-text' | 'text-only'

export interface DisplayModeState {
  sidebar: DisplayMode
  player: DisplayMode
}

type SetDisplayMode = (section: keyof DisplayModeState, mode: DisplayMode) => void

const STORAGE_KEY = 'ui-display-mode'
const DEFAULT: DisplayModeState = { sidebar: 'icons-only', player: 'icons-only' }

function load(): DisplayModeState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return DEFAULT
    const parsed = JSON.parse(raw)
    return {
      sidebar: parsed.sidebar ?? DEFAULT.sidebar,
      player: parsed.player ?? DEFAULT.player,
    }
  } catch {
    return DEFAULT
  }
}

const DisplayModeContext = createContext<DisplayModeState>(DEFAULT)
const SetDisplayModeContext = createContext<SetDisplayMode>(() => {})

export function DisplayModeProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<DisplayModeState>(load)

  function set(section: keyof DisplayModeState, mode: DisplayMode) {
    setState(prev => {
      const next = { ...prev, [section]: mode }
      localStorage.setItem(STORAGE_KEY, JSON.stringify(next))
      return next
    })
  }

  return (
    <SetDisplayModeContext.Provider value={set}>
      <DisplayModeContext.Provider value={state}>
        {children}
      </DisplayModeContext.Provider>
    </SetDisplayModeContext.Provider>
  )
}

export function useDisplayMode(): DisplayModeState {
  return useContext(DisplayModeContext)
}

export function useSetDisplayMode(): SetDisplayMode {
  return useContext(SetDisplayModeContext)
}
