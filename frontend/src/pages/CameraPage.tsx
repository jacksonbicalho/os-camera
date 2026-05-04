import { useEffect, useState, useRef } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { DayPicker } from 'react-day-picker'
import { format } from 'date-fns'
import { ptBR } from 'date-fns/locale'
import 'react-day-picker/style.css'
import { authHeaders, clearToken, getToken, getUsername } from '../auth'
import Header from '../components/Header'
import HLSPlayer from '../components/HLSPlayer'
import ListPanel from '../components/ListPanel'
import { useScrollToPlayer } from '../hooks/useScrollToPlayer'

interface Recording {
  filename: string
  start: string
  url: string
  is_recording: boolean
}

interface RecordingsResponse {
  recordings: Recording[]
  hasMore: boolean
}

interface MotionEvent {
  time: string
  score: number
}

const PAGE_SIZE = 10

async function loadRecordingsData(cameraId: string, date: Date, page: number, order: 'asc' | 'desc'): Promise<RecordingsResponse | 401> {
  const dateStr = format(date, 'yyyy-MM-dd')
  const res = await fetch(
    `/api/cameras/${cameraId}/recordings?date=${dateStr}&page=${page}&limit=${PAGE_SIZE}&order=${order}`,
    { headers: authHeaders() }
  )
  if (res.status === 401) return 401
  return res.json()
}

async function loadMotionEvents(cameraId: string, date: Date): Promise<MotionEvent[]> {
  const dateStr = format(date, 'yyyy-MM-dd')
  const res = await fetch(`/api/cameras/${cameraId}/motion?date=${dateStr}`, { headers: authHeaders() })
  if (!res.ok) return []
  const data = await res.json()
  return data.events ?? []
}

function formatRecordingTime(isoString: string, timezone: string): string {
  return new Date(isoString).toLocaleTimeString([], {
    timeZone: timezone,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  })
}

export default function CameraPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [timezone, setTimezone] = useState('UTC')
  const [selectedDate, setSelectedDate] = useState<Date>(new Date())
  const [recordings, setRecordings] = useState<Recording[]>([])
  const [hasMore, setHasMore] = useState(false)
  const [page, setPage] = useState(1)
  const [loadingMore, setLoadingMore] = useState(false)
  const [activeRecording, setActiveRecording] = useState<Recording | null>(null)
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc')
  const [motionEvents, setMotionEvents] = useState<MotionEvent[]>([])
  const [activeTab, setActiveTab] = useState<'recordings' | 'events'>('recordings')
  const [eventsPage, setEventsPage] = useState(1)
  const [eventsSortOrder, setEventsSortOrder] = useState<'asc' | 'desc'>('desc')
  const [activeEventIdx, setActiveEventIdx] = useState<number | null>(null)
  const playerRef = useRef<HTMLDivElement>(null)
  const pendingSeekRef = useRef<number | null>(null)
  const videoRef = useRef<HTMLVideoElement>(null)

  useScrollToPlayer(playerRef, activeRecording?.filename ?? null)

  useEffect(() => {
    fetch('/api/config')
      .then(r => r.json())
      .then(d => { if (d.timezone) setTimezone(d.timezone) })
      .catch(() => {})
  }, [])

  useEffect(() => {
    let cancelled = false

    async function load() {
      const [result, events] = await Promise.all([
        loadRecordingsData(id!, selectedDate, 1, sortOrder),
        loadMotionEvents(id!, selectedDate),
      ])
      if (cancelled) return
      if (result === 401) { clearToken(); navigate('/login'); return }
      setPage(1)
      setActiveRecording(null)
      setRecordings(result.recordings)
      setHasMore(result.hasMore)
      setMotionEvents(events)
      setEventsPage(1)
      setActiveEventIdx(null)
    }

    load()
    return () => { cancelled = true }
  }, [selectedDate, id, navigate, sortOrder])

  useEffect(() => {
    const today = new Date()
    const isToday =
      selectedDate.getFullYear() === today.getFullYear() &&
      selectedDate.getMonth() === today.getMonth() &&
      selectedDate.getDate() === today.getDate()
    if (!isToday) return

    const interval = setInterval(async () => {
      const result = await loadRecordingsData(id!, selectedDate, 1, sortOrder)
      if (result === 401) { clearToken(); navigate('/login'); return }
      setRecordings(prev => {
        const existingNames = new Set(prev.map(r => r.filename))
        const fresh = result.recordings.filter(r => !existingNames.has(r.filename))
        const merged = fresh.length > 0
          ? [...prev, ...fresh]
          : prev.map(r => result.recordings.find(u => u.filename === r.filename) ?? r)
        return merged.sort((a, b) =>
          sortOrder === 'desc'
            ? b.filename.localeCompare(a.filename)
            : a.filename.localeCompare(b.filename)
        )
      })
      if (!hasMore) setHasMore(result.hasMore)
    }, 30_000)

    return () => clearInterval(interval)
  }, [selectedDate, id, navigate, hasMore, sortOrder])

  async function loadMore() {
    setLoadingMore(true)
    const next = page + 1
    const result = await loadRecordingsData(id!, selectedDate, next, sortOrder)
    if (result === 401) { clearToken(); navigate('/login'); return }
    setPage(next)
    setRecordings(prev => [...prev, ...result.recordings])
    setHasMore(result.hasMore)
    setLoadingMore(false)
  }

  function playEventAt(ev: MotionEvent, sortedIdx: number) {
    const evTime = new Date(ev.time).getTime()
    const asc = [...recordings].sort((a, b) => a.filename.localeCompare(b.filename))
    for (let i = 0; i < asc.length; i++) {
      const recStart = new Date(asc[i].start).getTime()
      const nextStart = i + 1 < asc.length
        ? new Date(asc[i + 1].start).getTime()
        : recStart + 5 * 60 * 1000
      if (evTime >= recStart && evTime < nextStart) {
        if (asc[i].is_recording) return
        const seekTime = Math.max(0, (evTime - recStart) / 1000 - 10)
        setActiveEventIdx(sortedIdx)
        if (activeRecording?.filename === asc[i].filename) {
          if (videoRef.current) videoRef.current.currentTime = seekTime
        } else {
          pendingSeekRef.current = seekTime
          setActiveRecording(asc[i])
        }
        return
      }
    }
  }

  const liveUrl = `/stream/${id}/index.m3u8`
  const isLive = activeRecording === null
  const sortedEvents = [...motionEvents].sort((a, b) => {
    const diff = new Date(a.time).getTime() - new Date(b.time).getTime()
    return eventsSortOrder === 'asc' ? diff : -diff
  })
  const visibleEvents = sortedEvents.slice(0, eventsPage * PAGE_SIZE)
  const hasMoreEvents = sortedEvents.length > eventsPage * PAGE_SIZE

  return (
    <div className="min-h-screen flex flex-col bg-gray-950">
      <Header username={getUsername() ?? undefined} />
      <main className="flex-1 p-6 max-w-6xl mx-auto w-full">
        <div className="mb-4">
          <Link to="/" className="text-sm text-blue-400 hover:text-blue-300">← Câmeras</Link>
          <h2 className="text-lg font-semibold text-gray-200 mt-1">{id}</h2>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Player */}
          <div className="lg:col-span-2 flex flex-col gap-4">
            <div
              ref={playerRef}
              className={`bg-gray-900 border rounded-lg overflow-hidden transition-all duration-300 ${
                !isLive ? 'border-blue-600 ring-1 ring-blue-600' : 'border-gray-800'
              }`}
            >
              <div className="flex items-center justify-between px-3 py-2 border-b border-gray-800">
                <div className="flex items-center gap-2">
                  {isLive ? (
                    <span className="bg-red-600 text-white text-xs px-2 py-0.5 rounded font-medium">AO VIVO</span>
                  ) : (
                    <span className="text-xs text-gray-300">
                      {format(selectedDate, "d 'de' MMMM", { locale: ptBR })} · {formatRecordingTime(activeRecording.start, timezone)}
                    </span>
                  )}
                  <span className="text-xs text-gray-500">{id}</span>
                </div>
                {!isLive && (
                  <button
                    onClick={() => setActiveRecording(null)}
                    className="text-xs text-blue-400 hover:text-blue-300"
                  >
                    ← Ao vivo
                  </button>
                )}
              </div>

              {isLive ? (
                <HLSPlayer src={liveUrl} className="w-full aspect-video bg-black" />
              ) : (
                <video
                  ref={videoRef}
                  key={activeRecording.url}
                  src={`${activeRecording.url}?token=${getToken()}`}
                  className="w-full aspect-video bg-black"
                  controls
                  autoPlay
                  playsInline
                  onLoadedMetadata={e => {
                    if (pendingSeekRef.current !== null) {
                      e.currentTarget.currentTime = pendingSeekRef.current
                      pendingSeekRef.current = null
                    }
                  }}
                />
              )}
            </div>
          </div>

          {/* Painel direito */}
          <div className="flex flex-col gap-4">
            {/* Calendário */}
            <div className="bg-gray-900 border border-gray-800 rounded-lg p-3">
              <p className="text-xs text-gray-400 mb-2 font-medium uppercase tracking-wider">Gravações</p>
              <DayPicker
                mode="single"
                selected={selectedDate}
                onSelect={d => d && setSelectedDate(d)}
                locale={ptBR}
                classNames={{
                  root: 'text-gray-200 text-sm',
                  month_caption: 'text-gray-200 font-medium',
                  nav: 'text-gray-400',
                  day: 'text-gray-300 hover:bg-gray-700 rounded',
                  day_button: 'w-8 h-8 flex items-center justify-center rounded',
                  selected: 'bg-blue-600 text-white rounded',
                  today: 'text-blue-400 font-semibold',
                  outside: 'text-gray-600',
                  disabled: 'text-gray-700',
                }}
              />
            </div>

            {/* Lista de gravações / eventos */}
            <div className="bg-gray-900 border border-gray-800 rounded-lg overflow-hidden">
              {/* Abas */}
              <div className="flex border-b border-gray-800">
                {(['recordings', 'events'] as const).map(tab => (
                  <button
                    key={tab}
                    onClick={() => setActiveTab(tab)}
                    className={`flex-1 px-3 py-2 text-xs font-medium transition-colors ${
                      activeTab === tab
                        ? 'text-blue-400 border-b-2 border-blue-500 -mb-px'
                        : 'text-gray-500 hover:text-gray-300'
                    }`}
                  >
                    {tab === 'recordings' ? 'Gravações' : 'Eventos'}
                  </button>
                ))}
              </div>

              {activeTab === 'recordings' ? (
                <ListPanel
                  sortOrder={sortOrder}
                  onSortOrderChange={() => setSortOrder(o => o === 'desc' ? 'asc' : 'desc')}
                  hasMore={hasMore}
                  onLoadMore={loadMore}
                  loadingMore={loadingMore}
                  empty={recordings.length === 0}
                  emptyMessage="Sem gravações nesta data."
                >
                  {(() => {
                    const recordingsAsc = [...recordings].sort((a, b) => a.filename.localeCompare(b.filename))
                    return recordings.map(rec => {
                      const isActive = activeRecording?.filename === rec.filename
                      const recStart = new Date(rec.start).getTime()
                      const idx = recordingsAsc.findIndex(r => r.filename === rec.filename)
                      const nextStart = idx + 1 < recordingsAsc.length
                        ? new Date(recordingsAsc[idx + 1].start).getTime()
                        : recStart + 5 * 60 * 1000
                      const hasMotion = motionEvents.some(ev => {
                        const t = new Date(ev.time).getTime()
                        return t >= recStart && t < nextStart
                      })
                      return (
                        <button
                          key={rec.filename}
                          disabled={rec.is_recording}
                          onClick={() => !rec.is_recording && setActiveRecording(rec)}
                          className={`w-full flex items-center justify-between px-3 py-2 transition-colors text-left ${
                            rec.is_recording
                              ? 'opacity-50 cursor-not-allowed'
                              : isActive
                                ? 'bg-blue-900/40 border-l-2 border-blue-500'
                                : 'hover:bg-gray-800'
                          }`}
                        >
                          <span className={`text-sm ${isActive && !rec.is_recording ? 'text-blue-300' : 'text-gray-300'}`}>
                            {formatRecordingTime(rec.start, timezone)}
                          </span>
                          <div className="flex items-center gap-2">
                            {hasMotion && (
                              <span className="w-2 h-2 rounded-full bg-orange-400" title="Movimento detectado" />
                            )}
                            {rec.is_recording
                              ? <span className="text-xs text-red-400 font-medium">● REC</span>
                              : <span className="text-xs text-gray-500">▶ MP4</span>
                            }
                          </div>
                        </button>
                      )
                    })
                  })()}
                </ListPanel>
              ) : (
                <ListPanel
                  sortOrder={eventsSortOrder}
                  onSortOrderChange={() => setEventsSortOrder(o => o === 'desc' ? 'asc' : 'desc')}
                  hasMore={hasMoreEvents}
                  onLoadMore={() => setEventsPage(p => p + 1)}
                  empty={visibleEvents.length === 0}
                  emptyMessage="Sem eventos detectados nesta data."
                >
                  {visibleEvents.map((ev, i) => {
                    const isActive = activeEventIdx === i
                    return (
                      <button
                        key={i}
                        onClick={() => playEventAt(ev, i)}
                        className={`w-full flex items-center justify-between px-3 py-2 transition-colors text-left ${
                          isActive ? 'bg-blue-900/40 border-l-2 border-blue-500' : 'hover:bg-gray-800'
                        }`}
                      >
                        <span className={`text-sm ${isActive ? 'text-blue-300' : 'text-gray-300'}`}>
                          {formatRecordingTime(ev.time, timezone)}
                        </span>
                        <div className="flex items-center gap-2">
                          <span className="w-2 h-2 rounded-full bg-orange-400 shrink-0" />
                          <span className="text-xs text-gray-500">{(ev.score * 100).toFixed(1)}%</span>
                        </div>
                      </button>
                    )
                  })}
                </ListPanel>
              )}
            </div>
          </div>
        </div>
      </main>
    </div>
  )
}
