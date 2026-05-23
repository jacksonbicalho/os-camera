/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useState } from 'react'

export type SidebarDropdownOption = {
  label: string
  active?: boolean
  disabled?: boolean
  onClick: () => void
}

export type SidebarItem =
  | { type: 'button'; id: string; icon: React.ReactNode; title: string; onClick: () => void; active?: boolean; badge?: string | number; disabled?: boolean }
  | { type: 'link';   id: string; icon: React.ReactNode; title: string; to: string; state?: object }
  | { type: 'separator'; id: string }
  | { type: 'dropdown'; id: string; icon: React.ReactNode; title: string; options: SidebarDropdownOption[]; disabled?: boolean; active?: boolean }

const SidebarItemsContext = createContext<SidebarItem[]>([])
const SetSidebarItemsContext = createContext<(items: SidebarItem[]) => void>(() => {})

export function SidebarItemsProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = useState<SidebarItem[]>([])
  return (
    <SetSidebarItemsContext.Provider value={setItems}>
      <SidebarItemsContext.Provider value={items}>
        {children}
      </SidebarItemsContext.Provider>
    </SetSidebarItemsContext.Provider>
  )
}

/** Read sidebar items — use in Sidebar only (re-renders on every items change). */
export function useSidebarItems(): SidebarItem[] {
  return useContext(SidebarItemsContext)
}

/** Set sidebar items — stable reference, never causes re-renders in the caller. */
export function useSetSidebarItems(): (items: SidebarItem[]) => void {
  return useContext(SetSidebarItemsContext)
}
