import type { ReactNode } from 'react'
import { useEffect, useState } from 'react'
import { getUsername, authHeaders } from '../auth'
import Sidebar from './Sidebar'
import AlertBanner from './AlertBanner'
import StatusBar from './StatusBar'

interface AppLayoutProps {
  children: ReactNode
  mainClassName?: string
  fill?: boolean
}

interface AboutInfo {
  version: string
  built_at: string
  go_version: string
}

export default function AppLayout({ children, mainClassName = '', fill = false }: AppLayoutProps) {
  const [about, setAbout] = useState<AboutInfo | null>(null)

  useEffect(() => {
    fetch('/api/about', { headers: authHeaders() })
      .then(r => r.ok ? r.json() : null)
      .then(d => { if (d) setAbout(d) })
      .catch(() => {})
  }, [])

  const footer = <StatusBar version={about?.version} />

  // fill mode: CameraPage — tudo fica preso na viewport, sem scroll de página
  if (fill) {
    return (
      <div className="flex h-screen overflow-hidden bg-gray-950">
        <Sidebar username={getUsername() ?? undefined} />
        <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
          <AlertBanner />
          <main className="flex-1 min-h-0 overflow-hidden">{children}</main>
          {footer}
        </div>
      </div>
    )
  }

  // modo padrão: sidebar sticky, página rola naturalmente pelo browser
  return (
    <div className="flex min-h-screen bg-gray-950">
      <div className="sticky top-0 h-screen shrink-0 flex z-10">
        <Sidebar username={getUsername() ?? undefined} />
      </div>
      <div className="flex-1 flex flex-col min-w-0">
        <AlertBanner />
        <main className={`flex-1 p-6 ${mainClassName}`.trim()}>{children}</main>
        {footer}
      </div>
    </div>
  )
}
