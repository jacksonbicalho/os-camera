import { useEffect, useState, useCallback, useRef } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { DayPicker } from 'react-day-picker'
import { format } from 'date-fns'
import { ptBR } from 'date-fns/locale'
import 'react-day-picker/style.css'
import { authHeaders, clearToken, getToken, getUsername } from '../auth'
import Header from '../components/Header'
import HLSPlayer from '../components/HLSPlayer'
import { useScrollToPlayer } from '../hooks/useScrollToPlayer'

interface Recording {
  filename: string
  start: string
  url: string
}

interface RecordingsResponse {
  recordings: Recording[]
  hasMore: boolean
}

const PAGE_SIZE = 10

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
  const playerRef = useRef<HTMLDivElement>(null)

  useScrollToPlayer(playerRef, activeRecording?.filename ?? null)

  useEffect(() => {
    fetch('/api/config')
      .then(r => r.json())
      .then(d => { if (d.timezone) setTimezone(d.timezone) })
      .catch(() => {})
  }, [])

  const fetchRecordings = useCallback(async (date: Date, p: number, append: boolean) => {
    const dateStr = format(date, 'yyyy-MM-dd')
    const res = await fetch(
      `/api/cameras/${id}/recordings?date=${dateStr}&page=${p}&limit=${PAGE_SIZE}`,
      { headers: authHeaders() }
    )
    if (res.status === 401) { clearToken(); navigate('/login'); return }
    const data: RecordingsResponse = await res.json()
    setRecordings(prev => append ? [...prev, ...data.recordings] : data.recordings)
    setHasMore(data.hasMore)
  }, [id, navigate])

  useEffect(() => {
    setPage(1)
    setActiveRecording(null)
    fetchRecordings(selectedDate, 1, false)
  }, [selectedDate, fetchRecordings])

  async function loadMore() {
    setLoadingMore(true)
    const next = page + 1
    setPage(next)
    await fetchRecordings(selectedDate, next, true)
    setLoadingMore(false)
  }

  const liveUrl = `/stream/${id}/index.m3u8`
  const isLive = activeRecording === null

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
                  key={activeRecording.url}
                  src={`${activeRecording.url}?token=${getToken()}`}
                  className="w-full aspect-video bg-black"
                  controls
                  autoPlay
                  playsInline
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

            {/* Lista de chunks */}
            <div className="bg-gray-900 border border-gray-800 rounded-lg overflow-hidden">
              <div className="px-3 py-2 border-b border-gray-800 flex items-center justify-between">
                <p className="text-xs text-gray-400 font-medium uppercase tracking-wider">
                  {format(selectedDate, "d 'de' MMMM", { locale: ptBR })}
                </p>
                <button
                  onClick={() => setSortOrder(o => o === 'desc' ? 'asc' : 'desc')}
                  className="text-xs text-blue-400 hover:text-blue-300"
                >
                  {sortOrder === 'desc' ? '↓ Recente' : '↑ Antigo'}
                </button>
              </div>
              <div className="divide-y divide-gray-800">
                {recordings.length === 0 ? (
                  <p className="px-3 py-4 text-sm text-gray-500">Sem gravações nesta data.</p>
                ) : (
                  [...recordings]
                    .sort((a, b) => {
                      const diff = new Date(a.start).getTime() - new Date(b.start).getTime()
                      return sortOrder === 'asc' ? diff : -diff
                    })
                    .map(rec => {
                    const isActive = activeRecording?.filename === rec.filename
                    return (
                      <button
                        key={rec.filename}
                        onClick={() => setActiveRecording(rec)}
                        className={`w-full flex items-center justify-between px-3 py-2 transition-colors text-left ${
                          isActive ? 'bg-blue-900/40 border-l-2 border-blue-500' : 'hover:bg-gray-800'
                        }`}
                      >
                        <span className={`text-sm ${isActive ? 'text-blue-300' : 'text-gray-300'}`}>
                          {formatRecordingTime(rec.start, timezone)}
                        </span>
                        <span className="text-xs text-gray-500">▶ MP4</span>
                      </button>
                    )
                  })
                )}
              </div>
              {hasMore && (
                <div className="px-3 py-2 border-t border-gray-800">
                  <button
                    onClick={loadMore}
                    disabled={loadingMore}
                    className="text-sm text-blue-400 hover:text-blue-300 disabled:opacity-50"
                  >
                    {loadingMore ? 'Carregando...' : 'Carregar mais'}
                  </button>
                </div>
              )}
            </div>
          </div>
        </div>
      </main>
    </div>
  )
}
