import { describe, expect, it } from 'vitest'
import { eventCleanFrameURL } from './stateEventFrames'

describe('eventCleanFrameURL', () => {
  it('deriva o _frame.jpg limpo do nome do snapshot _motion.jpg', () => {
    const url = eventCleanFrameURL('cam1', {
      time: '2026-06-10T12:30:45Z',
      frame: '20260610123045_motion.jpg',
    })
    expect(url).toContain('/recordings/cam1/2026/06/10/20260610123045_frame.jpg')
  })
})
