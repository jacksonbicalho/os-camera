import { useCallback, useEffect, useState, useRef } from 'react'
import { useParams, useNavigate, useLocation } from 'react-router-dom'
import { VolumeX, Volume2, Gauge, Repeat, Film, Zap, CalendarDays, Code2, Settings, Maximize, Play, Pause, X, Trash2 } from '../components/Icons'
import { DayPicker } from 'react-day-picker'
import { format } from 'date-fns'
import { ptBR } from 'date-fns/locale'
import 'react-day-picker/style.css'
import { authHeaders, onUnauthorized, getToken, getRole } from '../auth'
import AppLayout from '../components/AppLayout'
import ConfirmDialog from '../components/ConfirmDialog'
import HLSPlayer, { type HLSPlayerHandle } from '../components/HLSPlayer'
import ListPanel from '../components/ListPanel'
import MotionScoreChart from '../components/MotionScoreChart'
import { useScrollToPlayer } from '../hooks/useScrollToPlayer'
import { useEventSource } from '../hooks/useEventSource'
import { useSettings, type CameraSettings } from '../hooks/useSettings'
import { useMotionPeak } from '../hooks/useMotionPeak'
import { mergeRecordings } from './cameraUtils'
import type { Recording, MotionEvent, Annotation } from './cameraUtils'
import VerticalTimeline from '../components/VerticalTimeline'
import { useNotifications } from '../contexts/NotificationContext'
import { useSetSidebarItems } from '../contexts/SidebarContext'
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

async function deleteRecording(cameraId: string, filename: string): Promise<{ ok: boolean; serverError: boolean }> {
  const res = await fetch(`/api/cameras/${cameraId}/recordings/${filename}`, {
    method: 'DELETE',
    headers: authHeaders(),
  })
  return { ok: res.status === 204, serverError: res.status >= 500 }
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

function formatRecordingDateTime(isoString: string, timezone: string): string {
  const d = new Date(isoString)
  const date = d.toLocaleDateString('pt-BR', { timeZone: timezone, day: '2-digit', month: '2-digit' })
  const time = d.toLocaleTimeString('pt-BR', { timeZone: timezone, hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false })
  return `${date} ${time}`
}

export default function CameraPage() {
  const { id, recording_id: recordingId } = useParams<{ id: string; recording_id?: string }>()
  const navigate = useNavigate()
  const location = useLocation()
  const isLiveRoute = location.pathname.startsWith('/camera/live/')
  const isAdmin = getRole() === 'admin'
  const [timezone, setTimezone] = useState('UTC')
  const [playbackLeadSeconds, setPlaybackLeadSeconds] = useState(10)
  const [selectedDate, setSelectedDate] = useState<Date>(() => {
    const state = isLiveRoute ? null : (location.state as { eventTime?: string } | null)
    if (state?.eventTime) {
      const t = new Date(state.eventTime)
      return new Date(t.getFullYear(), t.getMonth(), t.getDate())
    }
    return new Date()
  })
  const [calendarMonth, setCalendarMonth] = useState<Date>(() => {
    const state = isLiveRoute ? null : (location.state as { eventTime?: string } | null)
    if (state?.eventTime) {
      const t = new Date(state.eventTime)
      return new Date(t.getFullYear(), t.getMonth(), 1)
    }
    return new Date()
  })
  const [viewerCam, setViewerCam] = useState<CameraSettings | undefined>(undefined)
  const [recordings, setRecordings] = useState<Recording[]>([])
  const [recordingsTotal, setRecordingsTotal] = useState(0)
  const [activeRecording, setActiveRecording] = useState<Recording | null>(null)
  const [sortOrder, setSortOrder] = useState<'asc' | 'desc'>('desc')
  const [motionEvents, setMotionEvents] = useState<MotionEvent[]>([])
  const [activePanel, setActivePanel] = useState<null | 'recordings' | 'events' | 'calendar'>(() => {
    if (recordingId) return 'recordings'
    if (isLiveRoute) return null
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
  const [recPlayBlocked, setRecPlayBlocked] = useState(false)
  const [lastFrameDataUrl, setLastFrameDataUrl] = useState<string | null>(null)
  const [snapshotEvent, setSnapshotEvent] = useState<MotionEvent | null>(null)
  const [annotating, setAnnotating] = useState(false)
  const [annotations, setAnnotations] = useState<Annotation[]>([])
  const [annLabel, setAnnLabel] = useState('')
  const [annDraft, setAnnDraft] = useState<{ x: number; y: number; w: number; h: number } | null>(null)
  const annDragRef = useRef<{ startX: number; startY: number } | null>(null)
  const annImgRef = useRef<HTMLImageElement>(null)
  const [detectionModal, setDetectionModal] = useState<Array<{ label: string; confidence: number; frame_count: number }> | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<{ rec: Recording; hasMotion: boolean } | null>(null)
  const [deleteError, setDeleteError] = useState<string | null>(null)
  const [showDebug, setShowDebug] = useState(false)
  const [speedMenuOpen, setSpeedMenuOpen] = useState(false)
  const [debugStats, setDebugStats] = useState<{ fps: number; dropped: number; hlsStats: HLSStats | null; lagMs: number } | null>(null)
  const [showDebugChart, setShowDebugChart] = useState(false)
  const [debugPos, setDebugPos] = useState({ x: 8, y: 8 })
  const [playerHeight, setPlayerHeight] = useState<number | undefined>(undefined)
  const debugDragRef = useRef<{ startMouseX: number; startMouseY: number; startPosX: number; startPosY: number } | null>(null)
  const speedMenuRef = useRef<HTMLDivElement>(null)
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
  const selectedDateRef = useRef(selectedDate)
  const visibleEventsRef = useRef<typeof visibleEvents>([])
  const continuousPlayRef = useRef(continuousPlay)
  const recordingsDisplayPageRef = useRef(recordingsDisplayPage)
  const eventsPageRef = useRef(eventsPage)
  const sortedEventsRef = useRef<MotionEvent[]>([])
  const pendingEventRef = useRef<string | null>(
    isLiveRoute ? null : (location.state as { eventTime?: string } | null)?.eventTime ?? null
  )
  // Tracks the eventTime already handled on mount so we skip re-processing it
  const handledEventRef = useRef<string | null>(pendingEventRef.current)
  const pendingRecordingRef = useRef<{ filename: string } | null>(null)
  const mountRecordingIdRef = useRef(recordingId)

  useEffect(() => { recordingsRef.current = recordings }, [recordings])
  useEffect(() => { allMotionEventsRef.current = motionEvents }, [motionEvents])
  useEffect(() => { activeEventIdRef.current = activeEventId }, [activeEventId])
  useEffect(() => { selectedDateRef.current = selectedDate }, [selectedDate])

  useEffect(() => {
    if (!speedMenuOpen) return
    const handle = (e: MouseEvent) => {
      if (!speedMenuRef.current?.contains(e.target as Node)) setSpeedMenuOpen(false)
    }
    document.addEventListener('mousedown', handle)
    return () => document.removeEventListener('mousedown', handle)
  }, [speedMenuOpen])

  function closeSnapshotModal() {
    setSnapshotEvent(null)
    setAnnotating(false)
    setAnnotations([])
    setAnnLabel('')
    setAnnDraft(null)
  }

  useEffect(() => {
    if (!snapshotEvent) return
    function onKey(e: KeyboardEvent) { if (e.key === 'Escape') closeSnapshotModal() }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [snapshotEvent])

  useEffect(() => {
    if (!showDebug) return
    function onKey(e: KeyboardEvent) { if (e.key === 'Escape') setShowDebug(false) }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [showDebug])

  useEffect(() => {
    if (!annotating || !snapshotEvent?.id) return
    fetch(`/api/events/${snapshotEvent.id}/annotations`, { headers: authHeaders() })
      .then(r => r.json())
      .then((list: Annotation[]) => setAnnotations(list ?? []))
      .catch(() => {})
  }, [annotating, snapshotEvent])

  function getImgRect(): DOMRect | null {
    return annImgRef.current?.getBoundingClientRect() ?? null
  }

  function toRelative(clientX: number, clientY: number): { x: number; y: number } | null {
    const rect = getImgRect()
    if (!rect) return null
    return {
      x: Math.max(0, Math.min(1, (clientX - rect.left) / rect.width)),
      y: Math.max(0, Math.min(1, (clientY - rect.top) / rect.height)),
    }
  }

  function onAnnMouseDown(e: React.MouseEvent) {
    if (!annotating) return
    const rel = toRelative(e.clientX, e.clientY)
    if (!rel) return
    annDragRef.current = { startX: rel.x, startY: rel.y }
    setAnnDraft({ x: rel.x, y: rel.y, w: 0, h: 0 })
  }

  function onAnnMouseMove(e: React.MouseEvent) {
    if (!annDragRef.current) return
    const rel = toRelative(e.clientX, e.clientY)
    if (!rel) return
    const { startX, startY } = annDragRef.current
    setAnnDraft({
      x: Math.min(startX, rel.x),
      y: Math.min(startY, rel.y),
      w: Math.abs(rel.x - startX),
      h: Math.abs(rel.y - startY),
    })
  }

  function onAnnMouseUp() {
    annDragRef.current = null
  }

  async function saveAnnotation() {
    if (!snapshotEvent?.id || !annDraft || annLabel.trim() === '') return
    const rect = getImgRect()
    if (!rect) return
    const body = {
      label: annLabel.trim(),
      bbox_x: annDraft.x,
      bbox_y: annDraft.y,
      bbox_w: annDraft.w,
      bbox_h: annDraft.h,
    }
    const res = await fetch(`/api/events/${snapshotEvent.id}/annotations`, {
      method: 'POST',
      headers: { ...authHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
    if (!res.ok) return
    const fresh = await fetch(`/api/events/${snapshotEvent.id}/annotations`, { headers: authHeaders() })
    const list: Annotation[] = await fresh.json()
    setAnnotations(list ?? [])
    setAnnDraft(null)
    setAnnLabel('')
  }

  async function deleteAnnotation(annId: number) {
    await fetch(`/api/annotations/${annId}`, { method: 'DELETE', headers: authHeaders() })
    setAnnotations(prev => prev.filter(a => a.id !== annId))
  }

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
      openRecording(sorted[nextIdx])
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
    const idx = activeEventId !== null
      ? sortedEventsRef.current.findIndex(e => e.id === activeEventId)
      : sortedEventsRef.current.findIndex(e => e.time === activeEventTime)
    if (idx >= 0) {
      const neededPage = Math.ceil((idx + 1) / PAGE_SIZE)
      if (neededPage > eventsPageRef.current) {
        setEventsPage(neededPage)
        return
      }
    }
    activeEventItemRef.current?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
  }, [activeEventTime, activeEventId, eventsPage, scrollNonce])

  const activeRecordingFilename = activeRecording?.filename
  useEffect(() => {
    if (!activeRecordingFilename) return
    if (skipNextRecordingScrollRef.current) {
      skipNextRecordingScrollRef.current = false
      return
    }
    const idx = recordingsRef.current.findIndex(r => r.filename === activeRecordingFilename)
    if (idx >= 0) {
      const neededPage = Math.ceil((idx + 1) / PAGE_SIZE)
      if (neededPage > recordingsDisplayPageRef.current) {
        setRecordingsDisplayPage(neededPage)
        return
      }
    }
    activeRecordingItemRef.current?.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
  }, [activeRecordingFilename, recordingsDisplayPage])

  // Handles same-route navigation (component doesn't remount when already on this camera)
  useEffect(() => {
    if (isLiveRoute) return
    const state = location.state as { eventTime?: string } | null
    if (!state?.eventTime) return
    if (handledEventRef.current === state.eventTime) return // already handled by lazy init
    handledEventRef.current = state.eventTime
    pendingEventRef.current = state.eventTime
    const t = new Date(state.eventTime)
    setSelectedDate(new Date(t.getFullYear(), t.getMonth(), t.getDate()))
    setActivePanel('events')
  }, [location.state, isLiveRoute])

  // Runs only on mount: loads the recording indicated by the URL param so that
  // a direct navigation / page refresh restores the correct recording.
  // Must NOT re-run on internal navigations (openRecording calls replace: true
  // and changes recordingId, which would re-trigger a full reload mid-session).
  useEffect(() => {
    const recId = mountRecordingIdRef.current
    if (!recId || !id) return
    fetch(`/api/cameras/${id}/recordings/by-id/${recId}`, { headers: authHeaders() })
      .then(r => r.ok ? r.json() : null)
      .then((data: { filename: string; date: string } | null) => {
        if (!data) return
        pendingRecordingRef.current = { filename: data.filename }
        const [y, m, d] = data.date.split('-').map(Number)
        const date = new Date(y, m - 1, d)
        setSelectedDate(date)
        setCalendarMonth(new Date(y, m - 1, 1))
      })
      .catch(() => {})
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

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
      .then((list: Array<CameraSettings & { playback_lead_seconds?: number }> | null) => {
        const entry = list?.find(c => c.id === id)
        if (entry?.playback_lead_seconds) setPlaybackLeadSeconds(entry.playback_lead_seconds)
        if (entry) setViewerCam(entry)
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
      if (result === 401) { onUnauthorized(); return }
      setRecordingsDisplayPage(1)
      setRecordings(result.recordings)
      setRecordingsTotal(result.total)
      setMotionEvents(events)
      setEventsPage(1)
      setActiveEventTime(null)

      const pendingRec = pendingRecordingRef.current
      if (pendingRec) {
        pendingRecordingRef.current = null
        const rec = result.recordings.find(r => r.filename === pendingRec.filename)
        if (rec && !rec.is_recording) setActiveRecording(rec)
        else setActiveRecording(null)
      } else {
        setActiveRecording(null)
      }

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
  }, [selectedDate, id, sortOrder])

  useEffect(() => {
    const today = new Date()
    const isToday =
      selectedDate.getFullYear() === today.getFullYear() &&
      selectedDate.getMonth() === today.getMonth() &&
      selectedDate.getDate() === today.getDate()

    const interval = setInterval(async () => {
      const [result, events] = await Promise.all([
        loadRecordingsData(id!, selectedDate, 1, sortOrder, ALL_RECORDINGS_LIMIT),
        loadMotionEvents(id!, selectedDate),
      ])
      if (result === 401) { onUnauthorized(); return }
      setRecordings(prev => mergeRecordings(prev, result.recordings, sortOrder, result.hasMore))
      setRecordingsTotal(result.total)
      setMotionEvents(events)
    }, isToday ? 5_000 : 30_000)

    return () => clearInterval(interval)
  }, [selectedDate, id, sortOrder])

  const today = new Date()
  const isToday =
    selectedDate.getFullYear() === today.getFullYear() &&
    selectedDate.getMonth() === today.getMonth() &&
    selectedDate.getDate() === today.getDate()
  const calendarOnCurrentMonth =
    calendarMonth.getFullYear() === today.getFullYear() &&
    calendarMonth.getMonth() === today.getMonth()

  const handleLiveMotion = useCallback(() => {
    loadMotionEvents(id!, selectedDateRef.current).then(setMotionEvents)
  }, [id])

  useEventSource(
    isToday && id ? `/api/cameras/${id}/motion/live` : null,
    handleLiveMotion,
  )

  async function reloadRecordingsAndEvents() {
    const [result, events] = await Promise.all([
      loadRecordingsData(id!, selectedDate, 1, sortOrder, ALL_RECORDINGS_LIMIT),
      loadMotionEvents(id!, selectedDate),
    ])
    if (result === 401) { onUnauthorized(); return }
    setRecordings(result.recordings)
    setRecordingsTotal(result.total)
    setMotionEvents(events)
  }

  function openRecording(rec: Recording) {
    setActiveRecording(rec)
    if (rec.id) navigate(`/camera/recording/${id}/${rec.id}`, { replace: true })
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
      openRecording(recording)
    }
  }

  async function handleConfirmDelete() {
    if (!deleteTarget) return
    const target = deleteTarget
    setDeleteTarget(null)
    setDeleteError(null)
    const { ok, serverError } = await deleteRecording(id!, target.rec.filename)
    if (ok && activeRecording?.filename === target.rec.filename) setActiveRecording(null)
    if (serverError) setDeleteError('Erro ao excluir gravação. Tente novamente.')
    await reloadRecordingsAndEvents()
  }

  function findRecordingForEvent(ev: MotionEvent, recs: Recording[]): Recording | null {
    const evTime = new Date(ev.time).getTime()
    const asc = [...recs].sort((a, b) => a.filename.localeCompare(b.filename))
    for (let i = 0; i < asc.length; i++) {
      const recStart = new Date(asc[i].start).getTime()
      const nextStart = i + 1 < asc.length
        ? new Date(asc[i + 1].start).getTime()
        : recStart + 5 * 60 * 1000
      if (evTime >= recStart && evTime < nextStart) return asc[i]
    }
    return null
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
          openRecording(asc[i])
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

  const { settings } = useSettings()
  const motionPeak = useMotionPeak(id)
  const { markRead } = useNotifications()
  const setItems = useSetSidebarItems()
  const cam = settings?.cameras.find(c => c.id === id) ?? viewerCam
  const effectiveThreshold = cam?.motion?.threshold ?? 0

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


  const isLive = activeRecording === null && !recordingId

  useEffect(() => {
    setItems([])
    return () => setItems([])
  }, [setItems])

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
        x: debugDragRef.current.startPosX - (ev.clientX - debugDragRef.current.startMouseX),
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

  // Atualiza src do <video> imperativamente para evitar remonte do elemento DOM,
  // mantendo o último frame visível enquanto o novo vídeo carrega.
  useEffect(() => {
    const v = videoRef.current
    if (!v || !activeRecording) return
    setRecPlayBlocked(false)
    v.src = `${activeRecording.url}?token=${getToken()}`
    v.load()
  }, [activeRecording])

  const liveUrl = `/stream/${id}/index.m3u8`

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

  const sortedEvents = [...motionEvents]
    .filter(ev => recordings.length === 0 || findRecordingForEvent(ev, recordings) !== null)
    .sort((a, b) => {
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
  sortedEventsRef.current = sortedEvents
  continuousPlayRef.current = continuousPlay
  recordingsDisplayPageRef.current = recordingsDisplayPage
  eventsPageRef.current = eventsPage

  return (
    <AppLayout fill mainClassName="w-full p-3">
        <div className="flex h-full gap-0">
          {/* Coluna do player */}
          <div className={`relative flex flex-col min-w-0 h-full ${activePanel ? 'flex-1' : 'w-full'}`}>
            <div
              ref={playerRef}
              className={`flex flex-col h-full bg-gray-900 border rounded-lg overflow-hidden transition-all duration-300 ${
                !isLive ? 'border-blue-600 ring-1 ring-blue-600' : 'border-gray-800'
              }`}
            >
              <div className="flex-none flex items-center gap-2 px-4 py-2 border-b border-gray-700 min-w-0">
                <span className={`inline-flex items-center justify-center rounded px-2 py-1 text-[10px] font-bold leading-none shrink-0 ${isLive ? 'bg-red-600 text-white' : 'bg-gray-800 text-gray-400'}`}>
                  {isLive ? 'AO VIVO' : 'Reprodução'}
                </span>
                {!isLive && activeRecording && (() => {
                  const ev = activeEventIdx >= 0 ? visibleEvents[activeEventIdx] : null
                  return (
                    <span className="text-sm text-gray-300 min-w-0 truncate">
                      {cam?.name ?? id}
                      {' · '}
                      <span className="tabular-nums">{formatRecordingDateTime(activeRecording.start, timezone)}</span>
                      {recDuration > 0 && ` · ${formatRecTime(recDuration)}`}
                      {ev?.label && <span className="font-medium" style={{ color: ev.color ?? '#f97316' }}> · {ev.label}</span>}
                    </span>
                  )
                })()}
                {isLive && <span className="font-medium text-sm text-gray-200 truncate">{cam?.name ?? id}</span>}
                <div className="ml-auto shrink-0 flex items-center gap-1">
                  {/* Voltar ao vivo — visível só durante reprodução */}
                  {!isLive && (
                    <button
                      onClick={() => { setActiveRecording(null); navigate(`/camera/live/${id}`, { replace: true }); setActivePanel(null) }}
                      title="Voltar ao vivo"
                      className="p-1 transition-colors cursor-pointer text-gray-400 hover:text-gray-200"
                    >
                      <span className="text-[9px] font-bold leading-none tracking-wide">AO VIVO</span>
                    </button>
                  )}
                  {!isLive && <div className="w-px h-4 bg-gray-700 mx-0.5" />}
                  {/* Mute */}
                  <button
                    onClick={() => setVideoMuted(m => { const next = !m; if (videoRef.current) videoRef.current.muted = next; return next })}
                    title={videoMuted ? 'Ativar áudio' : 'Silenciar'}
                    className={`p-1 transition-colors cursor-pointer ${!videoMuted ? 'text-blue-400' : 'text-gray-400 hover:text-gray-200'}`}
                  >
                    {videoMuted ? (
                      <VolumeX className="w-[18px] h-[18px]" />
                    ) : (
                      <Volume2 className="w-[18px] h-[18px]" />
                    )}
                  </button>
                  {/* Speed dropdown — playback only */}
                  {!isLive && (
                    <div ref={speedMenuRef} className="relative">
                      <button
                        onClick={() => setSpeedMenuOpen(o => !o)}
                        title={`Velocidade ${playbackRate}×`}
                        className={`relative p-1 transition-colors cursor-pointer ${playbackRate > 1 ? 'text-blue-400' : 'text-gray-400 hover:text-gray-200'}`}
                      >
                        <Gauge className="w-4 h-4" />
                        {playbackRate > 1 && (
                          <span className="absolute -top-0.5 -right-0.5 min-w-[1.1rem] h-[1.1rem] flex items-center justify-center text-[9px] font-bold bg-gray-700 text-gray-200 rounded-full px-0.5">
                            {playbackRate}×
                          </span>
                        )}
                      </button>
                      {speedMenuOpen && (
                        <div className="absolute right-0 top-full mt-1 bg-gray-800 border border-gray-700 rounded shadow-lg z-50 py-1 min-w-[4rem]">
                          {[1, 2, 4, 8, 16, 32]
                            .filter(v => browserMaxRate === null || v <= browserMaxRate)
                            .map(v => (
                              <button
                                key={v}
                                onClick={() => { handleRateChange(v); setSpeedMenuOpen(false) }}
                                className={`w-full text-left px-3 py-1 text-xs ${v === playbackRate ? 'text-blue-400 font-semibold' : 'text-gray-300 hover:text-white hover:bg-gray-700'}`}
                              >
                                {v}×
                              </button>
                            ))}
                        </div>
                      )}
                    </div>
                  )}
                  {/* Continuous play — playback only */}
                  {!isLive && (
                    <button
                      onClick={() => setContinuousPlay(v => !v)}
                      title={continuousPlay ? 'Desativar reprodução contínua' : 'Ativar reprodução contínua'}
                      className={`p-1 transition-colors cursor-pointer ${continuousPlay ? 'text-blue-400' : 'text-gray-400 hover:text-gray-200'}`}
                    >
                      <Repeat className="w-4 h-4" />
                    </button>
                  )}
                  <div className="w-px h-4 bg-gray-700 mx-0.5" />
                  {/* Recordings */}
                  {(cam?.recording_enabled !== false) && (recordingsTotal > 0 || recordings.length > 0) && (
                    <button
                      onClick={() => {
                        const opening = activePanel !== 'recordings'
                        setActivePanel(p => p === 'recordings' ? null : 'recordings')
                        if (opening && recordings.length > 0) {
                          setActiveEventTime(null)
                          setActiveEventId(null)
                          openRecording(recordings[0])
                        }
                      }}
                      title="Gravações"
                      className={`relative p-1 transition-colors cursor-pointer ${activePanel === 'recordings' ? 'text-blue-400' : 'text-gray-400 hover:text-gray-200'}`}
                    >
                      <Film className="w-4 h-4" />
                      {(recordingsTotal || recordings.length) > 0 && (
                        <span className="absolute -top-0.5 -right-0.5 min-w-[1.1rem] h-[1.1rem] flex items-center justify-center text-[9px] font-bold bg-gray-700 text-gray-200 rounded-full px-0.5">
                          {recordingsTotal || recordings.length}
                        </span>
                      )}
                    </button>
                  )}
                  {/* Events */}
                  {sortedEvents.length > 0 && (
                    <button
                      onClick={() => {
                        const opening = activePanel !== 'events'
                        setActivePanel(p => p === 'events' ? null : 'events')
                        if (opening && sortedEvents.length > 0) {
                          playEventAt(sortedEvents[0])
                          setScrollNonce(n => n + 1)
                        }
                      }}
                      title="Eventos de movimento"
                      className={`relative p-1 transition-colors cursor-pointer ${activePanel === 'events' ? 'text-blue-400' : 'text-gray-400 hover:text-gray-200'}`}
                    >
                      <Zap className="w-4 h-4" />
                      <span className="absolute -top-0.5 -right-0.5 min-w-[1.1rem] h-[1.1rem] flex items-center justify-center text-[9px] font-bold bg-gray-700 text-gray-200 rounded-full px-0.5">
                        {sortedEvents.length}
                      </span>
                    </button>
                  )}
                  {/* Calendar */}
                  <button
                    onClick={() => setActivePanel(p => p === 'calendar' ? null : 'calendar')}
                    title={isToday ? 'Calendário' : `Calendário · ${format(selectedDate, "d MMM", { locale: ptBR })}`}
                    className={`p-1 transition-colors cursor-pointer ${activePanel === 'calendar' ? 'text-blue-400' : 'text-gray-400 hover:text-gray-200'}`}
                  >
                    <CalendarDays className="w-4 h-4" />
                  </button>
                  <div className="w-px h-4 bg-gray-700 mx-0.5" />
                  {/* Debug */}
                  <button
                    onClick={() => setShowDebug(d => !d)}
                    title="Debug"
                    className={`p-1 transition-colors cursor-pointer ${showDebug ? 'text-blue-400' : 'text-gray-400 hover:text-gray-200'}`}
                  >
                    <Code2 className="w-4 h-4" />
                  </button>
                  {/* Settings */}
                  {isAdmin && <button onClick={() => navigate(`/settings/cameras/${id}`, { state: { from: `/cameras/${id}`, editing: true } })} title="Configurar câmera" className="p-1 text-gray-400 hover:text-gray-200 transition-colors cursor-pointer">
                    <Settings className="w-4 h-4" />
                  </button>}
                  {/* Fullscreen */}
                  <button onClick={toggleFullscreen} title="Tela inteira" className="p-1 text-gray-400 hover:text-gray-200 transition-colors cursor-pointer">
                    <Maximize className="w-4 h-4" />
                  </button>
                </div>
              </div>

              {isLive ? (
                <HLSPlayer ref={hlsPlayerRef} src={liveUrl} containerClassName="flex-1 min-h-0" className="w-full h-full bg-black" cameraId={id} muted={videoMuted} segmentSeconds={cam?.hls_segment_seconds} onGoToEvent={handleGoToEvent} />
              ) : (
                <div
                  ref={recPlayerRef}
                  className="flex-1 min-h-0 relative"
                  onMouseMove={showRecControls}
                  onMouseLeave={() => { if (recPlaying) setRecControlsVisible(false) }}
                >
                  <video
                    ref={videoRef}
                    className="w-full h-full bg-black cursor-pointer"
                    playsInline
                    onClick={togglePlayRecording}
                    onPlay={() => { setRecPlaying(true); setRecPlayBlocked(false); setLastFrameDataUrl(null); showRecControls() }}
                    onPause={() => { setRecPlaying(false); setRecControlsVisible(true) }}
                    onCanPlay={() => setLastFrameDataUrl(null)}
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
                      e.currentTarget.play().catch((err: unknown) => {
                        if ((err as { name?: string })?.name === 'NotAllowedError') setRecPlayBlocked(true)
                      })
                    }}
                    onVolumeChange={e => setVideoMuted(e.currentTarget.muted)}
                    onEnded={() => {
                      if (!continuousPlayRef.current) return
                      // Captura último frame para overlay de transição
                      const v = videoRef.current
                      if (v && v.videoWidth > 0) {
                        try {
                          const canvas = document.createElement('canvas')
                          canvas.width = v.videoWidth
                          canvas.height = v.videoHeight
                          canvas.getContext('2d')?.drawImage(v, 0, 0)
                          setLastFrameDataUrl(canvas.toDataURL())
                        } catch { /* canvas bloqueado (CORS) — ignora */ }
                      }
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
                        openRecording(next)
                      }
                    }}
                  />
                  {/* Último frame: mantém imagem visível enquanto próxima gravação carrega */}
                  {lastFrameDataUrl && (
                    <img
                      src={lastFrameDataUrl}
                      className="absolute inset-0 w-full h-full object-contain bg-black pointer-events-none"
                      aria-hidden
                    />
                  )}
                  {/* Play bloqueado pelo browser: solicita gesto do usuário */}
                  {recPlayBlocked && (
                    <div className="absolute inset-0 flex items-center justify-center bg-black/60">
                      <button
                        className="flex items-center gap-2 bg-white/10 hover:bg-white/20 text-white px-6 py-3 rounded-full text-sm font-medium backdrop-blur-sm"
                        onClick={() => {
                          setRecPlayBlocked(false)
                          videoRef.current?.play().catch(() => {})
                        }}
                      >
                        <Play className="w-5 h-5 fill-current" />
                        Clique para continuar
                      </button>
                    </div>
                  )}
                  {/* Custom controls overlay — play/pause only; progress bar moved outside player */}
                  <div className={`absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/80 to-transparent px-3 pb-2 pt-8 transition-opacity duration-200 pointer-events-none ${recControlsVisible || !recPlaying ? 'opacity-100' : 'opacity-0'}`}>
                    <div className="flex items-center gap-2 pointer-events-auto">
                      <button onClick={togglePlayRecording} aria-label={recPlaying ? 'Pausar' : 'Reproduzir'} className="p-1 text-white/80 hover:text-white transition-colors">
                        {recPlaying ? (
                          <Pause className="w-4 h-4" />
                        ) : (
                          <Play className="w-4 h-4 fill-current" />
                        )}
                      </button>
                    </div>
                  </div>
                </div>
              )}
            </div>

            {/* Persistent progress bar — always visible during recording playback */}
            {!isLive && recDuration > 0 && (
              <div className="flex items-center gap-2 px-1 py-1.5">
                <span className="text-xs text-gray-500 tabular-nums shrink-0">{formatRecTime(recCurrentTime)}</span>
                <div
                  className="flex-1 h-1.5 rounded-full bg-gray-700 cursor-pointer relative group"
                  onClick={e => {
                    if (!videoRef.current || !recDuration) return
                    const rect = e.currentTarget.getBoundingClientRect()
                    const fraction = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width))
                    videoRef.current.currentTime = fraction * recDuration
                  }}
                >
                  <div
                    className="absolute inset-y-0 left-0 rounded-full bg-blue-500 pointer-events-none group-hover:bg-blue-400 transition-colors"
                    style={{ width: `${(recCurrentTime / recDuration) * 100}%` }}
                  />
                </div>
                <span className="text-xs text-gray-500 tabular-nums shrink-0">{formatRecTime(recDuration)}</span>
              </div>
            )}

            {showDebug && (
              <div
                style={{ right: debugPos.x, top: debugPos.y }}
                className="absolute z-30 bg-gray-900 border border-gray-700 rounded-lg shadow-xl select-none flex flex-col"
              >
                {/* Header — drag handle */}
                <div
                  className="flex items-center justify-between px-4 py-2 border-b border-gray-700 cursor-move"
                  onMouseDown={handleDebugDragStart}
                >
                  <span className="text-xs font-semibold text-gray-300 uppercase tracking-widest">Debug</span>
                  <button onClick={() => setShowDebug(false)} className="ml-6 text-gray-500 hover:text-white transition-colors">
                    <X className="w-3.5 h-3.5" />
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

          {/* Painel lateral condicional */}
          {activePanel && (
            <div className="w-72 shrink-0 border-l border-gray-800 bg-gray-900 flex flex-col h-full">
              {(activePanel === 'recordings' || activePanel === 'events' || activePanel === 'calendar') && (
                <>
                  <div className="flex items-center border-b border-gray-800 shrink-0">
                    <button
                      id="tab-recordings"
                      onClick={() => setActivePanel('recordings')}
                      className={`flex-1 flex flex-col items-center px-2 py-1.5 text-xs font-medium transition-colors ${
                        activePanel === 'recordings'
                          ? 'text-blue-400 border-b-2 border-blue-500 -mb-px'
                          : 'text-gray-500 hover:text-gray-300'
                      }`}
                    >
                      <span>Gravações</span>
                      <span className="tabular-nums">{recordingsTotal || recordings.length}</span>
                    </button>
                    <button
                      id="tab-events"
                      onClick={() => setActivePanel('events')}
                      className={`flex-1 flex flex-col items-center px-2 py-1.5 text-xs font-medium transition-colors ${
                        activePanel === 'events'
                          ? 'text-blue-400 border-b-2 border-blue-500 -mb-px'
                          : 'text-gray-500 hover:text-gray-300'
                      }`}
                    >
                      <span>Eventos</span>
                      <span className="tabular-nums">{sortedEvents.length}</span>
                    </button>
                    <button
                      id="tab-calendar"
                      onClick={() => setActivePanel('calendar')}
                      className={`flex-1 flex flex-col items-center px-2 py-1.5 text-xs font-medium transition-colors ${
                        activePanel === 'calendar'
                          ? 'text-blue-400 border-b-2 border-blue-500 -mb-px'
                          : 'text-gray-500 hover:text-gray-300'
                      }`}
                    >
                      <span>{format(selectedDate, "MMM", { locale: ptBR })}</span>
                      <span className="tabular-nums">{format(selectedDate, "d")}</span>
                    </button>
                    <button
                      onClick={() => setActivePanel(null)}
                      title="Fechar"
                      className="px-2.5 py-2 text-gray-600 hover:text-gray-300 transition-colors shrink-0"
                    >
                      <X className="w-3.5 h-3.5" />
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
                      emptyMessage={cam?.recording_enabled === false ? "Gravação desabilitada. Câmera disponível apenas ao vivo." : "Sem gravações nesta data."}
                    >
                      {(() => {
                        return displayedRecordings.map(rec => {
                          const isActive = activeRecording?.filename === rec.filename
                          const hasMotion = rec.has_motion
                          return (
                            <div
                              key={rec.filename}
                              ref={isActive ? el => { if (el) activeRecordingItemRef.current = el } : null}
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
                                onClick={() => { if (!rec.is_recording) { setActiveEventTime(null); setActiveEventId(null); openRecording(rec) } }}
                                className="flex-1 flex items-center justify-between text-left disabled:cursor-not-allowed"
                              >
                                <span className={`text-sm ${isActive && !rec.is_recording ? 'text-blue-300' : 'text-gray-300'}`}>
                                  {formatRecordingTime(rec.start, timezone)}
                                </span>
                                <div className="flex items-center gap-2 shrink-0 ml-2">
                                  {hasMotion && (
                                    <span className="w-2 h-2 rounded-full bg-orange-400" title="Movimento detectado" />
                                  )}
                                  {rec.is_recording
                                    ? <span className="text-xs text-red-400 font-medium">● REC</span>
                                    : <span className="text-xs text-gray-500">▶ MP4</span>
                                  }
                                </div>
                              </button>
                              {isAdmin && !rec.is_recording && (
                                <button
                                  onClick={() => setDeleteTarget({ rec, hasMotion })}
                                  title="Excluir gravação"
                                  className="ml-2 text-gray-600 hover:text-red-400 opacity-0 group-hover:opacity-100 transition-opacity"
                                >
                                  <Trash2 className="w-3.5 h-3.5" />
                                </button>
                              )}
                            </div>
                          )
                        })
                      })()}
                    </ListPanel>
                  ) : activePanel === 'events' ? (
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
                        const recDets = recordings.filter(r => r.start <= ev.time).sort((a, b) => b.start.localeCompare(a.start))[0]?.detections
                        return (
                          <button
                            key={ev.id ?? `${ev.time}-${i}`}
                            ref={isActive ? (el) => { if (el) activeEventItemRef.current = el } : null}
                            onClick={() => { playEventAt(ev); markRead(`${id}-${ev.time}`); setScrollNonce(n => n + 1) }}
                            className={`w-full flex flex-col px-3 py-2 transition-colors text-left ${
                              isActive ? 'bg-blue-900/40 border-l-2 border-blue-500' : 'hover:bg-gray-800'
                            }`}
                          >
                            <div className="flex items-center justify-between w-full gap-2">
                              <span className={`text-sm tabular-nums ${isActive ? 'text-blue-300' : 'text-gray-300'}`}>
                                {formatRecordingTime(ev.time, timezone)}
                              </span>
                              <div className="flex items-center gap-1.5 min-w-0">
                                {ev.label && (
                                  <>
                                    <span className="w-2 h-2 rounded-full shrink-0" style={{ backgroundColor: ev.color ?? '#fb923c' }} />
                                    <span className="text-xs font-medium truncate" style={{ color: ev.color ?? '#f97316' }}>{ev.label}</span>
                                  </>
                                )}
                                <span className="text-xs text-gray-500 shrink-0">[{(ev.score * 100).toFixed(1)}%]</span>
                              </div>
                            </div>
                            <div className="flex items-center justify-between gap-2 mt-1">
                              <span className="text-xs text-gray-400 truncate">
                                {ev.label ?? 'Movimento'}
                              </span>
                              {thumbURL && (
                                <img
                                  src={thumbURL}
                                  alt="snapshot"
                                  className="w-16 h-10 object-cover rounded cursor-zoom-in border border-gray-700 shrink-0"
                                  onClick={e => { e.stopPropagation(); setSnapshotEvent(ev) }}
                                />
                              )}
                            </div>
                            {recDets && recDets.length > 0 && (
                              <div className="flex flex-wrap gap-1 mt-1" onClick={e => { e.stopPropagation(); setDetectionModal(recDets) }}>
                                {recDets.map(d => (
                                  <span key={d.label} className="text-[10px] px-1.5 py-0.5 rounded bg-violet-900/60 text-violet-300 border border-violet-700/50 cursor-pointer hover:bg-violet-800/60">
                                    {d.label}
                                  </span>
                                ))}
                              </div>
                            )}
                          </button>
                        )
                      })}
                    </ListPanel>
                  ) : (
                    <div className="flex flex-col overflow-y-auto">
                      <div className="p-3">
                        <DayPicker
                          mode="single"
                          selected={selectedDate}
                          month={calendarMonth}
                          onMonthChange={setCalendarMonth}
                          onSelect={d => { if (d) { setSelectedDate(d); setCalendarMonth(d) } }}
                          locale={ptBR}
                          style={{ '--rdp-nav_button-width': '1.5rem', '--rdp-nav_button-height': '1.5rem', '--rdp-accent-color': '#94a3b8' } as React.CSSProperties}
                          components={{ Chevron: ({ orientation, disabled }) => {
                            const d = `M${orientation === 'left' ? '10 6 4 12 10 18' : orientation === 'right' ? '6 6 12 12 6 18' : orientation === 'up' ? '6 14 12 8 18 14' : '6 10 12 16 18 10'}`
                            return <svg xmlns="http://www.w3.org/2000/svg" width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" opacity={disabled ? 0.5 : 1}><path d={d} /></svg>
                          }}}
                          footer={(!isToday || !calendarOnCurrentMonth) && (
                            <div className="flex justify-center pt-1">
                              <button onClick={() => { setSelectedDate(new Date()); setCalendarMonth(new Date()) }} className="text-xs font-medium text-blue-400 hover:text-blue-300 transition-colors">
                                Hoje
                              </button>
                            </div>
                          )}
                          classNames={{
                            root: 'text-gray-200 text-sm',
                            month_caption: 'text-base text-gray-200 font-medium',
                            month_grid: 'mt-2',
                            day: 'text-gray-300 hover:bg-gray-700 rounded',
                            day_button: 'w-8 h-8 flex items-center justify-center rounded',
                            selected: 'bg-blue-600 text-white rounded',
                            today: 'text-blue-400 font-semibold',
                            outside: 'text-gray-600',
                            disabled: 'text-gray-700',
                          }}
                        />
                      </div>
                    </div>
                  )}
                </>
              )}
            </div>
          )}

          <VerticalTimeline
            recordings={recordings}
            motionEvents={motionEvents}
            activeRecording={activeRecording}
            activeTime={activeEventTime ?? activeRecording?.start ?? null}
            timezone={timezone}
            onSeek={handleTimelineSeek}
            onEventClick={activePanel === 'events' ? ev => { playEventAt(ev); markRead(`${id}-${ev.time}`); setScrollNonce(n => n + 1) } : undefined}
            maxHeight={playerHeight}
          />
        </div>

      {detectionModal && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
          onClick={() => setDetectionModal(null)}
        >
          <div
            className="bg-gray-900 border border-gray-700 rounded-xl shadow-2xl w-72 p-5"
            onClick={e => e.stopPropagation()}
          >
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-sm font-semibold text-gray-200">Detecções de objetos</h3>
              <button
                onClick={() => setDetectionModal(null)}
                className="text-gray-500 hover:text-gray-300 text-lg leading-none"
              >✕</button>
            </div>
            <div className="space-y-2">
              {detectionModal.map(d => (
                <div key={d.label} className="flex items-center justify-between">
                  <span className="text-sm text-violet-300 font-medium">{d.label}</span>
                  <div className="flex items-center gap-3 text-xs text-gray-400">
                    <span>{(d.confidence * 100).toFixed(1)}%</span>
                    <span>{d.frame_count} frames</span>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {snapshotEvent && snapshotEvent.frame && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/80"
          onClick={closeSnapshotModal}
        >
          <div className="relative max-w-3xl w-full mx-4" onClick={e => e.stopPropagation()}>
            <div className="absolute -top-8 left-0 right-0 flex justify-between items-center">
              <button
                className={`text-xs px-2 py-0.5 rounded border ${annotating ? 'border-blue-500 text-blue-400' : 'border-gray-600 text-gray-400 hover:text-white'}`}
                onClick={() => setAnnotating(v => !v)}
              >
                {annotating ? 'Cancelar anotação' : 'Anotar'}
              </button>
              <button
                className="text-gray-400 hover:text-white text-sm"
                onClick={closeSnapshotModal}
              >
                Fechar ✕
              </button>
            </div>

            {/* Image + annotation canvas */}
            <div
              className="relative select-none"
              onMouseDown={annotating ? onAnnMouseDown : undefined}
              onMouseMove={annotating ? onAnnMouseMove : undefined}
              onMouseUp={annotating ? onAnnMouseUp : undefined}
            >
              <img
                ref={annImgRef}
                src={snapshotURL(id!, snapshotEvent.time, snapshotEvent.frame)}
                alt="snapshot de movimento"
                className={`w-full rounded-lg border border-gray-700 ${annotating ? 'cursor-crosshair' : ''}`}
                draggable={false}
              />
              {/* Saved annotations overlay */}
              {annotations.map(a => (
                  <div
                    key={a.id}
                    className="absolute border-2 border-green-400 pointer-events-none"
                    style={{
                      left: `${a.bbox_x * 100}%`,
                      top: `${a.bbox_y * 100}%`,
                      width: `${a.bbox_w * 100}%`,
                      height: `${a.bbox_h * 100}%`,
                    }}
                  >
                    <span className="absolute -top-5 left-0 text-xs text-green-300 bg-black/60 px-1">{a.label}</span>
                  </div>
              ))}
              {/* Draft bbox */}
              {annDraft && annDraft.w > 0 && annDraft.h > 0 && (
                <div
                  className="absolute border-2 border-blue-400 bg-blue-400/10 pointer-events-none"
                  style={{
                    left: `${annDraft.x * 100}%`,
                    top: `${annDraft.y * 100}%`,
                    width: `${annDraft.w * 100}%`,
                    height: `${annDraft.h * 100}%`,
                  }}
                />
              )}
            </div>

            <p className="mt-2 text-xs text-gray-400 text-center">
              {formatRecordingTime(snapshotEvent.time, timezone)} — score: {(snapshotEvent.score * 100).toFixed(1)}%
            </p>

            {/* Annotation controls */}
            {annotating && (
              <div className="mt-3 space-y-2">
                {annDraft && annDraft.w > 0.01 && annDraft.h > 0.01 && (
                  <div className="flex gap-2">
                    <input
                      className="flex-1 bg-gray-800 border border-gray-600 rounded px-2 py-1 text-sm text-white placeholder-gray-500 focus:outline-none focus:border-blue-500"
                      placeholder="Label (ex: gato, pessoa…)"
                      value={annLabel}
                      onChange={e => setAnnLabel(e.target.value)}
                      onKeyDown={e => { if (e.key === 'Enter') saveAnnotation() }}
                      autoFocus
                    />
                    <button
                      className="px-3 py-1 bg-blue-600 hover:bg-blue-500 text-white text-sm rounded disabled:opacity-40"
                      disabled={annLabel.trim() === ''}
                      onClick={saveAnnotation}
                    >
                      Salvar
                    </button>
                  </div>
                )}
                {!annDraft && (
                  <p className="text-xs text-gray-500 text-center">Arraste para desenhar a bounding box</p>
                )}
                {annotations.length > 0 && (
                  <div className="mt-2 space-y-1">
                    {annotations.map(a => (
                      <div key={a.id} className="flex items-center justify-between bg-gray-800 rounded px-2 py-1">
                        <span className="text-sm text-green-300">{a.label}</span>
                        <button
                          className="text-gray-500 hover:text-red-400 text-xs"
                          onClick={() => deleteAnnotation(a.id)}
                        >
                          remover
                        </button>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      )}

      {deleteError && (
        <div className="fixed bottom-4 left-1/2 -translate-x-1/2 z-50 px-4 py-2 bg-red-900/90 border border-red-700 rounded text-sm text-red-200 shadow-lg">
          {deleteError}
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
