import { useEffect, useState } from 'react'
import AppLayout from '../components/AppLayout'
import { authHeaders, onUnauthorized } from '../auth'
import { categoryBuckets, type EventReport } from './reportsUtils'

const RANGES = [7, 30, 90]
const CAT_LABEL: Record<string, string> = { movimento: 'Movimento', pessoa: 'Pessoa', ia: 'IA', estados: 'Estados' }
const CAT_COLOR: Record<string, string> = { movimento: 'bg-amber-400', pessoa: 'bg-red-500', ia: 'bg-violet-500', estados: 'bg-green-500' }

export default function ReportsPage() {
  const [report, setReport] = useState<EventReport | null>(null)
  const [days, setDays] = useState(7)

  useEffect(() => {
    const to = new Date()
    const from = new Date(to.getTime() - days * 86_400_000)
    fetch(`/api/reports/events?from=${from.toISOString()}&to=${to.toISOString()}`, { headers: authHeaders() })
      .then(r => { if (r.status === 401) { onUnauthorized(); return null } return r.json() })
      .then(d => { if (d) setReport(d) })
      .catch(() => {})
  }, [days])

  const byDay = report?.by_day ?? []
  const maxDay = Math.max(1, ...byDay.map(d => d.count))
  const cats = report ? categoryBuckets(report.by_label) : { movimento: 0, pessoa: 0, ia: 0, estados: 0 }
  const catEntries = Object.entries(cats).filter(([, n]) => n > 0)
  const catTotal = catEntries.reduce((s, [, n]) => s + n, 0) || 1
  const byCamera = report ? Object.entries(report.by_camera).sort((a, b) => b[1] - a[1]) : []
  const maxCam = Math.max(1, ...byCamera.map(([, n]) => n))

  // donut (categorias)
  const R = 54
  const CIRC = 2 * Math.PI * R
  const HEXBG = '#ffffff'
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
      <div className="flex items-end justify-between mb-6">
        <div>
          <h2 className="text-2xl font-bold text-foreground">Relatórios</h2>
          <p className="text-sm text-muted mt-1">Estatísticas de eventos — {report?.total ?? 0} no período.</p>
        </div>
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

      {/* Gráfico de barras (SVG) — eventos por dia */}
      <div className="bg-surface border border-border rounded-lg p-4 mb-4">
        <p className="text-xs font-medium text-faint uppercase tracking-wider mb-3">Eventos por dia</p>
        {byDay.length === 0 ? (
          <p className="text-sm text-muted">Sem eventos no período.</p>
        ) : (
          <svg id="report-bars" viewBox={`0 0 ${byDay.length * 10} 100`} preserveAspectRatio="none" className="w-full h-40">
            <line x1="0" y1="99.5" x2={byDay.length * 10} y2="99.5" stroke="currentColor" strokeWidth="0.3" className="text-border" />
            {byDay.map((d, i) => {
              const h = (d.count / maxDay) * 92
              return (
                <rect key={d.day} x={i * 10 + 1.5} y={99 - h} width={7} height={h} rx={1} className="fill-primary">
                  <title>{`${d.day}: ${d.count}`}</title>
                </rect>
              )
            })}
          </svg>
        )}
        <div className="flex justify-between mt-1 text-[9px] text-faint tabular-nums">
          <span>{byDay[0]?.day.slice(5)}</span>
          <span>{byDay[byDay.length - 1]?.day.slice(5)}</span>
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        {/* Donut por categoria */}
        <div className="bg-surface border border-border rounded-lg p-4">
          <p className="text-xs font-medium text-faint uppercase tracking-wider mb-3">Por categoria</p>
          {segments.length === 0 ? (
            <p className="text-sm text-muted">—</p>
          ) : (
            <div className="flex items-center gap-4">
              <svg viewBox="0 0 140 140" className="w-32 h-32 shrink-0 -rotate-90">
                <circle cx="70" cy="70" r={R} fill="none" stroke={HEXBG} strokeOpacity="0.06" strokeWidth="18" />
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

        {/* Barras horizontais por câmera */}
        <div className="bg-surface border border-border rounded-lg p-4">
          <p className="text-xs font-medium text-faint uppercase tracking-wider mb-3">Por câmera</p>
          {byCamera.length === 0 && <p className="text-sm text-muted">—</p>}
          {byCamera.map(([cam, n]) => (
            <div key={cam} className="mb-2">
              <div className="flex items-center justify-between text-xs mb-0.5">
                <span className="text-foreground truncate">{cam}</span>
                <span className="text-muted tabular-nums shrink-0 ml-2">{n}</span>
              </div>
              <div className="h-2 rounded-full bg-surface-2 overflow-hidden">
                <div className="h-full bg-primary rounded-full" style={{ width: `${(n / maxCam) * 100}%` }} />
              </div>
            </div>
          ))}
        </div>
      </div>
    </AppLayout>
  )
}
