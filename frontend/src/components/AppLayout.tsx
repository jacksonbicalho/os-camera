import type { ReactNode } from 'react'
import { useEffect, useState } from 'react'
import { format, parseISO } from 'date-fns'
import { ptBR } from 'date-fns/locale'
import { getUsername, authHeaders } from '../auth'
import Sidebar from './Sidebar'

interface AppLayoutProps {
  children: ReactNode
  mainClassName?: string
  fill?: boolean
}

interface AboutInfo {
  version: string
  built_at: string
  uptime_seconds: number
  go_version: string
}

function formatUptime(seconds: number): string {
  const days = Math.floor(seconds / 86400)
  if (days >= 1) return `há ${days} dia${days !== 1 ? 's' : ''}`
  const hours = Math.floor(seconds / 3600)
  if (hours >= 1) return `há ${hours} hora${hours !== 1 ? 's' : ''}`
  const minutes = Math.floor(seconds / 60)
  if (minutes >= 1) return `há ${minutes} minuto${minutes !== 1 ? 's' : ''}`
  return 'agora mesmo'
}

function formatBuiltAt(builtAt: string): string {
  try {
    return format(parseISO(builtAt), "d MMM yyyy", { locale: ptBR })
  } catch {
    return builtAt
  }
}

export default function AppLayout({ children, mainClassName = '', fill = false }: AppLayoutProps) {
  const [about, setAbout] = useState<AboutInfo | null>(null)

  useEffect(() => {
    fetch('/api/about', { headers: authHeaders() })
      .then(r => r.ok ? r.json() : null)
      .then(d => { if (d) setAbout(d) })
      .catch(() => {})
  }, [])

  const footer = about ? (
    <footer className="py-2 px-4 text-xs text-gray-500 border-t border-gray-800/50">
      {about.version} · build: {formatBuiltAt(about.built_at)} · online {formatUptime(about.uptime_seconds)} · {about.go_version}
    </footer>
  ) : null

  // fill mode: CameraPage — tudo fica preso na viewport, sem scroll de página
  if (fill) {
    return (
      <div className="flex h-screen overflow-hidden bg-gray-950">
        <Sidebar username={getUsername() ?? undefined} />
        <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
          <main className="flex-1 min-h-0 overflow-hidden">{children}</main>
          {footer}
        </div>
      </div>
    )
  }

  // modo padrão: sidebar sticky, página rola naturalmente pelo browser
  return (
    <div className="flex min-h-screen bg-gray-950">
      <div className="sticky top-0 h-screen shrink-0 flex">
        <Sidebar username={getUsername() ?? undefined} />
      </div>
      <div className="flex-1 flex flex-col min-w-0">
        <main className={`flex-1 p-6 ${mainClassName}`.trim()}>{children}</main>
        {footer}
      </div>
    </div>
  )
}
