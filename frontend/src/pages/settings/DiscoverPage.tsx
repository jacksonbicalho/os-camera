import { useState, useEffect, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import SettingsLayout from '../../components/SettingsLayout'
import PageHeader from '../../components/PageHeader'
import { authHeaders } from '../../auth'
import { Loader2, Search } from '../../components/Icons'
import { Button } from '@/components/ui/button'
import { findRegisteredCameraName } from './discoverDedupe'

interface RegisteredCamera { id: string; name?: string; rtsp_url?: string }

interface DiscoveryResult {
  ip: string
  port: number
  onvif: boolean
  onvif_xaddr?: string
  name?: string
  rtsp_urls?: string[]
  services?: string[]
}

interface StreamURI {
  name: string
  url: string
}

type Status = 'idle' | 'scanning' | 'done' | 'error'
type AddStep = 'creds' | 'loading' | 'streams'

interface AddState {
  idx: number
  user: string
  pass: string
  step: AddStep
  streams: StreamURI[]
}

function injectCredentials(url: string, user: string, pass: string): string {
  const noAuth = url.replace(/^(rtsp:\/\/)([^@]+@)?/, '$1')
  return noAuth.replace('rtsp://', `rtsp://${encodeURIComponent(user)}:${encodeURIComponent(pass)}@`)
}

export default function DiscoverPage() {
  // Status inicial 'scanning': a tela já rastreia ao abrir (ver effect abaixo).
  const [status, setStatus] = useState<Status>('scanning')
  const [results, setResults] = useState<DiscoveryResult[]>([])
  const [error, setError] = useState<string | null>(null)
  const [adding, setAdding] = useState<AddState | null>(null)
  const [registered, setRegistered] = useState<RegisteredCamera[]>([])
  const navigate = useNavigate()

  // runScan só toca estado DEPOIS do await (assíncrono) — assim pode ser chamada do
  // effect de montagem sem disparar setState síncrono dentro do effect.
  const runScan = useCallback(async () => {
    try {
      const res = await fetch('/api/discover', { headers: authHeaders() })
      if (!res.ok) { setError(await res.text()); setStatus('error'); return }
      setResults(await res.json())
      setStatus('done')
    } catch (e) {
      setError(String(e))
      setStatus('error')
    }
  }, [])

  // Botão "Rastrear": reset síncrono (permitido em handler) + dispara o scan.
  function handleScan() {
    setStatus('scanning')
    setError(null)
    setResults([])
    setAdding(null)
    runScan()
  }

  // Ao abrir a tela: carrega as câmeras já cadastradas (para dedupe) e já inicia o
  // rastreamento automaticamente (status já começa 'scanning'). O scan é inlinado
  // aqui (setState só em callbacks .then/.catch) em vez de chamar runScan, para não
  // disparar setState síncrono dentro do effect.
  useEffect(() => {
    fetch('/api/settings/cameras', { headers: authHeaders() })
      .then(r => r.ok ? r.json() : [])
      .then((cams: RegisteredCamera[]) => setRegistered(cams))
      .catch(() => {})
    fetch('/api/discover', { headers: authHeaders() })
      .then(async res => {
        if (!res.ok) { setError(await res.text()); setStatus('error'); return }
        setResults(await res.json())
        setStatus('done')
      })
      .catch(e => { setError(String(e)); setStatus('error') })
  }, [])

  function startAdding(idx: number) {
    setAdding({ idx, user: '', pass: '', step: 'creds', streams: [] })
  }

  async function confirmCreds(r: DiscoveryResult) {
    if (!adding) return
    const { user, pass } = adding

    if (r.onvif && r.onvif_xaddr) {
      setAdding(a => a ? { ...a, step: 'loading' } : a)
      try {
        const res = await fetch('/api/discover/streams', {
          method: 'POST',
          headers: { ...authHeaders(), 'Content-Type': 'application/json' },
          body: JSON.stringify({ onvif_xaddr: r.onvif_xaddr, user, pass }),
        })
        const data: { streams: StreamURI[] } = await res.json()
        if (data.streams.length > 0) {
          setAdding(a => a ? { ...a, step: 'streams', streams: data.streams } : a)
          return
        }
      } catch { /* fallback */ }
    }

    navigateWithURL(r, user, pass, r.rtsp_urls?.[0] ?? `rtsp://${r.ip}:${r.port}/`)
  }

  function navigateWithURL(r: DiscoveryResult, user: string, pass: string, baseURL: string) {
    const rtsp = user ? injectCredentials(baseURL, user, pass) : baseURL
    const params = new URLSearchParams({ prefill_rtsp: rtsp })
    if (r.name) params.set('prefill_name', r.name)
    navigate(`/settings/cameras/new?${params}`)
  }

  function selectStream(r: DiscoveryResult, url: string) {
    if (!adding) return
    navigateWithURL(r, adding.user, adding.pass, url)
  }

  const inputClass = "bg-background border border-border rounded px-2.5 py-1 text-xs text-foreground focus:outline-none focus:border-ring w-36"

  return (
    <SettingsLayout>
      <div className="flex flex-col gap-6">
        <PageHeader
          size="section"
          className="mb-0"
          title="Rastrear câmeras na rede"
          subtitle="ONVIF WS-Discovery (multicast UDP) + varredura de porta 554 na subnet local"
          actions={
            <Button id="discover-scan" onClick={handleScan} disabled={status === 'scanning'}>
              {status === 'scanning' ? (
                <>
                  <Loader2 className="w-4 h-4 animate-spin" />
                  Rastreando…
                </>
              ) : (
                <>
                  <Search className="w-4 h-4" />
                  Rastrear
                </>
              )}
            </Button>
          }
        />

        {status === 'scanning' && (
          <p className="text-xs text-muted-foreground animate-pulse">
            Aguardando respostas ONVIF (3 s) e varrendo porta 554 na subnet…
          </p>
        )}

        {status === 'error' && (
          <p className="text-xs text-red-400">{error}</p>
        )}

        {status === 'done' && results.length === 0 && (
          <p className="text-sm text-muted-foreground">Nenhuma câmera encontrada na rede.</p>
        )}

        {results.length > 0 && (
          <div className="bg-surface border border-border rounded-lg overflow-hidden">
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-border text-muted-foreground uppercase tracking-wider">
                  <th className="text-left px-4 py-2.5">IP</th>
                  <th className="text-left px-4 py-2.5">Porta</th>
                  <th className="text-left px-4 py-2.5">Método</th>
                  <th className="text-left px-4 py-2.5">Nome / Modelo</th>
                  <th className="px-4 py-2.5" />
                </tr>
              </thead>
              <tbody>
                {results.map((r, i) => (
                  <>
                    <tr
                      key={i}
                      className={`border-b border-border ${adding?.idx === i ? '' : 'last:border-0'} hover:bg-accent/40 transition-colors`}
                    >
                      <td className="px-4 py-2.5 font-mono text-foreground">{r.ip}</td>
                      <td className="px-4 py-2.5 text-muted-foreground">{r.port}</td>
                      <td className="px-4 py-2.5">
                        {r.onvif ? (
                          <span className="px-1.5 py-0.5 rounded bg-blue-900/50 text-blue-300 text-[10px] font-medium">ONVIF</span>
                        ) : (
                          <span className="px-1.5 py-0.5 rounded bg-surface-2 text-muted-foreground text-[10px] font-medium">Scan</span>
                        )}
                      </td>
                      <td className="px-4 py-2.5 text-foreground">{r.name || <span className="text-muted-foreground">—</span>}</td>
                      <td className="px-4 py-2.5 text-right">
                        {(() => {
                          const regName = findRegisteredCameraName(r.ip, registered)
                          if (regName) {
                            return (
                              <span className="text-[11px] text-muted-foreground italic">
                                Já cadastrada como “{regName}”
                              </span>
                            )
                          }
                          return adding?.idx === i ? (
                            <Button variant="outline" size="sm" onClick={() => setAdding(null)}>
                              Cancelar
                            </Button>
                          ) : (
                            <Button size="sm" onClick={() => startAdding(i)}>
                              Adicionar
                            </Button>
                          )
                        })()}
                      </td>
                    </tr>

                    {adding?.idx === i && adding.step === 'creds' && (
                      <tr key={`${i}-creds`} className="border-b border-border last:border-0 bg-surface-2/50">
                        <td colSpan={5} className="px-4 py-3">
                          <div className="flex items-center gap-3 flex-wrap">
                            <input
                              placeholder="Usuário"
                              value={adding.user}
                              onChange={e => setAdding(a => a ? { ...a, user: e.target.value } : a)}
                              className={inputClass}
                              autoFocus
                            />
                            <input
                              type="password"
                              placeholder="Senha"
                              value={adding.pass}
                              onChange={e => setAdding(a => a ? { ...a, pass: e.target.value } : a)}
                              onKeyDown={e => e.key === 'Enter' && confirmCreds(r)}
                              className={inputClass}
                            />
                            <Button size="sm" onClick={() => confirmCreds(r)}>
                              Confirmar
                            </Button>
                          </div>
                        </td>
                      </tr>
                    )}

                    {adding?.idx === i && adding.step === 'loading' && (
                      <tr key={`${i}-loading`} className="border-b border-border last:border-0 bg-surface-2/50">
                        <td colSpan={5} className="px-4 py-3">
                          <p className="text-xs text-muted-foreground animate-pulse">Buscando streams disponíveis…</p>
                        </td>
                      </tr>
                    )}

                    {adding?.idx === i && adding.step === 'streams' && (
                      <tr key={`${i}-streams`} className="border-b border-border last:border-0 bg-surface-2/50">
                        <td colSpan={5} className="px-4 py-3">
                          <p className="text-xs text-muted-foreground mb-2">Escolha o stream:</p>
                          <div className="flex flex-wrap gap-2">
                            {adding.streams.map(s => (
                              <Button
                                key={s.url}
                                variant="outline"
                                size="sm"
                                className="h-auto py-1.5"
                                onClick={() => selectStream(r, s.url)}
                              >
                                <span className="font-medium">{s.name}</span>
                                <span className="text-muted-foreground ml-1.5 font-mono">{s.url.replace(/^rtsp:\/\/[^@]+@/, 'rtsp://…@')}</span>
                              </Button>
                            ))}
                          </div>
                        </td>
                      </tr>
                    )}
                  </>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </SettingsLayout>
  )
}
