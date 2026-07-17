import { afterEach, describe, expect, it, vi } from 'vitest'
import { createTextStreamController } from './textStream'

describe('createTextStreamController', () => {
  afterEach(() => vi.useRealTimers())

  it('renders short chunks one character per tick', () => {
    vi.useFakeTimers()
    const values: string[] = []
    const stream = createTextStreamController((value) => values.push(value), 10)
    stream.push('你好')
    vi.advanceTimersByTime(10)
    expect(values).toEqual(['你'])
    vi.advanceTimersByTime(10)
    expect(values).toEqual(['你', '你好'])
  })

  it('drains queued text before completing', () => {
    vi.useFakeTimers()
    let value = ''
    let finished = false
    const stream = createTextStreamController((next) => { value = next }, 10)
    stream.push('制度')
    stream.finish('制度回答', () => { finished = true })
    vi.runAllTimers()
    expect(value).toBe('制度回答')
    expect(finished).toBe(true)
  })
})
