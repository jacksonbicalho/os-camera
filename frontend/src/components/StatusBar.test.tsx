import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import StatusBar from './StatusBar'

vi.mock('../hooks/useStats', () => ({
  useStats: () => ({
    cpu_percent: 18,
    sys_mem_total_bytes: 1000,
    sys_mem_free_bytes: 550,
    net_mbps: 12,
  }),
}))

vi.mock('./FooterStates', () => ({ default: () => <div id="footer-states" /> }))

afterEach(cleanup)

function renderBar(version?: string) {
  return render(
    <MemoryRouter>
      <StatusBar version={version} />
    </MemoryRouter>,
  )
}

describe('StatusBar — rodapé de status do sistema', () => {
  it('exibe Sistema operacional, CPU, Memória e Rede', () => {
    renderBar()
    const bar = document.getElementById('status-bar')!
    expect(bar.textContent).toContain('Sistema operacional')
    expect(document.getElementById('status-cpu')!.textContent).toContain('18%')
    expect(document.getElementById('status-mem')!.textContent).toContain('45%')
    const net = document.getElementById('status-net')!.textContent ?? ''
    expect(net).toContain('12')
    expect(net).toContain('Mbps')
  })

  it('exibe versão e indicador Conectado à direita', () => {
    renderBar('1.2.0')
    expect(document.getElementById('status-version')!.textContent).toContain('1.2.0')
    expect(document.getElementById('status-connection')!.textContent).toContain('Conectado')
  })

  it('preserva o FooterStates na barra', () => {
    renderBar()
    expect(document.getElementById('status-bar')!.querySelector('#footer-states')).toBeTruthy()
  })
})
