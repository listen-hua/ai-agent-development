import { describe, expect, it } from 'vitest'
import { cartoonStrength, hasCartoonStrength, replaceCartoonStrength } from './imagePrompt'

describe('image prompt cartoon strength', () => {
  it('reads and updates the configured field', () => {
    const prompt = '{"style":"comic","cartoonization_strength": 0}'
    expect(cartoonStrength(prompt)).toBe(0)
    expect(replaceCartoonStrength(prompt, 0.75)).toContain('"cartoonization_strength": 0.75')
  })

  it('does not treat duplicate or invalid fields as controllable', () => {
    expect(hasCartoonStrength('"cartoonization_strength": 3')).toBe(false)
    expect(hasCartoonStrength('"cartoonization_strength": 0, "cartoonization_strength": 1')).toBe(false)
  })
})
