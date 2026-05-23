import { useEffect, useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { authHeaders, clearToken } from '../auth'
import AppLayout from '../components/AppLayout'
import MotionScoreChart from '../components/MotionScoreChart'
import { useStats } from '../hooks/useStats'
import { formatBytes, formatDuration } from './statsUtils'
import { formatDistanceToNow } from 'date-fns'
import { ptBR } from 'date-fns/locale'

interface CameraInfo {
  id: string
  name: string
  motion_threshold: number
}

function StatusDot({ online }: { online: boolean }) {
  return (
    <span className={`inline-block w-2 h-2 rounded-full flex-shrink-0 ${online ? 'bg-green-500' : 'bg-gray-600'}`} />
  )
}

function ChevronIcon({ open }: { open: boolean }) {
  return (
    <svg
      className={`w-4 h-4 text-gray-500 transition-transform flex-shrink-0 ${open ? 'rotate-180' : ''}`}
      fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}
    >
      <path strokeLinecap="round" strokeLinejoin="round" d="M19 9l-7 7-7-7" />
    </svg>
  )
}

export default function StatsPage() {
  const navigate = useNavigate()
  const stats = useStats('/stats')
  const [cameras, setCameras] = useState<CameraInfo[]>([])
  const [expandedCams, setExpandedCams] = useState<Set<string>>(new Set())

  useEffect(() => {
    fetch('/api/cameras', { headers: authHeaders() })
      .then(res => {
        if (res.status === 401) { clearToken(); navigate('/login', { state: { from: '/stats' }, replace: true }); return null }
        return res.json()
      })
      .then(data => { if (Array.isArray(data)) setCameras(data) })
      .catch(() => {})
  }, [navigate])

  function toggleCam(id: string) {
    setExpandedCams(prev => {
      const next = new Set(prev)
      if (next.has(id)) { next.delete(id) } else { next.add(id) }
      return next
    })
  }

  const hasLimit = (stats?.max_size_bytes ?? 0) > 0
  const limitRef = hasLimit ? stats!.max_size_bytes : (stats?.disk_total_bytes ?? 0)
  const usedPercent = limitRef > 0
    ? Math.min(100, Math.round((stats!.recordings_bytes / limitRef) * 100))
    : 0
  const warnThreshold = hasLimit && stats ? stats.warn_percent : 0
  const isWarning = warnThreshold > 0 && usedPercent >= warnThreshold
  const isOver = hasLimit && stats ? stats.recordings_bytes >= stats.max_size_bytes : false
  const barColor = isOver
    ? 'bg-gradient-to-r from-red-700 to-red-500'
    : isWarning
    ? 'bg-gradient-to-r from-yellow-600 to-yellow-400'
    : 'bg-gradient-to-r from-blue-700 to-blue-400'

  const cameraHealthMap = Object.fromEntries((stats?.cameras ?? []).map(c => [c.id, c]))

  const cpuPct = stats?.cpu_percent ?? -1
  const sysMemUsed = (stats?.sys_mem_total_bytes ?? 0) - (stats?.sys_mem_free_bytes ?? 0)

  return (
    <AppLayout mainClassName="max-w-4xl mx-auto w-full">
      <div className="mb-8">
        <Link to="/" className="text-sm text-blue-400 hover:text-blue-300">← Câmeras</Link>
        <h1 className="text-xl font-semibold text-gray-100 mt-2">Estatísticas</h1>
      </div>

      {!stats ? (
        <p className="text-gray-500 text-sm">Carregando...</p>
      ) : (
        <div className="space-y-4">

          {/* KPIs */}
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
              <p className="text-xs text-gray-500 uppercase tracking-wider mb-3">Gravações</p>
              <p className="text-3xl font-bold text-gray-100">{stats.recordings_count.toLocaleString()}</p>
              <p className="text-sm text-gray-400 mt-1">{formatBytes(stats.recordings_bytes)}</p>
            </div>
            <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
              <p className="text-xs text-gray-500 uppercase tracking-wider mb-3">Horas gravadas</p>
              <p className="text-3xl font-bold text-gray-100">{formatDuration(stats.recordings_duration_seconds)}</p>
              <p className="text-sm text-gray-400 mt-1">de vídeo em disco</p>
            </div>
            <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
              <p className="text-xs text-gray-500 uppercase tracking-wider mb-3">Câmeras</p>
              <p className="text-3xl font-bold text-gray-100">{stats.camera_count}</p>
              <p className="text-sm text-gray-400 mt-1">
                {stats.connected_clients} cliente{stats.connected_clients !== 1 ? 's' : ''} conectado{stats.connected_clients !== 1 ? 's' : ''}
              </p>
            </div>
          </div>

          {/* Disco */}
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-6">
            <p className="text-xs text-gray-500 uppercase tracking-wider mb-5">Armazenamento</p>
            <div className="grid grid-cols-3 gap-6 mb-5">
              <div>
                <p className="text-xs text-gray-500 mb-1">{hasLimit ? 'Limite' : 'Total'}</p>
                <p className="text-2xl font-bold text-gray-200">
                  {formatBytes(hasLimit ? stats.max_size_bytes : stats.disk_total_bytes)}
                </p>
              </div>
              <div>
                <p className="text-xs text-gray-500 mb-1">Gravações</p>
                <p className={`text-2xl font-bold ${isOver ? 'text-red-400' : isWarning ? 'text-yellow-400' : 'text-blue-400'}`}>
                  {formatBytes(stats.recordings_bytes)}
                </p>
              </div>
              <div>
                <p className="text-xs text-gray-500 mb-1">Disponível</p>
                <p className="text-2xl font-bold text-green-400">
                  {formatBytes(hasLimit
                    ? Math.max(0, stats.max_size_bytes - stats.recordings_bytes)
                    : stats.disk_free_bytes)}
                </p>
              </div>
            </div>
            <div className="h-3 bg-gray-800 rounded-full overflow-hidden">
              <div
                className={`h-full rounded-full transition-all duration-500 ${barColor}`}
                style={{ width: `${usedPercent}%` }}
              />
            </div>
            <div className="flex items-center justify-between mt-2">
              <p className="text-xs text-gray-500">
                {usedPercent}% {hasLimit ? `do limite de ${formatBytes(stats.max_size_bytes)}` : 'do disco'}
              </p>
              {isWarning && !isOver && <p className="text-xs text-yellow-500">⚠ próximo do limite</p>}
              {isOver && <p className="text-xs text-red-500">⚠ limite atingido</p>}
            </div>
            {stats.forecast_seconds > 0 && (
              <div className="mt-4 pt-4 border-t border-gray-800">
                <p className="text-xs text-gray-500">
                  Previsão de capacidade:{' '}
                  <span className="text-gray-300 font-medium">{formatDuration(stats.forecast_seconds)} restantes</span>
                </p>
              </div>
            )}
          </div>

          {/* Sistema */}
          <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
            <p className="text-xs text-gray-500 uppercase tracking-wider mb-4">Sistema</p>
            <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-x-6 gap-y-4">
              <div>
                <p className="text-xs text-gray-600 mb-1">OS</p>
                <p className="text-sm font-medium text-gray-300 truncate">{stats.os || '—'}</p>
              </div>
              <div>
                <p className="text-xs text-gray-600 mb-1">PID</p>
                <p className="text-sm font-mono text-gray-300">{stats.pid}</p>
              </div>
              <div>
                <p className="text-xs text-gray-600 mb-1">CPU</p>
                <p className="text-sm font-mono text-gray-300">
                  {cpuPct < 0 ? '—' : `${cpuPct.toFixed(1)}%`}
                </p>
                {cpuPct >= 0 && <p className="text-xs text-gray-700">amostra 30 s</p>}
              </div>
              <div>
                <p className="text-xs text-gray-600 mb-1">Mem. processo</p>
                <p className="text-sm font-mono text-gray-300">
                  {stats.mem_rss_bytes > 0 ? formatBytes(stats.mem_rss_bytes) : '—'}
                </p>
              </div>
              {stats.sys_mem_total_bytes > 0 && (
                <div>
                  <p className="text-xs text-gray-600 mb-1">RAM host</p>
                  <p className="text-sm font-mono text-gray-300">
                    {formatBytes(sysMemUsed)} / {formatBytes(stats.sys_mem_total_bytes)}
                  </p>
                  <p className="text-xs text-gray-700">livre: {formatBytes(stats.sys_mem_free_bytes)}</p>
                </div>
              )}
              <div>
                <p className="text-xs text-gray-600 mb-1">Goroutines</p>
                <p className="text-sm font-mono text-gray-300">{stats.goroutines}</p>
              </div>
            </div>
          </div>

          {/* Câmeras */}
          {cameras.length > 0 && (
            <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
              <div className="px-5 py-4 border-b border-gray-800">
                <p className="text-xs text-gray-500 uppercase tracking-wider font-medium">Câmeras</p>
              </div>
              <div className="divide-y divide-gray-800">
                {cameras.map(cam => {
                  const health = cameraHealthMap[cam.id]
                  const lastRec = health?.last_recording_at ? new Date(health.last_recording_at) : null
                  const isOpen = expandedCams.has(cam.id)
                  const hasMotion = health?.motion_enabled ?? false

                  return (
                    <div key={cam.id}>
                      <button
                        onClick={() => toggleCam(cam.id)}
                        className="w-full flex items-center gap-3 px-5 py-4 text-left hover:bg-gray-800/50 transition-colors"
                      >
                        <StatusDot online={health?.online ?? false} />
                        <div className="flex-1 min-w-0">
                          <p className="text-sm font-medium text-gray-200 truncate">{cam.name || cam.id}</p>
                          <p className="text-xs text-gray-600 font-mono">{cam.id}</p>
                        </div>
                        <div className="text-right flex-shrink-0 mr-2">
                          {lastRec ? (
                            <p className="text-xs text-gray-400">
                              {formatDistanceToNow(lastRec, { addSuffix: true, locale: ptBR })}
                            </p>
                          ) : (
                            <p className="text-xs text-gray-600">sem gravações</p>
                          )}
                          {hasMotion && (
                            <p className="text-xs text-blue-500">detecção ativa</p>
                          )}
                        </div>
                        <ChevronIcon open={isOpen} />
                      </button>

                      {isOpen && (
                        <div className="px-5 pb-5">
                          {hasMotion ? (
                            <MotionScoreChart
                              key={cam.id}
                              cameraId={cam.id}
                              threshold={cam.motion_threshold}
                            />
                          ) : (
                            <p className="text-xs text-gray-600 py-3">
                              Detecção de movimento desativada para esta câmera.
                            </p>
                          )}
                        </div>
                      )}
                    </div>
                  )
                })}
              </div>
            </div>
          )}

        </div>
      )}
    </AppLayout>
  )
}
