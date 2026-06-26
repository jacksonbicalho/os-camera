import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import { useStats } from './useStats'

vi.mock('../auth', () => ({
  authHeaders: () => ({}),
  onUnauthorized: vi.fn(),
}))

const okStats = { cpu_percent: 10, sys_mem_total_bytes: 100, sys_mem_free_bytes: 50, net_mbps: 5 }

function okResponse() {
  return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve(okStats) })
}
function errResponse() {
  return Promise.resolve({ ok: false, status: 503, json: () => Promise.resolve({}) })
}

beforeEach(() => { vi.useFakeTimers() })
afterEach(() => { vi.restoreAllMocks(); vi.useRealTimers() })

// flushMicro: resolve as promises pendentes (fetch → json → setState) sem mexer no
// relógio — usado após o fetch do mount. Promise.resolve não depende de timers.
const flushMicro = () => act(async () => { for (let i = 0; i < 5; i++) await Promise.resolve() })
// tick: avança o relógio (dispara o setInterval) e dá flush nos microtasks.
const tick = (ms: number) => act(async () => { await vi.advanceTimersByTimeAsync(ms) })

describe('useStats — estado de conexão', () => {
  it('sucesso → connected=true e stats preenchido', async () => {
    vi.stubGlobal('fetch', vi.fn(okResponse))
    const { result } = renderHook(() => useStats())
    await flushMicro()
    expect(result.current.connected).toBe(true)
    expect(result.current.stats?.cpu_percent).toBe(10)
  })

  it('2 falhas consecutivas após sucesso → connected=false (mantém o último stats)', async () => {
    const fetchMock = vi.fn()
      .mockImplementationOnce(okResponse)   // poll inicial: sucesso
      .mockImplementationOnce(errResponse)  // 1ª falha
      .mockImplementationOnce(errResponse)  // 2ª falha → desconecta
    vi.stubGlobal('fetch', fetchMock)

    const { result } = renderHook(() => useStats())
    await flushMicro()
    expect(result.current.connected).toBe(true)

    await tick(10_000) // 1ª falha
    expect(result.current.connected).toBe(true) // tolerância: ainda conectado

    await tick(10_000) // 2ª falha → desconecta
    expect(result.current.connected).toBe(false)
    // último stats preservado (a UI de dados não pisca)
    expect(result.current.stats?.cpu_percent).toBe(10)
  })

  it('1 falha isolada não derruba a conexão (recupera no próximo poll)', async () => {
    const fetchMock = vi.fn()
      .mockImplementationOnce(okResponse)   // sucesso
      .mockImplementationOnce(errResponse)  // 1 falha isolada
      .mockImplementation(okResponse)       // recupera
    vi.stubGlobal('fetch', fetchMock)

    const { result } = renderHook(() => useStats())
    await flushMicro()
    expect(result.current.connected).toBe(true)

    await tick(10_000) // falha isolada
    expect(result.current.connected).toBe(true)

    await tick(10_000) // sucesso de novo
    expect(result.current.connected).toBe(true)
  })
})
