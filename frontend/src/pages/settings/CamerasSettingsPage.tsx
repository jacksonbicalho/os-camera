import { Link } from 'react-router-dom'
import SettingsLayout from '../../components/SettingsLayout'
import { useSettings } from '../../hooks/useSettings'

export default function CamerasSettingsPage() {
  const settings = useSettings('/settings/cameras')

  return (
    <SettingsLayout>
      <h2 className="text-lg font-semibold text-gray-200 mb-6">Câmeras</h2>
      {!settings ? (
        <p className="text-gray-500 text-sm">Carregando...</p>
      ) : settings.cameras.length === 0 ? (
        <p className="text-gray-500 text-sm">Nenhuma câmera configurada.</p>
      ) : (
        <div className="flex flex-col gap-2">
          {settings.cameras.map(cam => (
            <Link
              key={cam.id}
              to={`/settings/cameras/${cam.id}`}
              className="flex items-center justify-between bg-gray-900 border border-gray-800 rounded-lg px-5 py-4 hover:border-blue-600 transition-colors"
            >
              <span className="text-sm font-mono text-gray-200">{cam.id}</span>
              <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
              </svg>
            </Link>
          ))}
        </div>
      )}
    </SettingsLayout>
  )
}
