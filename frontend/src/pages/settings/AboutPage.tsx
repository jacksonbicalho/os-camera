import { useState } from 'react'
import SettingsLayout from '../../components/SettingsLayout'
import SettingsSection from '../../components/SettingsSection'
import { useAbout } from '../../hooks/useSettings'
import { useUpdates } from '../../hooks/useUpdates'
import { getRole } from '../../auth'

function fmtUptime(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  if (h > 0) return `${h}h ${m}m ${s}s`
  if (m > 0) return `${m}m ${s}s`
  return `${s}s`
}

function UpdatesSection() {
  const { status, applyUpdate } = useUpdates()
  const [applying, setApplying] = useState(false)
  const [applyMsg, setApplyMsg] = useState('')
  const [applyErr, setApplyErr] = useState('')

  if (getRole() !== 'admin' || !status) return null

  const onApply = async () => {
    setApplying(true)
    setApplyErr('')
    const res = await applyUpdate()
    if (res.ok) {
      setApplyMsg('Atualizando… o servidor vai reiniciar em instantes.')
    } else {
      setApplyErr(res.error || 'Falha ao iniciar a atualização.')
      setApplying(false)
    }
  }

  return (
    <section id="updates-section" className="mt-8">
      <h4 className="text-h4 font-semibold text-foreground mb-3">Atualizações</h4>

      {status.error ? (
        <p id="update-error" className="text-sm text-muted-foreground">
          Não foi possível checar atualizações.
        </p>
      ) : !status.update_available ? (
        <p id="update-uptodate" className="text-sm text-muted-foreground">
          Você está na última versão ({status.current}).
        </p>
      ) : applyMsg ? (
        <p id="update-applying" className="text-sm text-foreground">{applyMsg}</p>
      ) : (
        <div className="rounded-lg border border-border bg-surface p-4">
          <p className="text-sm font-medium text-foreground">
            Nova versão <span className="font-mono">{status.latest}</span> disponível.
          </p>

          {status.notes_md && (
            <pre id="update-notes" className="mt-3 whitespace-pre-wrap text-xs text-muted-foreground font-sans">
              {status.notes_md}
            </pre>
          )}

          {status.apply_mode === 'self-replace' && (
            <button
              id="update-apply-button"
              onClick={onApply}
              disabled={applying}
              className="mt-4 rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-on-primary disabled:opacity-50"
            >
              {applying ? 'Atualizando…' : 'Atualizar agora'}
            </button>
          )}

          {status.apply_mode === 'docker' && (
            <div id="update-docker" className="mt-4 text-xs text-muted-foreground">
              <p>Atualize a imagem Docker e recrie o container:</p>
              <pre className="mt-1 rounded bg-surface-2 p-2 font-mono text-foreground">docker compose pull && docker compose up -d</pre>
              <p className="mt-1">Imagem: <span className="font-mono">{status.image}</span></p>
            </div>
          )}

          {status.apply_mode === 'notify' && (
            <p id="update-notify" className="mt-4 text-xs text-muted-foreground">
              Atualização automática indisponível neste ambiente — baixe a nova versão manualmente.
            </p>
          )}

          {applyErr && <p id="update-apply-error" className="mt-3 text-sm text-danger">{applyErr}</p>}
        </div>
      )}
    </section>
  )
}

export default function AboutPage() {
  const about = useAbout()

  return (
    <SettingsLayout>
      <h3 className="text-h2 font-semibold text-foreground">Sobre</h3>
      <p className="text-sm text-muted-foreground mt-1 mb-6">Versão instalada, commit e tempo de atividade.</p>
      {!about ? (
        <p className="text-muted-foreground text-sm">Carregando...</p>
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
      <UpdatesSection />
    </SettingsLayout>
  )
}
