import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useBrowserNotifications } from './useBrowserNotifications'

const PREF_KEY = 'camera_browser_notifications_enabled'

function makeNotificationMock(permission: NotificationPermission = 'default') {
  const ctor = vi.fn()
  ctor.permission = permission
  ctor.requestPermission = vi.fn()
  return ctor
}

beforeEach(() => {
  localStorage.clear()
})

afterEach(() => {
  vi.unstubAllGlobals()
  localStorage.clear()
})

describe('useBrowserNotifications — suporte', () => {
  it('supported = false quando Notification não existe no window', () => {
    vi.stubGlobal('Notification', undefined)
    const { result } = renderHook(() => useBrowserNotifications())
    expect(result.current.supported).toBe(false)
  })

  it('supported = true quando Notification existe no window', () => {
    vi.stubGlobal('Notification', makeNotificationMock())
    const { result } = renderHook(() => useBrowserNotifications())
    expect(result.current.supported).toBe(true)
  })
})

describe('useBrowserNotifications — permissão', () => {
  it('reflete a permissão atual do browser', () => {
    vi.stubGlobal('Notification', makeNotificationMock('granted'))
    const { result } = renderHook(() => useBrowserNotifications())
    expect(result.current.permission).toBe('granted')
  })

  it('requestAndEnable chama requestPermission e habilita quando concedido', async () => {
    const mock = makeNotificationMock('default')
    mock.requestPermission.mockResolvedValue('granted')
    vi.stubGlobal('Notification', mock)

    const { result } = renderHook(() => useBrowserNotifications())

    await act(async () => {
      await result.current.requestAndEnable()
    })

    expect(mock.requestPermission).toHaveBeenCalledOnce()
    expect(result.current.enabled).toBe(true)
    expect(result.current.permission).toBe('granted')
  })

  it('requestAndEnable não habilita quando permissão negada', async () => {
    const mock = makeNotificationMock('default')
    mock.requestPermission.mockResolvedValue('denied')
    vi.stubGlobal('Notification', mock)

    const { result } = renderHook(() => useBrowserNotifications())

    await act(async () => {
      await result.current.requestAndEnable()
    })

    expect(result.current.enabled).toBe(false)
    expect(result.current.permission).toBe('denied')
  })
})

describe('useBrowserNotifications — preferência persistida', () => {
  it('começa desabilitado por padrão', () => {
    vi.stubGlobal('Notification', makeNotificationMock('granted'))
    const { result } = renderHook(() => useBrowserNotifications())
    expect(result.current.enabled).toBe(false)
  })

  it('restaura preferência habilitada do localStorage', () => {
    localStorage.setItem(PREF_KEY, 'true')
    vi.stubGlobal('Notification', makeNotificationMock('granted'))
    const { result } = renderHook(() => useBrowserNotifications())
    expect(result.current.enabled).toBe(true)
  })

  it('disable salva preferência false no localStorage', () => {
    localStorage.setItem(PREF_KEY, 'true')
    vi.stubGlobal('Notification', makeNotificationMock('granted'))
    const { result } = renderHook(() => useBrowserNotifications())

    act(() => { result.current.disable() })

    expect(result.current.enabled).toBe(false)
    expect(localStorage.getItem(PREF_KEY)).toBe('false')
  })
})

describe('useBrowserNotifications — notify', () => {
  it('cria Notification nativa quando suportado, habilitado e permissão concedida', async () => {
    const mock = makeNotificationMock('default')
    mock.requestPermission.mockResolvedValue('granted')
    vi.stubGlobal('Notification', mock)

    const { result } = renderHook(() => useBrowserNotifications())

    await act(async () => {
      await result.current.requestAndEnable()
    })

    act(() => { result.current.notify('entrada', 0.042) })

    expect(mock).toHaveBeenCalledOnce()
    expect(mock).toHaveBeenCalledWith(
      expect.stringContaining('entrada'),
      expect.objectContaining({ body: expect.stringContaining('0.042') }),
    )
  })

  it('não cria Notification quando desabilitado', async () => {
    const mock = makeNotificationMock('granted')
    vi.stubGlobal('Notification', mock)

    const { result } = renderHook(() => useBrowserNotifications())
    // enabled = false por padrão

    act(() => { result.current.notify('entrada', 0.042) })

    expect(mock).not.toHaveBeenCalled()
  })

  it('não cria Notification quando permissão negada', async () => {
    const mock = makeNotificationMock('denied')
    vi.stubGlobal('Notification', mock)
    localStorage.setItem(PREF_KEY, 'true')

    const { result } = renderHook(() => useBrowserNotifications())

    act(() => { result.current.notify('entrada', 0.042) })

    expect(mock).not.toHaveBeenCalled()
  })

  it('chama onClick ao clicar na notificação', async () => {
    const instances: Array<{ onclick: (() => void) | null }> = []
    function NotificationCtor(this: { onclick: null }) { this.onclick = null; instances.push(this) }
    NotificationCtor.permission = 'default' as NotificationPermission
    NotificationCtor.requestPermission = vi.fn().mockResolvedValue('granted')
    vi.stubGlobal('Notification', NotificationCtor)

    const { result } = renderHook(() => useBrowserNotifications())
    await act(async () => { await result.current.requestAndEnable() })

    const onClick = vi.fn()
    act(() => { result.current.notify('entrada', 0.042, onClick) })

    instances[0]?.onclick?.()
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('não lança exceção quando Notification não é suportado', () => {
    vi.stubGlobal('Notification', undefined)
    const { result } = renderHook(() => useBrowserNotifications())

    expect(() => {
      act(() => { result.current.notify('entrada', 0.042) })
    }).not.toThrow()
  })
})
