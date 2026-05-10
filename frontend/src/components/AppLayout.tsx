import type { ReactNode } from 'react'
import { useEffect, useState } from 'react'
import { format, parseISO } from 'date-fns'
import { ptBR } from 'date-fns/locale'
import { getUsername, authHeaders } from '../auth'
import Header from './Header'

interface AppLayoutProps {
  children: ReactNode
  mainClassName?: string
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

export default function AppLayout({ children, mainClassName = '' }: AppLayoutProps) {
  const [about, setAbout] = useState<AboutInfo | null>(null)

  useEffect(() => {
    fetch('/api/about', { headers: authHeaders() })
      .then(r => r.ok ? r.json() : null)
      .then(d => { if (d) setAbout(d) })
      .catch(() => {})
  }, [])

  return (
    <div className="min-h-screen flex flex-col bg-gray-950">
      <Header username={getUsername() ?? undefined} />
      <main className={`flex-1 p-6 ${mainClassName}`.trim()}>{children}</main>
      {about && (
        <footer className="py-3 text-center text-xs text-gray-500 border-t border-gray-800/50">
          {about.version} · build: {formatBuiltAt(about.built_at)} · online {formatUptime(about.uptime_seconds)} · {about.go_version}
        </footer>
      )}
    </div>
  )
}
