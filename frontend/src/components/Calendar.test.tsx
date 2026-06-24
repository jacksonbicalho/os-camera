import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, render, screen } from '@testing-library/react'
import Calendar from './Calendar'

afterEach(cleanup)

describe('Calendar', () => {
  it('renderiza o mês com os dias', () => {
    render(<Calendar mode="single" selected={new Date(2026, 5, 15)} month={new Date(2026, 5, 1)} />)
    expect(screen.getByText('15')).toBeTruthy()
  })
})
