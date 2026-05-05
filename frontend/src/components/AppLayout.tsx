import type { ReactNode } from 'react'
import { getUsername } from '../auth'
import Header from './Header'

interface AppLayoutProps {
  children: ReactNode
  mainClassName?: string
}

export default function AppLayout({ children, mainClassName = '' }: AppLayoutProps) {
   return (
    <div className="min-h-screen flex flex-col bg-gray-950">
      <Header username={getUsername() ?? undefined} />
      <main className={`flex-1 p-6 ${mainClassName}`.trim()}>{children}</main>
    </div>
  )
}
