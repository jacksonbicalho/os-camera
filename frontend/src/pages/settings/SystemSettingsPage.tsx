import SettingsLayout from '../../components/SettingsLayout'
import SettingsSection from '../../components/SettingsSection'
import { useSettings } from '../../hooks/useSettings'
import { getRole } from '../../auth'

export default function SystemSettingsPage() {
  const isAdmin = getRole() === 'admin'
  const { settings } = useSettings()

  return (
    <SettingsLayout>
      <h3 className="text-h2 font-semibold text-foreground">Sistema</h3>
      <p className="text-sm text-muted-foreground mt-1 mb-6">Fuso horário e configurações de log.</p>
      {!isAdmin ? (
        <p className="text-muted-foreground text-sm">Acesso restrito.</p>
      ) : !settings ? (
        <p className="text-muted-foreground text-sm">Carregando...</p>
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
            groups={
              settings.log.output === 'file'
                ? [
                    [
                      { label: 'Destino', value: settings.log.output },
                      { label: 'Diretório', value: settings.log.path || '—' },
                      { label: 'Rotaciona em', value: `${settings.log.max_size_mb} MB` },
                    ],
                    [
                      {
                        label: 'Retenção',
                        value: settings.log.max_age_days > 0 ? `${settings.log.max_age_days} dias` : 'ilimitada',
                      },
                      {
                        label: 'Máx. de arquivos',
                        value: settings.log.max_backups > 0 ? String(settings.log.max_backups) : 'ilimitado',
                      },
                      { label: 'Compressão', value: settings.log.compress ? 'gzip' : 'desativada' },
                    ],
                  ]
                : [
                    [
                      { label: 'Destino', value: settings.log.output || 'stdout' },
                      { label: 'Diretório', value: settings.log.path || '—' },
                    ],
                  ]
            }
          />
          <SettingsSection
            title="Caminhos"
            fields={[
              { label: 'Segmentos HLS', value: settings.server.segments_path || '—' },
              { label: 'Gravações', value: settings.server.recordings_path || '—' },
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
