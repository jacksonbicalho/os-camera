import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { useState } from 'react'
import { ClassifierForm } from './CameraStatesSettingsPage'
import type { StateClassifier } from './stateClassifier'

vi.mock('../../auth', () => ({
  authHeaders: () => ({}),
  getToken: () => 'fake',
  getRole: () => 'admin',
  onUnauthorized: vi.fn(),
}))

const events = [
  { time: '2026-06-20T10:00:00Z', frame: 'a_motion.jpg' },
  { time: '2026-06-20T11:00:00Z', frame: 'b_motion.jpg' },
]

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn(async (url: unknown) => {
    const u = String(url)
    if (u.includes('/motion')) return new Response(JSON.stringify({ events }), { status: 200 })
    return new Response('[]', { status: 200 })
  }))
})
afterEach(() => { cleanup(); vi.unstubAllGlobals() })

function base(): StateClassifier {
  return {
    name: 'Janela', threshold: 0.8, trigger_motion: false, trigger_interval_seconds: 10,
    crop_x: 0.3, crop_y: 0.3, crop_w: 0.4, crop_h: 0.4, min_consecutive: 3, enabled: true,
    classes: ['acesa', 'apagada'],
    notify_enabled: false, footer_enabled: false, notify_user_ids: [], footer_user_ids: [],
  }
}

function Harness() {
  const [value, setValue] = useState<StateClassifier>(base())
  return (
    <MemoryRouter>
      <ClassifierForm cameraId="cam1" value={value} onChange={setValue} onDone={() => {}} onCancel={() => {}} />
    </MemoryRouter>
  )
}

const el = (id: string) => document.getElementById(id)

describe('EventPicker — rodapé com uma linha por classificação', () => {
  it('seleção fica no balde da classe atual; trocar o dropdown cria nova linha + linha "todos"', async () => {
    render(<Harness />)

    fireEvent.click(screen.getByText('Escolher dos eventos'))

    // espera os eventos carregarem
    const checks = await waitFor(() => {
      const cs = screen.getAllByTitle('Selecionar para adicionar em lote')
      expect(cs.length).toBe(2)
      return cs
    })

    // alvo padrão = primeira classe ("acesa"); marca o 1º evento → linha de "acesa"
    fireEvent.click(checks[0])
    await waitFor(() => expect(el('event-picker-row-acesa')).toBeTruthy())
    expect(el('event-picker-row-all')).toBeNull() // ainda só uma classificação

    // troca o dropdown para "apagada" e marca o 2º evento → segunda linha
    fireEvent.change(el('event-picker-target')!, { target: { value: 'apagada' } })
    fireEvent.click(checks[1])
    await waitFor(() => expect(el('event-picker-row-apagada')).toBeTruthy())

    // a primeira seleção permaneceu em "acesa" (não migrou ao trocar o dropdown)
    expect(el('event-picker-row-acesa')).toBeTruthy()
    // com mais de uma classificação aparece a linha "todos"
    expect(el('event-picker-row-all')).toBeTruthy()
  })

  it('"Limpar" remove a linha da classificação', async () => {
    render(<Harness />)
    fireEvent.click(screen.getByText('Escolher dos eventos'))
    const checks = await waitFor(() => {
      const cs = screen.getAllByTitle('Selecionar para adicionar em lote')
      expect(cs.length).toBe(2)
      return cs
    })

    fireEvent.click(checks[0])
    const row = await waitFor(() => { const r = el('event-picker-row-acesa'); expect(r).toBeTruthy(); return r! })

    fireEvent.click(within(row).getByText('Limpar'))
    await waitFor(() => expect(el('event-picker-row-acesa')).toBeNull())
  })
})
