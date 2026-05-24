import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import SettingsLayout from '../../components/SettingsLayout'
import { authHeaders } from '../../auth'

interface DiscoveryResult {
  ip: string
  port: number
  onvif: boolean
  name?: string
  rtsp_urls?: string[]
  services?: string[]
}

type Status = 'idle' | 'scanning' | 'done' | 'error'

export default function DiscoverPage() {
  const [status, setStatus] = useState<Status>('idle')
  const [results, setResults] = useState<DiscoveryResult[]>([])
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()

  async function handleScan() {
    setStatus('scanning')
    setError(null)
    setResults([])
    try {
      const res = await fetch('/api/discover', { headers: authHeaders() })
      if (!res.ok) { setError(await res.text()); setStatus('error'); return }
      const data: DiscoveryResult[] = await res.json()
      setResults(data)
      setStatus('done')
    } catch (e) {
      setError(String(e))
      setStatus('error')
    }
  }

  function handleAdd(r: DiscoveryResult) {
    const rtsp = r.rtsp_urls?.[0] ?? `rtsp://${r.ip}:${r.port === 554 ? '' : r.port}/`
    const params = new URLSearchParams({ prefill_rtsp: rtsp })
    if (r.name) params.set('prefill_name', r.name)
    navigate(`/settings/cameras/new?${params}`)
  }

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
                <svg className="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4l3-3-3-3v4a8 8 0 00-8 8h4z" />
                </svg>
                Rastreando…
              </>
            ) : (
              <>
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}>
                  <circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" />
                </svg>
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
                  <tr key={i} className="border-b border-gray-800 last:border-0 hover:bg-gray-800/40 transition-colors">
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
                      <button
                        onClick={() => handleAdd(r)}
                        className="px-3 py-1 bg-blue-700 hover:bg-blue-600 text-white rounded text-[11px] font-medium transition-colors"
                      >
                        Adicionar
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </SettingsLayout>
  )
}
