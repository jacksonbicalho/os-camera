import { useEffect, useState } from 'react'
import SettingsLayout from '../../components/SettingsLayout'
import ConfirmDialog from '../../components/ConfirmDialog'
import { useSettings } from '../../hooks/useSettings'
import { authHeaders, getRole } from '../../auth'

// ── helpers ──────────────────────────────────────────────────────────────────

function minutesToParts(m: number): { value: number; unit: 'min' | 'h' | 'd' } {
  if (m === 0) return { value: 0, unit: 'h' }
  if (m % (60 * 24) === 0) return { value: m / (60 * 24), unit: 'd' }
  if (m % 60 === 0) return { value: m / 60, unit: 'h' }
  return { value: m, unit: 'min' }
}

function partsToMinutes(value: number, unit: 'min' | 'h' | 'd'): number {
  if (unit === 'd') return value * 60 * 24
  if (unit === 'h') return value * 60
  return value
}

// ── sub-components ─────────────────────────────────────────────────────────

interface DurationInputProps {
  value: number
  unit: 'min' | 'h' | 'd'
  onValueChange: (v: number) => void
  onUnitChange: (u: 'min' | 'h' | 'd') => void
}

function DurationInput({ value, unit, onValueChange, onUnitChange }: DurationInputProps) {
  return (
    <div className="flex items-center gap-1">
      <input
        type="number"
        min={0}
        className="w-16 bg-gray-700 text-gray-200 text-sm rounded px-2 py-1 border border-gray-600 focus:outline-none focus:border-blue-500"
        value={value}
        onChange={e => onValueChange(Number(e.target.value))}
      />
      <select
        className="bg-gray-700 text-gray-200 text-sm rounded px-2 py-1 border border-gray-600"
        value={unit}
        onChange={e => onUnitChange(e.target.value as 'min' | 'h' | 'd')}
      >
        <option value="min">min</option>
        <option value="h">h</option>
        <option value="d">d</option>
      </select>
    </div>
  )
}

// ── types ─────────────────────────────────────────────────────────────────────

interface Drive {
  id: string
  name: string
  type: string
  endpoint: string
  bucket: string
  region: string
  prefix: string
}

interface RetentionConfig {
  category: string
  action: string
  drive_id: string
}

interface StorageOverrides {
  withMotionValue?: number
  withMotionUnit?: 'min' | 'h' | 'd'
  withoutMotionValue?: number
  withoutMotionUnit?: 'min' | 'h' | 'd'
  intervalValue?: number
  intervalUnit?: 'min' | 'h' | 'd'
  maxSizeGB?: number
  warnPercent?: number
}

const emptyDriveForm = () => ({
  name: '', endpoint: '', bucket: '', region: '',
  access_key: '', secret_key: '', prefix: '',
})

// ── component ────────────────────────────────────────────────────────────────

export default function StorageSettingsPage() {
  const isAdmin = getRole() === 'admin'
  const { settings, reload } = useSettings('/settings/storage')
  const s = settings?.storage

  const [drives, setDrives] = useState<Drive[]>([])
  const [retention, setRetention] = useState<RetentionConfig[]>([])
  // Local user edits overlay the server-provided values.
  const [overrides, setOverrides] = useState<StorageOverrides>({})
  const [storageSaving, setStorageSaving] = useState(false)
  const [storageSaved, setStorageSaved] = useState(false)

  const [showDriveForm, setShowDriveForm] = useState(false)
  const [editDrive, setEditDrive] = useState<Drive | null>(null)
  const [driveForm, setDriveForm] = useState(emptyDriveForm())
  const [driveSaving, setDriveSaving] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState<Drive | null>(null)

  const loadDrives = () =>
    fetch('/api/drives', { headers: authHeaders() })
      .then(r => r.json()).then(d => setDrives(d ?? [])).catch(() => {})

  const loadRetention = () =>
    fetch('/api/retention', { headers: authHeaders() })
      .then(r => r.json()).then(d => setRetention(d ?? [])).catch(() => {})

  useEffect(() => {
    if (!isAdmin) return
    loadDrives(); loadRetention()
  }, [isAdmin])

  // Derive current form values: server values merged with local overrides.
  const form = s ? (() => {
    const wm = minutesToParts(s.with_motion_minutes)
    const wom = minutesToParts(s.without_motion_minutes)
    const iv = minutesToParts(s.interval_minutes === 0 ? 60 : s.interval_minutes)
    return {
      withMotionValue:    overrides.withMotionValue    ?? wm.value,
      withMotionUnit:     overrides.withMotionUnit     ?? wm.unit,
      withoutMotionValue: overrides.withoutMotionValue ?? wom.value,
      withoutMotionUnit:  overrides.withoutMotionUnit  ?? wom.unit,
      intervalValue:      overrides.intervalValue      ?? iv.value,
      intervalUnit:       overrides.intervalUnit       ?? iv.unit,
      maxSizeGB:          overrides.maxSizeGB          ?? s.max_size_gb,
      warnPercent:        overrides.warnPercent        ?? s.warn_percent,
    }
  })() : null

  const set = (patch: StorageOverrides) => { setOverrides(o => ({ ...o, ...patch })); setStorageSaved(false) }

  const retentionFor = (category: string): RetentionConfig =>
    retention.find(r => r.category === category) ?? { category, action: 'delete', drive_id: '' }

  const handleRetentionChange = (category: string, action: string, driveId: string) =>
    fetch(`/api/retention/${category}`, {
      method: 'PUT',
      headers: { ...authHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify({ action, drive_id: driveId }),
    }).then(() => loadRetention()).catch(() => {})

  const handleStorageSave = () => {
    if (!form) return
    setStorageSaving(true); setStorageSaved(false)
    fetch('/api/settings/storage', {
      method: 'PUT',
      headers: { ...authHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify({
        with_motion_minutes:    partsToMinutes(form.withMotionValue, form.withMotionUnit),
        without_motion_minutes: partsToMinutes(form.withoutMotionValue, form.withoutMotionUnit),
        interval_minutes:       partsToMinutes(form.intervalValue, form.intervalUnit),
        max_size_gb:  form.maxSizeGB,
        warn_percent: form.warnPercent,
      }),
    })
      .then(() => { setOverrides({}); reload(); setStorageSaved(true) })
      .catch(() => {})
      .finally(() => setStorageSaving(false))
  }

  // ── drive CRUD ──────────────────────────────────────────────────────────────

  const openCreateDrive = () => { setEditDrive(null); setDriveForm(emptyDriveForm()); setShowDriveForm(true) }
  const openEditDrive = (dr: Drive) => {
    setEditDrive(dr)
    setDriveForm({ name: dr.name, endpoint: dr.endpoint, bucket: dr.bucket, region: dr.region, access_key: '', secret_key: '', prefix: dr.prefix })
    setShowDriveForm(true)
  }

  const handleDriveSave = () => {
    setDriveSaving(true)
    const method = editDrive ? 'PUT' : 'POST'
    const url = editDrive ? `/api/drives/${editDrive.id}` : '/api/drives'
    const body: Record<string, string> = { name: driveForm.name, type: 's3', endpoint: driveForm.endpoint, bucket: driveForm.bucket, region: driveForm.region, prefix: driveForm.prefix }
    if (driveForm.access_key) body.access_key = driveForm.access_key
    if (driveForm.secret_key) body.secret_key = driveForm.secret_key
    if (!editDrive) { body.access_key = driveForm.access_key; body.secret_key = driveForm.secret_key }
    fetch(url, { method, headers: { ...authHeaders(), 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
      .then(res => { if (res.ok) { setShowDriveForm(false); loadDrives() } })
      .catch(() => {})
      .finally(() => setDriveSaving(false))
  }

  const handleDriveDelete = (dr: Drive) =>
    fetch(`/api/drives/${dr.id}`, { method: 'DELETE', headers: authHeaders() })
      .then(res => { if (res.ok) { setConfirmDelete(null); loadDrives(); loadRetention() } })
      .catch(() => {})

  // ── render ──────────────────────────────────────────────────────────────────

  if (!isAdmin) {
    return (
      <SettingsLayout>
        <h3 className="text-lg font-semibold text-gray-200">Armazenamento</h3>
        <p className="text-sm text-gray-500 mt-1 mb-6">Retenção, limpeza automática e espaço em disco.</p>
        <p className="text-gray-500 text-sm">Acesso restrito.</p>
      </SettingsLayout>
    )
  }

  return (
    <SettingsLayout>
      <h3 className="text-lg font-semibold text-gray-200">Armazenamento</h3>
      <p className="text-sm text-gray-500 mt-1 mb-6">Retenção, limpeza automática e espaço em disco.</p>

      {s && (
        <div className="bg-gray-800 rounded-lg px-4 py-3 mb-4 flex items-center gap-3">
          <span className="text-xs text-gray-500 w-28 shrink-0">Diretório</span>
          <span className="text-sm text-gray-300">{s.path || '—'}</span>
        </div>
      )}

      {form ? (
        <div className="space-y-2 mb-4">
          {/* Retention rows */}
          {([
            { label: 'Com movimento',  vk: 'withMotionValue',    uk: 'withMotionUnit',    cat: 'with_motion' },
            { label: 'Sem movimento',  vk: 'withoutMotionValue', uk: 'withoutMotionUnit', cat: 'without_motion' },
          ] as const).map(({ label, vk, uk, cat }) => {
            const rc = retentionFor(cat)
            return (
              <div key={cat} className="flex items-center gap-3 bg-gray-800 rounded-lg px-4 py-3 flex-wrap">
                <span className="text-sm text-gray-400 w-28 shrink-0">{label}</span>
                <DurationInput
                  value={form[vk]}
                  unit={form[uk]}
                  onValueChange={v => set({ [vk]: v })}
                  onUnitChange={u => set({ [uk]: u })}
                />
                <span className="text-xs text-gray-500 mx-1">→ ao expirar</span>
                <select
                  className="bg-gray-700 text-gray-200 text-sm rounded px-2 py-1 border border-gray-600"
                  value={rc.action}
                  onChange={e => handleRetentionChange(cat, e.target.value, e.target.value === 'delete' ? '' : (rc.drive_id || drives[0]?.id || ''))}
                >
                  <option value="delete">Apagar</option>
                  <option value="send_to_drive" disabled={drives.length === 0}>Enviar para drive</option>
                </select>
                {rc.action === 'send_to_drive' && (
                  <select
                    className="bg-gray-700 text-gray-200 text-sm rounded px-2 py-1 border border-gray-600"
                    value={rc.drive_id}
                    onChange={e => handleRetentionChange(cat, 'send_to_drive', e.target.value)}
                  >
                    <option value="">Selecionar drive...</option>
                    {drives.map(dr => <option key={dr.id} value={dr.id}>{dr.name}</option>)}
                  </select>
                )}
              </div>
            )
          })}

          <div className="flex items-center gap-3 bg-gray-800 rounded-lg px-4 py-3">
            <span className="text-sm text-gray-400 w-28 shrink-0">Intervalo cleaner</span>
            <DurationInput
              value={form.intervalValue} unit={form.intervalUnit}
              onValueChange={v => set({ intervalValue: v })}
              onUnitChange={u => set({ intervalUnit: u })}
            />
          </div>

          <div className="flex items-center gap-3 bg-gray-800 rounded-lg px-4 py-3">
            <span className="text-sm text-gray-400 w-28 shrink-0">Máximo (GB)</span>
            <input type="number" min={0} step={0.1}
              className="w-20 bg-gray-700 text-gray-200 text-sm rounded px-2 py-1 border border-gray-600 focus:outline-none focus:border-blue-500"
              value={form.maxSizeGB}
              onChange={e => set({ maxSizeGB: Number(e.target.value) })}
            />
            <span className="text-xs text-gray-500">0 = desativado</span>
          </div>

          <div className="flex items-center gap-3 bg-gray-800 rounded-lg px-4 py-3">
            <span className="text-sm text-gray-400 w-28 shrink-0">Alerta (%)</span>
            <input type="number" min={0} max={100}
              className="w-20 bg-gray-700 text-gray-200 text-sm rounded px-2 py-1 border border-gray-600 focus:outline-none focus:border-blue-500"
              value={form.warnPercent}
              onChange={e => set({ warnPercent: Number(e.target.value) })}
            />
          </div>

          <div className="flex justify-end items-center gap-3 pt-1">
            {storageSaved && <span className="text-xs text-green-400">Salvo</span>}
            <button onClick={handleStorageSave} disabled={storageSaving}
              className="text-sm bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white px-4 py-2 rounded">
              {storageSaving ? 'Salvando...' : 'Salvar'}
            </button>
          </div>
        </div>
      ) : (
        <p className="text-gray-500 text-sm mb-4">Carregando...</p>
      )}

      {drives.length === 0 && retention.some(r => r.action === 'send_to_drive') && (
        <p className="text-xs text-amber-400 mb-4">Nenhum drive configurado — gravações com essa ação serão ignoradas pelo cleaner.</p>
      )}

      {/* Drives section */}
      <div className="mt-6">
        <div className="flex items-center justify-between mb-3">
          <h4 className="text-sm font-semibold text-gray-300">Drives</h4>
          <button onClick={openCreateDrive}
            className="text-xs bg-blue-600 hover:bg-blue-500 text-white px-3 py-1.5 rounded">
            + Adicionar drive
          </button>
        </div>

        {drives.length === 0 ? (
          <p className="text-sm text-gray-500">Nenhum drive configurado.</p>
        ) : (
          <div className="space-y-2">
            {drives.map(dr => (
              <div key={dr.id} className="flex items-center justify-between bg-gray-800 rounded-lg px-4 py-3">
                <div>
                  <span className="text-sm font-medium text-gray-200">{dr.name}</span>
                  <span className="ml-2 text-xs text-gray-500 uppercase">{dr.type}</span>
                  <p className="text-xs text-gray-500 mt-0.5">
                    {dr.bucket}{dr.endpoint ? ` · ${dr.endpoint}` : ''}{dr.prefix ? ` · /${dr.prefix}` : ''}
                  </p>
                </div>
                <div className="flex gap-2">
                  <button onClick={() => openEditDrive(dr)} className="text-xs text-gray-400 hover:text-gray-200 px-2 py-1">Editar</button>
                  <button onClick={() => setConfirmDelete(dr)} className="text-xs text-red-400 hover:text-red-300 px-2 py-1">Excluir</button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Drive form modal */}
      {showDriveForm && (
        <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
          <div className="bg-gray-900 rounded-xl p-6 w-full max-w-md border border-gray-700 shadow-xl">
            <h3 className="text-base font-semibold text-gray-200 mb-4">{editDrive ? 'Editar drive' : 'Novo drive S3'}</h3>
            <div className="space-y-3">
              {([
                { label: 'Nome', field: 'name', required: true },
                { label: 'Endpoint (opcional)', field: 'endpoint', placeholder: 'https://s3.amazonaws.com' },
                { label: 'Bucket', field: 'bucket', required: true },
                { label: 'Região', field: 'region', placeholder: 'us-east-1' },
                { label: 'Access Key', field: 'access_key', required: !editDrive, placeholder: editDrive ? '(manter atual)' : '' },
                { label: 'Secret Key', field: 'secret_key', required: !editDrive, placeholder: editDrive ? '(manter atual)' : '', password: true },
                { label: 'Prefixo (opcional)', field: 'prefix' },
              ] as Array<{ label: string; field: keyof typeof driveForm; required?: boolean; placeholder?: string; password?: boolean }>).map(({ label, field, required, placeholder, password }) => (
                <div key={field}>
                  <label className="block text-xs text-gray-400 mb-1">
                    {label}{required && <span className="text-red-400 ml-0.5">*</span>}
                  </label>
                  <input
                    type={password ? 'password' : 'text'}
                    className="w-full bg-gray-800 text-gray-200 text-sm rounded px-3 py-1.5 border border-gray-600 focus:outline-none focus:border-blue-500"
                    value={driveForm[field]}
                    placeholder={placeholder}
                    onChange={e => setDriveForm(f => ({ ...f, [field]: e.target.value }))}
                  />
                </div>
              ))}
            </div>
            <div className="flex justify-end gap-2 mt-5">
              <button onClick={() => setShowDriveForm(false)} className="text-sm text-gray-400 hover:text-gray-200 px-4 py-2">Cancelar</button>
              <button
                onClick={handleDriveSave}
                disabled={driveSaving || !driveForm.name || !driveForm.bucket || (!editDrive && (!driveForm.access_key || !driveForm.secret_key))}
                className="text-sm bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white px-4 py-2 rounded">
                {driveSaving ? 'Salvando...' : 'Salvar'}
              </button>
            </div>
          </div>
        </div>
      )}

      <ConfirmDialog
        open={confirmDelete !== null}
        title="Excluir drive"
        message={confirmDelete ? `Excluir o drive "${confirmDelete.name}"? Gravações associadas voltarão a ser apagadas.` : ''}
        onConfirm={() => confirmDelete && handleDriveDelete(confirmDelete)}
        onCancel={() => setConfirmDelete(null)}
        danger
      />
    </SettingsLayout>
  )
}
