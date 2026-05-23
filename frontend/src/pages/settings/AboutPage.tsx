import { useEffect, useState } from 'react'
import SettingsLayout from '../../components/SettingsLayout'
import SettingsSection from '../../components/SettingsSection'
import { useAbout } from '../../hooks/useSettings'
import { authHeaders } from '../../auth'
import { getRole } from '../../auth'

interface UpdateInfo {
  current: string
  latest: string
  update_available: boolean
  changelog_url: string
  mode?: string
  message?: string
}

function fmtUptime(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  if (h > 0) return `${h}h ${m}m ${s}s`
  if (m > 0) return `${m}m ${s}s`
  return `${s}s`
}

export default function AboutPage() {
  const about = useAbout('/settings/about')
  const isAdmin = getRole() === 'admin'
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null)
  const [checking, setChecking] = useState(false)
  const [applying, setApplying] = useState(false)
  const [applyMsg, setApplyMsg] = useState<string | null>(null)

  useEffect(() => {
    if (!isAdmin) return
    setChecking(true)
    fetch('/api/update/check', { headers: authHeaders() })
      .then(r => r.ok ? r.json() : null)
      .then(d => { if (d) setUpdateInfo(d) })
      .catch(() => {})
      .finally(() => setChecking(false))
  }, [isAdmin])

  async function handleUpdate() {
    setApplying(true)
    setApplyMsg(null)
    try {
      const r = await fetch('/api/update/apply', { method: 'POST', headers: authHeaders() })
      const data = await r.json()
      if (data.mode === 'docker') {
        setApplyMsg(data.message)
        setApplying(false)
        return
      }
      // binary update: poll /api/about until version changes
      setApplyMsg('Atualizando... aguardando reinício')
      const targetVersion = updateInfo?.latest
      let attempts = 0
      const poll = setInterval(async () => {
        attempts++
        try {
          const res = await fetch('/api/about', { headers: authHeaders() })
          if (res.ok) {
            const info = await res.json()
            if (info.version === targetVersion) {
              clearInterval(poll)
              window.location.reload()
            }
          }
        } catch {}
        if (attempts >= 15) {
          clearInterval(poll)
          setApplyMsg('Reinício demorou mais que o esperado. Recarregue a página manualmente.')
          setApplying(false)
        }
      }, 2000)
    } catch {
      setApplyMsg('Erro ao aplicar atualização.')
      setApplying(false)
    }
  }

  return (
    <SettingsLayout>
      <h2 className="text-lg font-semibold text-gray-200 mb-6">Sobre</h2>
      {!about ? (
        <p className="text-gray-500 text-sm">Carregando...</p>
      ) : (
        <div className="flex flex-col gap-4">
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

          {isAdmin && (
            <div className="bg-gray-900 border border-gray-800 rounded-xl p-5">
              <p className="text-xs text-gray-500 uppercase tracking-wider mb-4">Atualização</p>

              {checking && (
                <p className="text-sm text-gray-500">Verificando...</p>
              )}

              {!checking && updateInfo && !updateInfo.update_available && (
                <p className="text-sm text-gray-400">
                  Sistema na versão mais recente <span className="text-gray-600">({updateInfo.current})</span>
                </p>
              )}

              {!checking && updateInfo?.update_available && (
                <div className="flex items-center justify-between gap-4">
                  <div>
                    <p className="text-sm text-gray-200">
                      Nova versão disponível:{' '}
                      <a
                        href={updateInfo.changelog_url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-blue-400 hover:text-blue-300"
                      >
                        {updateInfo.latest}
                      </a>
                    </p>
                    <p className="text-xs text-gray-500 mt-0.5">atual: {updateInfo.current}</p>
                  </div>
                  <button
                    onClick={handleUpdate}
                    disabled={applying}
                    className="px-4 py-2 text-sm bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-lg transition-colors shrink-0"
                  >
                    {applying ? 'Atualizando...' : 'Atualizar'}
                  </button>
                </div>
              )}

              {applyMsg && (
                <p className="text-xs text-gray-400 mt-3">{applyMsg}</p>
              )}
            </div>
          )}
        </div>
      )}
    </SettingsLayout>
  )
}
