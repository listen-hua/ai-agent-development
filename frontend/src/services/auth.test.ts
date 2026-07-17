// @vitest-environment jsdom

import { afterEach, describe, expect, it, vi } from 'vitest'
import { authService, FeishuClientUnavailableError } from './auth'

afterEach(() => {
  delete window.h5sdk
  delete window.tt
})

describe('requestFeishuCode', () => {
  it('uses requestAuthCode for basic webpage SSO', async () => {
    type LegacyOptions = Parameters<NonNullable<NonNullable<Window['tt']>['requestAuthCode']>>[0]
    const requestAccess = vi.fn()
    const requestAuthCode = vi.fn((options: LegacyOptions) => {
      expect(options.appId).toBe('cli_test')
      options.success({ code: 'login-code' })
    })
    window.h5sdk = { ready: (callback) => callback() }
    window.tt = { requestAccess, requestAuthCode }

    await expect(authService.requestFeishuCode('cli_test')).resolves.toEqual({ code: 'login-code', auth_method: 'request_auth_code' })
    expect(requestAccess).not.toHaveBeenCalled()
  })

  it('uses requestAccess when requestAuthCode is unavailable', async () => {
    window.h5sdk = { ready: (callback) => callback() }
    window.tt = {
      requestAccess: (options) => {
        expect(options.appID).toBe('cli_test')
        expect(options.scopeList).toEqual([])
        options.success({ code: 'access-code', state: options.state })
      },
    }

    await expect(authService.requestFeishuCode('cli_test')).resolves.toEqual({ code: 'access-code', auth_method: 'request_access' })
  })

  it('reports that a normal browser has no Feishu bridge', async () => {
    await expect(authService.requestFeishuCode('cli_test')).rejects.toBeInstanceOf(FeishuClientUnavailableError)
  })

  it('returns the requestAuthCode error without invoking incremental authorization', async () => {
    const requestAccess = vi.fn()
    window.h5sdk = { ready: (callback) => callback() }
    window.tt = {
      requestAccess,
      requestAuthCode: (options) => options.fail({ errno: 20029, errString: 'invalid redirect url' }),
    }

    await expect(authService.requestFeishuCode('cli_test')).rejects.toThrow('20029')
    expect(requestAccess).not.toHaveBeenCalled()
  })
})
