import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, screen, fireEvent } from '@testing-library/react'
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

const users = [{ id: 1, username: 'alice' }, { id: 2, username: 'bob' }]

beforeEach(() => {
  vi.stubGlobal('fetch', vi.fn(async (url: unknown) => {
    const u = String(url)
    if (u.includes('/api/users')) return new Response(JSON.stringify(users), { status: 200 })
    return new Response('[]', { status: 200 })
  }))
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

const el = (id: string) => document.getElementById(id) as HTMLInputElement

describe('ClassifierForm — destinatários de notificação/rodapé', () => {
  it('checkbox "Enviar notificação" revela a lista e Todos/Nenhum/individual atualizam', async () => {
    render(<Harness initial={base()} />)

    // desabilitado: lista escondida
    expect(el('recipient-notify-all')).toBeNull()

    // habilita → lista aparece com os usuários (de GET /api/users)
    fireEvent.click(el('recipient-notify-enabled'))
    await screen.findByText('alice')
    await screen.findByText('bob')
    expect(screen.getByText('0 de 2')).toBeTruthy()

    // Todos → 2 de 2
    fireEvent.click(el('recipient-notify-all'))
    await screen.findByText('2 de 2')

    // Nenhum → 0 de 2
    fireEvent.click(el('recipient-notify-none'))
    await screen.findByText('0 de 2')

    // marca 1 usuário individual → 1 de 2
    fireEvent.click(screen.getAllByRole('checkbox').find(c => (c as HTMLInputElement).closest('label')?.textContent?.includes('alice'))!)
    await screen.findByText('1 de 2')
  })

  it('"Exibir no rodapé" é independente da notificação', async () => {
    render(<Harness initial={base()} />)

    // habilita só o rodapé
    fireEvent.click(el('recipient-footer-enabled'))
    await screen.findByText('alice')
    // notify continua desabilitado (sem sua lista)
    expect(el('recipient-notify-all')).toBeNull()
    expect(el('recipient-footer-all')).toBeTruthy()

    fireEvent.click(el('recipient-footer-all'))
    await screen.findByText('2 de 2')
  })
})
