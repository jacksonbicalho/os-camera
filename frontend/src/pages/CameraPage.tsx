import { useCallback, useEffect, useState, useRef } from 'react'
import { useParams, useNavigate, useLocation, Link } from 'react-router-dom'
import { DayPicker } from 'react-day-picker'
import { format } from 'date-fns'
import { ptBR } from 'date-fns/locale'
import 'react-day-picker/style.css'
import { authHeaders, clearToken, getToken } from '../auth'
import AppLayout from '../components/AppLayout'
import ConfirmDialog from '../components/ConfirmDialog'
import HLSPlayer, { type HLSPlayerHandle } from '../components/HLSPlayer'
import ListPanel from '../components/ListPanel'
import MotionScoreChart from '../components/MotionScoreChart'
import { useScrollToPlayer } from '../hooks/useScrollToPlayer'
import { useEventSource } from '../hooks/useEventSource'
import { useSettings } from '../hooks/useSettings'
import { useMotionPeak } from '../hooks/useMotionPeak'
import { mergeRecordings } from './cameraUtils'
import type { Recording, MotionEvent } from './cameraUtils'

interface RecordingsResponse {
  recordings: Recording[]
  hasMore: boolean
  total: number
}

const PAGE_SIZE = 10
const ALL_RECORDINGS_LIMIT = 1000

async function loadRecordingsData(cameraId: string, date: Date, page: number, order: 'asc' | 'desc', limit = PAGE_SIZE): Promise<RecordingsResponse | 401> {
  const dateStr = format(date, 'yyyy-MM-dd')
  const res = await fetch(
    `/api/cameras/${cameraId}/recordings?date=${dateStr}&page=${page}&limit=${limit}&order=${order}`,
    { headers: authHeaders() }
  )
  if (res.status === 401) return 401
  return res.json()
}

async function deleteRecording(cameraId: string, filename: string): Promise<boolean> {
  const res = await fetch(`/api/cameras/${cameraId}/recordings/${filename}`, {
    method: 'DELETE',
    headers: authHeaders(),
  })
  return res.status === 204
}

async function loadMotionEvents(cameraId: string, date: Date): Promise<MotionEvent[]> {
  const dateStr = format(date, 'yyyy-MM-dd')
  const res = await fetch(`/api/cameras/${cameraId}/motion?date=${dateStr}`, { headers: authHeaders() })
  if (!res.ok) return []
  const data = await res.json()
  return data.events ?? []
}

function snapshotURL(cameraId: string, eventTime: string, frame: string): string {
  const d = new Date(eventTime)
  const dateDir = `${d.getUTCFullYear()}/${String(d.getUTCMonth() + 1).padStart(2, '0')}/${String(d.getUTCDate()).padStart(2, '0')}`
  return `/recordings/${cameraId}/${dateDir}/${frame}?token=${getToken()}`
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
  const location = useLocation()
  const [timezone, setTimezone] = useState('UTC')
  const [selectedDate, setSelectedDate] = useState<Date>(() => {
    const state = location.state as { eventTime?: string } | null
    if (state?.eventTime) {
      const t = new Date(state.eventTime)
      return new Date(t.getFullYear(), t.getMonth(), t.getDate())
    }
    return new Date()
  })
  const [recordings, setRecordings] = useState<Recording[]>([])
  const [recordingsTotal, setRecordingsTotal] = useState(0)
  const [hasMore, setHasMore] = useState(false)
  const [activeRecording, setActiveRecording] = useState<Recording | null>(null)
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc')
  const [motionEvents, setMotionEvents] = useState<MotionEvent[]>([])
  const [activeTab, setActiveTab] = useState<'recordings' | 'events'>(() => {
    const state = location.state as { eventTime?: string } | null
    return state?.eventTime ? 'events' : 'recordings'
  })
  const [eventsPage, setEventsPage] = useState(1)
  const [eventsSortOrder, setEventsSortOrder] = useState<'asc' | 'desc'>('desc')
  const [activeEventTime, setActiveEventTime] = useState<string | null>(null)
  const [scrollNonce, setScrollNonce] = useState(0)
  const [recordingsDisplayPage, setRecordingsDisplayPage] = useState(1)
  const [playbackRate, setPlaybackRate] = useState(1)
  const [continuousPlay, setContinuousPlay] = useState(false)
  const [browserMaxRate, setBrowserMaxRate] = useState<number | null>(null)
  const [videoMuted, setVideoMuted] = useState(false)
  const [snapshotEvent, setSnapshotEvent] = useState<MotionEvent | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<{ rec: Recording; hasMotion: boolean } | null>(null)
  const playerRef = useRef<HTMLDivElement>(null)
  const pendingSeekRef = useRef<number | null>(null)
  const videoRef = useRef<HTMLVideoElement>(null)
  const hlsPlayerRef = useRef<HLSPlayerHandle>(null)
  const activeEventItemRef = useRef<HTMLButtonElement | null>(null)
  const recordingsRef = useRef(recordings)
  const activeEventTimeRef = useRef(activeEventTime)
  const allMotionEventsRef = useRef(motionEvents)
  const visibleEventsRef = useRef<typeof visibleEvents>([])
  const continuousPlayRef = useRef(continuousPlay)
  const recordingsDisplayPageRef = useRef(recordingsDisplayPage)
  const pendingEventRef = useRef<string | null>(
    (location.state as { eventTime?: string } | null)?.eventTime ?? null
  )
  // Tracks the eventTime already handled on mount so we skip re-processing it
  const handledEventRef = useRef<string | null>(pendingEventRef.current)

  useEffect(() => { recordingsRef.current = recordings }, [recordings])
  useEffect(() => { allMotionEventsRef.current = motionEvents }, [motionEvents])

  useEffect(() => {
    if (!snapshotEvent) return
    function onKey(e: KeyboardEvent) { if (e.key === 'Escape') setSnapshotEvent(null) }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [snapshotEvent])

  function handleRateChange(requested: number) {
    const options = [1, 2, 4, 8, 16, 32]
    const requestedIdx = options.indexOf(requested)
    const video = videoRef.current
    for (let i = requestedIdx; i >= 0; i--) {
      try {
        if (video) video.playbackRate = options[i]
        setPlaybackRate(options[i])
        setBrowserMaxRate(i < requestedIdx ? options[i] : null)
        return
      } catch { /* try next lower */ }
    }
    if (video) video.playbackRate = 1
    setPlaybackRate(1)
    setBrowserMaxRate(null)
  }

  useScrollToPlayer(playerRef, activeRecording?.filename ?? null, continuousPlay)

  useEffect(() => {
    if (activeEventTime === null) return
    if (continuousPlayRef.current) return
    activeEventItemRef.current?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
  }, [activeEventTime, scrollNonce])

  // Handles same-route navigation (component doesn't remount when already on this camera)
  useEffect(() => {
    const state = location.state as { eventTime?: string } | null
    if (!state?.eventTime) return
    if (handledEventRef.current === state.eventTime) return // already handled by lazy init
    handledEventRef.current = state.eventTime
    pendingEventRef.current = state.eventTime
    const t = new Date(state.eventTime)
    setSelectedDate(new Date(t.getFullYear(), t.getMonth(), t.getDate()))
    setActiveTab('events')
  }, [location.state])

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
        loadRecordingsData(id!, selectedDate, 1, sortOrder, ALL_RECORDINGS_LIMIT),
        loadMotionEvents(id!, selectedDate),
      ])
      if (cancelled) return
      if (result === 401) { clearToken(); navigate('/login', { state: { from: `/cameras/${id}` }, replace: true }); return }
      setRecordingsDisplayPage(1)
      setActiveRecording(null)
      setRecordings(result.recordings)
      setRecordingsTotal(result.total)
      setHasMore(result.hasMore)
      setMotionEvents(events)
      setEventsPage(1)
      setActiveEventTime(null)

      const pendingTime = pendingEventRef.current
      if (pendingTime) {
        pendingEventRef.current = null
        const ev = events.find(e => e.time === pendingTime)
        if (ev) playEventAt(ev, result.recordings)
      }
    }

    load()
    return () => { cancelled = true }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedDate, id, navigate, sortOrder])

  useEffect(() => {
    const today = new Date()
    const isToday =
      selectedDate.getFullYear() === today.getFullYear() &&
      selectedDate.getMonth() === today.getMonth() &&
      selectedDate.getDate() === today.getDate()
    if (!isToday) return

    const interval = setInterval(async () => {
      const result = await loadRecordingsData(id!, selectedDate, 1, sortOrder, ALL_RECORDINGS_LIMIT)
      if (result === 401) { clearToken(); navigate('/login', { state: { from: `/cameras/${id}` }, replace: true }); return }
      setRecordings(prev => mergeRecordings(prev, result.recordings, sortOrder, result.hasMore))
      setRecordingsTotal(result.total)
      setHasMore(result.hasMore)
    }, 30_000)

    return () => clearInterval(interval)
  }, [selectedDate, id, navigate, hasMore, sortOrder])

  const today = new Date()
  const isToday =
    selectedDate.getFullYear() === today.getFullYear() &&
    selectedDate.getMonth() === today.getMonth() &&
    selectedDate.getDate() === today.getDate()

  const handleLiveMotion = useCallback(() => {
    loadMotionEvents(id!, selectedDate).then(setMotionEvents)
  }, [id, selectedDate])

  useEventSource(
    isToday && id ? `/api/cameras/${id}/motion/live` : null,
    handleLiveMotion,
  )

  async function reloadRecordingsAndEvents() {
    const [result, events] = await Promise.all([
      loadRecordingsData(id!, selectedDate, 1, sortOrder, ALL_RECORDINGS_LIMIT),
      loadMotionEvents(id!, selectedDate),
    ])
    if (result === 401) { clearToken(); navigate('/login', { state: { from: `/cameras/${id}` }, replace: true }); return }
    setRecordings(result.recordings)
    setRecordingsTotal(result.total)
    setHasMore(result.hasMore)
    setMotionEvents(events)
  }

  async function handleConfirmDelete() {
    if (!deleteTarget) return
    setDeleteTarget(null)
    const ok = await deleteRecording(id!, deleteTarget.rec.filename)
    if (ok) {
      if (activeRecording?.filename === deleteTarget.rec.filename) setActiveRecording(null)
      await reloadRecordingsAndEvents()
    }
  }

  function playEventAt(ev: MotionEvent, recs: Recording[] = recordings, skipScroll = false) {
    const evTime = new Date(ev.time).getTime()
    const asc = [...recs].sort((a, b) => a.filename.localeCompare(b.filename))
    for (let i = 0; i < asc.length; i++) {
      const recStart = new Date(asc[i].start).getTime()
      const nextStart = i + 1 < asc.length
        ? new Date(asc[i + 1].start).getTime()
        : recStart + 5 * 60 * 1000
      if (evTime >= recStart && evTime < nextStart) {
        if (asc[i].is_recording) {
          setActiveEventTime(ev.time)
          setActiveRecording(null)
          hlsPlayerRef.current?.seekTo(ev.time)
          if (!skipScroll) setScrollNonce(n => n + 1)
          return
        }
        const seekTime = Math.max(0, (evTime - recStart) / 1000 - 10)
        setActiveEventTime(ev.time)
        if (!skipScroll) setScrollNonce(n => n + 1)
        if (activeRecording?.filename === asc[i].filename) {
          if (videoRef.current) {
            videoRef.current.currentTime = seekTime
            videoRef.current.play().catch(() => {})
          }
        } else {
          pendingSeekRef.current = seekTime
          setActiveRecording(asc[i])
        }
        return
      }
    }
  }

  function handleGoToEvent(eventTime: string) {
    const t = new Date(eventTime)
    const eventDate = new Date(t.getFullYear(), t.getMonth(), t.getDate())
    setActiveTab('events')

    // Se a data do evento difere da data selecionada, muda o calendário e usa o
    // fluxo de pendingEventRef (igual ao clique em notificação) para carregar os dados
    if (format(eventDate, 'yyyy-MM-dd') !== format(selectedDate, 'yyyy-MM-dd')) {
      pendingEventRef.current = eventTime
      setSelectedDate(eventDate)
      return
    }

    // Mesma data: busca o evento diretamente
    const ev = motionEvents.find(e => e.time === eventTime)
    if (!ev) return
    playEventAt(ev)
    // Força o scroll mesmo que activeEventTime já tenha o mesmo valor
    setScrollNonce(n => n + 1)
  }

  const settings = useSettings(`/cameras/${id}`)
  const motionPeak = useMotionPeak(id, `/cameras/${id}`)
  const cam = settings?.cameras.find(c => c.id === id)
  const effectiveThreshold = (cam?.motion ?? settings?.motion)?.threshold ?? 0

  const liveUrl = `/stream/${id}/index.m3u8`
  const isLive = activeRecording === null
  const sortedEvents = [...motionEvents].sort((a, b) => {
    const diff = new Date(a.time).getTime() - new Date(b.time).getTime()
    return eventsSortOrder === 'asc' ? diff : -diff
  })
  const visibleEvents = sortedEvents.slice(0, eventsPage * PAGE_SIZE)
  const hasMoreEvents = sortedEvents.length > eventsPage * PAGE_SIZE
  const activeEventIdx = activeEventTime !== null ? visibleEvents.findIndex(e => e.time === activeEventTime) : -1

  const displayedRecordings = recordings.slice(0, recordingsDisplayPage * PAGE_SIZE)
  const hasMoreDisplayedRecordings = displayedRecordings.length < recordings.length

  // Keep refs in sync for use inside onEnded (avoids stale closure)
  activeEventTimeRef.current = activeEventTime
  visibleEventsRef.current = visibleEvents
  continuousPlayRef.current = continuousPlay
  recordingsDisplayPageRef.current = recordingsDisplayPage

  return (
    <AppLayout mainClassName="max-w-6xl mx-auto w-full">
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
                <HLSPlayer ref={hlsPlayerRef} src={liveUrl} className="w-full aspect-video bg-black" cameraId={id} onGoToEvent={handleGoToEvent} />
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
                    try { e.currentTarget.playbackRate = playbackRate } catch { /* browser limit */ }
                    e.currentTarget.muted = videoMuted
                    if (pendingSeekRef.current !== null) {
                      e.currentTarget.currentTime = pendingSeekRef.current
                      pendingSeekRef.current = null
                    }
                  }}
                  onVolumeChange={e => setVideoMuted(e.currentTarget.muted)}
                  onEnded={() => {
                    if (!continuousPlayRef.current) return
                    // Events mode: advance to next event in ascending chronological order
                    if (activeEventTimeRef.current !== null) {
                      const allAsc = [...allMotionEventsRef.current]
                        .sort((a, b) => new Date(a.time).getTime() - new Date(b.time).getTime())
                      const curIdx = allAsc.findIndex(e => e.time === activeEventTimeRef.current)
                      const next = curIdx !== -1 ? allAsc[curIdx + 1] : null
                      if (next) playEventAt(next, recordingsRef.current, true)
                      return
                    }
                    // Recordings mode: advance to next recording in ascending chronological order
                    if (!activeRecording) return
                    const asc = [...recordingsRef.current].sort((a, b) => a.filename.localeCompare(b.filename))
                    const idx = asc.findIndex(r => r.filename === activeRecording.filename)
                    const next = asc[idx + 1]
                    if (next && !next.is_recording) {
                      const displayedCount = recordingsDisplayPageRef.current * PAGE_SIZE
                      const isVisible = recordingsRef.current.slice(0, displayedCount).some(r => r.filename === next.filename)
                      if (!isVisible) setRecordingsDisplayPage(p => p + 1)
                      setActiveRecording(next)
                    }
                  }}
                />
              )}
            </div>

            {/* Controles de reprodução */}
            <div className="flex flex-wrap items-center gap-x-5 gap-y-1 px-1">
              <button
                onClick={() => {
                  const video = videoRef.current
                  if (video) video.muted = !video.muted
                  setVideoMuted(m => !m)
                }}
                disabled={isLive}
                title={videoMuted ? 'Ativar áudio' : 'Silenciar'}
                className="flex items-center text-gray-400 hover:text-white disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
              >
                {videoMuted ? (
                  <svg xmlns="http://www.w3.org/2000/svg" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" />
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2" />
                  </svg>
                ) : (
                  <svg xmlns="http://www.w3.org/2000/svg" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.536 8.464a5 5 0 010 7.072M12 6v12m-3.536-9.536a5 5 0 000 7.072M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z" />
                  </svg>
                )}
              </button>
              <label className="flex items-center gap-2 text-xs text-gray-400">
                Velocidade
                <select
                  value={playbackRate}
                  onChange={e => handleRateChange(Number(e.target.value))}
                  disabled={isLive}
                  className="bg-gray-800 border border-gray-700 text-gray-200 text-xs rounded px-2 py-1 disabled:opacity-40 disabled:cursor-not-allowed"
                >
                  {[1, 2, 4, 8, 16, 32].map(v => (
                    <option key={v} value={v}>{v}x</option>
                  ))}
                </select>
              </label>
              <label className={`flex items-center gap-2 text-xs ${isLive ? 'text-gray-600 cursor-not-allowed' : 'text-gray-400 cursor-pointer'}`}>
                <input
                  type="checkbox"
                  checked={continuousPlay}
                  onChange={e => setContinuousPlay(e.target.checked)}
                  disabled={isLive}
                  className="accent-blue-500 w-3 h-3 cursor-pointer"
                />
                Reprodução contínua
              </label>
              {continuousPlay && !isLive && (
                <span className="text-xs text-blue-400">· reproduzindo em sequência</span>
              )}
              {browserMaxRate !== null && (
                <span className="text-xs text-yellow-500">· {browserMaxRate}x é o máximo suportado</span>
              )}
            </div>

            {/* Detecção de movimento */}
            <Link
              to={`/settings/cameras/${id}/motion`}
              className="block bg-gray-900 border border-gray-800 rounded-lg overflow-hidden hover:border-gray-700 transition-colors group"
            >
              <div className="flex items-center justify-between px-5 py-3 border-b border-gray-800">
                <p className="text-xs font-medium text-gray-400 uppercase tracking-wider">Detecção de movimento</p>
                <div className="flex items-center gap-3">
                  {motionPeak !== null && effectiveThreshold > 0 && (
                    <span className="text-xs text-gray-500 font-mono">
                      pico {(() => {
                        const v = motionPeak.peak_raw_score
                        if (v <= 0) return '—'
                        if (v >= 1) return v.toFixed(2)
                        const d = Math.max(2, -Math.floor(Math.log10(v)) + 1)
                        return v.toFixed(d)
                      })()} · {(motionPeak.peak_raw_score / effectiveThreshold).toFixed(2)}× limiar
                    </span>
                  )}
                  <span className="text-xs text-blue-400 group-hover:text-blue-300 transition-colors">Configurar →</span>
                </div>
              </div>
              <div className="px-5 py-4">
                <MotionScoreChart cameraId={id!} threshold={effectiveThreshold} />
              </div>
            </Link>
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
                    {tab === 'recordings'
                      ? <>Gravações <span className="ml-1 text-gray-500">{recordingsTotal || recordings.length}</span></>
                      : <>Eventos <span className="ml-1 text-gray-500">{motionEvents.length}</span></>
                    }
                  </button>
                ))}
              </div>

              {activeTab === 'recordings' ? (
                <ListPanel
                  sortOrder={sortOrder}
                  onSortOrderChange={() => { setSortOrder(o => o === 'desc' ? 'asc' : 'desc'); setRecordingsDisplayPage(1) }}
                  hasMore={hasMoreDisplayedRecordings}
                  onLoadMore={() => setRecordingsDisplayPage(p => p + 1)}
                  empty={recordings.length === 0}
                  emptyMessage="Sem gravações nesta data."
                >
                  {(() => {
                    const recordingsAsc = [...recordings].sort((a, b) => a.filename.localeCompare(b.filename))
                    return displayedRecordings.map(rec => {
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
                        <div
                          key={rec.filename}
                          className={`group flex items-center justify-between px-3 py-2 transition-colors ${
                            rec.is_recording
                              ? 'opacity-50'
                              : isActive
                                ? 'bg-blue-900/40 border-l-2 border-blue-500'
                                : 'hover:bg-gray-800'
                          }`}
                        >
                          <button
                            disabled={rec.is_recording}
                            onClick={() => !rec.is_recording && setActiveRecording(rec)}
                            className="flex-1 flex items-center justify-between text-left disabled:cursor-not-allowed"
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
                          {!rec.is_recording && (
                            <button
                              onClick={() => setDeleteTarget({ rec, hasMotion })}
                              title="Excluir gravação"
                              className="ml-2 text-gray-600 hover:text-red-400 opacity-0 group-hover:opacity-100 transition-opacity"
                            >
                              <svg xmlns="http://www.w3.org/2000/svg" className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                              </svg>
                            </button>
                          )}
                        </div>
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
                    const thumbURL = ev.frame ? snapshotURL(id!, ev.time, ev.frame) : null
                    return (
                      <button
                        key={ev.time}
                        ref={isActive ? (el) => { activeEventItemRef.current = el } : null}
                        onClick={() => { playEventAt(ev); setScrollNonce(n => n + 1) }}
                        className={`w-full flex items-center justify-between px-3 py-2 transition-colors text-left ${
                          isActive ? 'bg-blue-900/40 border-l-2 border-blue-500' : 'hover:bg-gray-800'
                        }`}
                      >
                        <span className={`text-sm ${isActive ? 'text-blue-300' : 'text-gray-300'}`}>
                          {formatRecordingTime(ev.time, timezone)}
                        </span>
                        <div className="flex items-center gap-2">
                          {thumbURL && (
                            <img
                              src={thumbURL}
                              alt="snapshot"
                              className="w-16 h-10 object-cover rounded cursor-zoom-in border border-gray-700"
                              onClick={e => { e.stopPropagation(); setSnapshotEvent(ev) }}
                            />
                          )}
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

      {snapshotEvent && snapshotEvent.frame && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/80"
          onClick={() => setSnapshotEvent(null)}
        >
          <div className="relative max-w-3xl w-full mx-4" onClick={e => e.stopPropagation()}>
            <button
              className="absolute -top-8 right-0 text-gray-400 hover:text-white text-sm"
              onClick={() => setSnapshotEvent(null)}
            >
              Fechar ✕
            </button>
            <img
              src={snapshotURL(id!, snapshotEvent.time, snapshotEvent.frame)}
              alt="snapshot de movimento"
              className="w-full rounded-lg border border-gray-700"
            />
            <p className="mt-2 text-xs text-gray-400 text-center">
              {formatRecordingTime(snapshotEvent.time, timezone)} — score: {(snapshotEvent.score * 100).toFixed(1)}%
            </p>
          </div>
        </div>
      )}

      <ConfirmDialog
        open={deleteTarget !== null}
        title="Excluir gravação"
        message={
          deleteTarget?.hasMotion
            ? `Excluir esta gravação? Os eventos de movimento associados e seus snapshots também serão excluídos. Esta ação não pode ser desfeita.`
            : `Excluir esta gravação? Esta ação não pode ser desfeita.`
        }
        confirmLabel="Excluir"
        danger
        onConfirm={handleConfirmDelete}
        onCancel={() => setDeleteTarget(null)}
      />

    </AppLayout>
  )
}
