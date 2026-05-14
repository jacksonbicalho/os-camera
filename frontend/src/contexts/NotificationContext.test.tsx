import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act, cleanup } from '@testing-library/react'
import React from 'react'
import { MemoryRouter } from 'react-router-dom'
import { NotificationProvider, useNotifications } from './NotificationContext'

const STORAGE_KEY = 'camera_notifications'

// Minimal EventSource mock
class FakeEventSource {
  static instances: FakeEventSource[] = []
  onmessage: ((e: MessageEvent) => void) | null = null
  onerror: ((e: Event) => void) | null = null
  url: string
  closed = false

  constructor(url: string) {
    this.url = url
    FakeEventSource.instances.push(this)
  }

  emit(data: string) {
    this.onmessage?.({ data } as MessageEvent)
  }

  close() {
    this.closed = true
  }
}

const wrapper = ({ children }: { children: React.ReactNode }) => (
  <MemoryRouter>
    <NotificationProvider>{children}</NotificationProvider>
  </MemoryRouter>
)

// Aguarda efeitos assíncronos
const flush = () => act(async () => {
  await new Promise((r) => setTimeout(r, 0))
})

beforeEach(() => {
  cleanup()
  FakeEventSource.instances = []
  vi.stubGlobal('EventSource', FakeEventSource)
  localStorage.clear()
  localStorage.setItem('camera_token', 'fake-token')
})

afterEach(() => {
  vi.unstubAllGlobals()
  localStorage.clear()
})

describe('NotificationContext — estado inicial', () => {
  it('começa sem notificações', () => {
    const { result } = renderHook(() => useNotifications(), { wrapper })
    expect(result.current.notifications).toEqual([])
    expect(result.current.unreadCount).toBe(0)
  })

  it('restaura notificações salvas no localStorage', () => {
    const saved = [
      { id: 'cam1-1000', type: 'motion', cameraId: 'cam1', time: '2026-01-01T00:00:00Z', score: 0.5, read: false },
    ]
    localStorage.setItem(STORAGE_KEY, JSON.stringify(saved))

    const { result } = renderHook(() => useNotifications(), { wrapper })
    expect(result.current.notifications).toHaveLength(1)
    expect(result.current.unreadCount).toBe(1)
  })
})

describe('NotificationContext — recebimento de eventos SSE', () => {
  it('cria notificação ao receber evento de movimento', async () => {
    const { result } = renderHook(() => useNotifications(), { wrapper })

    await flush()

    const es = FakeEventSource.instances.find((e) => e.url.includes('/api/motion/live'))
    expect(es).toBeDefined()

    act(() => {
      es!.emit(JSON.stringify({ camera_id: 'cam1', score: 0.42, time: '2026-01-01T12:00:00Z' }))
    })

    expect(result.current.notifications).toHaveLength(1)
    expect(result.current.notifications[0].cameraId).toBe('cam1')
    expect(result.current.notifications[0].score).toBe(0.42)
    expect(result.current.notifications[0].read).toBe(false)
    expect(result.current.unreadCount).toBe(1)
  })

  it('persiste notificação no localStorage ao receber evento', async () => {
    const { result } = renderHook(() => useNotifications(), { wrapper })

    await flush()

    const es = FakeEventSource.instances.find((e) => e.url.includes('/api/motion/live'))!

    act(() => {
      es.emit(JSON.stringify({ camera_id: 'cam1', score: 0.3, time: '2026-01-01T12:00:00Z' }))
    })

    const stored = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]')
    expect(stored).toHaveLength(1)
    expect(result.current.notifications).toHaveLength(1)
  })

  it('mantém máximo de 100 notificações descartando as mais antigas', async () => {
    const existing = Array.from({ length: 100 }, (_, i) => ({
      id: `cam1-${i}`,
      type: 'motion',
      cameraId: 'cam1',
      time: `2026-01-01T00:00:0${String(i).padStart(2, '0')}Z`,
      score: 0.1,
      read: true,
    }))
    localStorage.setItem(STORAGE_KEY, JSON.stringify(existing))

    const { result } = renderHook(() => useNotifications(), { wrapper })

    await flush()

    const es = FakeEventSource.instances.find((e) => e.url.includes('/api/motion/live'))!

    act(() => {
      es.emit(JSON.stringify({ camera_id: 'cam1', score: 0.9, time: '2026-01-02T00:00:00Z' }))
    })

    expect(result.current.notifications).toHaveLength(100)
    // O mais recente deve estar no topo
    expect(result.current.notifications[0].score).toBe(0.9)
  })
})

describe('NotificationContext — operações', () => {
  async function setupWithNotification() {
    const { result } = renderHook(() => useNotifications(), { wrapper })
    await flush()

    const es = FakeEventSource.instances.find((e) => e.url.includes('/api/motion/live'))!
    act(() => {
      es.emit(JSON.stringify({ camera_id: 'cam1', score: 0.5, time: '2026-01-01T12:00:00Z' }))
    })
    return result
  }

  it('markRead marca uma notificação como lida', async () => {
    const result = await setupWithNotification()
    const id = result.current.notifications[0].id

    act(() => { result.current.markRead(id) })

    expect(result.current.notifications[0].read).toBe(true)
    expect(result.current.unreadCount).toBe(0)
  })

  it('markAllRead marca todas as notificações como lidas', async () => {
    const result = await setupWithNotification()

    act(() => { result.current.markAllRead() })

    expect(result.current.notifications.every((n) => n.read)).toBe(true)
    expect(result.current.unreadCount).toBe(0)
  })

  it('remove exclui uma notificação por id', async () => {
    const result = await setupWithNotification()
    const id = result.current.notifications[0].id

    act(() => { result.current.remove(id) })

    expect(result.current.notifications).toHaveLength(0)
  })

  it('markAllUnread marca notificações selecionadas como não lidas', async () => {
    const result = await setupWithNotification()
    const id = result.current.notifications[0].id

    act(() => { result.current.markRead(id) })
    expect(result.current.notifications[0].read).toBe(true)

    act(() => { result.current.markAllUnread([id]) })
    expect(result.current.notifications[0].read).toBe(false)
    expect(result.current.unreadCount).toBe(1)
  })

  it('removeAll limpa todas as notificações', async () => {
    const result = await setupWithNotification()

    act(() => { result.current.removeAll() })

    expect(result.current.notifications).toHaveLength(0)
    expect(result.current.unreadCount).toBe(0)
    expect(localStorage.getItem(STORAGE_KEY)).toBeNull()
  })
})

describe('NotificationContext — sem token', () => {
  it('não abre EventSource quando não há token', async () => {
    localStorage.removeItem('camera_token')

    renderHook(() => useNotifications(), { wrapper })

    await flush()

    expect(FakeEventSource.instances).toHaveLength(0)
  })
})
