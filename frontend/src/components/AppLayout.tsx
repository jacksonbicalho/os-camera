import type { ReactNode } from 'react'
import { useEffect, useState } from 'react'
import { format, parseISO } from 'date-fns'
import { ptBR } from 'date-fns/locale'
import { getUsername, authHeaders } from '../auth'
import Sidebar from './Sidebar'
import { Github, CameraLogo } from './Icons'

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


function formatBuiltAt(builtAt: string): string {
  try {
    return format(parseISO(builtAt), "d MMM yyyy", { locale: ptBR })
  } catch {
    return builtAt
  }
}

function builtAtYear(builtAt: string): number {
  try {
    const y = parseISO(builtAt).getFullYear()
    return Number.isNaN(y) ? new Date().getFullYear() : y
  } catch {
    return new Date().getFullYear()
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
    <footer className="shrink-0 border-t border-gray-800/50 px-6 py-3 flex items-center gap-8">
      {/* Esquerda: marca centralizada verticalmente no rodapé */}
      <div className="flex items-center gap-2.5 shrink-0">
        <CameraLogo className="w-6 h-6 shrink-0" />
        <span className="text-sm font-semibold text-gray-300">os-camera</span>
      </div>

      {/* Direita: info empilhada */}
      <div className="flex-1 flex flex-col gap-1.5">
        <div className="flex justify-end text-[11px] text-gray-500">
          <span><span className="text-gray-300 font-mono">{about.version}</span> · build {formatBuiltAt(about.built_at)}</span>
        </div>
        <div className="grid grid-cols-[1fr_auto_1fr] items-center border-t border-gray-800/50 pt-1.5 text-[10px] text-gray-500">
          <span/>
          <span className="text-gray-600">copyright © {builtAtYear(about.built_at)}</span>
          <div className="flex items-center gap-1.5 justify-end">
            <span>{about.go_version}</span>
            <span className="text-gray-700">·</span>
            <span>Desenvolvido por Jackson Bicalho</span>
            <a
              href="https://github.com/jacksonbicalho/camera"
              target="_blank"
              rel="noopener noreferrer"
              className="hover:text-gray-400 transition-colors"
              title="GitHub"
            >
              <Github className="w-3 h-3" />
            </a>
          </div>
        </div>
      </div>
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
      <div className="sticky top-0 h-screen shrink-0 flex z-10">
        <Sidebar username={getUsername() ?? undefined} />
      </div>
      <div className="flex-1 flex flex-col min-w-0">
        <main className={`flex-1 p-6 ${mainClassName}`.trim()}>{children}</main>
        {footer}
      </div>
    </div>
  )
}
