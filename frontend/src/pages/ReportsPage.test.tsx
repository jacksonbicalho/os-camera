import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, waitFor, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import ReportsPage from './ReportsPage'

vi.mock('../auth', () => ({
  authHeaders: () => ({}),
  onUnauthorized: vi.fn(),
}))
vi.mock('../components/AppLayout', () => ({ default: ({ children }: { children: React.ReactNode }) => <div>{children}</div> }))
vi.mock('../components/DatePicker', () => ({ default: () => <div data-testid="datepicker" /> }))

const cameras = [{ id: 'cam1', name: 'Corredor' }]

// 3 dias × 24h zero-fill, com 22/9h = 4 (máximo) e 23/12h = 2
const HEAT_DATES = ['2026-06-22', '2026-06-23', '2026-06-24']
function makeHeatmap() {
  const cells: { date: string; hour: number; count: number }[] = []
  for (const date of HEAT_DATES) {
    for (let h = 0; h < 24; h++) {
      let count = 0
      if (date === '2026-06-22' && h === 9) count = 4
      if (date === '2026-06-23' && h === 12) count = 2
      cells.push({ date, hour: h, count })
    }
  }
  return cells
}

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn((url: string) => {
    if (url.startsWith('/api/cameras')) return Promise.resolve({ status: 200, json: () => Promise.resolve(cameras) })
    if (url.includes('bucket=heatmap')) return Promise.resolve({ status: 200, json: () => Promise.resolve({ total: 6, heatmap: makeHeatmap() }) })
    if (url.startsWith('/api/reports/events')) return Promise.resolve({ status: 200, json: () => Promise.resolve({ total: 0, by_day: [], by_label: {} }) })
    return Promise.resolve({ status: 404, json: () => Promise.resolve({}) })
  }))
})
afterEach(() => { cleanup(); vi.unstubAllGlobals() })

describe('ReportsPage heatmap', () => {
  it('busca o bucket=heatmap e renderiza uma linha por dia, rotulada DD + dia da semana', async () => {
    render(
      <MemoryRouter initialEntries={['/reports']}>
        <ReportsPage />
      </MemoryRouter>,
    )

    await waitFor(() => {
      const grid = document.getElementById('report-heatmap')
      if (!grid) throw new Error('heatmap não renderizou')
    })

    // uma linha por dia (3 dias) com 24 células cada
    const rows = document.querySelectorAll('[id^="report-heatmap-row-"]')
    expect(rows.length).toBe(HEAT_DATES.length)
    expect(document.querySelectorAll('[id^="report-heatmap-cell-2026-06-22-"]').length).toBe(24)

    // ordem mais recente no topo: a 1ª linha é o último dia (2026-06-24)
    expect(rows[0].id).toBe('report-heatmap-row-2026-06-24')

    // rótulo "22/06/2026 Seg" (2026-06-22 é segunda-feira); data dd/mm/yyyy + dia da semana
    const row22 = document.getElementById('report-heatmap-row-2026-06-22')
    expect(row22?.textContent).toContain('22/06/2026 Seg')

    // célula com mais eventos (22/9h) tem o título com a contagem
    expect(document.getElementById('report-heatmap-cell-2026-06-22-9')?.getAttribute('title')).toContain('4')

    // um fetch com bucket=heatmap foi disparado
    const fetchMock = fetch as unknown as ReturnType<typeof vi.fn>
    expect(fetchMock.mock.calls.some(([u]: [string]) => u.includes('bucket=heatmap'))).toBe(true)
  })

  it('rotula todas as 24 horas (0–23) no cabeçalho do heatmap', async () => {
    render(<MemoryRouter initialEntries={['/reports']}><ReportsPage /></MemoryRouter>)
    const grid = await waitFor(() => {
      const el = document.getElementById('report-heatmap')
      if (!el) throw new Error('heatmap não renderizou')
      return el
    })
    const header = grid.querySelector('div')!.firstElementChild as HTMLElement
    const labels = Array.from(header.children).map(c => c.textContent)
    expect(labels).toEqual(Array.from({ length: 24 }, (_, h) => String(h)))
  })

  it('no modo "1 dia" esconde o heatmap e busca barras por hora', async () => {
    const fetchMock = fetch as unknown as ReturnType<typeof vi.fn>
    render(<MemoryRouter initialEntries={['/reports']}><ReportsPage /></MemoryRouter>)
    await waitFor(() => {
      if (!document.getElementById('report-heatmap')) throw new Error('heatmap não renderizou')
    })

    const range = document.getElementById('report-range-select') as HTMLSelectElement
    fireEvent.change(range, { target: { value: '1' } })

    await waitFor(() => {
      if (document.getElementById('report-heatmap')) throw new Error('heatmap deveria sumir no modo 1 dia')
    })
    expect(fetchMock.mock.calls.some(([u]: [string]) => u.includes('bucket=hour'))).toBe(true)
  })
})
