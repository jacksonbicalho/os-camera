/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useState } from 'react'

export type SidebarItem =
  | { type: 'button'; id: string; icon: React.ReactNode; title: string; onClick: () => void; active?: boolean; badge?: string | number; disabled?: boolean }
  | { type: 'link';   id: string; icon: React.ReactNode; title: string; to: string; state?: object }
  | { type: 'separator'; id: string }

interface SidebarContextValue {
  items: SidebarItem[]
  setItems: (items: SidebarItem[]) => void
}

const SidebarContext = createContext<SidebarContextValue | null>(null)

export function SidebarItemsProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = useState<SidebarItem[]>([])
  return (
    <SidebarContext.Provider value={{ items, setItems }}>
      {children}
    </SidebarContext.Provider>
  )
}

export function useSidebarItems(): SidebarContextValue {
  const ctx = useContext(SidebarContext)
  if (!ctx) throw new Error('useSidebarItems must be used inside SidebarItemsProvider')
  return ctx
}
