import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { authHeaders, clearToken } from '../auth'
import AppLayout from '../components/AppLayout'
import HLSPlayer from '../components/HLSPlayer'

interface Camera {
  id: string
}

export default function DashboardPage() {
  const [cameras, setCameras] = useState<Camera[]>([])
  const navigate = useNavigate()

  useEffect(() => {
    fetch('/api/cameras', { headers: authHeaders() })
      .then(res => {
        if (res.status === 401) { clearToken(); navigate('/login'); return [] }
        return res.json()
      })
      .then(data => Array.isArray(data) && setCameras(data))
  }, [navigate])

  return (
    <AppLayout>
        <h2 className="text-lg font-semibold text-gray-200 mb-4">Câmeras ao vivo</h2>
        {cameras.length === 0 ? (
          <p className="text-gray-500 text-sm">Nenhuma câmera configurada.</p>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {cameras.map(cam => (
              <button
                key={cam.id}
                onClick={() => navigate(`/cameras/${cam.id}`)}
                className="bg-gray-900 border border-gray-800 rounded-lg overflow-hidden hover:border-blue-600 transition-colors text-left group"
              >
                <div className="relative">
                  <HLSPlayer
                    src={`/stream/${cam.id}/index.m3u8`}
                    className="w-full aspect-video object-cover bg-black pointer-events-none"
                  />
                  <span className="absolute top-2 left-2 bg-red-600 text-white text-xs px-2 py-0.5 rounded font-medium">
                    AO VIVO
                  </span>
                </div>
                <div className="px-3 py-2">
                  <p className="text-sm font-medium text-gray-200 group-hover:text-white">{cam.id}</p>
                </div>
              </button>
            ))}
          </div>
        )}
    </AppLayout>
  )
}
