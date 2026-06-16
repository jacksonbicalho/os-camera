import { describe, expect, it } from 'vitest'
import { postChangeRedirect } from './changePasswordRedirect'

describe('postChangeRedirect', () => {
  it('fluxo forçado: admin sem câmeras vai para o cadastro de câmera', () => {
    expect(postChangeRedirect({ forced: true, adminWithNoCameras: true })).toBe('/settings/cameras/new')
  })

  it('fluxo forçado: caso geral vai para a home', () => {
    expect(postChangeRedirect({ forced: true, adminWithNoCameras: false })).toBe('/')
  })

  it('fluxo manual: volta para a origem (from)', () => {
    expect(postChangeRedirect({ forced: false, from: '/settings/users/5', adminWithNoCameras: false })).toBe('/settings/users/5')
  })

  it('fluxo manual sem origem: cai na home', () => {
    expect(postChangeRedirect({ forced: false, from: null, adminWithNoCameras: true })).toBe('/')
  })
})
