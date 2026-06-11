import { afterEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { ThemeProvider, useTheme } from './ThemeContext'

afterEach(() => {
  cleanup()
  document.documentElement.removeAttribute('data-theme')
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

function Probe() {
  const { theme, setTheme } = useTheme()
  return (
    <>
      <span data-testid="theme">{theme}</span>
      <button onClick={() => setTheme('light')}>set-light</button>
      <button onClick={() => setTheme('dark')}>set-dark</button>
    </>
  )
}

describe('ThemeContext', () => {
  it('loads the saved theme and applies data-theme on <html>', async () => {
    mockFetch.mockResolvedValue({ status: 200, json: async () => ({ theme: 'light' }) })

    render(<ThemeProvider><Probe /></ThemeProvider>)

    await waitFor(() => expect(screen.getByTestId('theme').textContent).toBe('light'))
    expect(document.documentElement.getAttribute('data-theme')).toBe('light')
  })

  it('"system" resolves to dark when the OS prefers dark', async () => {
    mockMatchMedia(true)
    mockFetch.mockResolvedValue({ status: 200, json: async () => ({ theme: 'system' }) })

    render(<ThemeProvider><Probe /></ThemeProvider>)

    await waitFor(() => expect(screen.getByTestId('theme').textContent).toBe('system'))
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark')
  })

  it('"system" resolves to light when the OS prefers light', async () => {
    mockMatchMedia(false)
    mockFetch.mockResolvedValue({ status: 200, json: async () => ({ theme: 'system' }) })

    render(<ThemeProvider><Probe /></ThemeProvider>)

    await waitFor(() => expect(screen.getByTestId('theme').textContent).toBe('system'))
    expect(document.documentElement.getAttribute('data-theme')).toBe('light')
  })

  it('setTheme applies and persists via PUT', async () => {
    mockFetch.mockResolvedValue({ status: 200, json: async () => ({ theme: 'dark' }) })

    render(<ThemeProvider><Probe /></ThemeProvider>)
    await waitFor(() => expect(screen.getByTestId('theme').textContent).toBe('dark'))

    mockFetch.mockClear()
    mockFetch.mockResolvedValue({ status: 200, json: async () => ({}) })
    act(() => { fireEvent.click(screen.getByText('set-light')) })

    expect(document.documentElement.getAttribute('data-theme')).toBe('light')
    expect(screen.getByTestId('theme').textContent).toBe('light')

    const put = mockFetch.mock.calls.find(
      (c: unknown[]) => c[0] === '/api/me/preferences' && (c[1] as RequestInit)?.method === 'PUT'
    )
    expect(put).toBeTruthy()
    expect(JSON.parse((put![1] as RequestInit).body as string)).toEqual({ theme: 'light' })
  })
})
