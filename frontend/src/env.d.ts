/// <reference types="vite/client" />

interface Window {
  h5sdk?: { ready: (callback: () => void) => void }
  tt?: {
    requestAccess?: (options: { appID: string; scopeList: string[]; state?: string; success: (result: { code: string; state?: string }) => void; fail: (error: unknown) => void }) => void
    requestAuthCode?: (options: { appId: string; success: (result: { code: string }) => void; fail: (error: unknown) => void }) => void
  }
}
