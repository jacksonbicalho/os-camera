import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import ReportsPage from './ReportsPage'

vi.mock('../auth', () => ({
  authHeaders: () => ({}),
  onUnauthorized: vi.fn(),
}))
vi.mock('../components/AppLayout', () => ({ default: ({ children }: { children: React.ReactNode }) => <div>{children}</div> }))
vi.mock('../components/DatePicker', () => ({ default: () => <div data-testid="datepicker" /> }))

const cameras = [{ id: 'cam1', name: 'Corredor' }]

// 168 células zero, com domingo/9h = 4 (máximo) e segunda/12h = 2
function makeHeatmap() {
  const cells: { weekday: number; hour: number; count: number }[] = []
  for (let wd = 0; wd < 7; wd++) {
    for (let h = 0; h < 24; h++) {
      let count = 0
      if (wd === 0 && h === 9) count = 4
      if (wd === 1 && h === 12) count = 2
      cells.push({ weekday: wd, hour: h, count })
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
  it('busca o bucket=heatmap e renderiza a grade 7×24 com células', async () => {
    render(
      <MemoryRouter initialEntries={['/reports']}>
        <ReportsPage />
      </MemoryRouter>,
    )

    await waitFor(() => {
      const grid = document.getElementById('report-heatmap')
      if (!grid) throw new Error('heatmap não renderizou')
    })

    // 168 células presentes
    const cells = document.querySelectorAll('[id^="report-heatmap-cell-"]')
    expect(cells.length).toBe(168)

    // célula com mais eventos (dom/9h) tem o título com a contagem
    const hot = document.getElementById('report-heatmap-cell-0-9')
    expect(hot?.getAttribute('title')).toContain('4')

    // um fetch com bucket=heatmap foi disparado
    const fetchMock = fetch as unknown as ReturnType<typeof vi.fn>
    expect(fetchMock.mock.calls.some(([u]: [string]) => u.includes('bucket=heatmap'))).toBe(true)
  })
})
