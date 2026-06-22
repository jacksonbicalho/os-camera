import { useEffect, useState } from 'react'
import AppLayout from '../components/AppLayout'
import { authHeaders, onUnauthorized } from '../auth'
import { categoryBuckets, axisTicks, type EventReport } from './reportsUtils'

const RANGES = [7, 30, 90]
const CAT_LABEL: Record<string, string> = { movimento: 'Movimento', pessoa: 'Pessoa', ia: 'IA', estados: 'Estados' }
const CAT_COLOR: Record<string, string> = { movimento: 'bg-amber-400', pessoa: 'bg-red-500', ia: 'bg-violet-500', estados: 'bg-green-500' }
// Ordem de empilhamento das barras (de baixo p/ cima).
const STACK_ORDER = ['movimento', 'pessoa', 'ia', 'estados'] as const

interface CameraOption { id: string; name: string }

export default function ReportsPage() {
  const [report, setReport] = useState<EventReport | null>(null)
  const [days, setDays] = useState(7)
  const [cameras, setCameras] = useState<CameraOption[]>([])
  const [camera, setCamera] = useState('')

  // Lista de câmeras do usuário → popula o seletor. Inicia na primeira (o relatório é
  // sempre de uma câmera; não há modo "Todas").
  useEffect(() => {
    fetch('/api/cameras', { headers: authHeaders() })
      .then(r => { if (r.status === 401) { onUnauthorized(); return null } return r.json() })
      .then((list: CameraOption[] | null) => {
        if (!list) return
        setCameras(list)
        if (list.length > 0) setCamera(c => c || list[0].id)
      })
      .catch(() => {})
  }, [])

  useEffect(() => {
    if (!camera) return
    const to = new Date()
    const from = new Date(to.getTime() - days * 86_400_000)
    fetch(`/api/reports/events?from=${from.toISOString()}&to=${to.toISOString()}&camera=${encodeURIComponent(camera)}`, { headers: authHeaders() })
      .then(r => { if (r.status === 401) { onUnauthorized(); return null } return r.json() })
      .then(d => { if (d) setReport(d) })
      .catch(() => {})
  }, [days, camera])

  const byDay = report?.by_day ?? []
  const maxDay = Math.max(1, ...byDay.map(d => d.count))
  const cats = report ? categoryBuckets(report.by_label, report.by_category) : { movimento: 0, pessoa: 0, ia: 0, estados: 0 }
  const catEntries = Object.entries(cats).filter(([, n]) => n > 0)
  const catTotal = catEntries.reduce((s, [, n]) => s + n, 0) || 1

  // donut (categorias)
  const R = 54
  const CIRC = 2 * Math.PI * R
  let acc = 0
  const HEX: Record<string, string> = { movimento: '#fbbf24', pessoa: '#ef4444', ia: '#8b5cf6', estados: '#22c55e' }
  const segments = catEntries.map(([cat, n]) => {
    const len = (n / catTotal) * CIRC
    const seg = { cat, n, len, offset: acc }
    acc += len
    return seg
  })

  return (
    <AppLayout>
      <div className="flex items-end justify-between mb-6 gap-4">
        <div>
          <h2 className="text-2xl font-bold text-foreground">Relatórios</h2>
          <p className="text-sm text-muted mt-1">Estatísticas de eventos — {report?.total ?? 0} no período.</p>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <select
            id="report-camera-select"
            value={camera}
            onChange={e => setCamera(e.target.value)}
            disabled={cameras.length <= 1}
            className="bg-surface-2 text-foreground text-xs rounded px-2 py-1 border border-border max-w-44 disabled:opacity-70"
          >
            {cameras.map(c => (
              <option key={c.id} value={c.id}>{c.name}</option>
            ))}
          </select>
          <div className="flex gap-1">
            {RANGES.map(r => (
              <button
                key={r}
                id={`report-range-${r}`}
                onClick={() => setDays(r)}
                className={`px-2.5 py-1 rounded text-xs transition-colors ${r === days ? 'bg-primary text-primary-foreground' : 'bg-surface-2 text-muted hover:text-foreground'}`}
              >
                {r}d
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Barras (div flex) — eventos por dia da câmera selecionada */}
      <div className="bg-surface border border-border rounded-lg p-4 mb-4">
        <p className="text-xs font-medium text-faint uppercase tracking-wider mb-3">Eventos por dia</p>
        {byDay.length === 0 ? (
          <p className="text-sm text-muted">Sem eventos no período.</p>
        ) : (
          <>
            <div id="report-bars" className="flex items-end gap-1 h-40 border-b border-border">
              {byDay.map(d => {
                const bc = d.by_category ?? {}
                const title = `${d.day} — ` + STACK_ORDER
                  .filter(c => (bc[c] ?? 0) > 0)
                  .map(c => `${CAT_LABEL[c]}: ${bc[c]}`)
                  .join(', ')
                return (
                  <div key={d.day} className="flex-1 flex flex-col-reverse h-full rounded-t overflow-hidden" title={title || `${d.day}: ${d.count}`}>
                    {STACK_ORDER.map(c => {
                      const n = bc[c] ?? 0
                      if (n === 0) return null
                      return (
                        <div
                          key={c}
                          className={`w-full ${CAT_COLOR[c]} min-h-[2px]`}
                          style={{ height: `${(n / maxDay) * 100}%` }}
                        />
                      )
                    })}
                  </div>
                )
              })}
            </div>
            <div className="relative h-3 mt-1 text-[9px] text-faint tabular-nums">
              {axisTicks(byDay.map(d => d.day), 6).map(t => (
                <span
                  key={t.index}
                  className="absolute -translate-x-1/2 whitespace-nowrap"
                  style={{ left: `${((t.index + 0.5) / byDay.length) * 100}%` }}
                >
                  {t.label}
                </span>
              ))}
            </div>
          </>
        )}
      </div>

      {/* Donut por categoria */}
      <div className="bg-surface border border-border rounded-lg p-4 max-w-md">
        <p className="text-xs font-medium text-faint uppercase tracking-wider mb-3">Por categoria</p>
        {segments.length === 0 ? (
          <p className="text-sm text-muted">—</p>
        ) : (
          <div className="flex items-center gap-4">
            <svg viewBox="0 0 140 140" className="w-32 h-32 shrink-0 -rotate-90">
              <circle cx="70" cy="70" r={R} fill="none" stroke="#ffffff" strokeOpacity="0.06" strokeWidth="18" />
              {segments.map(s => (
                <circle
                  key={s.cat}
                  cx="70" cy="70" r={R} fill="none"
                  stroke={HEX[s.cat]} strokeWidth="18"
                  strokeDasharray={`${s.len} ${CIRC - s.len}`}
                  strokeDashoffset={-s.offset}
                >
                  <title>{`${CAT_LABEL[s.cat]}: ${s.n}`}</title>
                </circle>
              ))}
            </svg>
            <div className="flex-1 min-w-0">
              {Object.entries(cats).map(([cat, n]) => (
                <div key={cat} className="flex items-center gap-2 text-sm mb-1.5">
                  <span className={`w-2.5 h-2.5 rounded-full ${CAT_COLOR[cat]}`} />
                  <span className="text-foreground flex-1">{CAT_LABEL[cat]}</span>
                  <span className="text-muted tabular-nums">{n}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </AppLayout>
  )
}
