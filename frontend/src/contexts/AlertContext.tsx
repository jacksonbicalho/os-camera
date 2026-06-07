/* eslint-disable react-refresh/only-export-components */
import { createContext, useCallback, useContext, useMemo, useRef, useState } from 'react'

export type AlertType = 'success' | 'error' | 'warning' | 'info'

export interface Alert {
  id: number
  type: AlertType
  message: string
}

export interface ShowAlertInput {
  type: AlertType
  message: string
  /** Auto-dismiss after this many ms. Omit to use the per-type default
   *  (success/info auto-dismiss; error/warning persist until closed). 0 = persist. */
  durationMs?: number
}

export interface AlertActions {
  showAlert: (input: ShowAlertInput) => number
  dismissAlert: (id: number) => void
  clearAlerts: () => void
}

const defaultDurations: Record<AlertType, number> = {
  success: 4000,
  info: 4000,
  warning: 0,
  error: 0,
}

// Split contexts (same pattern as SidebarContext): readers subscribe to state,
// callers use the stable actions without re-rendering on every alert change.
const AlertStateContext = createContext<Alert[]>([])
const AlertActionsContext = createContext<AlertActions>({
  showAlert: () => 0,
  dismissAlert: () => {},
  clearAlerts: () => {},
})

export function AlertProvider({ children }: { children: React.ReactNode }) {
  const [alerts, setAlerts] = useState<Alert[]>([])
  const nextId = useRef(1)

  const dismissAlert = useCallback((id: number) => {
    setAlerts(curr => curr.filter(a => a.id !== id))
  }, [])

  const clearAlerts = useCallback(() => setAlerts([]), [])

  const showAlert = useCallback((input: ShowAlertInput) => {
    const id = nextId.current++
    setAlerts(curr => [...curr, { id, type: input.type, message: input.message }])
    const duration = input.durationMs ?? defaultDurations[input.type]
    if (duration > 0) {
      setTimeout(() => dismissAlert(id), duration)
    }
    return id
  }, [dismissAlert])

  const actions = useMemo<AlertActions>(
    () => ({ showAlert, dismissAlert, clearAlerts }),
    [showAlert, dismissAlert, clearAlerts]
  )

  return (
    <AlertActionsContext.Provider value={actions}>
      <AlertStateContext.Provider value={alerts}>
        {children}
      </AlertStateContext.Provider>
    </AlertActionsContext.Provider>
  )
}

/** Read the active alerts — use in AlertBanner only. */
export function useAlertState(): Alert[] {
  return useContext(AlertStateContext)
}

/** Stable alert actions — safe to use anywhere without causing re-renders. */
export function useAlert(): AlertActions {
  return useContext(AlertActionsContext)
}
