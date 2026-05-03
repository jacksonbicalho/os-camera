import { useEffect, type RefObject } from 'react'

export function useScrollToPlayer(ref: RefObject<HTMLElement | null>, activeKey: string | null) {
  useEffect(() => {
    if (activeKey !== null) {
      ref.current?.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }
  }, [activeKey, ref])
}
