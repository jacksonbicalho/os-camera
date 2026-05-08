import { NavLink } from 'react-router-dom'
import AppLayout from './AppLayout'

const NAV_LINKS = [
  { to: '/settings/stats', label: 'Estatísticas' },
  { to: '/settings/cameras', label: 'Câmeras' },
  { to: '/settings/server', label: 'Servidor' },
  { to: '/settings/storage', label: 'Armazenamento' },
  { to: '/settings/system', label: 'Sistema' },
  { to: '/settings/about', label: 'Sobre' },
]

interface SettingsLayoutProps {
  children: React.ReactNode
}

export default function SettingsLayout({ children }: SettingsLayoutProps) {
  return (
    <AppLayout mainClassName="max-w-5xl mx-auto w-full">
      <div className="flex gap-8">
        <nav className="w-44 shrink-0">
          <p className="text-xs text-gray-500 uppercase tracking-wider font-medium mb-3">Configurações</p>
          <ul className="flex flex-col gap-0.5">
            {NAV_LINKS.map(({ to, label }) => (
              <li key={to}>
                <NavLink
                  to={to}
                  className={({ isActive }) =>
                    `block px-3 py-1.5 rounded text-sm transition-colors ${
                      isActive
                        ? 'bg-gray-800 text-white font-medium'
                        : 'text-gray-400 hover:text-white hover:bg-gray-800'
                    }`
                  }
                >
                  {label}
                </NavLink>
              </li>
            ))}
          </ul>
        </nav>
        <div className="flex-1 min-w-0">{children}</div>
      </div>
    </AppLayout>
  )
}
