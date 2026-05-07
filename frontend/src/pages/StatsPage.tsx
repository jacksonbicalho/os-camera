import { useEffect, useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { authHeaders, clearToken } from '../auth'
import AppLayout from '../components/AppLayout'
import MotionScoreChart from '../components/MotionScoreChart'
import StatCard from '../components/StatCard'

interface CameraInfo {
  id: string
  motion_threshold: number
}

interface Stats {
  recordings_bytes: number
  recordings_count: number
  recordings_duration_seconds: number
  forecast_seconds: number
  disk_total_bytes: number
  disk_free_bytes: number
  camera_count: number
  connected_clients: number
  max_size_bytes: number
  warn_percent: number
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
}

function formatDuration(seconds: number): string {
  if (seconds <= 0) return '—'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (h === 0) return `${m}m`
  if (m === 0) return `${h}h`
  return `${h}h ${m}m`
}

export default function StatsPage() {
  const navigate = useNavigate()
  const [stats, setStats] = useState<Stats | null>(null)
  const [cameras, setCameras] = useState<CameraInfo[]>([])
  const [monitorOpen, setMonitorOpen] = useState(false)
  const [monitorCam, setMonitorCam] = useState<string | null>(null)

  useEffect(() => {
    fetch('/api/cameras', { headers: authHeaders() })
      .then(res => {
        if (res.status === 401) { clearToken(); navigate('/login', { state: { from: '/stats' }, replace: true }); return null }
        return res.json()
      })
      .then(data => { if (Array.isArray(data)) setCameras(data) })
      .catch(() => {})
  }, [navigate])

  useEffect(() => {
    const cancelled = { value: false }
    function fetchStats() {
      fetch('/api/stats', { headers: authHeaders() })
        .then(res => {
          if (res.status === 401) { clearToken(); navigate('/login', { state: { from: '/stats' }, replace: true }); return null }
          return res.json()
        })
        .then(data => { if (!cancelled.value && data) setStats(data) })
        .catch(() => {})
    }
    fetchStats()
    const interval = setInterval(fetchStats, 30_000)
    return () => { cancelled.value = true; clearInterval(interval) }
  }, [navigate])

  const hasLimit = (stats?.max_size_bytes ?? 0) > 0
  const limitRef = hasLimit ? stats!.max_size_bytes : (stats?.disk_total_bytes ?? 0)
  const usedPercent = limitRef > 0
    ? Math.min(100, Math.round((stats!.recordings_bytes / limitRef) * 100))
    : 0
  const warnThreshold = hasLimit && stats ? stats.warn_percent : 0
  const isWarning = warnThreshold > 0 && usedPercent >= warnThreshold
  const isOver = hasLimit && stats ? stats.recordings_bytes >= stats.max_size_bytes : false

  const barColor = isOver ? 'bg-red-600' : isWarning ? 'bg-yellow-500' : 'bg-blue-600'

  const activeCamId = monitorCam ?? (cameras.length > 0 ? cameras[0].id : null)
  const selectedCam = cameras.find(c => c.id === activeCamId)

  return (
    <AppLayout mainClassName="max-w-4xl mx-auto w-full">
        <div className="mb-6">
          <Link to="/" className="text-sm text-blue-400 hover:text-blue-300">← Câmeras</Link>
          <h2 className="text-lg font-semibold text-gray-200 mt-1">Estatísticas</h2>
        </div>

        {!stats ? (
          <p className="text-gray-500 text-sm">Carregando...</p>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            {/* Disco */}
            <div className="bg-gray-900 border border-gray-800 rounded-lg p-5 sm:col-span-2">
              <p className="text-xs text-gray-400 uppercase tracking-wider font-medium mb-4">Disco</p>
              <div className="grid grid-cols-3 gap-4 mb-4">
                <div>
                  <p className="text-xs text-gray-500 mb-1">{hasLimit ? 'Limite' : 'Total'}</p>
                  <p className="text-lg font-semibold text-gray-200">
                    {formatBytes(hasLimit ? stats.max_size_bytes : stats.disk_total_bytes)}
                  </p>
                </div>
                <div>
                  <p className="text-xs text-gray-500 mb-1">Gravações</p>
                  <p className={`text-lg font-semibold ${isOver ? 'text-red-400' : isWarning ? 'text-yellow-400' : 'text-blue-400'}`}>
                    {formatBytes(stats.recordings_bytes)}
                  </p>
                </div>
                <div>
                  <p className="text-xs text-gray-500 mb-1">Disponível</p>
                  <p className="text-lg font-semibold text-green-400">
                    {formatBytes(hasLimit
                      ? Math.max(0, stats.max_size_bytes - stats.recordings_bytes)
                      : stats.disk_free_bytes)}
                  </p>
                </div>
              </div>
              <div className="h-2 bg-gray-800 rounded-full overflow-hidden">
                <div className={`h-full rounded-full transition-all ${barColor}`} style={{ width: `${usedPercent}%` }} />
              </div>
              <p className="text-xs text-gray-500 mt-1">
                {usedPercent}% {hasLimit ? `do limite de ${formatBytes(stats.max_size_bytes)}` : 'do disco'}
                {isWarning && !isOver && <span className="text-yellow-500 ml-2">⚠ próximo do limite</span>}
                {isOver && <span className="text-red-500 ml-2">⚠ limite atingido</span>}
              </p>
            </div>

            <StatCard
              label="Câmeras"
              value={stats.camera_count}
              subtext="configuradas"
            />

            <StatCard
              label="Clientes conectados"
              value={stats.connected_clients}
              subtext="ativos no stream (30s)"
            />

            <StatCard
              label="Gravações"
              value={stats.recordings_count.toLocaleString()}
              subtext={`arquivos MP4 · ${formatBytes(stats.recordings_bytes)}`}
            />

            <StatCard
              label="Horas gravadas"
              value={formatDuration(stats.recordings_duration_seconds)}
              subtext="de vídeo em disco"
            />

            <StatCard
              label="Previsão de capacidade"
              value={formatDuration(stats.forecast_seconds)}
              subtext={stats.forecast_seconds > 0
                ? 'restantes com o espaço disponível'
                : 'dados insuficientes para estimar'}
            />

            {/* Monitor de movimento em tempo real */}
            {cameras.length > 0 && (
              <div className="bg-gray-900 border border-gray-800 rounded-lg sm:col-span-2 overflow-hidden">
                <button
                  onClick={() => setMonitorOpen(o => !o)}
                  className="w-full flex items-center justify-between p-5 text-left hover:bg-gray-800 transition-colors"
                >
                  <div>
                    <p className="text-xs text-gray-400 uppercase tracking-wider font-medium">Monitor de movimento</p>
                    <p className="text-xs text-gray-600 mt-0.5">score bruto por frame · limiar configurado</p>
                  </div>
                  <svg
                    className={`w-4 h-4 text-gray-500 transition-transform ${monitorOpen ? 'rotate-180' : ''}`}
                    fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}
                  >
                    <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
                  </svg>
                </button>

                {monitorOpen && (
                  <div className="px-5 pb-5">
                    {/* Camera selector */}
                    {cameras.length > 1 && (
                      <div className="flex gap-2 mb-4 flex-wrap">
                        {cameras.map(cam => (
                          <button
                            key={cam.id}
                            onClick={() => setMonitorCam(cam.id)}
                            className={`px-3 py-1 rounded text-xs font-mono transition-colors ${
                              activeCamId === cam.id
                                ? 'bg-blue-600 text-white'
                                : 'bg-gray-800 text-gray-400 hover:bg-gray-700'
                            }`}
                          >
                            {cam.id}
                          </button>
                        ))}
                      </div>
                    )}

                    {selectedCam ? (
                      <MotionScoreChart
                        key={selectedCam.id}
                        cameraId={selectedCam.id}
                        threshold={selectedCam.motion_threshold}
                      />
                    ) : (
                      <p className="text-xs text-gray-600 text-center py-4">Selecione uma câmera</p>
                    )}
                  </div>
                )}
              </div>
            )}
          </div>
        )}
    </AppLayout>
  )
}
