import SettingsLayout from '../../components/SettingsLayout'
import SettingsSection from '../../components/SettingsSection'
import { useSettings } from '../../hooks/useSettings'

export default function SystemSettingsPage() {
  const { settings } = useSettings('/settings/system')

  return (
    <SettingsLayout>
      <h2 className="text-lg font-semibold text-gray-200 mb-6">Sistema</h2>
      {!settings ? (
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
          <SettingsSection
            title="Detecção de movimento (global)"
            fields={[
              { label: 'Ativado', value: settings.motion.enabled ? 'sim' : 'não' },
              { label: 'Limiar', value: settings.motion.threshold },
              { label: 'FPS de amostragem', value: settings.motion.fps },
              { label: 'Cooldown (segundos)', value: settings.motion.cooldown_seconds === 0 ? 'desativado' : settings.motion.cooldown_seconds },
            ]}
          />
        </div>
      )}
    </SettingsLayout>
  )
}
