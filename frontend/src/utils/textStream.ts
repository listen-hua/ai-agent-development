export interface TextStreamController {
  push(delta: string): void
  finish(finalText: string, onFinished: () => void): void
  cancel(): void
}

export function createTextStreamController(
  onText: (value: string) => void,
  intervalMs = 16,
): TextStreamController {
  let rendered = ''
  let queue: string[] = []
  let timer: ReturnType<typeof setTimeout> | undefined
  let finishing = false
  let finishCallback: (() => void) | undefined
  let cancelled = false

  function schedule() {
    if (cancelled || timer) return
    timer = setTimeout(tick, intervalMs)
  }

  function tick() {
    timer = undefined
    if (cancelled) return
    if (queue.length) {
      const count = queue.length > 400 ? 8 : queue.length > 180 ? 4 : queue.length > 60 ? 2 : 1
      rendered += queue.splice(0, count).join('')
      onText(rendered)
      schedule()
      return
    }
    if (finishing) {
      finishing = false
      const callback = finishCallback
      finishCallback = undefined
      callback?.()
    }
  }

  function push(delta: string) {
    if (cancelled || !delta) return
    queue.push(...Array.from(delta))
    schedule()
  }

  function finish(finalText: string, onFinished: () => void) {
    if (cancelled) return
    const buffered = rendered + queue.join('')
    if (finalText.startsWith(buffered)) {
      queue.push(...Array.from(finalText.slice(buffered.length)))
    } else if (finalText.startsWith(rendered)) {
      queue = Array.from(finalText.slice(rendered.length))
    } else {
      rendered = finalText
      queue = []
      onText(rendered)
    }
    finishing = true
    finishCallback = onFinished
    schedule()
  }

  function cancel() {
    cancelled = true
    queue = []
    finishing = false
    finishCallback = undefined
    if (timer) clearTimeout(timer)
    timer = undefined
  }

  return { push, finish, cancel }
}
