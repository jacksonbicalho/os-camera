import { describe, it, expect, afterEach } from 'vitest'
import { setToken, clearToken, getUsername } from './auth'

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
