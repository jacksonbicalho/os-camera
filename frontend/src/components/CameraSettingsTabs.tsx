import { Link } from 'react-router-dom'

type Tab = 'detail' | 'motion' | 'zones'

interface Props {
  id: string
  active: Tab
  camName?: string
}

const TABS: { key: Tab; label: string; path: (id: string) => string }[] = [
  { key: 'detail', label: 'Câmera', path: id => `/settings/cameras/${id}` },
  { key: 'motion', label: 'Detecção de movimento', path: id => `/settings/cameras/${id}/motion` },
  { key: 'zones', label: 'Zonas', path: id => `/settings/cameras/${id}/motion/zones` },
]

export default function CameraSettingsTabs({ id, active, camName }: Props) {
  return (
    <div className="mb-6">
      <nav className="flex items-center gap-1.5 text-xs text-gray-500 mb-4">
        <Link to="/settings/cameras" className="hover:text-gray-300 transition-colors">Câmeras</Link>
        <span>/</span>
        <span className="text-gray-300">{camName || id}</span>
      </nav>
      <div className="flex gap-1 border-b border-gray-800">
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
    </div>
  )
}
