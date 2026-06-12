import { afterEach, describe, expect, it, vi } from 'vitest'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import ThemeModeNav from './ThemeModeNav'

const setMode = vi.fn()
let currentMode: 'dark' | 'light' | 'system' = 'dark'
let osDark = true

vi.mock('../contexts/ThemeContext', () => ({
  useTheme: () => ({ mode: currentMode, setMode, theme: 'default' }),
  resolveMode: (m: 'dark' | 'light' | 'system') =>
    m === 'system' ? (osDark ? 'dark' : 'light') : m,
}))

afterEach(() => {
  cleanup()
  setMode.mockClear()
  currentMode = 'dark'
  osDark = true
})

const trigger = () => document.getElementById('theme-nav-current')!

describe('ThemeModeNav', () => {
  it('colapsado: o gatilho mostra o modo selecionado e as opções ficam ocultas', () => {
    currentMode = 'dark'
    render(<ThemeModeNav />)
    expect(trigger().textContent).toContain('Dark')
    expect(document.getElementById('theme-mode-light')).toBeNull()
    expect(document.getElementById('theme-mode-system')).toBeNull()
  })

  it('o gatilho reflete o modo concreto (light/dark explícito)', () => {
    currentMode = 'light'
    render(<ThemeModeNav />)
    expect(trigger().textContent).toContain('Light')
  })

  it('com "Sistema" ativo + SO dark: gatilho e ✓ mostram Dark (não Sistema)', () => {
    currentMode = 'system'
    osDark = true
    render(<ThemeModeNav />)
    expect(trigger().textContent).toContain('Dark')
    expect(trigger().textContent).not.toContain('Sistema')
    fireEvent.click(trigger())
    expect(document.getElementById('theme-mode-dark')!.getAttribute('aria-current')).toBe('true')
    expect(document.getElementById('theme-mode-system')!.getAttribute('aria-current')).toBeNull()
  })

  it('com "Sistema" ativo + SO light: gatilho e ✓ mostram Light', () => {
    currentMode = 'system'
    osDark = false
    render(<ThemeModeNav />)
    expect(trigger().textContent).toContain('Light')
    fireEvent.click(trigger())
    expect(document.getElementById('theme-mode-light')!.getAttribute('aria-current')).toBe('true')
    expect(document.getElementById('theme-mode-system')!.getAttribute('aria-current')).toBeNull()
  })

  it('"Sistema" continua sendo uma opção selecionável', () => {
    render(<ThemeModeNav />)
    fireEvent.click(trigger())
    fireEvent.click(document.getElementById('theme-mode-system')!)
    expect(setMode).toHaveBeenCalledWith('system')
  })

  it('não exibe rótulos "Modo" nem "Tema"', () => {
    render(<ThemeModeNav />)
    fireEvent.click(trigger())
    expect(screen.queryByText('Modo')).toBeNull()
    expect(screen.queryByText('Tema')).toBeNull()
  })

  it('clicar no gatilho abre as opções Light/Dark/Sistema', () => {
    render(<ThemeModeNav />)
    fireEvent.click(trigger())
    expect(document.getElementById('theme-mode-light')).toBeTruthy()
    expect(document.getElementById('theme-mode-dark')).toBeTruthy()
    expect(document.getElementById('theme-mode-system')).toBeTruthy()
  })

  it('as opções abrem num flyout para a direita (left-full)', () => {
    render(<ThemeModeNav />)
    fireEvent.click(trigger())
    const flyout = document.getElementById('theme-mode-flyout')!
    expect(flyout.className).toContain('left-full')
  })

  it('selecionar uma opção aplica o modo e fecha a lista', () => {
    render(<ThemeModeNav />)
    fireEvent.click(trigger())
    fireEvent.click(document.getElementById('theme-mode-light')!)
    expect(setMode).toHaveBeenCalledWith('light')
    // lista fecha após selecionar
    expect(document.getElementById('theme-mode-light')).toBeNull()
  })

  it('marca a opção ativa com aria-current', () => {
    currentMode = 'light'
    render(<ThemeModeNav />)
    fireEvent.click(trigger())
    expect(document.getElementById('theme-mode-light')!.getAttribute('aria-current')).toBe('true')
    expect(document.getElementById('theme-mode-dark')!.getAttribute('aria-current')).toBeNull()
    expect(document.getElementById('theme-mode-system')!.getAttribute('aria-current')).toBeNull()
  })
})
