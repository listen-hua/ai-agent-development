const cartoonPattern = /("cartoonization_strength"\s*:\s*)(-?(?:\d+(?:\.\d*)?|\.\d+))/g

export const IMAGE_PROMPT_MAX_LENGTH = 15_000

export const cartoonStrengthLabels: Record<number, string> = {
  0: 'Very weak stylization.',
  0.25: 'Light stylization.',
  0.5: 'Medium stylization.',
  0.75: 'Strong stylization.',
  1: 'Maximum stylization.',
}

export function cartoonStrength(prompt: string): number | null {
  cartoonPattern.lastIndex = 0
  const matches = [...prompt.matchAll(cartoonPattern)]
  if (matches.length !== 1) return null
  const value = Number(matches[0]?.[2])
  return Number.isFinite(value) && value >= 0 && value <= 1 ? value : null
}

export function hasCartoonStrength(prompt: string): boolean {
  return cartoonStrength(prompt) !== null
}

export function replaceCartoonStrength(prompt: string, value: number): string {
  cartoonPattern.lastIndex = 0
  return prompt.replace(cartoonPattern, `$1${formatStrength(value)}`)
}

function formatStrength(value: number): string {
  if (value === 0 || value === 1) return String(value)
  return value.toFixed(2).replace(/0$/, '')
}
