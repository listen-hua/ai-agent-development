// @vitest-environment jsdom

import { afterEach, describe, expect, it, vi } from 'vitest'
import { authService, FeishuClientUnavailableError } from './auth'

afterEach(() => {
  delete window.h5sdk
  delete window.tt
})

describe('requestFeishuCode', () => {
  it('prefers requestAccess for webpage SSO', async () => {
    type AccessOptions = Parameters<NonNullable<NonNullable<Window['tt']>['requestAccess']>>[0]
    const requestAccess = vi.fn((options: AccessOptions) => {
      expect(options.appID).toBe('cli_test')
      expect(options.scopeList).toEqual([])
      options.success({ code: 'access-code', state: options.state })
    })
    const requestAuthCode = vi.fn()
    window.h5sdk = { ready: (callback) => callback() }
    window.tt = { requestAccess, requestAuthCode }

    await expect(authService.requestFeishuCode('cli_test')).resolves.toEqual({ code: 'access-code', auth_method: 'request_access' })
    expect(requestAuthCode).not.toHaveBeenCalled()
  })

  it('uses requestAuthCode only when requestAccess is unavailable', async () => {
    type LegacyOptions = Parameters<NonNullable<NonNullable<Window['tt']>['requestAuthCode']>>[0]
    window.h5sdk = { ready: (callback) => callback() }
    window.tt = {
      requestAuthCode: (options: LegacyOptions) => {
        expect(options.appId).toBe('cli_test')
        options.success({ code: 'legacy-code' })
      },
    }

    await expect(authService.requestFeishuCode('cli_test')).resolves.toEqual({ code: 'legacy-code', auth_method: 'request_auth_code' })
  })

  it('reports that a normal browser has no Feishu bridge', async () => {
    await expect(authService.requestFeishuCode('cli_test')).rejects.toBeInstanceOf(FeishuClientUnavailableError)
  })

  it('falls back to requestAuthCode when requestAccess fails', async () => {
    const requestAuthCode = vi.fn((options: Parameters<NonNullable<NonNullable<Window['tt']>['requestAuthCode']>>[0]) => {
      options.success({ code: 'legacy-code' })
    })
    window.h5sdk = { ready: (callback) => callback() }
    window.tt = {
      requestAccess: (options) => options.fail({ errno: 2700002, errString: 'authorization failed' }),
      requestAuthCode,
    }

    await expect(authService.requestFeishuCode('cli_test')).resolves.toEqual({ code: 'legacy-code', auth_method: 'request_auth_code' })
    expect(requestAuthCode).toHaveBeenCalledOnce()
  })

  it('explains app mismatch when both authorization methods fail', async () => {
    window.h5sdk = { ready: (callback) => callback() }
    window.tt = {
      requestAccess: (options) => options.fail({ errno: 2700002, errString: 'authorization failed' }),
      requestAuthCode: (options) => options.fail({ errno: 2602002, errString: 'server error' }),
    }

    await expect(authService.requestFeishuCode('cli_test')).rejects.toThrow('FEISHU_APP_ID')
  })
})
