export interface TextInsertionResult {
  value: string
  cursor: number
}

export function insertTextAtSelection(
  value: string,
  text: string,
  selectionStart: number | null | undefined,
  selectionEnd: number | null | undefined,
  maxLength: number,
): TextInsertionResult | undefined {
  const start = Math.max(0, Math.min(selectionStart ?? value.length, value.length))
  const end = Math.max(start, Math.min(selectionEnd ?? start, value.length))
  const nextValue = `${value.slice(0, start)}${text}${value.slice(end)}`
  if (Array.from(nextValue).length > maxLength) return undefined
  return { value: nextValue, cursor: start + text.length }
}
