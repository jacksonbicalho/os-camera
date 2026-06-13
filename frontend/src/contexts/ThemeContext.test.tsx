import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { ThemeProvider, useTheme } from './ThemeContext'

afterEach(() => {
  cleanup()
  document.documentElement.removeAttribute('data-mode')
})

// jsdom doesn't implement matchMedia; install a controllable mock.
function mockMatchMedia(dark: boolean) {
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: query.includes('dark') ? dark : !dark,
    media: query,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
    onchange: null,
  })) as unknown as typeof window.matchMedia
}

vi.mock('../auth', () => ({
  getToken: () => 'fake',
  authHeaders: () => ({}),
  onUnauthorized: vi.fn(),
}))

const mockFetch = vi.fn()
global.fetch = mockFetch

// dark/light/system are COLOR MODES; the theme identity is separate ("default").
function Probe() {
  const { mode, setMode, theme } = useTheme()
  return (
    <>
      <span data-testid="mode">{mode}</span>
      <span data-testid="theme">{theme}</span>
      <button onClick={() => setMode('light')}>set-light</button>
    </>
  )
}

describe('ThemeContext', () => {
  it('loads the saved color mode and applies data-mode on <html>; theme is "default"', async () => {
    mockFetch.mockResolvedValue({ status: 200, json: async () => ({ theme: 'light' }) })

    render(<ThemeProvider><Probe /></ThemeProvider>)

    await waitFor(() => expect(screen.getByTestId('mode').textContent).toBe('light'))
    expect(document.documentElement.getAttribute('data-mode')).toBe('light')
    expect(screen.getByTestId('theme').textContent).toBe('default')
  })

  it('"system" resolves to dark when the OS prefers dark', async () => {
    mockMatchMedia(true)
    mockFetch.mockResolvedValue({ status: 200, json: async () => ({ theme: 'system' }) })

    render(<ThemeProvider><Probe /></ThemeProvider>)

    await waitFor(() => expect(screen.getByTestId('mode').textContent).toBe('system'))
    expect(document.documentElement.getAttribute('data-mode')).toBe('dark')
  })

  it('"system" resolves to light when the OS prefers light', async () => {
    mockMatchMedia(false)
    mockFetch.mockResolvedValue({ status: 200, json: async () => ({ theme: 'system' }) })

    render(<ThemeProvider><Probe /></ThemeProvider>)

    await waitFor(() => expect(screen.getByTestId('mode').textContent).toBe('system'))
    expect(document.documentElement.getAttribute('data-mode')).toBe('light')
  })

  it('setMode applies data-mode and persists via PUT (as the preference value)', async () => {
    mockFetch.mockResolvedValue({ status: 200, json: async () => ({ theme: 'dark' }) })

    render(<ThemeProvider><Probe /></ThemeProvider>)
    await waitFor(() => expect(screen.getByTestId('mode').textContent).toBe('dark'))

    mockFetch.mockClear()
    mockFetch.mockResolvedValue({ status: 200, json: async () => ({}) })
    act(() => { fireEvent.click(screen.getByText('set-light')) })

    expect(document.documentElement.getAttribute('data-mode')).toBe('light')
    expect(screen.getByTestId('mode').textContent).toBe('light')

    const put = mockFetch.mock.calls.find(
      (c: unknown[]) => c[0] === '/api/me/preferences' && (c[1] as RequestInit)?.method === 'PUT'
    )
    expect(put).toBeTruthy()
    expect(JSON.parse((put![1] as RequestInit).body as string)).toEqual({ theme: 'light' })
  })
})
