import { useEffect, useState } from 'react'
import { format } from 'date-fns'
import AppLayout from '../components/AppLayout'
import DatePicker from '../components/DatePicker'
import { authHeaders, onUnauthorized } from '../auth'
import { categoryBuckets, axisTicks, categoryDetail, type EventReport } from './reportsUtils'
import type { EventCategory } from './eventCategory'

const RANGE_OPTS = [1, 2, 3, 4, 5, 6, 7, 14, 30, 90]
const CAT_LABEL: Record<string, string> = { movimento: 'Movimento', pessoa: 'Pessoa', ia: 'IA', estados: 'Estados' }
const CAT_COLOR: Record<string, string> = { movimento: 'bg-amber-400', pessoa: 'bg-red-500', ia: 'bg-violet-500', estados: 'bg-green-500' }
const CAT_DESC: Record<string, string> = {
  movimento: 'Movimento detectado por diferença de pixels, sem classificação.',
  pessoa: 'Detecções classificadas como pessoa.',
  ia: 'Detecções de modelos de IA (exceto pessoa).',
  estados: 'Transições de classificadores de estado.',
}
// Ordem de empilhamento das barras (de baixo p/ cima).
const STACK_ORDER = ['movimento', 'pessoa', 'ia', 'estados'] as const

interface CameraOption { id: string; name: string }
interface Bar { key: string; count: number; bc: Record<string, number> }

const WEEKDAY_LABEL = ['Dom', 'Seg', 'Ter', 'Qua', 'Qui', 'Sex', 'Sáb']

export default function ReportsPage() {
  const [report, setReport] = useState<EventReport | null>(null)
  const [heatmap, setHeatmap] = useState<EventReport | null>(null)
  const [days, setDays] = useState(7)
  const [date, setDate] = useState<Date>(new Date())
  const [cameras, setCameras] = useState<CameraOption[]>([])
  const [camera, setCamera] = useState('')
  const [modalCat, setModalCat] = useState<EventCategory | null>(null)

  // Fecha o modal de categoria no Esc.
  useEffect(() => {
    if (!modalCat) return
    const h = (e: KeyboardEvent) => { if (e.key === 'Escape') setModalCat(null) }
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  }, [modalCat])

  // A data = FIM do período; `days` = tamanho da janela. "1 dia" = barras por hora do
  // dia; >1 dia = N dias terminando na data.
  const dayMode = days === 1
  const periodEnd = new Date(date.getFullYear(), date.getMonth(), date.getDate() + 1, 0, 0, 0, 0)
  const periodStart = dayMode
    ? new Date(date.getFullYear(), date.getMonth(), date.getDate(), 0, 0, 0, 0)
    : new Date(periodEnd.getTime() - days * 86_400_000)
  const periodLabel = dayMode
    ? `${WEEKDAY_LABEL[date.getDay()]} ${format(date, 'dd/MM')}`
    : `${format(periodStart, 'dd/MM')} – ${format(date, 'dd/MM')}`

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
    const bucket = dayMode ? '&bucket=hour' : ''
    const url = `/api/reports/events?from=${periodStart.toISOString()}&to=${periodEnd.toISOString()}&camera=${encodeURIComponent(camera)}${bucket}`
    fetch(url, { headers: authHeaders() })
      .then(r => { if (r.status === 401) { onUnauthorized(); return null } return r.json() })
      .then(d => { if (d) setReport(d) })
      .catch(() => {})
    // periodStart/periodEnd derivam de date+days; date/days nas deps cobrem.
  }, [days, camera, date]) // eslint-disable-line react-hooks/exhaustive-deps

  // Heatmap (dia da semana × hora): só faz sentido multi-dia, então usa a MESMA janela
  // do período (N dias terminando na data) e não roda no modo "1 dia".
  useEffect(() => {
    if (!camera || dayMode) return
    const url = `/api/reports/events?from=${periodStart.toISOString()}&to=${periodEnd.toISOString()}&camera=${encodeURIComponent(camera)}&bucket=heatmap`
    fetch(url, { headers: authHeaders() })
      .then(r => { if (r.status === 401) { onUnauthorized(); return null } return r.json() })
      .then(d => { if (d) setHeatmap(d) })
      .catch(() => {})
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [days, camera, date])

  const bars: Bar[] = dayMode
    ? (report?.by_hour ?? []).map(h => ({ key: `${h.hour}`, count: h.count, bc: h.by_category ?? {} }))
    : (report?.by_day ?? []).map(d => ({ key: d.day, count: d.count, bc: d.by_category ?? {} }))
  const maxVal = Math.max(1, ...bars.map(b => b.count))

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

  // Heatmap: uma linha por dia (cronológico). Agrupa as células por data preservando a
  // ordem (o backend já devolve por data asc) e monta a contagem por hora de cada dia.
  const heatCells = heatmap?.heatmap ?? []
  const heatRows: { date: string; hours: number[] }[] = []
  const heatRowIdx = new Map<string, number>()
  for (const c of heatCells) {
    let i = heatRowIdx.get(c.date)
    if (i === undefined) {
      i = heatRows.length
      heatRowIdx.set(c.date, i)
      heatRows.push({ date: c.date, hours: Array(24).fill(0) })
    }
    if (c.hour >= 0 && c.hour < 24) heatRows[i].hours[c.hour] = c.count
  }
  // Exibe do mais recente (topo) ao mais antigo: quando o range excede os dados, os dias
  // com atividade ficam no topo (visíveis) e os dias vazios mais antigos escorrem p/ baixo.
  const heatRowsDesc = [...heatRows].reverse()
  const heatMax = Math.max(1, ...heatCells.map(c => c.count))
  const heatTotal = heatCells.reduce((s, c) => s + c.count, 0)
  // "27/03/2026 Sex" — data dd/mm/yyyy + dia da semana (data local, sem deslocamento de fuso).
  const heatRowLabel = (date: string) => {
    const [y, m, d] = date.split('-').map(Number)
    const dt = new Date(y, m - 1, d)
    return `${format(dt, 'dd/MM/yyyy')} ${WEEKDAY_LABEL[dt.getDay()]}`
  }

  const barTitle = (b: Bar) => {
    const head = dayMode ? `${b.key}h` : b.key
    const parts = STACK_ORDER.filter(c => (b.bc[c] ?? 0) > 0).map(c => `${CAT_LABEL[c]}: ${b.bc[c]}`)
    return parts.length > 0 ? `${head} — ${parts.join(', ')}` : `${head}: ${b.count}`
  }

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
          <DatePicker
            id="report-day-picker"
            value={date}
            onChange={d => setDate(d)}
            disableFuture
            align="right"
          />
          <select
            id="report-range-select"
            value={String(days)}
            onChange={e => setDays(Number(e.target.value))}
            className="bg-surface-2 text-foreground text-xs rounded px-2 py-1 border border-border"
          >
            {RANGE_OPTS.map(n => (
              <option key={n} value={n}>{n === 1 ? '1 dia' : `${n} dias`}</option>
            ))}
          </select>
        </div>
      </div>

      {/* Barras empilhadas — por dia (intervalo) ou por hora (modo dia) */}
      <div className="bg-surface border border-border rounded-lg p-4 mb-4">
        <p className="text-xs font-medium text-faint uppercase tracking-wider mb-3">
          {dayMode ? `Eventos por hora — ${periodLabel}` : `Eventos por dia — ${periodLabel}`}
        </p>
        {bars.length === 0 ? (
          <p className="text-sm text-muted">Sem eventos no período.</p>
        ) : (
          <>
            <div id="report-bars" className="flex items-end gap-1 h-40 border-b border-border">
              {bars.map(b => (
                <div key={b.key} className="flex-1 flex flex-col-reverse h-full" title={barTitle(b)}>
                  {STACK_ORDER.map(c => {
                    const n = b.bc[c] ?? 0
                    if (n === 0) return null
                    return (
                      <button
                        key={c}
                        type="button"
                        onClick={() => setModalCat(c)}
                        className={`w-full ${CAT_COLOR[c]} min-h-[2px] relative cursor-pointer transition-transform duration-150 origin-center hover:scale-110 hover:brightness-110 hover:z-10`}
                        style={{ height: `${(n / maxVal) * 100}%` }}
                        title={`${CAT_LABEL[c]}: ${n}`}
                      />
                    )
                  })}
                </div>
              ))}
            </div>
            <div className="relative h-3 mt-1 text-[9px] text-faint tabular-nums">
              {dayMode
                ? Array.from({ length: 24 }, (_, h) => (
                    <span key={h} className="absolute -translate-x-1/2 whitespace-nowrap" style={{ left: `${((h + 0.5) / 24) * 100}%` }}>
                      {h}
                    </span>
                  ))
                : axisTicks(bars.map(b => b.key), 6).map(t => (
                    <span key={t.index} className="absolute -translate-x-1/2 whitespace-nowrap" style={{ left: `${((t.index + 0.5) / bars.length) * 100}%` }}>
                      {t.label}
                    </span>
                  ))}
            </div>
          </>
        )}
      </div>

      {/* Mapa de atividade — uma linha por dia × hora (só no modo intervalo) */}
      {!dayMode && (
      <div className="bg-surface border border-border rounded-lg p-4 mb-4">
        <p className="text-xs font-medium text-faint uppercase tracking-wider mb-3">
          Mapa de atividade — dia × hora — {periodLabel}
        </p>
        {heatTotal === 0 ? (
          <p className="text-sm text-muted">Sem eventos no período.</p>
        ) : (
          <div id="report-heatmap" className="overflow-x-auto">
            <div className="inline-block min-w-full">
              {/* Cabeçalho de horas (0–23) */}
              <div className="flex pl-28 mb-1">
                {Array.from({ length: 24 }, (_, h) => (
                  <div key={h} className="flex-1 text-[9px] text-faint tabular-nums text-center">
                    {h}
                  </div>
                ))}
              </div>
              <div className="max-h-[420px] overflow-y-auto">
                {heatRowsDesc.map(row => {
                  const label = heatRowLabel(row.date)
                  return (
                    <div key={row.date} id={`report-heatmap-row-${row.date}`} className="flex items-center">
                      <div className="w-28 shrink-0 text-[10px] text-muted pr-1 text-right tabular-nums">{label}</div>
                      {row.hours.map((n, h) => {
                        const intensity = n === 0 ? 0 : 0.18 + 0.82 * (n / heatMax)
                        return (
                          <div
                            key={h}
                            id={`report-heatmap-cell-${row.date}-${h}`}
                            className={`flex-1 h-6 m-px rounded-sm ${n === 0 ? 'bg-surface-2' : 'bg-primary'}`}
                            style={n === 0 ? undefined : { opacity: intensity }}
                            title={`${label} ${String(h).padStart(2, '0')}h — ${n} ${n === 1 ? 'evento' : 'eventos'}`}
                          />
                        )
                      })}
                    </div>
                  )
                })}
              </div>
            </div>
          </div>
        )}
      </div>
      )}

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
                <button
                  key={cat}
                  type="button"
                  onClick={() => setModalCat(cat as EventCategory)}
                  className="w-full flex items-center gap-2 text-sm mb-1.5 hover:bg-surface-2 rounded px-1 -mx-1 transition-colors"
                >
                  <span className={`w-2.5 h-2.5 rounded-full ${CAT_COLOR[cat]}`} />
                  <span className="text-foreground flex-1 text-left">{CAT_LABEL[cat]}</span>
                  <span className="text-muted tabular-nums">{n}</span>
                </button>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Modal de detalhe da categoria (clique no segmento ou na legenda) */}
      {modalCat && (() => {
        const det = categoryDetail(modalCat, report?.by_label ?? {}, report?.by_category)
        return (
          <div
            className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
            onClick={() => setModalCat(null)}
          >
            <div
              id="report-category-modal"
              className="bg-surface border border-border rounded-lg p-5 w-full max-w-sm"
              onClick={e => e.stopPropagation()}
            >
              <div className="flex items-center gap-2 mb-2">
                <span className={`w-3 h-3 rounded-full shrink-0 ${CAT_COLOR[modalCat]}`} />
                <h3 className="text-base font-semibold text-foreground flex-1">
                  {CAT_LABEL[modalCat]} — {det.total} {det.total === 1 ? 'evento' : 'eventos'}
                </h3>
                <button onClick={() => setModalCat(null)} className="text-faint hover:text-foreground" aria-label="Fechar">✕</button>
              </div>
              <p className="text-sm text-muted mb-3">{CAT_DESC[modalCat]}</p>
              {det.labels.length > 0 && (
                <>
                  <p className="text-xs font-medium text-faint uppercase tracking-wider mb-1">Por label</p>
                  <ul className="text-sm max-h-60 overflow-y-auto">
                    {det.labels.map(l => (
                      <li key={l.label} className="flex justify-between py-0.5">
                        <span className="text-foreground truncate">{l.label}</span>
                        <span className="text-muted tabular-nums ml-2 shrink-0">{l.count}</span>
                      </li>
                    ))}
                  </ul>
                </>
              )}
            </div>
          </div>
        )
      })()}
    </AppLayout>
  )
}
