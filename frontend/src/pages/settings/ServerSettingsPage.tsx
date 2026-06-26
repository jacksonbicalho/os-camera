import SettingsLayout from '../../components/SettingsLayout'
import PageHeader from '../../components/PageHeader'
import SettingsSection from '../../components/SettingsSection'
import { useSettings } from '../../hooks/useSettings'
import { getRole } from '../../auth'

export default function ServerSettingsPage() {
  const isAdmin = getRole() === 'admin'
  const { settings } = useSettings()
  const s = settings?.server

  return (
    <SettingsLayout>
      <PageHeader size="section" title="Servidor" subtitle="Porta, JWT e configurações de rede." />
      {!isAdmin ? (
        <p className="text-muted-foreground text-sm">Acesso restrito.</p>
      ) : !s ? (
        <p className="text-muted-foreground text-sm">Carregando...</p>
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
