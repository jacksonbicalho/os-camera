import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import SettingsLayout from '../../components/SettingsLayout'
import { authHeaders } from '../../auth'
import { Loader2, Search } from '../../components/Icons'

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
  const [status, setStatus] = useState<Status>('idle')
  const [results, setResults] = useState<DiscoveryResult[]>([])
  const [error, setError] = useState<string | null>(null)
  const [adding, setAdding] = useState<AddState | null>(null)
  const navigate = useNavigate()

  async function handleScan() {
    setStatus('scanning')
    setError(null)
    setResults([])
    setAdding(null)
    try {
      const res = await fetch('/api/discover', { headers: authHeaders() })
      if (!res.ok) { setError(await res.text()); setStatus('error'); return }
      setResults(await res.json())
      setStatus('done')
    } catch (e) {
      setError(String(e))
      setStatus('error')
    }
  }

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

  const inputClass = "bg-gray-950 border border-gray-700 rounded px-2.5 py-1 text-xs text-gray-200 focus:outline-none focus:border-blue-500 w-36"

  return (
    <SettingsLayout>
      <div className="flex flex-col gap-6">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-base font-semibold text-white">Rastrear câmeras na rede</h3>
            <p className="text-xs text-gray-500 mt-0.5">
              ONVIF WS-Discovery (multicast UDP) + varredura de porta 554 na subnet local
            </p>
          </div>
          <button
            onClick={handleScan}
            disabled={status === 'scanning'}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
          >
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
          </button>
        </div>

        {status === 'scanning' && (
          <p className="text-xs text-gray-400 animate-pulse">
            Aguardando respostas ONVIF (3 s) e varrendo porta 554 na subnet…
          </p>
        )}

        {status === 'error' && (
          <p className="text-xs text-red-400">{error}</p>
        )}

        {status === 'done' && results.length === 0 && (
          <p className="text-sm text-gray-500">Nenhuma câmera encontrada na rede.</p>
        )}

        {results.length > 0 && (
          <div className="bg-gray-900 border border-gray-800 rounded-lg overflow-hidden">
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-gray-800 text-gray-500 uppercase tracking-wider">
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
                      className={`border-b border-gray-800 ${adding?.idx === i ? '' : 'last:border-0'} hover:bg-gray-800/40 transition-colors`}
                    >
                      <td className="px-4 py-2.5 font-mono text-gray-200">{r.ip}</td>
                      <td className="px-4 py-2.5 text-gray-400">{r.port}</td>
                      <td className="px-4 py-2.5">
                        {r.onvif ? (
                          <span className="px-1.5 py-0.5 rounded bg-blue-900/50 text-blue-300 text-[10px] font-medium">ONVIF</span>
                        ) : (
                          <span className="px-1.5 py-0.5 rounded bg-gray-800 text-gray-400 text-[10px] font-medium">Scan</span>
                        )}
                      </td>
                      <td className="px-4 py-2.5 text-gray-300">{r.name || <span className="text-gray-600">—</span>}</td>
                      <td className="px-4 py-2.5 text-right">
                        {adding?.idx === i ? (
                          <button
                            onClick={() => setAdding(null)}
                            className="px-3 py-1 text-gray-400 hover:text-white border border-gray-700 rounded text-[11px] font-medium transition-colors"
                          >
                            Cancelar
                          </button>
                        ) : (
                          <button
                            onClick={() => startAdding(i)}
                            className="px-3 py-1 bg-blue-700 hover:bg-blue-600 text-white rounded text-[11px] font-medium transition-colors"
                          >
                            Adicionar
                          </button>
                        )}
                      </td>
                    </tr>

                    {adding?.idx === i && adding.step === 'creds' && (
                      <tr key={`${i}-creds`} className="border-b border-gray-800 last:border-0 bg-gray-800/50">
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
                            <button
                              onClick={() => confirmCreds(r)}
                              className="px-3 py-1 bg-blue-600 hover:bg-blue-500 text-white rounded text-[11px] font-medium transition-colors"
                            >
                              Confirmar
                            </button>
                          </div>
                        </td>
                      </tr>
                    )}

                    {adding?.idx === i && adding.step === 'loading' && (
                      <tr key={`${i}-loading`} className="border-b border-gray-800 last:border-0 bg-gray-800/50">
                        <td colSpan={5} className="px-4 py-3">
                          <p className="text-xs text-gray-400 animate-pulse">Buscando streams disponíveis…</p>
                        </td>
                      </tr>
                    )}

                    {adding?.idx === i && adding.step === 'streams' && (
                      <tr key={`${i}-streams`} className="border-b border-gray-800 last:border-0 bg-gray-800/50">
                        <td colSpan={5} className="px-4 py-3">
                          <p className="text-xs text-gray-500 mb-2">Escolha o stream:</p>
                          <div className="flex flex-wrap gap-2">
                            {adding.streams.map(s => (
                              <button
                                key={s.url}
                                onClick={() => selectStream(r, s.url)}
                                className="px-3 py-1.5 bg-gray-900 hover:bg-blue-700 border border-gray-700 hover:border-blue-500 text-gray-200 rounded text-[11px] transition-colors"
                              >
                                <span className="font-medium">{s.name}</span>
                                <span className="text-gray-500 ml-1.5 font-mono">{s.url.replace(/^rtsp:\/\/[^@]+@/, 'rtsp://…@')}</span>
                              </button>
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
