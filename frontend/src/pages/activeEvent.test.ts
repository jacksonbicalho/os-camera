import { describe, expect, it } from 'vitest'
import { activeEventForPlayhead } from './activeEvent'

function ev(id: number, ms: number) {
  return { id, time: new Date(ms).toISOString() }
}

describe('activeEventForPlayhead', () => {
  const evs = [ev(1, 0), ev(2, 60_000), ev(3, 120_000)]
  it('devolve o evento mais próximo dentro da tolerância', () => {
    expect(activeEventForPlayhead(evs, 55_000, 10_000)?.id).toBe(2)
    expect(activeEventForPlayhead(evs, 2_000, 10_000)?.id).toBe(1)
    expect(activeEventForPlayhead(evs, 118_000, 10_000)?.id).toBe(3)
  })
  it('null quando nenhum evento cai na tolerância', () => {
    expect(activeEventForPlayhead(evs, 200_000, 10_000)).toBeNull()
  })
  it('lista vazia → null', () => {
    expect(activeEventForPlayhead([], 0, 10_000)).toBeNull()
  })
})
