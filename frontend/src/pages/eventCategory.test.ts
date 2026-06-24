import { describe, expect, it } from 'vitest'
import { eventCategory, filterEventsByCategory, eventTitle, recordingCategory, eventCardLines, firstEventInChunk } from './eventCategory'

describe('recordingCategory', () => {
  const recAt = (ms: number) => ({ start: new Date(ms).toISOString() })
  it('continua quando não há evento no chunk', () => {
    expect(recordingCategory(recAt(0), [], 300_000)).toBe('continua')
  })
  it('usa a maior prioridade (pessoa > movimento) dos eventos no chunk', () => {
    const events = [
      { time: new Date(60_000).toISOString(), label: '' },
      { time: new Date(120_000).toISOString(), label: 'pessoa' },
    ]
    expect(recordingCategory(recAt(0), events, 300_000)).toBe('pessoa')
  })
  it('ignora eventos fora do intervalo do chunk', () => {
    const events = [{ time: new Date(400_000).toISOString(), label: 'pessoa' }]
    expect(recordingCategory(recAt(0), events, 300_000)).toBe('continua')
  })
  it('chunk só com transição de estado (kind=state) → estados (verde)', () => {
    const events = [{ time: new Date(60_000).toISOString(), label: 'aberto', kind: 'state' as const }]
    expect(recordingCategory(recAt(0), events, 300_000)).toBe('estados')
  })
  it('detecção real predomina sobre estado no mesmo chunk', () => {
    const events = [
      { time: new Date(60_000).toISOString(), label: 'aberto', kind: 'state' as const },
      { time: new Date(90_000).toISOString(), label: '' },
    ]
    expect(recordingCategory(recAt(0), events, 300_000)).toBe('movimento')
  })
  it('estado tem prioridade sobre continua mas perde para pessoa/ia/movimento', () => {
    const stateOnly = [{ time: new Date(60_000).toISOString(), label: 'fechado', kind: 'state' as const }]
    expect(recordingCategory(recAt(0), stateOnly, 300_000)).toBe('estados')
    const withPerson = [
      { time: new Date(60_000).toISOString(), label: 'fechado', kind: 'state' as const },
      { time: new Date(90_000).toISOString(), label: 'pessoa' },
    ]
    expect(recordingCategory(recAt(0), withPerson, 300_000)).toBe('pessoa')
  })
})

describe('eventCategory', () => {
  it('sem label → movimento', () => {
    expect(eventCategory({})).toBe('movimento')
    expect(eventCategory({ label: '' })).toBe('movimento')
    expect(eventCategory({ label: '   ' })).toBe('movimento')
  })
  it('label pessoa/person → pessoa', () => {
    expect(eventCategory({ label: 'pessoa' })).toBe('pessoa')
    expect(eventCategory({ label: 'Pessoa com chapéu' })).toBe('pessoa')
    expect(eventCategory({ label: 'person' })).toBe('pessoa')
  })
  it('outro label de modelo → ia', () => {
    expect(eventCategory({ label: 'carro' })).toBe('ia')
    expect(eventCategory({ label: 'cachorro' })).toBe('ia')
  })
  it('kind=state → estados (independe do label)', () => {
    expect(eventCategory({ kind: 'state', label: 'aberto' })).toBe('estados')
    expect(eventCategory({ kind: 'state', label: 'pessoa' })).toBe('estados')
    expect(eventCategory({ kind: 'state', label: '' })).toBe('estados')
  })
})

describe('eventTitle', () => {
  it('título por categoria', () => {
    expect(eventTitle({})).toBe('Movimento detectado')
    expect(eventTitle({ label: 'pessoa' })).toBe('Pessoa detectada')
    expect(eventTitle({ label: 'carro' })).toBe('carro')
  })
})

describe('firstEventInChunk', () => {
  const recAt = (ms: number) => ({ start: new Date(ms).toISOString() })
  const ev = (id: number, ms: number, extra: Record<string, unknown> = {}) =>
    ({ id, time: new Date(ms).toISOString(), ...extra })

  it('devolve o evento mais antigo (por time) dentro de [start, start+chunk)', () => {
    const events = [ev(2, 120_000), ev(1, 60_000), ev(3, 200_000)]
    expect(firstEventInChunk(recAt(0), events, 300_000)?.id).toBe(1)
  })
  it('ignora eventos fora da janela do chunk', () => {
    const events = [ev(9, 400_000)]
    expect(firstEventInChunk(recAt(0), events, 300_000)).toBeNull()
  })
  it('inclui transições de estado (kind=state)', () => {
    const events = [ev(5, 90_000, { kind: 'state', label: 'aberto' })]
    expect(firstEventInChunk(recAt(0), events, 300_000)?.id).toBe(5)
  })
  it('sem eventos → null', () => {
    expect(firstEventInChunk(recAt(0), [], 300_000)).toBeNull()
  })
})

describe('eventCardLines', () => {
  it('estado: classificador no título, estado no subtítulo (Janela / apagada)', () => {
    expect(eventCardLines({ kind: 'state', label: 'apagada', classifier_name: 'Janela' }, 'Cam1'))
      .toEqual({ title: 'Janela', subtitle: 'apagada' })
  })
  it('estado sem classifier_name cai pra câmera no título', () => {
    expect(eventCardLines({ kind: 'state', label: 'apagada' }, 'Cam1'))
      .toEqual({ title: 'Cam1', subtitle: 'apagada' })
  })
  it('movimento: descrição no título, câmera no subtítulo', () => {
    expect(eventCardLines({ label: '' }, 'Cam1'))
      .toEqual({ title: 'Movimento detectado', subtitle: 'Cam1' })
  })
  it('ia: label no título, câmera no subtítulo', () => {
    expect(eventCardLines({ label: 'carro' }, 'Cam1'))
      .toEqual({ title: 'carro', subtitle: 'Cam1' })
  })
})

describe('filterEventsByCategory', () => {
  const evs = [
    { id: 1, label: '' },
    { id: 2, label: 'pessoa' },
    { id: 3, label: 'carro' },
    { id: 4, label: 'Pessoa detectada' },
    { id: 5, label: 'aberto', kind: 'state' as const },
  ]
  it('todos devolve tudo', () => {
    expect(filterEventsByCategory(evs, 'todos')).toHaveLength(5)
  })
  it('filtra por movimento', () => {
    expect(filterEventsByCategory(evs, 'movimento').map(e => e.id)).toEqual([1])
  })
  it('filtra por pessoa', () => {
    expect(filterEventsByCategory(evs, 'pessoa').map(e => e.id)).toEqual([2, 4])
  })
  it('filtra por ia', () => {
    expect(filterEventsByCategory(evs, 'ia').map(e => e.id)).toEqual([3])
  })
  it('filtra por estados (transições kind=state)', () => {
    expect(filterEventsByCategory(evs, 'estados').map(e => e.id)).toEqual([5])
  })
})
