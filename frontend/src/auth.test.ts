import { describe, it, expect, afterEach } from 'vitest'
import { setToken, clearToken, getUsername, mustChangePassword } from './auth'

function makeJwt(payload: object): string {
  const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' }))
  const body = btoa(JSON.stringify(payload))
  return `${header}.${body}.fakesignature`
}

describe('getUsername', () => {
  afterEach(() => clearToken())

  it('returns null when no token is stored', () => {
    expect(getUsername()).toBeNull()
  })

  it('returns the sub claim from a valid JWT payload', () => {
    setToken(makeJwt({ sub: 'master', exp: 9999999999 }))
    expect(getUsername()).toBe('master')
  })

  it('returns null when token is malformed', () => {
    setToken('not.a.jwt')
    expect(getUsername()).toBeNull()
  })
})

describe('mustChangePassword', () => {
  afterEach(() => clearToken())

  it('returns false when no token is stored', () => {
    expect(mustChangePassword()).toBe(false)
  })

  it('returns true when claim is true', () => {
    setToken(makeJwt({ sub: 'admin', must_change_password: true }))
    expect(mustChangePassword()).toBe(true)
  })

  it('returns false when claim is false', () => {
    setToken(makeJwt({ sub: 'admin', must_change_password: false }))
    expect(mustChangePassword()).toBe(false)
  })

  it('returns false when claim is absent', () => {
    setToken(makeJwt({ sub: 'admin' }))
    expect(mustChangePassword()).toBe(false)
  })
})
