import { useEffect, useState, Fragment } from 'react'
import AppLayout from '../components/AppLayout'
import { authHeaders, onUnauthorized } from '../auth'

interface Device {
  id: string
  name: string
  collected_at: string | null
  values: Record<string, string>
}

const FIELDS: [string, string][] = [
  ['model', 'Modelo'],
  ['serial', 'Serial'],
  ['firmware', 'Firmware'],
  ['mac', 'MAC'],
  ['collector', 'Coletor'],
]

export default function DevicesPage() {
  const [devices, setDevices] = useState<Device[]>([])

  useEffect(() => {
    fetch('/api/devices', { headers: authHeaders() })
      .then(r => { if (r.status === 401) { onUnauthorized(); return null } return r.json() })
      .then(d => { if (Array.isArray(d)) setDevices(d) })
      .catch(() => {})
  }, [])

  return (
    <AppLayout>
      <h2 className="text-2xl font-bold text-foreground">Dispositivos</h2>
      <p className="text-sm text-muted mt-1 mb-6">Hardware e metadados das câmeras.</p>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {devices.map(d => (
          <div key={d.id} id={`device-${d.id}`} className="bg-surface border border-border rounded-lg p-4">
            <div className="font-medium text-foreground mb-2 truncate">{d.name}</div>
            <dl className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-xs">
              {FIELDS.map(([k, label]) => (
                <Fragment key={k}>
                  <dt className="text-faint">{label}</dt>
                  <dd className="text-foreground truncate" title={d.values[k]}>{d.values[k] || '—'}</dd>
                </Fragment>
              ))}
            </dl>
          </div>
        ))}
        {devices.length === 0 && <p className="text-sm text-muted">Nenhum dispositivo encontrado.</p>}
      </div>
    </AppLayout>
  )
}
