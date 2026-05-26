import SettingsLayout from '../../components/SettingsLayout'
import SettingsSection from '../../components/SettingsSection'
import { useSettings } from '../../hooks/useSettings'
import { getRole } from '../../auth'

export default function ServerSettingsPage() {
  const isAdmin = getRole() === 'admin'
  const { settings } = useSettings('/settings/server')
  const s = settings?.server

  return (
    <SettingsLayout>
      <h3 className="text-lg font-semibold text-gray-200">Servidor</h3>
      <p className="text-sm text-gray-500 mt-1 mb-6">Porta, JWT e configurações de rede.</p>
      {!isAdmin ? (
        <p className="text-gray-500 text-sm">Acesso restrito.</p>
      ) : !s ? (
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
