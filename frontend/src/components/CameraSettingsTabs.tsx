import { Link } from 'react-router-dom'
import { getRole } from '../auth'
import { Button } from '@/components/ui/button'
import { Plus } from './Icons'

type Tab = 'detail' | 'motion' | 'zones' | 'analysis'

interface Props {
  id: string
  active: Tab
  camName?: string
}

const TABS: { key: Tab; label: string; path: (id: string) => string }[] = [
  { key: 'detail', label: 'Câmera', path: id => `/settings/cameras/${id}` },
  { key: 'motion', label: 'Detecção de movimento', path: id => `/settings/cameras/motion/${id}` },
  { key: 'zones', label: 'Zonas', path: id => `/settings/cameras/zones/${id}` },
  { key: 'analysis', label: 'Análise', path: id => `/settings/cameras/analysis/${id}` },
]

export default function CameraSettingsTabs({ id, active, camName }: Props) {
  const isAdmin = getRole() === 'admin'
  return (
    <div className="mb-6">
      <nav className="flex items-center gap-1.5 text-xs text-gray-500 mb-4">
        <Link to="/settings/cameras" className="hover:text-gray-300 transition-colors">Câmeras</Link>
        <span>/</span>
        <span className="text-gray-300">{camName || id}</span>
      </nav>
      <div className="flex items-center justify-between border-b border-gray-800">
        <div className="flex gap-1">
          {TABS.map(tab => (
            <Link
              key={tab.key}
              to={tab.path(id)}
              className={`px-4 py-2 text-sm font-medium transition-colors border-b-2 -mb-px ${
                active === tab.key
                  ? 'border-blue-500 text-blue-400'
                  : 'border-transparent text-gray-400 hover:text-gray-200 hover:border-gray-600'
              }`}
            >
              {tab.label}
            </Link>
          ))}
        </div>
        {isAdmin && (
          <Button asChild className="mb-1">
            <Link to="/settings/cameras/new">
              <Plus className="w-3.5 h-3.5" /> Nova câmera
            </Link>
          </Button>
        )}
      </div>
    </div>
  )
}
