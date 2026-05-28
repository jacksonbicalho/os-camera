import SettingsLayout from '../../components/SettingsLayout'
import SettingsSection from '../../components/SettingsSection'
import { useSettings } from '../../hooks/useSettings'
import { getRole } from '../../auth'

export default function SystemSettingsPage() {
  const isAdmin = getRole() === 'admin'
  const { settings } = useSettings()

  return (
    <SettingsLayout>
      <h3 className="text-lg font-semibold text-gray-200">Sistema</h3>
      <p className="text-sm text-gray-500 mt-1 mb-6">Fuso horário e configurações de log.</p>
      {!isAdmin ? (
        <p className="text-gray-500 text-sm">Acesso restrito.</p>
      ) : !settings ? (
        <p className="text-gray-500 text-sm">Carregando...</p>
      ) : (
        <div className="flex flex-col gap-4">
          <SettingsSection
            title="Geral"
            fields={[
              { label: 'Fuso horário', value: settings.timezone || '—' },
              { label: 'Modo debug', value: settings.debug ? 'ativado' : 'desativado' },
            ]}
          />
          <SettingsSection
            title="Logs"
            fields={[
              { label: 'Destino', value: settings.log.output || 'stdout' },
              { label: 'Diretório', value: settings.log.path || '—' },
            ]}
          />
          <SettingsSection
            title="Caminhos"
            fields={[
              { label: 'Segmentos HLS', value: settings.server.segments_path || '—' },
              { label: 'Gravações', value: settings.server.recordings_path || '—' },
              { label: 'Janela DVR (segundos)', value: settings.server.hls_dvr_seconds === 0 ? 'desativado' : settings.server.hls_dvr_seconds },
            ]}
          />
          <SettingsSection
            title="Padrões de câmera"
            fields={[
              { label: 'Duração do chunk', value: settings.defaults.chunk_duration },
              { label: 'Intervalo de reconexão', value: settings.defaults.reconnect_interval },
            ]}
          />
        </div>
      )}
    </SettingsLayout>
  )
}
