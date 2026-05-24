import SettingsLayout from '../../components/SettingsLayout'
import SettingsSection from '../../components/SettingsSection'
import { useSettings } from '../../hooks/useSettings'

function fmtMinutes(m: number): string {
  if (m === 0) return 'desativado'
  if (m < 60) return `${m} min`
  const h = Math.floor(m / 60)
  const rem = m % 60
  return rem === 0 ? `${h}h` : `${h}h ${rem}min`
}

export default function StorageSettingsPage() {
  const { settings } = useSettings('/settings/storage')
  const s = settings?.storage

  return (
    <SettingsLayout>
      <h3 className="text-lg font-semibold text-gray-200">Armazenamento</h3>
      <p className="text-sm text-gray-500 mt-1 mb-6">Retenção, limpeza automática e espaço em disco.</p>
      {!s ? (
        <p className="text-gray-500 text-sm">Carregando...</p>
      ) : (
        <SettingsSection
          title="Armazenamento"
          fields={[
            { label: 'Diretório', value: s.path || '—' },
            { label: 'Retenção com movimento', value: s.with_motion_minutes === 0 ? 'indefinido' : fmtMinutes(s.with_motion_minutes) },
            { label: 'Retenção sem movimento', value: fmtMinutes(s.without_motion_minutes) },
            { label: 'Intervalo do cleaner', value: s.interval_minutes === 0 ? 'padrão (60 min)' : fmtMinutes(s.interval_minutes) },
            { label: 'Tamanho máximo (GB)', value: s.max_size_gb === 0 ? 'desativado' : s.max_size_gb },
            { label: 'Alerta de uso', value: s.warn_percent === 0 ? 'desativado' : `${s.warn_percent}%` },
          ]}
        />
      )}
    </SettingsLayout>
  )
}
