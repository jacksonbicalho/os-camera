import { useEffect, useState } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { authHeaders, clearToken } from '../auth'
import AppLayout from '../components/AppLayout'
import MotionScoreChart from '../components/MotionScoreChart'
import StatCard from '../components/StatCard'
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
  const barColor = isOver ? 'bg-red-600' : isWarning ? 'bg-yellow-500' : 'bg-blue-600'

  const cameraHealthMap = Object.fromEntries((stats?.cameras ?? []).map(c => [c.id, c]))

  const cpuPct = stats?.cpu_percent ?? -1
  const sysMemUsed = (stats?.sys_mem_total_bytes ?? 0) - (stats?.sys_mem_free_bytes ?? 0)

  return (
    <AppLayout mainClassName="max-w-4xl mx-auto w-full">
      <div className="mb-6">
        <Link to="/" className="text-sm text-blue-400 hover:text-blue-300">← Câmeras</Link>
        <h2 className="text-lg font-semibold text-gray-200 mt-1">Estatísticas</h2>
      </div>

      {!stats ? (
        <p className="text-gray-500 text-sm">Carregando...</p>
      ) : (
        <div className="space-y-4">

          {/* Sistema */}
          <div className="bg-gray-900 border border-gray-800 rounded-lg p-5">
            <p className="text-xs text-gray-400 uppercase tracking-wider font-medium mb-4">Sistema</p>
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-4">
              <div>
                <p className="text-xs text-gray-500 mb-1">Sistema operacional</p>
                <p className="text-sm font-medium text-gray-200">{stats.os || '—'}</p>
              </div>
              <div>
                <p className="text-xs text-gray-500 mb-1">PID</p>
                <p className="text-sm font-mono text-gray-200">{stats.pid}</p>
              </div>
              <div>
                <p className="text-xs text-gray-500 mb-1">Goroutines</p>
                <p className="text-sm font-mono text-gray-200">{stats.goroutines}</p>
              </div>
              <div>
                <p className="text-xs text-gray-500 mb-1">CPU (processo)</p>
                <p className="text-sm font-mono text-gray-200">
                  {cpuPct < 0 ? '—' : `${cpuPct.toFixed(1)}%`}
                </p>
                {cpuPct >= 0 && <p className="text-xs text-gray-600">amostra 30 s</p>}
              </div>
              <div>
                <p className="text-xs text-gray-500 mb-1">Memória (processo)</p>
                <p className="text-sm font-mono text-gray-200">
                  {stats.mem_rss_bytes > 0 ? formatBytes(stats.mem_rss_bytes) : '—'}
                </p>
              </div>
              {stats.sys_mem_total_bytes > 0 && (
                <div>
                  <p className="text-xs text-gray-500 mb-1">RAM do host</p>
                  <p className="text-sm font-mono text-gray-200">
                    {formatBytes(sysMemUsed)} / {formatBytes(stats.sys_mem_total_bytes)}
                  </p>
                  <p className="text-xs text-gray-600">livre: {formatBytes(stats.sys_mem_free_bytes)}</p>
                </div>
              )}
            </div>
          </div>

          {/* Disco */}
          <div className="bg-gray-900 border border-gray-800 rounded-lg p-5">
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

          {/* Cards de gravações */}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <StatCard label="Câmeras" value={stats.camera_count} subtext="configuradas" />
            <StatCard label="Clientes conectados" value={stats.connected_clients} subtext="ativos no stream (30s)" />
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
              colSpan2
            />
          </div>

          {/* Câmeras: saúde + monitor de movimento */}
          {cameras.length > 0 && (
            <div className="bg-gray-900 border border-gray-800 rounded-lg overflow-hidden">
              <div className="px-5 py-4 border-b border-gray-800">
                <p className="text-xs text-gray-400 uppercase tracking-wider font-medium">Câmeras</p>
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
                        className="w-full flex items-center gap-3 px-5 py-3 text-left hover:bg-gray-800 transition-colors"
                      >
                        <StatusDot online={health?.online ?? false} />
                        <div className="flex-1 min-w-0">
                          <p className="text-sm font-medium text-gray-200 truncate">{cam.name || cam.id}</p>
                          <p className="text-xs text-gray-500 font-mono">{cam.id}</p>
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
                        <div className="px-5 pb-5 bg-gray-850">
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
