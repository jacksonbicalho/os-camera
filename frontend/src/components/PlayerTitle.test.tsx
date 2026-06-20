import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, render } from '@testing-library/react'
import PlayerTitle from './PlayerTitle'

afterEach(cleanup)

describe('PlayerTitle', () => {
  it('ao vivo: nome + badge AO VIVO + indicador pulsante', () => {
    render(<PlayerTitle isLive name="Corredor de entrada" />)
    expect(document.getElementById('player-title')!.textContent).toContain('Corredor de entrada')
    const badge = document.getElementById('live-badge')!
    expect(badge.textContent).toContain('AO VIVO')
    const dot = document.getElementById('live-indicator')!
    expect(dot.className).toContain('animate-pulse')
  })

  it('reprodução: badge Reprodução, sem indicador pulsante, com subtítulo', () => {
    render(<PlayerTitle isLive={false} name="Cam 1" subtitle={<span>20-06 18:00 · 0:30</span>} />)
    expect(document.getElementById('playback-badge')!.textContent).toContain('Reprodução')
    expect(document.getElementById('live-badge')).toBeNull()
    expect(document.getElementById('live-indicator')).toBeNull()
    expect(document.getElementById('player-title')!.textContent).toContain('20-06 18:00')
  })
})
