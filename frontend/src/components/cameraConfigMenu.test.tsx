import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, render, fireEvent } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import CameraConfigMenu from './CameraConfigMenu'

vi.mock('../auth', () => ({ getRole: () => 'admin' }))

afterEach(() => cleanup())

const el = (id: string) => document.getElementById(id)

function renderMenu() {
  return render(
    <MemoryRouter>
      <CameraConfigMenu cameraId="cam1" />
    </MemoryRouter>,
  )
}

describe('CameraConfigMenu — dropdown de configurações da câmera', () => {
  it('começa fechado e abre ao clicar, com as seções e hrefs por câmera', () => {
    renderMenu()
    expect(el('camera-config-menu-list')).toBeNull()

    fireEvent.click(el('camera-config-menu')!)
    expect(el('camera-config-menu-list')).toBeTruthy()

    expect(el('camera-config-item-detail')!.getAttribute('href')).toBe('/settings/cameras/cam1')
    expect(el('camera-config-item-edit')!.getAttribute('href')).toBe('/settings/cameras/edit/cam1')
    expect(el('camera-config-item-motion')!.getAttribute('href')).toBe('/settings/cameras/motion/cam1')
    expect(el('camera-config-item-zones')!.getAttribute('href')).toBe('/settings/cameras/zones/cam1')
    expect(el('camera-config-item-analysis')!.getAttribute('href')).toBe('/settings/cameras/analysis/cam1')
    expect(el('camera-config-item-states')!.getAttribute('href')).toBe('/settings/cameras/states/cam1')
  })

  it('fecha ao apertar Esc', () => {
    renderMenu()
    fireEvent.click(el('camera-config-menu')!)
    expect(el('camera-config-menu-list')).toBeTruthy()
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(el('camera-config-menu-list')).toBeNull()
  })

  it('fecha ao clicar fora', () => {
    renderMenu()
    fireEvent.click(el('camera-config-menu')!)
    expect(el('camera-config-menu-list')).toBeTruthy()
    fireEvent.mouseDown(document.body)
    expect(el('camera-config-menu-list')).toBeNull()
  })
})
