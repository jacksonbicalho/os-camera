import type { ReactNode } from 'react'
import { useEffect, useState } from 'react'
import { getUsername } from '../auth'
import Header from './Header'

interface AppLayoutProps {
  children: ReactNode
  mainClassName?: string
}

export default function AppLayout({ children, mainClassName = '' }: AppLayoutProps) {
  const [version, setVersion] = useState('')

  useEffect(() => {
    fetch('/api/config')
      .then(r => r.json())
      .then(d => { if (d.version) setVersion(d.version) })
      .catch(() => {})
  }, [])

  return (
    <div className="min-h-screen flex flex-col bg-gray-950">
      <Header username={getUsername() ?? undefined} />
      <main className={`flex-1 p-6 ${mainClassName}`.trim()}>{children}</main>
      {version && (
        <footer className="py-3 text-center text-xs text-gray-600">
          {version}
        </footer>
      )}
    </div>
  )
}
