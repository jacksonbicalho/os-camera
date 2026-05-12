import { useEffect, type RefObject } from 'react'

export function useScrollToPlayer(ref: RefObject<HTMLElement | null>, activeKey: string | null, disabled = false) {
  useEffect(() => {
    if (activeKey !== null && !disabled) {
      ref.current?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }
  }, [activeKey, ref, disabled])
}
