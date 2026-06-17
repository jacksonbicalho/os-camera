import { render, waitFor } from '@testing-library/react'
import { useState } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ClassifierForm } from './CameraStatesSettingsPage'
import type { StateClassifier } from './stateClassifier'

function validValue(): StateClassifier {
  return {
    name: 'Portão',
    threshold: 0.8,
    trigger_motion: false,
    trigger_interval_seconds: 10,
    crop_x: 0.3, crop_y: 0.3, crop_w: 0.4, crop_h: 0.4,
    min_consecutive: 3,
    enabled: true,
    classes: ['fechado', 'aberto'],
  }
}

// Wrapper controlado: simula o pai, propagando o que o form devolve via onChange
// (é justamente o ponto do bug — o id criado precisa voltar para o value).
function Harness() {
  const [value, setValue] = useState<StateClassifier>(validValue())
  return (
    <ClassifierForm
      cameraId="cam1"
      value={value}
      onChange={setValue}
      onDone={() => {}}
      onCancel={() => {}}
    />
  )
}

const calls: { url: string; method: string }[] = []

beforeEach(() => {
  calls.length = 0
  localStorage.setItem('token', 't')
  vi.stubGlobal('fetch', vi.fn(async (url: unknown, opts: { method?: string } = {}) => {
    const method = opts.method ?? 'GET'
    const u = String(url)
    calls.push({ url: u, method })
    if (method === 'POST' && u.split('?')[0].endsWith('/classifiers')) {
      return new Response(JSON.stringify({ id: 12 }), { status: 200 })
    }
    return new Response(JSON.stringify({ samples: {} }), { status: 200 })
  }))
})

afterEach(() => {
  vi.unstubAllGlobals()
  localStorage.clear()
})

function createCalls() {
  return calls.filter(c => c.method === 'POST' && c.url.split('?')[0].endsWith('/classifiers'))
}

describe('ClassifierForm — não duplica ao salvar duas vezes', () => {
  it('o 2º Salvar vira PUT (id propagado após criar) — um único POST de create', async () => {
    render(<Harness />)
    const saveBtn = document.getElementById('state-classifier-save') as HTMLButtonElement

    saveBtn.click()
    await waitFor(() => expect(createCalls()).toHaveLength(1))

    saveBtn.click()
    await waitFor(() => expect(calls.some(c => c.method === 'PUT')).toBe(true))

    // não recriou: continua um único POST de create
    expect(createCalls()).toHaveLength(1)
  })
})
