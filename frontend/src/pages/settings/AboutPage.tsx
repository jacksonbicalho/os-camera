import SettingsLayout from '../../components/SettingsLayout'
import SettingsSection from '../../components/SettingsSection'
import { useAbout } from '../../hooks/useSettings'

function fmtUptime(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  if (h > 0) return `${h}h ${m}m ${s}s`
  if (m > 0) return `${m}m ${s}s`
  return `${s}s`
}

export default function AboutPage() {
  const about = useAbout()

  return (
    <SettingsLayout>
      <h3 className="text-h2 font-semibold text-gray-200">Sobre</h3>
      <p className="text-sm text-gray-500 mt-1 mb-6">Versão instalada, commit e tempo de atividade.</p>
      {!about ? (
        <p className="text-gray-500 text-sm">Carregando...</p>
      ) : (
        <SettingsSection
          title="Informações do servidor"
          fields={[
            { label: 'Versão', value: about.version || 'dev' },
            { label: 'Commit', value: about.commit || '—' },
            { label: 'Build', value: about.built_at || '—' },
            { label: 'Ativo há', value: fmtUptime(about.uptime_seconds) },
            { label: 'Go', value: about.go_version },
          ]}
        />
      )}
    </SettingsLayout>
  )
}
