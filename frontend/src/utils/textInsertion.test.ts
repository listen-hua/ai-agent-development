import { describe, expect, it } from 'vitest'
import { insertTextAtSelection } from './textInsertion'

describe('insertTextAtSelection', () => {
  it('inserts at the cursor and replaces a selection', () => {
    expect(insertTextAtSelection('通知正文', ':OK:', 2, 2, 20)).toEqual({ value: '通知:OK:正文', cursor: 6 })
    expect(insertTextAtSelection('通知正文', ':OK:', 0, 2, 20)).toEqual({ value: ':OK:正文', cursor: 4 })
  })

  it('appends without an active input and rejects values beyond the limit', () => {
    expect(insertTextAtSelection('通知', ':OK:', undefined, undefined, 20)).toEqual({ value: '通知:OK:', cursor: 6 })
    expect(insertTextAtSelection('通知', ':OK:', 2, 2, 5)).toBeUndefined()
  })
})
