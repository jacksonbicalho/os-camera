import { useEffect, useState, type ReactNode } from 'react'
import { authHeaders } from '../auth'
import { groupDeviceInfo, type DeviceInfoField } from '../pages/deviceInfoUtils'

interface DeviceInfoData {
  collected_at: string
  values: Record<string, string>
}

type State =
  | { kind: 'loading' }
  | { kind: 'empty' }
  | { kind: 'error'; msg: string }
  | { kind: 'data'; data: DeviceInfoData }

function fmtCollectedAt(iso: string): string {
  const d = new Date(iso)
  return isNaN(d.getTime()) ? iso : d.toLocaleString('pt-BR')
}

function fmtValue(key: string, value: string): string {
  if (key === 'ntp.enabled') return value === 'true' ? 'ativo' : 'inativo'
  return value
}

function chunk<T>(arr: T[], size: number): T[][] {
  const out: T[][] = []
  for (let i = 0; i < arr.length; i += size) out.push(arr.slice(i, i + size))
  return out
}

function rowClass(n: number): string {
  if (n === 3) return 'grid grid-cols-3 divide-x divide-gray-800'
  if (n === 2) return 'grid grid-cols-2 divide-x divide-gray-800'
  return ''
}

function Cell({ field }: { field: DeviceInfoField }) {
  return (
    <div className="px-5 py-3 min-w-0">
      <dt className="mb-1 text-xs text-gray-500">{field.label}</dt>
      <dd className="break-all font-mono text-sm text-gray-200">{fmtValue(field.key, field.value)}</dd>
    </div>
  )
}

export default function DeviceInfoPanel({ cameraId, isAdmin }: { cameraId: string; isAdmin: boolean }) {
  const [state, setState] = useState<State>({ kind: 'loading' })
  const [refreshing, setRefreshing] = useState(false)

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const res = await fetch(`/api/cameras/${cameraId}/device-info`, { headers: authHeaders() })
        if (cancelled) return
        if (res.status === 404) { setState({ kind: 'empty' }); return }
        if (!res.ok) { setState({ kind: 'error', msg: 'Erro ao carregar informações do dispositivo' }); return }
        const data = await res.json()
        if (!cancelled) setState({ kind: 'data', data })
      } catch {
        if (!cancelled) setState({ kind: 'error', msg: 'Erro ao carregar informações do dispositivo' })
      }
    })()
    return () => { cancelled = true }
  }, [cameraId])

  const refresh = async () => {
    setRefreshing(true)
    try {
      const res = await fetch(`/api/cameras/${cameraId}/device-info/refresh`, {
        method: 'POST',
        headers: authHeaders(),
      })
      if (res.ok) setState({ kind: 'data', data: await res.json() })
      else setState({ kind: 'error', msg: 'Erro ao reanalisar o dispositivo' })
    } catch {
      setState({ kind: 'error', msg: 'Erro ao reanalisar o dispositivo' })
    } finally {
      setRefreshing(false)
    }
  }

  let body: ReactNode
  if (state.kind === 'loading') {
    body = <p className="px-5 py-4 text-gray-500 text-sm">Carregando…</p>
  } else if (state.kind === 'empty') {
    body = <p className="px-5 py-4 text-gray-500 text-sm">Dispositivo ainda não capturado.</p>
  } else if (state.kind === 'error') {
    body = <p className="px-5 py-4 text-red-400 text-sm">{state.msg}</p>
  } else {
    const grouped = groupDeviceInfo(state.data.values)
    body = (
      <>
        <div className="divide-y divide-gray-800">
          {grouped.sections.map((sec) => (
            <div key={sec.title}>
              <p className="px-5 pt-3 pb-1 text-[11px] text-gray-500 uppercase tracking-wider">{sec.title}</p>
              <div className="divide-y divide-gray-800">
                {chunk(sec.fields, 3).map((row, i) => (
                  <div key={i} className={rowClass(row.length)}>
                    {row.map((f) => (
                      <Cell key={f.key} field={f} />
                    ))}
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
        {grouped.raw.length > 0 && (
          <details id="device-info-raw" className="border-t border-gray-800">
            <summary
              id="device-info-raw-toggle"
              className="cursor-pointer px-5 py-3 text-[11px] text-gray-500 uppercase tracking-wider"
            >
              Dados brutos ({grouped.raw.length})
            </summary>
            <div className="max-h-80 overflow-auto divide-y divide-gray-800 border-t border-gray-800">
              {grouped.raw.map((f) => (
                <div key={f.key} className="grid grid-cols-2 divide-x divide-gray-800">
                  <span className="px-5 py-1.5 text-xs text-gray-500 break-all font-mono">{f.key}</span>
                  <span className="px-5 py-1.5 text-xs text-gray-300 break-all font-mono">{f.value}</span>
                </div>
              ))}
            </div>
          </details>
        )}
      </>
    )
  }

  return (
    <div id="device-info-section" className="bg-gray-900 border border-gray-800 rounded-lg overflow-hidden">
      <div className="flex items-center justify-between gap-3 px-5 pt-4 pb-3 border-b border-gray-800">
        <div className="min-w-0">
          <h2 className="text-xs text-gray-400 uppercase tracking-wider font-medium">Informações do dispositivo</h2>
          {state.kind === 'data' && (
            <p className="text-[11px] text-gray-500 mt-0.5">Última coleta: {fmtCollectedAt(state.data.collected_at)}</p>
          )}
        </div>
        {isAdmin && (
          <button
            id="device-info-refresh"
            onClick={refresh}
            disabled={refreshing}
            className="shrink-0 px-3 py-1.5 text-xs bg-gray-800 hover:bg-gray-700 border border-gray-700 text-gray-300 hover:text-white rounded transition-colors disabled:opacity-50"
          >
            {refreshing ? 'Reanalisando…' : state.kind === 'empty' ? 'Capturar agora' : 'Reanalisar'}
          </button>
        )}
      </div>
      {body}
    </div>
  )
}
