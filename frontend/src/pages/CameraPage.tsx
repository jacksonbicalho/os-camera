import { useCallback, useEffect, useState, useRef } from 'react'
import { useParams, useNavigate, useLocation } from 'react-router-dom'
import { VolumeX, Volume2, Gauge, Repeat, Zap, Code2, Maximize, Play, Pause, X, CameraCapture, ZoomOut } from '../components/Icons'
import { format } from 'date-fns'
import Calendar from '../components/Calendar'
import { authHeaders, onUnauthorized, getToken, getRole } from '../auth'
import AppLayout from '../components/AppLayout'
import ConfirmDialog from '../components/ConfirmDialog'
import HLSPlayer, { type HLSPlayerHandle } from '../components/HLSPlayer'
import ListPanel from '../components/ListPanel'
import MotionScoreChart from '../components/MotionScoreChart'
import { useScrollToPlayer } from '../hooks/useScrollToPlayer'
import { usePlayerZoom } from '../hooks/usePlayerZoom'
import { useEventSource } from '../hooks/useEventSource'
import { useSettings, type CameraSettings } from '../hooks/useSettings'
import { useMotionPeak } from '../hooks/useMotionPeak'
import { useEscapeKey } from '../hooks/useEscapeKey'
import { useDebugTools } from '../hooks/useDebugTools'
import { applyFrameStep, applySameChunkStep, loadedMetadataSeek, mergeRecordings, secondStepTarget } from './cameraUtils'
import type { Recording, MotionEvent } from './cameraUtils'
import VerticalTimeline from '../components/VerticalTimeline'
import BboxCanvas, { type BboxRect } from '../components/BboxCanvas'
import CameraConfigMenu from '../components/CameraConfigMenu'
import PlayerTitle from '../components/PlayerTitle'
import CameraSwitcher from '../components/CameraSwitcher'
import EventFilterChips from '../components/EventFilterChips'
import EventDetailCard from '../components/EventDetailCard'
import { filterEventsByCategory, eventCategory, eventTitle, type EventFilter } from './eventCategory'
import { activeEventForPlayhead } from './activeEvent'
import HorizontalTimeline from '../components/HorizontalTimeline'
import Filmstrip from '../components/Filmstrip'
import { timelineWindow, type TimelineRange } from '../components/timelineScale'
import { zoneThresholdLabel } from './settings/zoneThreshold'
import { adjacentRecording, filterRecordings, nextRecording } from './recordingsFilter'
import { videoDownloadName } from './videoDownload'
import { recordingsForEventWindow } from './eventRecordings'
import { useMarkActiveEventRead } from '../hooks/useMarkActiveEventRead'
import { useSetSidebarItems } from '../contexts/SidebarContext'
import { useDisplayMode } from '../contexts/DisplayModeContext'
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
  const [, setRecordingsTotal] = useState(0)
  const [activeRecording, setActiveRecording] = useState<Recording | null>(null)
  const [sortOrder] = useState<'asc' | 'desc'>('desc')
  const [motionEvents, setMotionEvents] = useState<MotionEvent[]>([])
  const [activePanel, setActivePanel] = useState<null | 'events' | 'timeline'>(() => {
    if (recordingId) return 'events'
    if (isLiveRoute) return null
    const state = location.state as { eventTime?: string; showRecordings?: boolean } | null
    if (state?.showRecordings) return 'events'
    return state?.eventTime ? 'events' : null
  })
  const [eventsPage, setEventsPage] = useState(1)
  const [eventFilter, setEventFilter] = useState<EventFilter>('todos')
  const [timelineRange, setTimelineRange] = useState<TimelineRange>('24h')
  const [eventsSortOrder, setEventsSortOrder] = useState<'asc' | 'desc'>('desc')
  const [activeEventTime, setActiveEventTime] = useState<string | null>(null)
  const [activeEventId, setActiveEventId] = useState<number | null>(null)
  const [scrollNonce, setScrollNonce] = useState(0)
  const [recordingsDisplayPage, setRecordingsDisplayPage] = useState(1)
  const [onlyMotion] = useState(false)
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
  const [annLabel, setAnnLabel] = useState('')
  const [annSaving, setAnnSaving] = useState(false)
  const [annSaveOk, setAnnSaveOk] = useState(false)
  const [annBox, setAnnBox] = useState<BboxRect | null>(null)
  const [existingAnn, setExistingAnn] = useState<BboxRect | null>(null)
  const [existingAnnId, setExistingAnnId] = useState<number | null>(null)
  const [existingAnnLabel, setExistingAnnLabel] = useState('')
  const [detectionModal, setDetectionModal] = useState<Array<{ label: string; confidence: number; frame_count: number; custom_model?: boolean }> | null>(null)
  const [pendingSnapBlob, setPendingSnapBlob] = useState<{ blob: Blob; eventId: number } | null>(null)
  const [thumbCacheBust, setThumbCacheBust] = useState<Map<number, number>>(new Map())
  const [thumbOverrides, setThumbOverrides] = useState<Map<number, string>>(new Map())
  const [thumbFlash, setThumbFlash] = useState<Set<number>>(new Set())
  const [deleteTarget, setDeleteTarget] = useState<{ rec: Recording; hasMotion: boolean } | null>(null)
  const [deleteError, setDeleteError] = useState<string | null>(null)
  // Painel de Debug + ferramentas efêmeras ("Analisar limiar" desenha uma região sobre
  // o vídeo ao vivo e mostra o score/limiar em tempo real). closeDebug reseta tudo.
  const {
    showDebug, setShowDebug, closeDebug,
    showDebugChart, setShowDebugChart,
    analyzeMode, setAnalyzeMode,
    analyzeBox, setAnalyzeBox,
    analyzeScore, setAnalyzeScore,
  } = useDebugTools()
  const [speedMenuOpen, setSpeedMenuOpen] = useState(false)
  const [debugStats, setDebugStats] = useState<{ fps: number; dropped: number; hlsStats: HLSStats | null; lagMs: number } | null>(null)
  const [debugPos, setDebugPos] = useState({ x: 8, y: 8 })
  // When the timeline pointer is parked over a gap, the UTC ms with no recording.
  const [noRecordingAt, setNoRecordingAt] = useState<number | null>(null)
  const debugDragRef = useRef<{ startMouseX: number; startMouseY: number; startPosX: number; startPosY: number } | null>(null)
  const speedMenuRef = useRef<HTMLDivElement>(null)
  const playerRef = useRef<HTMLDivElement>(null)
  const pendingSeekRef = useRef<number | null>(null)
  // Seconds-from-end pending seek (Ctrl+Shift+Down crossing into the previous chunk).
  const pendingSeekFromEndRef = useRef<number | null>(null)
  // Sinaliza que o próximo load veio de um passo Ctrl+Shift+seta que cruzou a
  // fronteira do chunk: o vídeo deve carregar parado, sem autoplay.
  const stepPauseRef = useRef(false)
  const videoRef = useRef<HTMLVideoElement>(null)
  // Duração de 1 frame da gravação ativa, estimada via requestVideoFrameCallback
  // (fallback ~1/30s). Usada pelo passo frame a frame (←/→).
  const frameDurationRef = useRef(1 / 30)
  const recPlayerRef = useRef<HTMLDivElement>(null)
  const recHideTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const hlsPlayerRef = useRef<HLSPlayerHandle>(null)
  const pendingLiveSeekRef = useRef<string | null>(null)
  const activeEventItemRef = useRef<HTMLButtonElement | null>(null)
  const activeRecordingItemRef = useRef<HTMLDivElement | null>(null)
  const skipNextRecordingScrollRef = useRef(false)
  const recordingsRef = useRef(recordings)
  const activeRecordingRef = useRef<Recording | null>(null)
  const activeEventTimeRef = useRef(activeEventTime)
  const activeEventIdRef = useRef(activeEventId)
  const allMotionEventsRef = useRef(motionEvents)
  const selectedDateRef = useRef(selectedDate)
  const visibleEventsRef = useRef<typeof visibleEvents>([])
  const continuousPlayRef = useRef(continuousPlay)
  const onlyMotionRef = useRef(onlyMotion)
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

  // Digital zoom no player: aplica o transform no <video> ativo (live ou gravação).
  const getActiveVideo = useCallback(
    () => videoRef.current ?? hlsPlayerRef.current?.getVideoElement() ?? null,
    [],
  )
  const playerZoom = usePlayerZoom(getActiveVideo)
  // Combina o ref do wrapper de gravação (controles) com o do zoom.
  const recContainerRef = useCallback((node: HTMLDivElement | null) => {
    recPlayerRef.current = node
    playerZoom.setContainer(node)
  }, [playerZoom])

  useEffect(() => { recordingsRef.current = recordings }, [recordings])
  useEffect(() => { activeRecordingRef.current = activeRecording }, [activeRecording])
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
    setAnnLabel('')
    setAnnBox(null)
    setAnnSaveOk(false)
    setExistingAnn(null)
    setExistingAnnId(null)
    setExistingAnnLabel('')
  }

  useEscapeKey(closeSnapshotModal, !!snapshotEvent)
  useEscapeKey(closeDebug, showDebug)
  useEscapeKey(() => setDetectionModal(null), detectionModal !== null)

  function openSnapshotModal(ev: MotionEvent) {
    setAnnBox(null)
    setAnnSaveOk(false)
    setExistingAnn(null)
    setExistingAnnId(null)
    setExistingAnnLabel('')
    setAnnLabel(ev.label ?? '')
    setSnapshotEvent(ev)
  }

  useEffect(() => {
    if (!snapshotEvent?.id) return
    fetch(`/api/events/${snapshotEvent.id}/annotations`, { headers: authHeaders() })
      .then(r => r.json())
      .then((list: Array<{ id: number; label: string; bbox_x: number; bbox_y: number; bbox_w: number; bbox_h: number; rotation_deg?: number }>) => {
        const a = list[0]
        if (a) {
          setExistingAnnId(a.id)
          setExistingAnnLabel(a.label ?? '')
          setExistingAnn({ x: a.bbox_x - a.bbox_w / 2, y: a.bbox_y - a.bbox_h / 2, w: a.bbox_w, h: a.bbox_h, rotation_deg: a.rotation_deg })
          if (a.label) setAnnLabel(a.label)
        }
      })
      .catch(() => {})
  }, [snapshotEvent])

  function handleAnnBoxChange(box: BboxRect | null) {
    if (box === null) {
      setAnnBox(null)
      setAnnSaveOk(false)
      if (existingAnn !== null) {
        setExistingAnn(null)
        setExistingAnnId(null)
        setExistingAnnLabel('')
        if (existingAnnId !== null) deleteAnnotation()
      }
      return
    }
    setAnnBox(box)
    setAnnSaveOk(false)
  }

  function downloadRecording(rec: Recording) {
    // <a download> streams to disk (better than blob for large MP4s). Same-origin
    // so the download attribute (filename) is honored.
    const a = document.createElement('a')
    a.href = `${rec.url}?token=${getToken()}`
    a.download = videoDownloadName(cam?.name, rec.start)
    document.body.appendChild(a)
    a.click()
    a.remove()
  }

  function downloadVideo() {
    if (activeRecording) downloadRecording(activeRecording)
  }

  // Baixa a(s) gravação(ões) cujo range cruza a janela do evento [occurred−lead, +trail].
  function downloadEventVideos(ev: MotionEvent) {
    const lead = cam?.motion?.playback_lead_seconds ?? 0
    const trail = cam?.motion?.playback_trail_seconds ?? 0
    recordingsForEventWindow(recordings, ev.time, lead, trail).forEach(downloadRecording)
  }

  function downloadBlob(blob: Blob, filename: string) {
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    a.click()
    URL.revokeObjectURL(url)
  }

  function takeSnapshot() {
    const video = isLive
      ? hlsPlayerRef.current?.getVideoElement() ?? null
      : videoRef.current
    if (!video || video.videoWidth === 0) return
    const canvas = document.createElement('canvas')
    canvas.width = video.videoWidth
    canvas.height = video.videoHeight
    canvas.getContext('2d')?.drawImage(video, 0, 0)
    const cameraName = (cam?.name ?? id ?? 'camera').replace(/[^a-zA-Z0-9]/g, '_')
    const ts = isLive
      ? new Date().toISOString()
      : new Date(new Date(activeRecording?.start ?? 0).getTime() + recCurrentTime * 1000).toISOString()
    const filename = `${cameraName}_${ts.replace(/[:.]/g, '-')}.jpg`
    let eventId = activeEventIdRef.current
    if (eventId === null && activeRecording) {
      const currentMs = new Date(activeRecording.start).getTime() + recCurrentTime * 1000
      const nearest = allMotionEventsRef.current
        .filter(e => {
          const t = new Date(e.time).getTime()
          return t <= currentMs && currentMs - t < 60_000
        })
        .sort((a, b) => new Date(b.time).getTime() - new Date(a.time).getTime())[0]
      eventId = nearest?.id ?? null
    }
    canvas.toBlob(blob => {
      if (!blob) return
      if (eventId !== null) {
        setPendingSnapBlob({ blob, eventId })
      } else {
        downloadBlob(blob, filename)
      }
    }, 'image/jpeg', 0.92)
  }

  function replaceEventThumb() {
    if (!pendingSnapBlob) return
    const { blob, eventId } = pendingSnapBlob
    setPendingSnapBlob(null)

    const blobUrl = URL.createObjectURL(blob)
    setThumbOverrides(prev => new Map(prev).set(eventId, blobUrl))

    fetch(`/api/events/${eventId}/frame`, {
      method: 'PUT',
      headers: { ...authHeaders(), 'Content-Type': 'image/jpeg' },
      body: blob,
    }).then(() => {
      URL.revokeObjectURL(blobUrl)
      setThumbOverrides(prev => { const m = new Map(prev); m.delete(eventId); return m })
      const bust = Date.now()
      setThumbCacheBust(prev => new Map(prev).set(eventId, bust))
      setThumbFlash(prev => new Set(prev).add(eventId))
      setTimeout(() => setThumbFlash(prev => { const s = new Set(prev); s.delete(eventId); return s }), 900)
    }).catch(() => {
      URL.revokeObjectURL(blobUrl)
      setThumbOverrides(prev => { const m = new Map(prev); m.delete(eventId); return m })
    })
  }

  function downloadPendingSnap() {
    if (!pendingSnapBlob) return
    const { blob } = pendingSnapBlob
    const cameraName = (cam?.name ?? id ?? 'camera').replace(/[^a-zA-Z0-9]/g, '_')
    const filename = `${cameraName}_${new Date().toISOString().replace(/[:.]/g, '-')}.jpg`
    downloadBlob(blob, filename)
    setPendingSnapBlob(null)
  }

  async function saveAnnotation() {
    if (!snapshotEvent?.id || !annBox || annBox.w < 0.01 || annBox.h < 0.01) return
    setAnnSaving(true)
    try {
      const payload = {
        label: annLabel.trim(),
        bbox_x: annBox.x + annBox.w / 2,
        bbox_y: annBox.y + annBox.h / 2,
        bbox_w: annBox.w,
        bbox_h: annBox.h,
        rotation_deg: annBox.rotation_deg ?? 0,
      }
      let res: Response
      if (existingAnnId !== null) {
        res = await fetch(`/api/annotations/${existingAnnId}`, {
          method: 'PATCH',
          headers: { ...authHeaders(), 'Content-Type': 'application/json' },
          body: JSON.stringify(payload),
        })
      } else {
        res = await fetch(`/api/events/${snapshotEvent.id}/annotations`, {
          method: 'POST',
          headers: { ...authHeaders(), 'Content-Type': 'application/json' },
          body: JSON.stringify(payload),
        })
        if (res.ok) {
          const data = await res.json()
          setExistingAnnId(data.id)
        }
      }
      if (!res.ok) return
      const currentEventLabel = snapshotEvent.label ?? ''
      if (annLabel.trim() && annLabel.trim() !== currentEventLabel) {
        await fetch(`/api/events/${snapshotEvent.id}/label`, {
          method: 'PATCH',
          headers: { ...authHeaders(), 'Content-Type': 'application/json' },
          body: JSON.stringify({ label: annLabel.trim() }),
        })
      }
      setExistingAnnLabel(annLabel.trim())
      setExistingAnn({ ...annBox })
      setAnnBox(null)
      setAnnSaveOk(true)
      setTimeout(() => setAnnSaveOk(false), 2000)
    } finally {
      setAnnSaving(false)
    }
  }

  async function deleteAnnotation() {
    if (!snapshotEvent?.id) return
    const r = await fetch(`/api/events/${snapshotEvent.id}/annotations`, { method: 'DELETE', headers: authHeaders() })
    if (r.ok) {
      setExistingAnn(null)
      setExistingAnnId(null)
      setExistingAnnLabel('')
      setAnnLabel('')
    }
  }

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      // Ctrl+←/→: passo frame a frame na gravação ativa.
      if (e.ctrlKey && !e.shiftKey && !e.altKey && !e.metaKey &&
          (e.key === 'ArrowLeft' || e.key === 'ArrowRight')) {
        const t = e.target as HTMLElement | null
        if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable)) return
        const v = videoRef.current
        if (!v || !activeRecording) return
        e.preventDefault()
        const dir: 1 | -1 = e.key === 'ArrowRight' ? 1 : -1
        const res = applyFrameStep(v, recDuration, frameDurationRef.current, dir)
        if (res.kind === 'applied' || res.kind === 'busy') return
        // chegou na borda do chunk → carrega o vizinho (próximo/anterior visível)
        const sorted = [...filterRecordings(recordingsRef.current, onlyMotionRef.current)]
          .sort((a, b) => a.filename.localeCompare(b.filename))
        const curIdx = sorted.findIndex(r => r.filename === activeRecording.filename)
        if (curIdx === -1) return
        let target: Recording | null = null
        if (res.kind === 'cross-forward') {
          for (let i = curIdx + 1; i < sorted.length; i++) if (!sorted[i].is_recording) { target = sorted[i]; break }
          if (!target) return
          pendingSeekRef.current = res.overflow
        } else {
          for (let i = curIdx - 1; i >= 0; i--) if (!sorted[i].is_recording) { target = sorted[i]; break }
          if (!target) return
          pendingSeekFromEndRef.current = res.overflow
        }
        setActiveEventTime(null)
        setActiveEventId(null)
        stepPauseRef.current = true
        openRecording(target)
        setScrollNonce(n => n + 1)
        return
      }
      if (!e.ctrlKey) return
      if (e.key !== 'ArrowUp' && e.key !== 'ArrowDown') return
      e.preventDefault()
      const recs = recordingsRef.current
      if (recs.length === 0) return
      const onlyMotion = onlyMotionRef.current
      // Navegação (Ctrl+seta e Ctrl+Shift+seta) percorre apenas a lista visível:
      // com o filtro ligado, só gravações com movimento.
      const sorted = [...filterRecordings(recs, onlyMotion)].sort((a, b) => a.filename.localeCompare(b.filename))

      // Ctrl+Shift+seta: navega segundo a segundo (↑ avança, ↓ retrocede).
      if (e.shiftKey) {
        const v = videoRef.current
        if (!v || !activeRecording) return
        const dir: 1 | -1 = e.key === 'ArrowUp' ? 1 : -1
        const target = secondStepTarget(sorted, activeRecording.filename, v.currentTime, v.duration || recDuration, dir)
        if (!target) return
        if (target.kind === 'same') { applySameChunkStep(v, target.time); return }
        setActiveEventTime(null)
        setActiveEventId(null)
        if (target.fromEnd) pendingSeekFromEndRef.current = target.offsetSeconds
        else pendingSeekRef.current = target.offsetSeconds
        stepPauseRef.current = true
        openRecording(target.rec)
        setScrollNonce(n => n + 1)
        return
      }

      if (!activeRecording) return
      const next = adjacentRecording(recs, activeRecording.filename, e.key, sortOrder, onlyMotion)
      if (!next) return
      const labeledEv = firstLabeledEventForRecording(next, recordingsRef.current, sortedEventsRef.current)
      setActiveEventTime(labeledEv?.time ?? null)
      setActiveEventId(labeledEv?.id ?? null)
      openRecording(next)
      setScrollNonce(n => n + 1)
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [activeRecording, sortOrder, recDuration, openRecording])

  // Estima a duração de 1 frame da gravação ativa via requestVideoFrameCallback
  // (delta de mediaTime entre frames apresentados durante a reprodução). Cacheia
  // em frameDurationRef para o passo frame a frame; fallback mantém 1/30s.
  useEffect(() => {
    const v = videoRef.current
    if (!v || typeof v.requestVideoFrameCallback !== 'function') return
    let last = -1
    let handle = 0
    const cb: VideoFrameRequestCallback = (_now, meta) => {
      if (last >= 0) {
        const d = meta.mediaTime - last
        if (d > 0.001 && d < 1) frameDurationRef.current = d
      }
      last = meta.mediaTime
      handle = v.requestVideoFrameCallback(cb)
    }
    handle = v.requestVideoFrameCallback(cb)
    return () => { v.cancelVideoFrameCallback?.(handle) }
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
    const state = location.state as { eventTime?: string; showRecordings?: boolean } | null
    if (!state?.eventTime) return
    if (handledEventRef.current === state.eventTime) return // already handled by lazy init
    handledEventRef.current = state.eventTime
    pendingEventRef.current = state.eventTime
    const t = new Date(state.eventTime)
    setSelectedDate(new Date(t.getFullYear(), t.getMonth(), t.getDate()))
    // Aba Gravações foi removida; histórico de estado e eventos de movimento
    // abrem o painel de eventos.
    setActivePanel('events')
  }, [location.state, isLiveRoute])

  // Sidebar camera link: navigate to same URL with goLive state to reset to live mode
  const handledGoLiveRef = useRef(0)
  useEffect(() => {
    const state = location.state as { goLive?: number } | null
    if (!state?.goLive) return
    if (handledGoLiveRef.current === state.goLive) return
    handledGoLiveRef.current = state.goLive
    setActiveRecording(null)
    setActivePanel(null)
  }, [location.state])

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
        if (rec && !rec.is_recording) {
          const labeledEv = firstLabeledEventForRecording(rec, result.recordings, events)
          setActiveEventTime(labeledEv?.time ?? null)
          setActiveEventId(labeledEv?.id ?? null)
          setActiveRecording(rec)
        } else setActiveRecording(null)
      } else {
        setActiveRecording(null)
      }

      const pendingTime = pendingEventRef.current
      if (pendingTime) {
        pendingEventRef.current = null
        const ev = events.find(e => e.time === pendingTime)
        if (ev) playEventAt(ev, result.recordings)
        // Histórico de estado: não há MotionEvent nesse instante — busca a gravação
        // que cobre o timestamp e dá seek direto (senão ficaria no ao vivo).
        else seekToRecordingTime(pendingTime, result.recordings)
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
  // Fim da janela da timeline horizontal: agora (se hoje) ou fim do dia selecionado.
  const timelineEndOfDayMs = (() => {
    const d = new Date(selectedDate)
    d.setHours(23, 59, 59, 999)
    return d.getTime()
  })()
  // Início da gravação mais recente do dia carregado — fallback do ponteiro quando
  // não há reprodução ativa (mesmo critério da VerticalTimeline: ponteiro sempre
  // visível e pronto para arrastar).
  const latestRecordingMs = recordings.reduce<number | undefined>((max, r) => {
    const ms = Date.parse(r.start)
    if (Number.isNaN(ms)) return max
    return max === undefined || ms > max ? ms : max
  }, undefined)

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
    setNoRecordingAt(null)
    setActiveRecording(rec)
    if (rec.id) navigate(`/camera/recording/${id}/${rec.id}`, { replace: true })
  }

  function handleTimelineSeek(recording: Recording, offsetSeconds: number) {
    setNoRecordingAt(null)
    setActiveEventTime(null)
    setActiveEventId(null)
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

  // Lightweight preview while dragging the timeline pointer: seek to the exact moment
  // under the pointer without switching panels, scrolling or forcing play. Uses a ref
  // for the loaded recording so rapid drags within a chunk don't reload the video.
  function handleTimelineScrub(recording: Recording, offsetSeconds: number) {
    setNoRecordingAt(null)
    if (activeRecordingRef.current?.filename === recording.filename) {
      if (videoRef.current && videoRef.current.readyState >= 1) {
        videoRef.current.currentTime = offsetSeconds
      } else {
        pendingSeekRef.current = offsetSeconds
      }
    } else {
      activeRecordingRef.current = recording
      pendingSeekRef.current = offsetSeconds
      openRecording(recording)
    }
  }

  // Pointer is over a position with no recording: pause and show a message.
  function handleTimelineGap(timestampMs: number) {
    videoRef.current?.pause()
    setNoRecordingAt(timestampMs)
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

  function firstLabeledEventForRecording(rec: Recording, allRecs: Recording[], events: MotionEvent[]): MotionEvent | null {
    if (!rec.has_motion) return null
    const asc = [...allRecs].sort((a, b) => a.filename.localeCompare(b.filename))
    const idx = asc.findIndex(r => r.filename === rec.filename)
    if (idx === -1) return null
    const recStart = new Date(rec.start).getTime()
    const nextStart = idx + 1 < asc.length
      ? new Date(asc[idx + 1].start).getTime()
      : recStart + 5 * 60 * 1000
    const inRange = events.filter(ev => {
      const t = new Date(ev.time).getTime()
      return t >= recStart && t < nextStart
    })
    return inRange.find(ev => !!ev.label) ?? inRange.find(ev => !!ev.frame) ?? null
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

  // seekToRecordingTime resolve a gravação que cobre `timeISO` e dá seek nela (com
  // lead de playbackLeadSeconds). Núcleo compartilhado por playEventAt (evento de
  // movimento) e pelo deep-link de histórico de estado (que não tem MotionEvent
  // correspondente). Retorna true quando achou gravação cobrindo o instante.
  function seekToRecordingTime(timeISO: string, recs: Recording[]): boolean {
    const evTime = new Date(timeISO).getTime()
    const asc = [...recs].sort((a, b) => a.filename.localeCompare(b.filename))
    for (let i = 0; i < asc.length; i++) {
      const recStart = new Date(asc[i].start).getTime()
      const nextStart = i + 1 < asc.length
        ? new Date(asc[i + 1].start).getTime()
        : recStart + 5 * 60 * 1000
      if (evTime >= recStart && evTime < nextStart) {
        if (asc[i].is_recording) {
          setActiveRecording(null)
          const leadTime = new Date(evTime - playbackLeadSeconds * 1000).toISOString()
          if (hlsPlayerRef.current) {
            hlsPlayerRef.current.seekTo(leadTime)
          } else {
            pendingLiveSeekRef.current = leadTime
          }
          return true
        }
        const seekTime = Math.max(0, (evTime - recStart) / 1000 - playbackLeadSeconds)
        if (activeRecording?.filename === asc[i].filename) {
          if (videoRef.current) {
            videoRef.current.currentTime = seekTime
            videoRef.current.play().catch(() => {})
          }
        } else {
          pendingSeekRef.current = seekTime
          openRecording(asc[i])
        }
        return true
      }
    }
    return false
  }

  function playEventAt(ev: MotionEvent, recs: Recording[] = recordings, skipScroll = false) {
    setActiveEventTime(ev.time)
    setActiveEventId(ev.id ?? null)
    if (!skipScroll) setScrollNonce(n => n + 1)
    seekToRecordingTime(ev.time, recs)
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
  useMarkActiveEventRead(id ?? '', activeEventTime)
  const setItems = useSetSidebarItems()
  const { player: playerMode } = useDisplayMode()
  const playerShowIcon = playerMode !== 'text-only'
  const playerShowLabel = playerMode !== 'icons-only'
  const playerBtn = (icon: React.ReactNode, label: string) => (
    <>
      {playerShowIcon && icon}
      {playerShowLabel && <span className="text-[11px] leading-none">{label}</span>}
    </>
  )
  const cam = settings?.cameras.find(c => c.id === id) ?? viewerCam
  const effectiveThreshold = cam?.motion?.threshold ?? 0

  // Durante a reprodução, a seleção do evento segue o playhead: o evento mais
  // próximo (dentro da janela do clipe) vira o ativo — card de detalhe + destaque
  // na lista da aba "Linha do tempo" + scroll até ele. Sem evento próximo, limpa.
  useEffect(() => {
    if (!activeRecording) return
    const playheadMs = Date.parse(activeRecording.start) + recCurrentTime * 1000
    const lead = cam?.motion?.playback_lead_seconds ?? 0
    const trail = cam?.motion?.playback_trail_seconds ?? 0
    const tol = Math.max(3000, (lead + trail) * 1000)
    const ev = activeEventForPlayhead(sortedEventsRef.current, playheadMs, tol)
    const nextId = ev?.id ?? null
    if (nextId !== activeEventIdRef.current) {
      setActiveEventId(nextId)
      setActiveEventTime(ev?.time ?? null)
      if (nextId !== null) setScrollNonce(n => n + 1)
    }
  }, [recCurrentTime, activeRecording, cam?.motion?.playback_lead_seconds, cam?.motion?.playback_trail_seconds])

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

  // Zera o zoom ao trocar de fonte (live ↔ gravação ↔ outra gravação).
  useEffect(() => { playerZoom.reset() }, [isLive, activeRecording, recordingId, playerZoom])

  // Live region score for the ephemeral "Analisar limiar" tool (live view only).
  const onAnalyzeScore = useCallback((data: string) => {
    try {
      const p = JSON.parse(data)
      if (typeof p.score === 'number') setAnalyzeScore(p.score)
    } catch { /* ignore malformed */ }
  }, [])
  const analyzeURL = isLive && analyzeMode && analyzeBox && id
    ? `/api/cameras/${id}/motion/region-score?x=${analyzeBox.x}&y=${analyzeBox.y}&w=${analyzeBox.w}&h=${analyzeBox.h}`
    : null
  useEventSource(analyzeURL, onAnalyzeScore)

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
  const filteredEvents = filterEventsByCategory(sortedEvents, eventFilter)
  const visibleEvents = filteredEvents.slice(0, eventsPage * PAGE_SIZE)
  const hasMoreEvents = filteredEvents.length > eventsPage * PAGE_SIZE
  const eventCounts: Partial<Record<EventFilter, number>> = { todos: sortedEvents.length }
  for (const ev of sortedEvents) {
    const cat = eventCategory(ev)
    eventCounts[cat] = (eventCounts[cat] ?? 0) + 1
  }
  const activeEventIdx = activeEventId !== null
    ? visibleEvents.findIndex(e => e.id === activeEventId)
    : activeEventTime !== null
      ? visibleEvents.findIndex(e => e.time === activeEventTime)
      : -1

  const filteredRecordings = filterRecordings(recordings, onlyMotion)

  // Keep refs in sync for use inside onEnded (avoids stale closure)
  activeEventTimeRef.current = activeEventTime
  visibleEventsRef.current = visibleEvents
  sortedEventsRef.current = sortedEvents
  continuousPlayRef.current = continuousPlay
  onlyMotionRef.current = onlyMotion
  recordingsDisplayPageRef.current = recordingsDisplayPage
  eventsPageRef.current = eventsPage

  return (
    <AppLayout fill mainClassName="w-full p-3">
        <div className="flex h-full gap-0">
          {/* Coluna do player */}
          <div className={`relative flex flex-col min-w-0 h-full gap-2 ${activePanel ? 'flex-1' : 'w-full'}`}>
            <div
              ref={playerRef}
              className={`flex flex-col flex-1 min-h-0 bg-background border rounded-lg overflow-hidden transition-all duration-300 ${
                !isLive ? 'border-primary ring-1 ring-primary' : 'border-border'
              }`}
            >
              <div className="flex-none flex items-center gap-2 px-4 py-2 border-b border-border min-w-0">
                <CameraSwitcher />
                <PlayerTitle
                  isLive={isLive}
                  name={cam?.name ?? id ?? ''}
                  subtitle={!isLive && activeRecording ? (() => {
                    const ev = activeEventIdx >= 0 ? visibleEvents[activeEventIdx] : null
                    return (
                      <>
                        <span className="tabular-nums">{formatRecordingDateTime(activeRecording.start, timezone)}</span>
                        {recDuration > 0 && ` · ${formatRecTime(recDuration)}`}
                        {ev?.label && <span className="font-medium" style={{ color: ev.color ?? '#f97316' }}> · {ev.label}</span>}
                      </>
                    )
                  })() : undefined}
                />
                <div className="ml-auto shrink-0 flex items-center gap-1">
                  {/* Voltar ao vivo — visível só durante reprodução */}
                  {!isLive && (
                    <button
                      onClick={() => { setNoRecordingAt(null); setActiveRecording(null); navigate(`/camera/live/${id}`, { replace: true }); setActivePanel(null) }}
                      title="Voltar ao vivo"
                      className="p-1 transition-colors cursor-pointer text-muted hover:text-foreground"
                    >
                      <span className="text-[9px] font-bold leading-none tracking-wide">AO VIVO</span>
                    </button>
                  )}
                  {!isLive && <div className="w-px h-4 bg-surface-2 mx-0.5" />}
                  {/* Mute */}
                  <button
                    id="player-mute"
                    onClick={() => setVideoMuted(m => { const next = !m; if (videoRef.current) videoRef.current.muted = next; return next })}
                    title={videoMuted ? 'Ativar áudio' : 'Silenciar'}
                    className={`flex items-center gap-1 px-1 py-1 transition-colors cursor-pointer ${!videoMuted ? 'text-primary' : 'text-muted hover:text-foreground'}`}
                  >
                    {playerBtn(
                      videoMuted ? <VolumeX className="w-[18px] h-[18px]" /> : <Volume2 className="w-[18px] h-[18px]" />,
                      videoMuted ? 'Mudo' : 'Áudio'
                    )}
                  </button>
                  {/* Speed dropdown — playback only */}
                  {!isLive && (
                    <div ref={speedMenuRef} className="relative">
                      <button
                        onClick={() => setSpeedMenuOpen(o => !o)}
                        title={`Velocidade ${playbackRate}×`}
                        className={`relative flex items-center gap-1 px-1 py-1 transition-colors cursor-pointer ${playbackRate > 1 ? 'text-primary' : 'text-muted hover:text-foreground'}`}
                      >
                        {playerBtn(
                          <>
                            <Gauge className="w-4 h-4" />
                            {playbackRate > 1 && (
                              <span className="absolute -top-0.5 -right-0.5 min-w-[1.1rem] h-[1.1rem] flex items-center justify-center text-[9px] font-bold bg-surface-2 text-foreground rounded-full px-0.5">
                                {playbackRate}×
                              </span>
                            )}
                          </>,
                          `${playbackRate}×`
                        )}
                      </button>
                      {speedMenuOpen && (
                        <div className="absolute right-0 top-full mt-1 bg-surface border border-border rounded shadow-lg z-50 py-1 min-w-[4rem]">
                          {[1, 2, 4, 8, 16, 32]
                            .filter(v => browserMaxRate === null || v <= browserMaxRate)
                            .map(v => (
                              <button
                                key={v}
                                onClick={() => { handleRateChange(v); setSpeedMenuOpen(false) }}
                                className={`w-full text-left px-3 py-1 text-xs ${v === playbackRate ? 'text-primary font-semibold' : 'text-foreground hover:text-foreground hover:bg-surface-2'}`}
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
                      className={`flex items-center gap-1 px-1 py-1 transition-colors cursor-pointer ${continuousPlay ? 'text-primary' : 'text-muted hover:text-foreground'}`}
                    >
                      {playerBtn(<Repeat className="w-4 h-4" />, 'Contínua')}
                    </button>
                  )}
                  <div className="w-px h-4 bg-surface-2 mx-0.5" />
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
                      className={`relative flex items-center gap-1 px-1 py-1 transition-colors cursor-pointer ${activePanel === 'events' ? 'text-primary' : 'text-muted hover:text-foreground'}`}
                    >
                      {playerShowIcon && <Zap className="w-4 h-4" />}
                      {!playerShowLabel && (
                        <span className="absolute -top-0.5 -right-0.5 min-w-[1.1rem] h-[1.1rem] flex items-center justify-center text-[9px] font-bold bg-surface-2 text-foreground rounded-full px-0.5">
                          {sortedEvents.length}
                        </span>
                      )}
                      {playerShowLabel && (
                        <>
                          <span className="text-[11px] leading-none">Eventos</span>
                          <span className="inline-flex items-center justify-center min-w-[1.1rem] h-[1.1rem] text-[9px] font-bold bg-surface-2 text-foreground rounded-full px-0.5">
                            {sortedEvents.length}
                          </span>
                        </>
                      )}
                    </button>
                  )}
                  <div className="w-px h-4 bg-surface-2 mx-0.5" />
                  {/* Debug */}
                  <button
                    onClick={() => showDebug ? closeDebug() : setShowDebug(true)}
                    title="Debug"
                    className={`flex items-center gap-1 px-1 py-1 transition-colors cursor-pointer ${showDebug ? 'text-primary' : 'text-muted hover:text-foreground'}`}
                  >
                    {playerBtn(<Code2 className="w-4 h-4" />, 'Debug')}
                  </button>
                  {/* Settings — dropdown com as seções de config da câmera */}
                  {isAdmin && (
                    <CameraConfigMenu cameraId={id!} showIcon={playerShowIcon} showLabel={playerShowLabel} />
                  )}
                  {/* Fullscreen */}
                  <button
                    id="player-fullscreen"
                    onClick={toggleFullscreen}
                    title="Tela inteira"
                    className="flex items-center gap-1 px-1 py-1 text-muted hover:text-foreground transition-colors cursor-pointer"
                  >
                    {playerBtn(<Maximize className="w-4 h-4" />, 'Expandir')}
                  </button>
                  <button
                    id="player-snapshot"
                    onClick={takeSnapshot}
                    title="Tirar snapshot"
                    className="flex items-center gap-1 px-1 py-1 text-muted hover:text-foreground transition-colors cursor-pointer"
                  >
                    {playerBtn(<CameraCapture className="w-4 h-4" />, 'Snapshot')}
                  </button>
                  {!isLive && activeRecording && (
                    <button
                      onClick={downloadVideo}
                      title="Baixar vídeo"
                      className="flex items-center gap-1 px-1 py-1 text-muted hover:text-foreground transition-colors cursor-pointer"
                    >
                      {playerBtn(
                        <svg className="w-4 h-4" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true">
                          <path d="M10 3v9m0 0 3.5-3.5M10 12 6.5 8.5" strokeLinecap="round" strokeLinejoin="round" />
                          <path d="M4 14.5V16a1 1 0 0 0 1 1h10a1 1 0 0 0 1-1v-1.5" strokeLinecap="round" />
                        </svg>,
                        'Baixar vídeo',
                      )}
                    </button>
                  )}
                </div>
              </div>

              {isLive ? (
                <div
                  ref={playerZoom.setContainer}
                  className={`flex-1 min-h-0 relative overflow-hidden${playerZoom.isZoomed ? ' cursor-grab' : ''}`}
                  onPointerDown={playerZoom.onPointerDown}
                  onPointerMove={playerZoom.onPointerMove}
                  onPointerUp={playerZoom.onPointerUp}
                >
                  <HLSPlayer ref={hlsPlayerRef} src={liveUrl} containerClassName="w-full h-full" className="w-full h-full bg-background" cameraId={id} muted={videoMuted} segmentSeconds={cam?.hls_segment_seconds} onGoToEvent={handleGoToEvent} />
                  {playerZoom.isZoomed && (
                    <button
                      onClick={playerZoom.reset}
                      aria-label="Reiniciar zoom"
                      title="Reiniciar zoom"
                      className="absolute top-2 right-2 z-20 flex items-center gap-1 px-2 py-1 rounded bg-black/70 hover:bg-black/90 text-white text-xs font-medium tabular-nums"
                    >
                      <ZoomOut className="w-3.5 h-3.5" /> {playerZoom.scale.toFixed(1)}×
                    </button>
                  )}
                  {analyzeMode && (
                    <div className="absolute inset-0">
                      <BboxCanvas box={analyzeBox} onChange={setAnalyzeBox} className="w-full h-full" />
                      {analyzeBox && (
                        <span
                          className="absolute z-10 px-1.5 py-0.5 rounded bg-black/70 text-emerald-300 text-xs font-mono tabular-nums pointer-events-none"
                          style={{ left: `${analyzeBox.x * 100}%`, top: `${analyzeBox.y * 100}%` }}
                        >
                          {zoneThresholdLabel(analyzeScore, undefined, effectiveThreshold)}
                        </span>
                      )}
                    </div>
                  )}
                </div>
              ) : (
                <div
                  ref={recContainerRef}
                  className="flex-1 min-h-0 relative overflow-hidden"
                  onMouseMove={showRecControls}
                  onMouseLeave={() => { if (recPlaying) setRecControlsVisible(false) }}
                  onPointerDown={playerZoom.onPointerDown}
                  onPointerMove={playerZoom.onPointerMove}
                  onPointerUp={playerZoom.onPointerUp}
                >
                  <video
                    ref={videoRef}
                    className={`w-full h-full bg-background ${playerZoom.isZoomed ? 'cursor-grab' : 'cursor-pointer'}`}
                    playsInline
                    onClick={() => { if (playerZoom.consumeDrag()) return; togglePlayRecording() }}
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
                      const stepPaused = stepPauseRef.current
                      stepPauseRef.current = false
                      const { seekTo, shouldPlay } = loadedMetadataSeek(
                        e.currentTarget.duration,
                        pendingSeekFromEndRef.current,
                        pendingSeekRef.current,
                        stepPaused,
                      )
                      pendingSeekFromEndRef.current = null
                      pendingSeekRef.current = null
                      if (seekTo !== null) e.currentTarget.currentTime = seekTo
                      if (!shouldPlay) { e.currentTarget.pause(); return }
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
                      const onlyMotion = onlyMotionRef.current
                      const next = nextRecording(recordingsRef.current, activeRecording.filename, onlyMotion)
                      if (next) {
                        const pool = filterRecordings(recordingsRef.current, onlyMotion)
                          .sort((a, b) => a.filename.localeCompare(b.filename))
                        const displayedCount = recordingsDisplayPageRef.current * PAGE_SIZE
                        const isVisible = pool.slice(0, displayedCount).some(r => r.filename === next.filename)
                        if (!isVisible) setRecordingsDisplayPage(p => p + 1)
                        openRecording(next)
                      }
                    }}
                  />
                  {playerZoom.isZoomed && (
                    <button
                      onClick={playerZoom.reset}
                      aria-label="Reiniciar zoom"
                      title="Reiniciar zoom"
                      className="absolute top-2 right-2 z-20 flex items-center gap-1 px-2 py-1 rounded bg-black/70 hover:bg-black/90 text-white text-xs font-medium tabular-nums"
                    >
                      <ZoomOut className="w-3.5 h-3.5" /> {playerZoom.scale.toFixed(1)}×
                    </button>
                  )}
                  {noRecordingAt !== null && (
                    <div className="absolute inset-0 z-20 flex items-center justify-center pointer-events-none">
                      <div className="px-4 py-2.5 rounded-lg bg-black/75 text-center">
                        <div className="text-sm font-medium text-white">Sem gravação neste horário</div>
                        <div className="mt-0.5 text-xs text-foreground tabular-nums">
                          {new Intl.DateTimeFormat('pt-BR', { timeZone: timezone, hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false }).format(new Date(noRecordingAt))}
                        </div>
                      </div>
                    </div>
                  )}
                  {/* Último frame: mantém imagem visível enquanto próxima gravação carrega */}
                  {lastFrameDataUrl && (
                    <img
                      src={lastFrameDataUrl}
                      className="absolute inset-0 w-full h-full object-contain bg-background pointer-events-none"
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
                <span className="text-xs text-faint tabular-nums shrink-0">{formatRecTime(recCurrentTime)}</span>
                <div
                  className="flex-1 h-1.5 rounded-full bg-surface-2 cursor-pointer relative group"
                  onClick={e => {
                    if (!videoRef.current || !recDuration) return
                    const rect = e.currentTarget.getBoundingClientRect()
                    const fraction = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width))
                    videoRef.current.currentTime = fraction * recDuration
                  }}
                >
                  <div
                    className="absolute inset-y-0 left-0 rounded-full bg-primary pointer-events-none group-hover:bg-primary transition-colors"
                    style={{ width: `${(recCurrentTime / recDuration) * 100}%` }}
                  />
                </div>
                <span className="text-xs text-faint tabular-nums shrink-0">{formatRecTime(recDuration)}</span>
              </div>
            )}

            {showDebug && (
              <div
                style={{ right: debugPos.x, top: debugPos.y }}
                className="absolute z-30 bg-background border border-border rounded-lg shadow-xl select-none flex flex-col"
              >
                {/* Header — drag handle */}
                <div
                  className="flex items-center justify-between px-4 py-2 border-b border-border cursor-move"
                  onMouseDown={handleDebugDragStart}
                >
                  <span className="text-xs font-semibold text-foreground uppercase tracking-widest">Debug</span>
                  <button onClick={closeDebug} className="ml-6 text-faint hover:text-foreground transition-colors">
                    <X className="w-3.5 h-3.5" />
                  </button>
                </div>
                {/* Stats */}
                <div className="px-4 py-3 flex flex-col gap-3 min-w-64">
                  <div>
                    <p className="text-xs font-semibold text-faint uppercase tracking-wider mb-1.5">Stream</p>
                    <div className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
                      <span className="text-xs text-faint">Codec</span>
                      <span className="text-sm text-foreground font-mono">{cam?.video_codec || '—'}</span>
                      <span className="text-xs text-faint">Resolução</span>
                      <span className="text-sm text-foreground font-mono">{cam?.width && cam.height ? `${cam.width}×${cam.height}` : '—'}</span>
                      <span className="text-xs text-faint">Áudio</span>
                      <span className="text-sm text-foreground">{cam?.has_audio == null ? '—' : cam.has_audio ? 'sim' : 'não'}</span>
                    </div>
                  </div>
                  <div>
                    <p className="text-xs font-semibold text-faint uppercase tracking-wider mb-1.5">Reprodução</p>
                    <div className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
                      {!isLive && (
                        <>
                          <span className="text-xs text-faint">Posição</span>
                          <span className="text-sm text-foreground font-mono tabular-nums">{formatRecTime(recCurrentTime)} / {formatRecTime(recDuration)}</span>
                          <span className="text-xs text-faint">Velocidade</span>
                          <span className="text-sm text-foreground font-mono tabular-nums">{playbackRate}×</span>
                        </>
                      )}
                      <span className="text-xs text-faint">FPS</span>
                      <span className="text-sm text-foreground font-mono tabular-nums">{debugStats ? `${debugStats.fps} fps` : '—'}</span>
                      <span className="text-xs text-faint">Descartados</span>
                      <span className={`text-sm font-mono tabular-nums ${debugStats && debugStats.dropped > 0 ? 'text-yellow-400' : 'text-foreground'}`}>
                        {debugStats ? debugStats.dropped : '—'}
                      </span>
                      <span className="text-xs text-faint">CPU</span>
                      <span className={`text-sm font-mono tabular-nums ${debugStats && debugStats.lagMs > 150 ? 'text-red-400' : debugStats && debugStats.lagMs > 50 ? 'text-yellow-400' : 'text-foreground'}`}>
                        {debugStats ? `${debugStats.lagMs} ms` : '—'}
                      </span>
                    </div>
                  </div>
                  {isLive && (
                    <div>
                      <p className="text-xs font-semibold text-faint uppercase tracking-wider mb-1.5">Rede</p>
                      <div className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
                        <span className="text-xs text-faint">Bitrate</span>
                        <span className="text-sm text-foreground font-mono tabular-nums">{debugStats?.hlsStats ? `${debugStats.hlsStats.bandwidthKbps} kbps` : '—'}</span>
                        <span className="text-xs text-faint">Latência</span>
                        <span className="text-sm text-foreground font-mono tabular-nums">{debugStats?.hlsStats ? `${debugStats.hlsStats.latencySeconds.toFixed(1)} s` : '—'}</span>
                      </div>
                    </div>
                  )}
                  {cam?.motion && (
                    <div>
                      <p className="text-xs font-semibold text-faint uppercase tracking-wider mb-1.5">Movimento</p>
                      <div className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1">
                        <span className="text-xs text-faint">FPS captura</span>
                        <span className="text-sm text-foreground font-mono tabular-nums">{cam.motion.fps ?? '—'}</span>
                        {(cam.motion.capture_width ?? 0) > 0 && (
                          <>
                            <span className="text-xs text-faint">Resolução</span>
                            <span className="text-sm text-foreground font-mono">{cam.motion.capture_width}×{cam.motion.capture_height}</span>
                          </>
                        )}
                        <span className="text-xs text-faint">Limiar</span>
                        <span className="text-sm text-foreground font-mono tabular-nums">{effectiveThreshold}</span>
                        {debugMotionValue !== null && effectiveThreshold > 0 && (
                          <>
                            <span className="text-xs text-faint">{debugMotionValue.label}</span>
                            <span className="text-sm text-foreground font-mono tabular-nums">
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
                      <label className="mt-3 flex items-center gap-1.5 text-xs text-muted hover:text-foreground cursor-pointer transition-colors">
                        <input
                          type="checkbox"
                          checked={showDebugChart}
                          onChange={e => setShowDebugChart(e.target.checked)}
                          className="accent-blue-500 w-3 h-3"
                        />
                        gráfico limiar
                      </label>
                      {isLive && (
                        <label className="mt-2 flex items-center gap-1.5 text-xs text-muted hover:text-foreground cursor-pointer transition-colors">
                          <input
                            type="checkbox"
                            checked={analyzeMode}
                            onChange={e => {
                              setAnalyzeMode(e.target.checked)
                              if (!e.target.checked) { setAnalyzeBox(null); setAnalyzeScore(null) }
                            }}
                            className="accent-blue-500 w-3 h-3"
                          />
                          analisar limiar (desenhe uma região)
                        </label>
                      )}
                    </div>
                  )}
                </div>
                {/* Gráfico — abaixo dos stats, só quando checkbox ativo */}
                {showDebugChart && cam?.motion && (
                  <div className="border-t border-border p-3 min-w-[640px]">
                    {!isLive && (
                      <p className="text-xs text-yellow-500/80 mb-2">scores ao vivo — não reflete a gravação em curso</p>
                    )}
                    <MotionScoreChart cameraId={id!} threshold={effectiveThreshold} />
                  </div>
                )}
              </div>
            )}

            <HorizontalTimeline
              recordings={recordings}
              events={sortedEvents}
              range={timelineRange}
              onRangeChange={setTimelineRange}
              endMs={isToday ? Date.now() : timelineEndOfDayMs}
              selectedDate={selectedDate}
              onSelectDate={(d) => { setSelectedDate(d); setCalendarMonth(d) }}
              formatTick={(ms) => format(new Date(ms), 'HH:mm')}
              playheadMs={activeRecording ? Date.parse(activeRecording.start) + recCurrentTime * 1000 : isToday ? Date.now() : latestRecordingMs}
              onSeek={handleTimelineSeek}
              onScrub={handleTimelineScrub}
              onGap={handleTimelineGap}
            />

            <Filmstrip
              recordings={recordings}
              win={timelineWindow(isToday ? Date.now() : timelineEndOfDayMs, timelineRange)}
              thumbSrc={(ms) => `/api/cameras/${id}/event-frame?time=${encodeURIComponent(new Date(ms).toISOString())}&token=${getToken()}`}
              formatTime={(ms) => format(new Date(ms), 'HH:mm:ss')}
              onSeek={handleTimelineSeek}
              activeRecordingId={activeRecording?.id}
            />
          </div>

          {/* Painel lateral condicional */}
          {activePanel && (
            <div className="w-72 shrink-0 border-l border-border bg-background flex flex-col h-full">
              {(activePanel === 'events' || activePanel === 'timeline') && (
                <>
                  <div className="flex items-center border-b border-border shrink-0">
                    <button
                      id="tab-events"
                      onClick={() => setActivePanel('events')}
                      className={`flex-1 flex flex-col items-center px-2 py-1.5 text-xs font-medium transition-colors ${
                        activePanel === 'events'
                          ? 'text-primary border-b-2 border-primary -mb-px'
                          : 'text-faint hover:text-foreground'
                      }`}
                    >
                      <span>Eventos</span>
                      <span className="tabular-nums">{sortedEvents.length}</span>
                    </button>
                    <button
                      id="tab-timeline"
                      onClick={() => setActivePanel('timeline')}
                      className={`flex-1 flex flex-col items-center px-2 py-1.5 text-xs font-medium transition-colors ${
                        activePanel === 'timeline'
                          ? 'text-primary border-b-2 border-primary -mb-px'
                          : 'text-faint hover:text-foreground'
                      }`}
                    >
                      <span>Linha do tempo</span>
                      <span className="tabular-nums">{sortedEvents.length}</span>
                    </button>
                    <button
                      onClick={() => setActivePanel(null)}
                      title="Fechar"
                      className="px-2.5 py-2 text-faint hover:text-foreground transition-colors shrink-0"
                    >
                      <X className="w-3.5 h-3.5" />
                    </button>
                  </div>
                  {(
                    <div className="flex flex-col flex-1 min-h-0">
                    {activePanel === 'timeline' ? (
                      (() => {
                        const activeEv = activeEventIdx >= 0 ? visibleEvents[activeEventIdx] : null
                        return (
                          <EventDetailCard
                            event={activeEv}
                            cameraName={cam?.name ?? id ?? ''}
                            durationSeconds={(cam?.motion?.playback_lead_seconds ?? 0) + (cam?.motion?.playback_trail_seconds ?? 0)}
                            thumbSrc={activeEv?.frame ? snapshotURL(id!, activeEv.time, activeEv.frame) : null}
                            onPlay={() => { if (activeEv) { playEventAt(activeEv); setScrollNonce(n => n + 1) } }}
                            onDownload={() => { if (activeEv) downloadEventVideos(activeEv) }}
                            onMark={() => { if (activeEv) openSnapshotModal(activeEv) }}
                          />
                        )
                      })()
                    ) : (
                      <EventFilterChips
                        value={eventFilter}
                        onChange={(f) => { setEventFilter(f); setEventsPage(1) }}
                        counts={eventCounts}
                      />
                    )}
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
                        const bust = thumbCacheBust.get(ev.id)
                        const thumbOverride = thumbOverrides.get(ev.id)
                        const thumbURL = thumbOverride ?? (ev.frame ? snapshotURL(id!, ev.time, ev.frame) + (bust ? `&t=${bust}` : '') : null)
                        const recDets = recordings.filter(r => r.start <= ev.time).sort((a, b) => b.start.localeCompare(a.start))[0]?.detections
                        const evRecs = recordingsForEventWindow(recordings, ev.time, cam?.motion?.playback_lead_seconds ?? 0, cam?.motion?.playback_trail_seconds ?? 0)
                        return (
                          <button
                            key={ev.id ?? `${ev.time}-${i}`}
                            ref={isActive ? (el) => { if (el) activeEventItemRef.current = el } : null}
                            onClick={() => { playEventAt(ev); setScrollNonce(n => n + 1) }}
                            className={`group w-full flex flex-col px-3 py-2 transition-colors text-left ${
                              isActive ? 'bg-primary/10 border-l-2 border-primary' : 'hover:bg-surface'
                            }`}
                          >
                            <div className="flex items-center justify-between w-full gap-2">
                              <span className={`text-sm tabular-nums ${isActive ? 'text-primary' : 'text-foreground'}`}>
                                {formatRecordingTime(ev.time, timezone)}
                              </span>
                              <div className="flex items-center gap-1.5 min-w-0">
                                {evRecs.length > 0 && (
                                  <span
                                    role="button"
                                    tabIndex={-1}
                                    title="Baixar vídeo(s) do evento"
                                    onClick={(e) => { e.stopPropagation(); downloadEventVideos(ev) }}
                                    className="shrink-0 text-faint hover:text-foreground opacity-0 group-hover:opacity-100 transition-opacity"
                                  >
                                    <svg className="w-3.5 h-3.5" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true">
                                      <path d="M10 3v9m0 0 3.5-3.5M10 12 6.5 8.5" strokeLinecap="round" strokeLinejoin="round" />
                                      <path d="M4 14.5V16a1 1 0 0 0 1 1h10a1 1 0 0 0 1-1v-1.5" strokeLinecap="round" />
                                    </svg>
                                  </span>
                                )}
                                {ev.label && ev.color && (
                                  <>
                                    <span className="w-2 h-2 rounded-full shrink-0" style={{ backgroundColor: ev.color }} />
                                    <span className="text-xs font-medium truncate" style={{ color: ev.color }}>{ev.label}</span>
                                  </>
                                )}
                                {ev.label && !ev.color && (
                                  <span className="text-[10px] px-1.5 py-0.5 rounded bg-primary/15 text-primary border border-primary/40 truncate">
                                    {ev.label}
                                  </span>
                                )}
                                <span className="text-xs text-faint shrink-0">[{(ev.score * 100).toFixed(1)}%]</span>
                              </div>
                            </div>
                            <div className="flex items-center justify-between gap-2 mt-1">
                              <div className="flex items-center gap-2 min-w-0">
                                {thumbURL && (
                                  <img
                                    key={`thumb-${ev.id}-${thumbOverride ?? bust ?? 0}`}
                                    src={thumbURL}
                                    alt="snapshot"
                                    className="w-16 h-10 object-cover rounded cursor-zoom-in border border-border shrink-0"
                                    style={thumbFlash.has(ev.id) ? { animation: 'thumb-flash 0.9s ease-out' } : undefined}
                                    onClick={e => { e.stopPropagation(); openSnapshotModal(ev) }}
                                  />
                                )}
                                <div className="flex flex-col min-w-0">
                                  <span className="text-xs font-medium text-foreground truncate">{eventTitle(ev)}</span>
                                  <span className="text-[11px] text-muted truncate">{cam?.name ?? id}</span>
                                </div>
                              </div>
                              <span
                                aria-hidden="true"
                                className="shrink-0 flex items-center justify-center w-7 h-7 rounded-full bg-primary/15 text-primary group-hover:bg-primary group-hover:text-primary-foreground transition-colors"
                              >
                                <Play className="w-3.5 h-3.5 fill-current" />
                              </span>
                            </div>
                            {recDets && recDets.length > 0 && (
                              <div className="flex flex-wrap gap-1 mt-1" onClick={e => { e.stopPropagation(); setDetectionModal(recDets) }}>
                                {recDets.map(d => (
                                  <span key={d.label} className={`text-[10px] px-1.5 py-0.5 rounded border cursor-pointer ${d.custom_model ? 'bg-emerald-900/60 text-emerald-300 border-emerald-700/50 hover:bg-emerald-800/60' : 'bg-violet-900/60 text-violet-300 border-violet-700/50 hover:bg-violet-800/60'}`}>
                                    {d.label}
                                  </span>
                                ))}
                              </div>
                            )}
                          </button>
                        )
                      })}
                    </ListPanel>
                    {activePanel === 'events' && (
                    <div className="shrink-0 border-t border-border max-h-80 overflow-y-auto">
                      <div className="p-3">
                        <Calendar
                          mode="single"
                          selected={selectedDate}
                          month={calendarMonth}
                          onMonthChange={setCalendarMonth}
                          onSelect={d => { if (d) { setSelectedDate(d); setCalendarMonth(d) } }}
                          footer={(!isToday || !calendarOnCurrentMonth) && (
                            <div className="flex justify-center pt-1">
                              <button onClick={() => { setSelectedDate(new Date()); setCalendarMonth(new Date()) }} className="text-xs font-medium text-primary hover:text-primary transition-colors">
                                Hoje
                              </button>
                            </div>
                          )}
                        />
                      </div>
                    </div>
                    )}
                    </div>
                  )}
                </>
              )}
            </div>
          )}

          <VerticalTimeline
            recordings={filteredRecordings}
            motionEvents={motionEvents}
            activeRecording={activeRecording}
            activeTime={
              activeRecording && !isLive
                ? new Date(new Date(activeRecording.start).getTime() + recCurrentTime * 1000).toISOString()
                : activeEventTime ?? null
            }
            timezone={timezone}
            sortOrder={activePanel === 'events' ? eventsSortOrder : sortOrder}
            onSeek={handleTimelineSeek}
            onScrub={handleTimelineScrub}
            onGap={handleTimelineGap}
            onEventClick={activePanel === 'events' ? ev => { playEventAt(ev); setScrollNonce(n => n + 1) } : undefined}
          />
        </div>

      {detectionModal && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
          onClick={() => setDetectionModal(null)}
        >
          <div
            className="bg-background border border-border rounded-xl shadow-2xl w-72 p-5"
            onClick={e => e.stopPropagation()}
          >
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-sm font-semibold text-foreground">Detecções de objetos</h3>
              <button
                onClick={() => setDetectionModal(null)}
                className="text-faint hover:text-foreground text-lg leading-none"
              >✕</button>
            </div>
            <div className="space-y-2">
              {detectionModal.map(d => (
                <div key={d.label} className="flex items-center justify-between">
                  <span className={`text-sm font-medium ${d.custom_model ? 'text-emerald-300' : 'text-violet-300'}`}>{d.label}</span>
                  <div className="flex items-center gap-3 text-xs text-muted">
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
          <div className="flex flex-col gap-3 items-center" onClick={e => e.stopPropagation()}>
            <div className="relative rounded overflow-hidden shadow-2xl" style={{ maxWidth: '90vw', maxHeight: '75vh' }}>
              <img
                src={snapshotURL(id!, snapshotEvent.time, snapshotEvent.frame) + (thumbCacheBust.get(snapshotEvent.id) ? `&t=${thumbCacheBust.get(snapshotEvent.id)}` : '')}
                alt="snapshot de movimento"
                className="block max-w-full max-h-full"
                draggable={false}
              />
              <BboxCanvas
                box={annBox ?? existingAnn}
                onChange={handleAnnBoxChange}
                readonly={annSaveOk}
                className="absolute inset-0 w-full h-full select-none"
              />
            </div>

            <p className="text-xs text-muted">
              {formatRecordingTime(snapshotEvent.time, timezone)} — score: {(snapshotEvent.score * 100).toFixed(1)}%
            </p>

            <div className="flex items-center gap-2 w-full max-w-md">
              {annSaveOk && (
                <span className="text-xs text-emerald-400">Anotação salva</span>
              )}
              {!annSaveOk && annBox && annBox.w > 0.01 && annBox.h > 0.01 && (
                <>
                  <input
                    type="text"
                    placeholder="label da região…"
                    value={annLabel}
                    onChange={e => setAnnLabel(e.target.value)}
                    onKeyDown={e => e.key === 'Enter' && saveAnnotation()}
                    autoFocus
                    className="flex-1 bg-surface text-foreground text-sm rounded px-3 py-1.5 border border-border focus:outline-none focus:border-emerald-500"
                  />
                  <button
                    onClick={saveAnnotation}
                    disabled={annSaving}
                    className="px-3 py-1.5 text-sm bg-emerald-700 hover:bg-emerald-600 text-white rounded disabled:opacity-60 disabled:cursor-not-allowed"
                  >
                    {annSaving ? 'Salvando...' : 'Salvar'}
                  </button>
                  <button
                    onClick={() => setAnnBox(null)}
                    className="px-3 py-1.5 text-sm bg-surface-2 hover:bg-surface-2 text-foreground rounded"
                  >
                    Cancelar
                  </button>
                </>
              )}
              {!annSaveOk && !annBox && existingAnn && (
                <span className="text-xs text-muted flex items-center gap-2">
                  {existingAnnLabel
                    ? <><span className="font-medium text-foreground">{existingAnnLabel}</span> · Arraste para substituir</>
                    : 'Região salva · Arraste para substituir'
                  }
                  {existingAnnId && (
                    <button
                      onClick={() => deleteAnnotation()}
                      className="px-2 py-0.5 text-xs text-red-400 hover:text-red-300 hover:bg-red-900/30 border border-red-700/40 rounded transition-colors"
                    >
                      Excluir anotação
                    </button>
                  )}
                </span>
              )}
              {!annSaveOk && !annBox && !existingAnn && (
                <span className="text-xs text-faint">Arraste para marcar · mova · redimensione · rotacione</span>
              )}
              <button
                onClick={closeSnapshotModal}
                className="ml-auto px-3 py-1.5 text-sm bg-surface-2 hover:bg-surface-2 text-foreground rounded"
              >
                Fechar
              </button>
            </div>
          </div>
        </div>
      )}

      {deleteError && (
        <div className="fixed bottom-4 left-1/2 -translate-x-1/2 z-50 px-4 py-2 bg-red-900/90 border border-red-700 rounded text-sm text-red-200 shadow-lg">
          {deleteError}
        </div>
      )}

      <ConfirmDialog
        open={pendingSnapBlob !== null}
        title="Snapshot do evento"
        message="Usar este snapshot como thumbnail do evento, ou baixá-lo?"
        confirmLabel="Substituir thumbnail"
        cancelLabel="Cancelar"
        onConfirm={replaceEventThumb}
        onCancel={() => setPendingSnapBlob(null)}
      >
        <button
          onClick={downloadPendingSnap}
          className="self-start px-4 py-1.5 text-xs text-foreground border border-border rounded hover:bg-surface-2 transition-colors"
        >
          Baixar snapshot
        </button>
      </ConfirmDialog>

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
