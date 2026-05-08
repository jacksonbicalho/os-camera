import SettingsLayout from '../../components/SettingsLayout'
import StatCard from '../../components/StatCard'
import { useStats } from '../../hooks/useStats'
import { formatBytes, formatDuration } from '../statsUtils'

export default function StatsSettingsPage() {
  const stats = useStats('/settings/stats')

  const hasLimit = (stats?.max_size_bytes ?? 0) > 0
  const limitRef = hasLimit ? stats!.max_size_bytes : (stats?.disk_total_bytes ?? 0)
  const usedPercent = limitRef > 0
    ? Math.min(100, Math.round((stats!.recordings_bytes / limitRef) * 100))
    : 0
  const warnThreshold = hasLimit && stats ? stats.warn_percent : 0
  const isWarning = warnThreshold > 0 && usedPercent >= warnThreshold
  const isOver = hasLimit && stats ? stats.recordings_bytes >= stats.max_size_bytes : false
  const barColor = isOver ? 'bg-red-600' : isWarning ? 'bg-yellow-500' : 'bg-blue-600'

  return (
    <SettingsLayout>
      <h2 className="text-lg font-semibold text-gray-200 mb-6">Estatísticas</h2>
      {!stats ? (
        <p className="text-gray-500 text-sm">Carregando...</p>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
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

          <StatCard label="Câmeras" value={stats.camera_count} subtext="configuradas" />
          <StatCard label="Clientes conectados" value={stats.connected_clients} subtext="ativos no stream (30s)" />
          <StatCard
            label="Gravações"
            value={stats.recordings_count.toLocaleString()}
            subtext={`arquivos MP4 · ${formatBytes(stats.recordings_bytes)}`}
          />
          <StatCard label="Horas gravadas" value={formatDuration(stats.recordings_duration_seconds)} subtext="de vídeo em disco" />
          <StatCard
            label="Previsão de capacidade"
            value={formatDuration(stats.forecast_seconds)}
            subtext={stats.forecast_seconds > 0 ? 'restantes com o espaço disponível' : 'dados insuficientes para estimar'}
          />
        </div>
      )}
    </SettingsLayout>
  )
}
