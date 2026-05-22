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
import VerticalTimeline from '../components/VerticalTimeline'
import { useNotifications } from '../contexts/NotificationContext'
import type { HLSStats } from '../components/HLSPlayer'

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
  const [playbackLeadSeconds, setPlaybackLeadSeconds] = useState(10)
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
  const [activePanel, setActivePanel] = useState<null | 'recordings' | 'events' | 'calendar'>(() => {
    const state = location.state as { eventTime?: string } | null
    return state?.eventTime ? 'events' : null
  })
  const [eventsPage, setEventsPage] = useState(1)
  const [eventsSortOrder, setEventsSortOrder] = useState<'asc' | 'desc'>('desc')
  const [activeEventTime, setActiveEventTime] = useState<string | null>(null)
  const [activeEventId, setActiveEventId] = useState<number | null>(null)
  const [scrollNonce, setScrollNonce] = useState(0)
  const [recordingsDisplayPage, setRecordingsDisplayPage] = useState(1)
  const [playbackRate, setPlaybackRate] = useState(1)
  const [continuousPlay, setContinuousPlay] = useState(false)
  const [browserMaxRate, setBrowserMaxRate] = useState<number | null>(null)
  const [videoMuted, setVideoMuted] = useState(true)
  const [recPlaying, setRecPlaying] = useState(false)
  const [recCurrentTime, setRecCurrentTime] = useState(0)
  const [recDuration, setRecDuration] = useState(0)
  const [recControlsVisible, setRecControlsVisible] = useState(true)
  const [snapshotEvent, setSnapshotEvent] = useState<MotionEvent | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<{ rec: Recording; hasMotion: boolean } | null>(null)
  const [showDebug, setShowDebug] = useState(false)
  const [debugStats, setDebugStats] = useState<{ fps: number; dropped: number; hlsStats: HLSStats | null; lagMs: number } | null>(null)
  const [showDebugChart, setShowDebugChart] = useState(false)
  const [debugPos, setDebugPos] = useState({ x: 8, y: 48 })
  const [playerHeight, setPlayerHeight] = useState<number | undefined>(undefined)
  const debugDragRef = useRef<{ startMouseX: number; startMouseY: number; startPosX: number; startPosY: number } | null>(null)
  const playerRef = useRef<HTMLDivElement>(null)
  const pendingSeekRef = useRef<number | null>(null)
  const videoRef = useRef<HTMLVideoElement>(null)
  const recPlayerRef = useRef<HTMLDivElement>(null)
  const recHideTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const hlsPlayerRef = useRef<HLSPlayerHandle>(null)
  const pendingLiveSeekRef = useRef<string | null>(null)
  const activeEventItemRef = useRef<HTMLButtonElement | null>(null)
  const activeRecordingItemRef = useRef<HTMLDivElement | null>(null)
  const skipNextRecordingScrollRef = useRef(false)
  const recordingsRef = useRef(recordings)
  const activeEventTimeRef = useRef(activeEventTime)
  const activeEventIdRef = useRef(activeEventId)
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
  useEffect(() => { activeEventIdRef.current = activeEventId }, [activeEventId])

  useEffect(() => {
    if (!snapshotEvent) return
    function onKey(e: KeyboardEvent) { if (e.key === 'Escape') setSnapshotEvent(null) }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [snapshotEvent])

  useEffect(() => {
    const el = playerRef.current
    if (!el) return
    const ro = new ResizeObserver(() => setPlayerHeight(el.getBoundingClientRect().height))
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (!e.ctrlKey) return
      if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown') return
      e.preventDefault()
      const recs = recordingsRef.current
      if (recs.length === 0) return
      const sorted = [...recs].sort((a, b) => a.filename.localeCompare(b.filename))
      const idx = activeRecording ? sorted.findIndex(r => r.filename === activeRecording.filename) : -1
      const nextIdx = e.key === 'ArrowDown' ? idx + 1 : idx - 1
      if (nextIdx < 0 || nextIdx >= sorted.length) return
      setActiveRecording(sorted[nextIdx])
      setActiveEventTime(null)
      setActiveEventId(null)
      setScrollNonce(n => n + 1)
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [activeRecording])

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

  const activeRecordingFilename = activeRecording?.filename
  useEffect(() => {
    if (!activeRecordingFilename) return
    if (skipNextRecordingScrollRef.current) {
      skipNextRecordingScrollRef.current = false
      return
    }
    const idx = recordings.findIndex(r => r.filename === activeRecordingFilename)
    if (idx >= 0) {
      const neededPage = Math.ceil((idx + 1) / PAGE_SIZE)
      if (neededPage > recordingsDisplayPageRef.current) {
        setRecordingsDisplayPage(neededPage)
        return
      }
    }
    activeRecordingItemRef.current?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
  }, [activeRecordingFilename, recordingsDisplayPage, recordings])

  // Handles same-route navigation (component doesn't remount when already on this camera)
  useEffect(() => {
    const state = location.state as { eventTime?: string } | null
    if (!state?.eventTime) return
    if (handledEventRef.current === state.eventTime) return // already handled by lazy init
    handledEventRef.current = state.eventTime
    pendingEventRef.current = state.eventTime
    const t = new Date(state.eventTime)
    setSelectedDate(new Date(t.getFullYear(), t.getMonth(), t.getDate()))
    setActivePanel('events')
  }, [location.state])

  useEffect(() => {
    fetch('/api/config')
      .then(r => r.json())
      .then(d => { if (d.timezone) setTimezone(d.timezone) })
      .catch(() => {})
  }, [])

  useEffect(() => {
    if (!id) return
    fetch('/api/cameras', { headers: authHeaders() })
      .then(r => r.ok ? r.json() : null)
      .then((list: Array<{ id: string; playback_lead_seconds?: number }> | null) => {
        const cam = list?.find(c => c.id === id)
        if (cam?.playback_lead_seconds) setPlaybackLeadSeconds(cam.playback_lead_seconds)
      })
      .catch(() => {})
  }, [id])

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

  function handleTimelineSeek(recording: Recording, offsetSeconds: number) {
    setActiveEventTime(null)
    setActiveEventId(null)
    setActivePanel('recordings')
    setScrollNonce(n => n + 1)

    if (activeRecording?.filename === recording.filename) {
      if (videoRef.current) {
        videoRef.current.currentTime = offsetSeconds
        videoRef.current.play().catch(() => {})
      }
    } else {
      pendingSeekRef.current = offsetSeconds
      setActiveRecording(recording)
    }
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
          setActiveEventId(ev.id ?? null)
          setActiveRecording(null)
          const leadTime = new Date(evTime - playbackLeadSeconds * 1000).toISOString()
          if (hlsPlayerRef.current) {
            hlsPlayerRef.current.seekTo(leadTime)
          } else {
            pendingLiveSeekRef.current = leadTime
          }
          if (!skipScroll) setScrollNonce(n => n + 1)
          return
        }
        const seekTime = Math.max(0, (evTime - recStart) / 1000 - playbackLeadSeconds)
        setActiveEventTime(ev.time)
        setActiveEventId(ev.id ?? null)
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
    // Nenhuma gravação corresponde: seleciona o evento na lista sem acionar playback
    setActiveEventTime(ev.time)
    setActiveEventId(ev.id ?? null)
    if (!skipScroll) setScrollNonce(n => n + 1)
  }

  function handleGoToEvent(eventTime: string) {
    const t = new Date(eventTime)
    const eventDate = new Date(t.getFullYear(), t.getMonth(), t.getDate())
    setActivePanel('events')

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

  const { settings } = useSettings(`/cameras/${id}`)
  const motionPeak = useMotionPeak(id, `/cameras/${id}`)
  const { markRead } = useNotifications()
  const cam = settings?.cameras.find(c => c.id === id)
  const effectiveThreshold = (cam?.motion ?? settings?.motion)?.threshold ?? 0

  // Score contextual para o painel de debug: evento ativo > pico da gravação > pico diário
  const debugMotionValue = (() => {
    if (!cam?.motion) return null
    if (activeEventId !== null || activeEventTime !== null) {
      const ev = motionEvents.find(e =>
        activeEventId !== null ? e.id === activeEventId : e.time === activeEventTime
      )
      return ev ? { label: 'Score evento', score: ev.score } : null
    }
    if (activeRecording) {
      const recStart = new Date(activeRecording.start).getTime()
      const recsAsc = [...recordings].sort((a, b) => a.filename.localeCompare(b.filename))
      const idx = recsAsc.findIndex(r => r.filename === activeRecording.filename)
      const nextStart = idx + 1 < recsAsc.length
        ? new Date(recsAsc[idx + 1].start).getTime()
        : recStart + 5 * 60 * 1000
      const evs = motionEvents.filter(e => {
        const t = new Date(e.time).getTime()
        return t >= recStart && t < nextStart
      })
      if (evs.length === 0) return null
      return { label: 'Pico gravação', score: Math.max(...evs.map(e => e.score)) }
    }
    if (motionPeak !== null) return { label: 'Pico diário', score: motionPeak.peak_raw_score }
    return null
  })()


  function formatRecTime(s: number): string {
    if (!isFinite(s) || isNaN(s)) return '0:00'
    const m = Math.floor(s / 60)
    const sec = Math.floor(s % 60)
    return `${m}:${sec.toString().padStart(2, '0')}`
  }

  function showRecControls() {
    setRecControlsVisible(true)
    if (recHideTimerRef.current) clearTimeout(recHideTimerRef.current)
    recHideTimerRef.current = setTimeout(() => setRecControlsVisible(false), 3000)
  }

  function togglePlayRecording() {
    const v = videoRef.current
    if (!v) return
    if (v.paused) v.play().catch(() => {})
    else v.pause()
  }

function toggleFullscreen() {
    if (document.fullscreenElement) document.exitFullscreen().catch(() => {})
    else playerRef.current?.requestFullscreen().catch(() => {})
  }

  function handleDebugDragStart(e: React.MouseEvent) {
    e.preventDefault()
    debugDragRef.current = { startMouseX: e.clientX, startMouseY: e.clientY, startPosX: debugPos.x, startPosY: debugPos.y }
    function onMove(ev: MouseEvent) {
      if (!debugDragRef.current) return
      setDebugPos({
        x: debugDragRef.current.startPosX + (ev.clientX - debugDragRef.current.startMouseX),
        y: debugDragRef.current.startPosY + (ev.clientY - debugDragRef.current.startMouseY),
      })
    }
    function onUp() {
      debugDragRef.current = null
      document.removeEventListener('mousemove', onMove)
      document.removeEventListener('mouseup', onUp)
    }
    document.addEventListener('mousemove', onMove)
    document.addEventListener('mouseup', onUp)
  }

  const liveUrl = `/stream/${id}/index.m3u8`
  const isLive = activeRecording === null

  // Aplica seek pendente no HLS quando o player monta (troca de MP4 → live)
  useEffect(() => {
    if (!isLive || !pendingLiveSeekRef.current) return
    const t = pendingLiveSeekRef.current
    pendingLiveSeekRef.current = null
    hlsPlayerRef.current?.seekTo(t)
  }, [isLive])

  // Polling de métricas de debug (FPS, frames descartados, stats HLS, lag do event loop)
  useEffect(() => {
    if (!showDebug) return
    let lastFrames = 0
    let lastTs = performance.now()
    let lastTickAt = performance.now()

    function sample() {
      const now = performance.now()
      const lagMs = Math.max(0, Math.round(now - lastTickAt - 2000))
      lastTickAt = now

      const q = isLive
        ? hlsPlayerRef.current?.getVideoQuality() ?? null
        : videoRef.current?.getVideoPlaybackQuality?.() ?? null
      let fps = 0
      let dropped = 0
      if (q) {
        const dt = (now - lastTs) / 1000
        const df = q.totalVideoFrames - lastFrames
        fps = dt > 0 ? Math.round(df / dt) : 0
        dropped = q.droppedVideoFrames
        lastFrames = q.totalVideoFrames
        lastTs = now
      }
      const hlsStats = isLive ? (hlsPlayerRef.current?.getStats() ?? null) : null
      setDebugStats({ fps, dropped, hlsStats, lagMs })
    }

    sample()
    const timer = setInterval(sample, 2000)
    return () => { clearInterval(timer); setDebugStats(null) }
  }, [showDebug, isLive])

  const sortedEvents = [...motionEvents].sort((a, b) => {
    const diff = new Date(a.time).getTime() - new Date(b.time).getTime()
    return eventsSortOrder === 'asc' ? diff : -diff
  })
  const visibleEvents = sortedEvents.slice(0, eventsPage * PAGE_SIZE)
  const hasMoreEvents = sortedEvents.length > eventsPage * PAGE_SIZE
  const activeEventIdx = activeEventId !== null
    ? visibleEvents.findIndex(e => e.id === activeEventId)
    : activeEventTime !== null
      ? visibleEvents.findIndex(e => e.time === activeEventTime)
      : -1

  const displayedRecordings = recordings.slice(0, recordingsDisplayPage * PAGE_SIZE)
  const hasMoreDisplayedRecordings = displayedRecordings.length < recordings.length

  // Keep refs in sync for use inside onEnded (avoids stale closure)
  activeEventTimeRef.current = activeEventTime
  visibleEventsRef.current = visibleEvents
  continuousPlayRef.current = continuousPlay
  recordingsDisplayPageRef.current = recordingsDisplayPage

  return (
    <AppLayout mainClassName="w-full">
        <div className="flex">
          {/* Coluna do player */}
          <div className={`relative flex flex-col min-w-0 ${activePanel ? 'flex-1' : 'w-full'}`}>
            <div
              ref={playerRef}
              className={`bg-gray-900 border rounded-lg overflow-hidden transition-all duration-300 ${
                !isLive ? 'border-blue-600 ring-1 ring-blue-600' : 'border-gray-800'
              }`}
            >
              <div className="flex flex-col gap-2 px-4 py-2 border-b border-gray-700">
                <div className="flex items-center justify-between gap-3">
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2 text-xs text-gray-400">
                      <Link to="/" className="text-blue-400 hover:text-blue-300 transition-colors">← Câmeras</Link>
                      <span className="text-gray-600">/</span>
                      <span className="font-medium text-gray-200 truncate">{cam?.name ?? id}</span>
                    </div>
                  </div>
                </div>

                <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className={`inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold ${isLive ? 'bg-red-600 text-white' : 'bg-gray-800 text-gray-300'}`}>
                      {isLive ? 'AO VIVO' : 'Reprodução'}
                    </span>
                    <button
                      onClick={() => setVideoMuted(m => { const next = !m; if (videoRef.current) videoRef.current.muted = next; return next })}
                      title={videoMuted ? 'Ativar áudio' : 'Silenciar'}
                      className="inline-flex items-center justify-center rounded-full border border-gray-700 bg-gray-800 px-3 py-1 text-gray-300 hover:border-blue-500 hover:text-white transition-colors"
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
                    <select
                      value={playbackRate}
                      onChange={e => handleRateChange(Number(e.target.value))}
                      disabled={isLive}
                      className="rounded-full border border-gray-700 bg-gray-800 px-3 py-1 text-xs text-gray-300 disabled:opacity-40 disabled:cursor-not-allowed"
                    >
                      {[1, 2, 4, 8, 16, 32].map(v => (
                        <option key={v} value={v}>{v}x</option>
                      ))}
                    </select>
                    <label className={`inline-flex items-center gap-1.5 rounded-full border border-gray-700 bg-gray-800 px-3 py-1 text-xs ${isLive ? 'text-gray-500' : 'text-gray-200'} ${isLive ? 'cursor-not-allowed' : 'cursor-pointer'}`}>
                      <input
                        type="checkbox"
                        checked={continuousPlay}
                        onChange={e => setContinuousPlay(e.target.checked)}
                        disabled={isLive}
                        className="accent-blue-500 h-4 w-4 rounded"
                      />
                      Contínua
                    </label>
                    {continuousPlay && !isLive && (
                      <span className="text-xs text-blue-400">em sequência</span>
                    )}
                    {browserMaxRate !== null && (
                      <span className="text-xs text-yellow-300">máx. {browserMaxRate}x</span>
                    )}
                  </div>

                  <div className="flex flex-wrap items-center gap-2 text-xs text-gray-300">
                    <button
                      onClick={() => setActivePanel(p => p === 'recordings' ? null : 'recordings')}
                      className={`rounded-full px-3 py-1 transition-colors ${activePanel === 'recordings' ? 'bg-blue-500 text-white' : 'bg-gray-800 hover:bg-gray-700'}`}
                    >
                      Gravações <span className="text-gray-400">{recordingsTotal || recordings.length}</span>
                    </button>
                    <button
                      onClick={() => setActivePanel(p => p === 'events' ? null : 'events')}
                      className={`rounded-full px-3 py-1 transition-colors ${activePanel === 'events' ? 'bg-blue-500 text-white' : 'bg-gray-800 hover:bg-gray-700'}`}
                    >
                      Eventos <span className="text-gray-400">{motionEvents.length}</span>
                    </button>
                    <button
                      onClick={() => setActivePanel(p => p === 'calendar' ? null : 'calendar')}
                      className={`rounded-full px-3 py-1 transition-colors ${activePanel === 'calendar' ? 'bg-blue-500 text-white' : 'bg-gray-800 hover:bg-gray-700'} ${!isToday ? 'text-yellow-300' : ''}`}
                    >
                      {format(selectedDate, "d MMM", { locale: ptBR })}
                    </button>
                    {!isToday && (
                      <button
                        onClick={() => { setSelectedDate(new Date()); setActiveRecording(null); setActivePanel(null); }}
                        className="rounded-full bg-gray-800 px-3 py-1 text-green-300 hover:bg-gray-700 transition-colors"
                      >
                        Hoje
                      </button>
                    )}
                    {!isLive && (
                      <button
                        onClick={() => setActiveRecording(null)}
                        className="rounded-full bg-gray-800 px-3 py-1 text-blue-300 hover:bg-gray-700 transition-colors"
                      >
                        ← Ao vivo
                      </button>
                    )}
                    <Link
                      to={`/settings/cameras/${id}`}
                      state={{ from: `/cameras/${id}`, editing: true }}
                      className="rounded-full bg-gray-800 px-3 py-1 text-gray-300 hover:bg-gray-700 transition-colors"
                    >Configurar</Link>
                    <button
                      onClick={() => setShowDebug(d => !d)}
                      className={`rounded-full px-3 py-1 transition-colors ${showDebug ? 'bg-blue-500 text-white' : 'bg-gray-800 hover:bg-gray-700'}`}
                    >
                      Debug
                    </button>
                    <button onClick={toggleFullscreen} aria-label="Tela inteira" className="rounded-full bg-gray-800 p-2 text-gray-300 hover:bg-gray-700 transition-colors">
                      <svg xmlns="http://www.w3.org/2000/svg" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 8V6a2 2 0 012-2h2M4 16v2a2 2 0 002 2h2m8-16h2a2 2 0 012 2v2m0 8v2a2 2 0 01-2 2h-2" />
                      </svg>
                    </button>
                  </div>
                </div>
              </div>

              {isLive ? (
                <HLSPlayer ref={hlsPlayerRef} src={liveUrl} className="w-full aspect-video bg-black" cameraId={id} muted={videoMuted} segmentSeconds={cam?.hls_segment_seconds} onGoToEvent={handleGoToEvent} />
              ) : (
                <div
                  ref={recPlayerRef}
                  className="relative"
                  onMouseMove={showRecControls}
                  onMouseLeave={() => { if (recPlaying) setRecControlsVisible(false) }}
                >
                  <video
                    ref={videoRef}
                    key={activeRecording.url}
                    src={`${activeRecording.url}?token=${getToken()}`}
                    className="w-full aspect-video bg-black cursor-pointer"
                    autoPlay
                    playsInline
                    onClick={togglePlayRecording}
                    onPlay={() => { setRecPlaying(true); showRecControls() }}
                    onPause={() => { setRecPlaying(false); setRecControlsVisible(true) }}
                    onTimeUpdate={e => setRecCurrentTime(e.currentTarget.currentTime)}
                    onDurationChange={e => setRecDuration(e.currentTarget.duration)}
                    onLoadedMetadata={e => {
                      setRecCurrentTime(0)
                      setRecDuration(e.currentTarget.duration)
                      setRecControlsVisible(true)
                      if (recHideTimerRef.current) clearTimeout(recHideTimerRef.current)
                      try { e.currentTarget.playbackRate = playbackRate } catch { /* browser limit */ }
                      e.currentTarget.muted = videoMuted
                      if (pendingSeekRef.current !== null) {
                        e.currentTarget.currentTime = pendingSeekRef.current
                        pendingSeekRef.current = null
                      }
                      e.currentTarget.play().catch(() => {})
                    }}
                    onVolumeChange={e => setVideoMuted(e.currentTarget.muted)}
                    onEnded={() => {
                      if (!continuousPlayRef.current) return
                      if (activeEventTimeRef.current !== null) {
                        const allAsc = [...allMotionEventsRef.current]
                          .sort((a, b) => new Date(a.time).getTime() - new Date(b.time).getTime())
                        const curIdx = allAsc.findIndex(e =>
                          activeEventIdRef.current !== null
                            ? e.id === activeEventIdRef.current
                            : e.time === activeEventTimeRef.current
                        )
                        const next = curIdx !== -1 ? allAsc[curIdx + 1] : null
                        if (next) playEventAt(next, recordingsRef.current, true)
                        return
                      }
                      if (!activeRecording) return
                      const asc = [...recordingsRef.current].sort((a, b) => a.filename.localeCompare(b.filename))
                      const idx = asc.findIndex(r => r.filename === activeRecording.filename)
                      const next = asc[idx + 1]
                      if (next && !next.is_recording) {
                        const displayedCount = recordingsDisplayPageRef.current * PAGE_SIZE
                        const isVisible = recordingsRef.current.slice(0, displayedCount).some(r => r.filename === next.filename)
                        if (!isVisible) setRecordingsDisplayPage(p => p + 1)
                        skipNextRecordingScrollRef.current = true
                        setActiveRecording(next)
                      }
                    }}
                  />
                  {/* Custom controls overlay */}
                  <div className={`absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/80 to-transparent px-3 pb-2 pt-8 transition-opacity duration-200 pointer-events-none ${recControlsVisible || !recPlaying ? 'opacity-100' : 'opacity-0'}`}>
                    {/* Progress bar */}
                    <div
                      className="h-1 rounded-full bg-white/30 cursor-pointer relative mb-2 pointer-events-auto"
                      onClick={e => {
                        if (!videoRef.current || !recDuration) return
                        const rect = e.currentTarget.getBoundingClientRect()
                        const fraction = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width))
                        videoRef.current.currentTime = fraction * recDuration
                      }}
                    >
                      <div
                        className="absolute inset-y-0 left-0 rounded-full bg-blue-500 pointer-events-none"
                        style={{ width: `${recDuration ? (recCurrentTime / recDuration) * 100 : 0}%` }}
                      />
                    </div>
                    {/* Buttons */}
                    <div className="flex items-center gap-2 pointer-events-auto">
                      <button onClick={togglePlayRecording} aria-label={recPlaying ? 'Pausar' : 'Reproduzir'} className="p-1 text-white/80 hover:text-white transition-colors">
                        {recPlaying ? (
                          <svg xmlns="http://www.w3.org/2000/svg" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 9v6m4-6v6" />
                          </svg>
                        ) : (
                          <svg xmlns="http://www.w3.org/2000/svg" className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                            <path d="M8 5v14l11-7z" />
                          </svg>
                        )}
                      </button>
                      <span className="text-xs text-white/70 tabular-nums">{formatRecTime(recCurrentTime)} / {formatRecTime(recDuration)}</span>
                    </div>
                  </div>
                </div>
              )}
            </div>

            {showDebug && (
              <div
                style={{ left: debugPos.x, top: debugPos.y }}
                className="absolute z-30 bg-gray-900 border border-gray-700 rounded-lg shadow-xl select-none flex flex-col"
              >
                {/* Header — drag handle */}
                <div
                  className="flex items-center justify-between px-4 py-2 border-b border-gray-700 cursor-move"
                  onMouseDown={handleDebugDragStart}
                >
                  <span className="text-xs font-semibold text-gray-300 uppercase tracking-widest">Debug</span>
                  <button onClick={() => setShowDebug(false)} className="ml-6 text-gray-500 hover:text-white transition-colors">
                    <svg xmlns="http://www.w3.org/2000/svg" className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                    </svg>
                  </button>
                </div>
                {/* Stats */}
                <div className="px-4 py-3 flex flex-col gap-3 min-w-64">
                  <div>
                    <p className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-1.5">Stream</p>
                    <div className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
                      <span className="text-xs text-gray-500">Codec</span>
                      <span className="text-sm text-gray-200 font-mono">{cam?.video_codec || '—'}</span>
                      <span className="text-xs text-gray-500">Resolução</span>
                      <span className="text-sm text-gray-200 font-mono">{cam?.width && cam.height ? `${cam.width}×${cam.height}` : '—'}</span>
                      <span className="text-xs text-gray-500">Áudio</span>
                      <span className="text-sm text-gray-200">{cam?.has_audio == null ? '—' : cam.has_audio ? 'sim' : 'não'}</span>
                    </div>
                  </div>
                  <div>
                    <p className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-1.5">Reprodução</p>
                    <div className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
                      {!isLive && (
                        <>
                          <span className="text-xs text-gray-500">Posição</span>
                          <span className="text-sm text-gray-200 font-mono tabular-nums">{formatRecTime(recCurrentTime)} / {formatRecTime(recDuration)}</span>
                          <span className="text-xs text-gray-500">Velocidade</span>
                          <span className="text-sm text-gray-200 font-mono tabular-nums">{playbackRate}×</span>
                        </>
                      )}
                      <span className="text-xs text-gray-500">FPS</span>
                      <span className="text-sm text-gray-200 font-mono tabular-nums">{debugStats ? `${debugStats.fps} fps` : '—'}</span>
                      <span className="text-xs text-gray-500">Descartados</span>
                      <span className={`text-sm font-mono tabular-nums ${debugStats && debugStats.dropped > 0 ? 'text-yellow-400' : 'text-gray-200'}`}>
                        {debugStats ? debugStats.dropped : '—'}
                      </span>
                      <span className="text-xs text-gray-500">CPU</span>
                      <span className={`text-sm font-mono tabular-nums ${debugStats && debugStats.lagMs > 150 ? 'text-red-400' : debugStats && debugStats.lagMs > 50 ? 'text-yellow-400' : 'text-gray-200'}`}>
                        {debugStats ? `${debugStats.lagMs} ms` : '—'}
                      </span>
                    </div>
                  </div>
                  {isLive && (
                    <div>
                      <p className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-1.5">Rede</p>
                      <div className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
                        <span className="text-xs text-gray-500">Bitrate</span>
                        <span className="text-sm text-gray-200 font-mono tabular-nums">{debugStats?.hlsStats ? `${debugStats.hlsStats.bandwidthKbps} kbps` : '—'}</span>
                        <span className="text-xs text-gray-500">Latência</span>
                        <span className="text-sm text-gray-200 font-mono tabular-nums">{debugStats?.hlsStats ? `${debugStats.hlsStats.latencySeconds.toFixed(1)} s` : '—'}</span>
                      </div>
                    </div>
                  )}
                  {cam?.motion && (
                    <div>
                      <p className="text-xs font-semibold text-gray-500 uppercase tracking-wider mb-1.5">Movimento</p>
                      <div className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
                        <span className="text-xs text-gray-500">FPS captura</span>
                        <span className="text-sm text-gray-200 font-mono tabular-nums">{cam.motion.fps ?? '—'}</span>
                        {(cam.motion.capture_width ?? 0) > 0 && (
                          <>
                            <span className="text-xs text-gray-500">Resolução</span>
                            <span className="text-sm text-gray-200 font-mono">{cam.motion.capture_width}×{cam.motion.capture_height}</span>
                          </>
                        )}
                        <span className="text-xs text-gray-500">Limiar</span>
                        <span className="text-sm text-gray-200 font-mono tabular-nums">{effectiveThreshold}</span>
                        {debugMotionValue !== null && effectiveThreshold > 0 && (
                          <>
                            <span className="text-xs text-gray-500">{debugMotionValue.label}</span>
                            <span className="text-sm text-gray-200 font-mono tabular-nums">
                              {(() => {
                                const v = debugMotionValue.score
                                if (v <= 0) return '—'
                                if (v >= 1) return v.toFixed(2)
                                const d = Math.max(2, -Math.floor(Math.log10(v)) + 1)
                                return `${v.toFixed(d)} (${(v / effectiveThreshold).toFixed(2)}×)`
                              })()}
                            </span>
                          </>
                        )}
                      </div>
                      <label className="mt-3 flex items-center gap-1.5 text-xs text-gray-400 hover:text-gray-200 cursor-pointer transition-colors">
                        <input
                          type="checkbox"
                          checked={showDebugChart}
                          onChange={e => setShowDebugChart(e.target.checked)}
                          className="accent-blue-500 w-3 h-3"
                        />
                        gráfico limiar
                      </label>
                    </div>
                  )}
                </div>
                {/* Gráfico — abaixo dos stats, só quando checkbox ativo */}
                {showDebugChart && cam?.motion && (
                  <div className="border-t border-gray-700 p-3 min-w-[640px]">
                    {!isLive && (
                      <p className="text-xs text-yellow-500/80 mb-2">scores ao vivo — não reflete a gravação em curso</p>
                    )}
                    <MotionScoreChart cameraId={id!} threshold={effectiveThreshold} />
                  </div>
                )}
              </div>
            )}

          </div>

          <VerticalTimeline
            recordings={recordings}
            motionEvents={motionEvents}
            activeRecording={activeRecording}
            activeTime={activeEventTime ?? activeRecording?.start ?? null}
            timezone={timezone}
            onSeek={handleTimelineSeek}
            maxHeight={playerHeight}
          />

          {/* Painel lateral condicional */}
          {activePanel && (
            <div className="w-72 shrink-0 border-l border-gray-800 bg-gray-900 flex flex-col overflow-y-auto">
              {activePanel === 'calendar' && (
                <div className="p-3">
                  <DayPicker
                    mode="single"
                    selected={selectedDate}
                    onSelect={d => { if (d) { setSelectedDate(d); setActivePanel(null) } }}
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
              )}
              {(activePanel === 'recordings' || activePanel === 'events') && (
                <>
                  <div className="flex border-b border-gray-800 shrink-0">
                    <button
                      onClick={() => setActivePanel('recordings')}
                      className={`flex-1 px-3 py-2 text-xs font-medium transition-colors ${
                        activePanel === 'recordings'
                          ? 'text-blue-400 border-b-2 border-blue-500 -mb-px'
                          : 'text-gray-500 hover:text-gray-300'
                      }`}
                    >
                      Gravações <span className="ml-1 text-gray-500">{recordingsTotal || recordings.length}</span>
                    </button>
                    <button
                      onClick={() => setActivePanel('events')}
                      className={`flex-1 px-3 py-2 text-xs font-medium transition-colors ${
                        activePanel === 'events'
                          ? 'text-blue-400 border-b-2 border-blue-500 -mb-px'
                          : 'text-gray-500 hover:text-gray-300'
                      }`}
                    >
                      Eventos <span className="ml-1 text-gray-500">{motionEvents.length}</span>
                    </button>
                  </div>
                  {activePanel === 'recordings' ? (
                    <ListPanel
                      key="recordings"
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
                              ref={isActive ? el => { activeRecordingItemRef.current = el } : null}
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
                      key="events"
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
                            key={ev.id ?? `${ev.time}-${i}`}
                            ref={isActive ? (el) => { activeEventItemRef.current = el } : null}
                            onClick={() => { playEventAt(ev); markRead(`${id}-${ev.time}`); setScrollNonce(n => n + 1) }}
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
                              {ev.label && (
                                <span className="text-xs font-medium" style={{ color: ev.color ?? '#f97316' }}>{ev.label}</span>
                              )}
                              <span className="w-2 h-2 rounded-full shrink-0" style={{ backgroundColor: ev.color ?? '#fb923c' }} />
                              <span className="text-xs text-gray-500">{(ev.score * 100).toFixed(1)}%</span>
                            </div>
                          </button>
                        )
                      })}
                    </ListPanel>
                  )}
                </>
              )}
            </div>
          )}
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
