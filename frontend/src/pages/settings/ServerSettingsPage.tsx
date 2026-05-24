import SettingsLayout from '../../components/SettingsLayout'
import SettingsSection from '../../components/SettingsSection'
import { useSettings } from '../../hooks/useSettings'

export default function ServerSettingsPage() {
  const { settings } = useSettings('/settings/server')
  const s = settings?.server

  return (
    <SettingsLayout>
      <h3 className="text-lg font-semibold text-gray-200">Servidor</h3>
      <p className="text-sm text-gray-500 mt-1 mb-6">Porta, JWT e configurações de rede.</p>
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
