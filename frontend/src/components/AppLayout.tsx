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
        <svg className="w-6 h-6 shrink-0" viewBox="0 0 512 512" xmlns="http://www.w3.org/2000/svg">
          <rect width="512" height="512" rx="112" fill="#18181b"/>
          <rect x="68" y="172" width="376" height="252" rx="36" fill="#09090b" stroke="#f59e0b" strokeWidth="20"/>
          <path d="M188 172 L188 132 Q188 112 208 112 L304 112 Q324 112 324 132 L324 172" fill="#09090b" stroke="#f59e0b" strokeWidth="20" strokeLinejoin="round"/>
          <circle cx="256" cy="298" r="90" fill="#09090b" stroke="#f59e0b" strokeWidth="20"/>
          <circle cx="256" cy="298" r="56" fill="#09090b" stroke="#f59e0b" strokeWidth="12"/>
          <circle cx="256" cy="298" r="26" fill="#f59e0b"/>
          <circle cx="394" cy="216" r="18" fill="#ef4444"/>
        </svg>
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
              <svg className="w-3 h-3" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 0C5.374 0 0 5.373 0 12c0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23A11.509 11.509 0 0112 5.803c.996.005 2.002.138 2.998.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576C20.566 21.797 24 17.3 24 12c0-6.627-5.373-12-12-12z"/>
              </svg>
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
