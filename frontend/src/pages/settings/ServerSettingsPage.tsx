import SettingsLayout from '../../components/SettingsLayout'
import SettingsSection from '../../components/SettingsSection'
import { useSettings } from '../../hooks/useSettings'

export default function ServerSettingsPage() {
  const settings = useSettings('/settings/server')
  const s = settings?.server

  return (
    <SettingsLayout>
      <h2 className="text-lg font-semibold text-gray-200 mb-6">Servidor</h2>
      {!s ? (
        <p className="text-gray-500 text-sm">Carregando...</p>
      ) : (
        <SettingsSection
          title="Servidor web"
          fields={[
            { label: 'Porta HTTP', value: s.port },
            { label: 'Usuário', value: s.username },
          ]}
        />
      )}
    </SettingsLayout>
  )
}
