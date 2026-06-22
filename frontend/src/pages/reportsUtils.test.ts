import { describe, expect, it } from 'vitest'
import { categoryBuckets, axisTicks } from './reportsUtils'

describe('categoryBuckets', () => {
  it('dobra labels nas categorias (vazio→movimento, pessoa, outro→ia)', () => {
    expect(categoryBuckets({ '': 5, pessoa: 3, carro: 2 })).toEqual({
      movimento: 5, pessoa: 3, ia: 2, estados: 0,
    })
  })
  it('soma múltiplos labels da mesma categoria', () => {
    expect(categoryBuckets({ pessoa: 2, 'Pessoa com chapéu': 1 }).pessoa).toBe(3)
  })
  it('inclui estados vindos do byCategory do backend', () => {
    expect(categoryBuckets({ '': 5, pessoa: 3 }, { estados: 7 })).toEqual({
      movimento: 5, pessoa: 3, ia: 0, estados: 7,
    })
  })
})

describe('axisTicks', () => {
  const days = (n: number) => Array.from({ length: n }, (_, i) => `2026-06-${String(i + 1).padStart(2, '0')}`)

  it('lista vazia → sem ticks', () => {
    expect(axisTicks([], 6)).toEqual([])
  })
  it('poucos dias (<= maxTicks) → um tick por dia (rótulo MM-DD)', () => {
    expect(axisTicks(days(3), 6)).toEqual([
      { index: 0, label: '06-01' },
      { index: 1, label: '06-02' },
      { index: 2, label: '06-03' },
    ])
  })
  it('muitos dias → no máximo maxTicks, sempre incluindo primeiro e último', () => {
    const t = axisTicks(days(30), 6)
    expect(t.length).toBeLessThanOrEqual(6)
    expect(t[0]).toEqual({ index: 0, label: '06-01' })
    expect(t[t.length - 1]).toEqual({ index: 29, label: '06-30' })
  })
  it('índices são crescentes e sem duplicatas', () => {
    const t = axisTicks(days(30), 6)
    const idx = t.map(x => x.index)
    expect(idx).toEqual([...idx].sort((a, b) => a - b))
    expect(new Set(idx).size).toBe(idx.length)
  })
})
