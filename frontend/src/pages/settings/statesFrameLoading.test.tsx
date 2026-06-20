import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, fireEvent, waitFor } from '@testing-library/react'
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

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn(async () => new Response('[]', { status: 200 })))
})
afterEach(() => { cleanup(); vi.unstubAllGlobals() })

function base(): StateClassifier {
  return {
    name: 'Portão', threshold: 0.8, trigger_motion: false, trigger_interval_seconds: 10,
    crop_x: 0.3, crop_y: 0.3, crop_w: 0.4, crop_h: 0.4, min_consecutive: 3, enabled: true,
    classes: ['aberto', 'fechado'],
    notify_enabled: false, footer_enabled: false, notify_user_ids: [], footer_user_ids: [],
  }
}

function Harness({ initial }: { initial: StateClassifier }) {
  const [value, setValue] = useState(initial)
  return (
    <MemoryRouter>
      <ClassifierForm cameraId="cam1" value={value} onChange={setValue} onDone={() => {}} onCancel={() => {}} />
    </MemoryRouter>
  )
}

const el = (id: string) => document.getElementById(id)

describe('ClassifierForm — quadro com loading no tamanho real', () => {
  it('reserva a proporção do quadro e mostra loading até a imagem carregar', async () => {
    render(<Harness initial={base()} />)

    // O container do quadro reserva proporção própria — não depende da altura da <img>.
    const frame = el('state-frame')
    expect(frame).toBeTruthy()
    expect(frame!.className).toContain('aspect-video')

    // Antes do onLoad, o indicador de carregamento está visível e o recorte está
    // oculto (só aparece junto com a imagem).
    expect(el('state-frame-loading')).toBeTruthy()
    expect(el('state-frame-overlay')!.className).toContain('opacity-0')

    // Dispara o load da imagem do quadro → o loading some e o recorte aparece.
    const img = frame!.querySelector('img') as HTMLImageElement
    expect(img).toBeTruthy()
    fireEvent.load(img)

    await waitFor(() => expect(el('state-frame-loading')).toBeNull())
    expect(el('state-frame-overlay')!.className).toContain('opacity-100')
  })
})
