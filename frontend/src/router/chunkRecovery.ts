import type { Router } from 'vue-router'

const reloadMarkerKey = 'ai-agent:chunk-reload-target'
const chunkErrorPatterns = [
  /failed to fetch dynamically imported module/i,
  /error loading dynamically imported module/i,
  /importing a module script failed/i,
  /failed to load module script/i,
  /expected a javascript-or-wasm module script/i,
  /unable to preload css/i,
  /loading chunk .+ failed/i,
  /chunkloaderror/i,
]

export function isChunkLoadError(error: unknown) {
  const message = error instanceof Error ? `${error.name}: ${error.message}` : String(error ?? '')
  return chunkErrorPatterns.some((pattern) => pattern.test(message))
}

export function installChunkLoadRecovery(router: Router) {
  router.onError((error, to) => {
    if (!isChunkLoadError(error)) return
    const target = to.fullPath || `${window.location.pathname}${window.location.search}${window.location.hash}`
    if (window.sessionStorage.getItem(reloadMarkerKey) === target) {
      window.sessionStorage.removeItem(reloadMarkerKey)
      return
    }
    window.sessionStorage.setItem(reloadMarkerKey, target)
    window.location.replace(target)
  })

  router.afterEach((to) => {
    if (window.sessionStorage.getItem(reloadMarkerKey) === to.fullPath) {
      window.sessionStorage.removeItem(reloadMarkerKey)
    }
  })
}
