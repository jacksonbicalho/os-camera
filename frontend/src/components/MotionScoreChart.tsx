import { useCallback, useEffect, useRef, useState } from 'react'
import { useEventSource } from '../hooks/useEventSource'

interface Sample {
  time: number  // Date.now()
  score: number
}

interface Props {
  cameraId: string
  threshold: number
  windowMs?: number  // visible time window; default 30s
}

const DEFAULT_WINDOW = 30_000
const LOG_MIN = -4   // log10(0.0001)
const LOG_MAX = 0    // log10(1.0)
const Y_TICKS = [0.0001, 0.001, 0.01, 0.1, 1.0]

export default function MotionScoreChart({ cameraId, threshold, windowMs = DEFAULT_WINDOW }: Props) {
  const [samples, setSamples] = useState<Sample[]>([])
  const [tick, setTick] = useState(() => Date.now())
  const windowMsRef = useRef(windowMs)
  useEffect(() => { windowMsRef.current = windowMs }, [windowMs])

  const handleScoreMessage = useCallback((data: string) => {
    const ev = JSON.parse(data) as { score: number }
    const now = Date.now()
    setSamples(prev => {
      const cutoff = now - windowMsRef.current
      return [...prev.filter(s => s.time >= cutoff), { time: now, score: ev.score }]
    })
  }, [])

  useEventSource(`/api/cameras/${cameraId}/motion/scores`, handleScoreMessage)

  // Redraw tick + prune old samples
  useEffect(() => {
    const id = setInterval(() => {
      const now = Date.now()
      setTick(now)
      setSamples(prev => prev.filter(s => s.time >= now - windowMs))
    }, 200)
    return () => clearInterval(id)
  }, [windowMs])

  const width = 600
  const height = 160
  const padLeft = 44
  const padRight = 8
  const padTop = 8
  const padBottom = 20
  const chartW = width - padLeft - padRight
  const chartH = height - padTop - padBottom

  const now = tick
  const timeMin = now - windowMs

  function toLogY(score: number): number {
    if (score <= 0) return padTop + chartH
    const logVal = Math.max(LOG_MIN, Math.min(LOG_MAX, Math.log10(score)))
    return padTop + chartH - ((logVal - LOG_MIN) / (LOG_MAX - LOG_MIN)) * chartH
  }

  function toX(time: number): number {
    return padLeft + ((time - timeMin) / windowMs) * chartW
  }

  function formatScore(v: number): string {
    if (v === 0) return '0'
    if (v >= 1) return v.toFixed(2)
    const decimals = Math.max(2, -Math.floor(Math.log10(v)) + 1)
    return v.toFixed(decimals)
  }

  const points = samples.map(s => `${toX(s.time).toFixed(1)},${toLogY(s.score).toFixed(1)}`).join(' ')
  const thresholdY = toLogY(threshold)
  const lastSample = samples.length > 0 ? samples[samples.length - 1] : null

  return (
    <svg viewBox={`0 0 ${width} ${height}`} className="w-full h-auto" style={{ minHeight: 100 }}>
      {/* background */}
      <rect x={padLeft} y={padTop} width={chartW} height={chartH} fill="#111827" rx="4" />

      {/* y-axis grid lines at each tick */}
      {Y_TICKS.map(v => (
        <line
          key={v}
          x1={padLeft} y1={toLogY(v).toFixed(1)}
          x2={padLeft + chartW} y2={toLogY(v).toFixed(1)}
          stroke="#374151" strokeWidth="0.5"
        />
      ))}

      {/* threshold line */}
      <line
        x1={padLeft} y1={thresholdY.toFixed(1)}
        x2={padLeft + chartW} y2={thresholdY.toFixed(1)}
        stroke="#ef4444" strokeWidth="1.5" strokeDasharray="4 3"
      />
      <text x={padLeft + chartW - 2} y={thresholdY - 3} fill="#ef4444" fontSize="9" textAnchor="end">
        limiar {formatScore(threshold)}
      </text>

      {/* live score reference line */}
      {lastSample && (
        <line
          x1={padLeft} y1={toLogY(lastSample.score).toFixed(1)}
          x2={padLeft + chartW} y2={toLogY(lastSample.score).toFixed(1)}
          stroke="#60a5fa" strokeWidth="1" strokeDasharray="2 2" opacity="0.4"
        />
      )}

      {/* score polyline */}
      {samples.length > 1 && (
        <polyline points={points} fill="none" stroke="#60a5fa" strokeWidth="1.5" strokeLinejoin="round" />
      )}

      {/* latest score dot */}
      {lastSample && (
        <circle cx={toX(lastSample.time)} cy={toLogY(lastSample.score)} r="3" fill="#60a5fa" />
      )}

      {/* y-axis labels */}
      {Y_TICKS.map(v => (
        <text key={v} x={padLeft - 3} y={toLogY(v) + 3} fill="#9ca3af" fontSize="8" textAnchor="end">
          {formatScore(v)}
        </text>
      ))}

      {/* live score label */}
      {lastSample && (
        <text x={padLeft + 4} y={padTop + 10} fill="#60a5fa" fontSize="9" textAnchor="start">
          score: {formatScore(lastSample.score)}
        </text>
      )}

      {/* x-axis label */}
      <text x={padLeft + chartW / 2} y={height - 4} fill="#6b7280" fontSize="8" textAnchor="middle">
        últimos {windowMs / 1000}s
      </text>

      {/* "waiting" label when no data yet */}
      {samples.length === 0 && (
        <text x={padLeft + chartW / 2} y={padTop + chartH / 2 + 4} fill="#4b5563" fontSize="10" textAnchor="middle">
          aguardando frames…
        </text>
      )}
    </svg>
  )
}
