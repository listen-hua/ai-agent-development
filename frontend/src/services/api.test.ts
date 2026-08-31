import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const axiosMock = vi.hoisted(() => ({
  request: vi.fn(),
  defaults: {} as { baseURL?: string },
}))

vi.mock('axios', () => {
  class MockAxiosError extends Error {
    response?: { status: number; data?: unknown }
  }
  return {
    default: { create: () => axiosMock },
    AxiosError: MockAxiosError,
  }
})

import { AxiosError } from 'axios'
import { api, setSessionRecoveryHandler } from './api'

function unauthorized() {
  const error = new AxiosError('unauthorized')
  Object.assign(error, { response: { status: 401, data: { message: '登录已过期' } } })
  return error
}

describe('API session recovery', () => {
  beforeEach(() => {
    axiosMock.request.mockReset()
    setSessionRecoveryHandler(undefined)
  })

  afterEach(() => setSessionRecoveryHandler(undefined))

  it('shares one recovery across concurrent 401 responses and retries each request once', async () => {
    let finishRecovery!: () => void
    const recovery = vi.fn(() => new Promise<void>((resolve) => { finishRecovery = resolve }))
    setSessionRecoveryHandler(recovery)
    axiosMock.request
      .mockRejectedValueOnce(unauthorized())
      .mockRejectedValueOnce(unauthorized())
      .mockResolvedValueOnce({ status: 200, data: { id: 1 } })
      .mockResolvedValueOnce({ status: 200, data: { id: 2 } })

    const first = api.get<{ id: number }>('/api/v1/admin/one')
    const second = api.get<{ id: number }>('/api/v1/admin/two')
    await vi.waitFor(() => expect(recovery).toHaveBeenCalledTimes(1))
    finishRecovery()

    await expect(Promise.all([first, second])).resolves.toEqual([{ id: 1 }, { id: 2 }])
    expect(axiosMock.request).toHaveBeenCalledTimes(4)
  })

  it('does not recover recursively when the retried request is still unauthorized', async () => {
    const recovery = vi.fn().mockResolvedValue(undefined)
    setSessionRecoveryHandler(recovery)
    axiosMock.request.mockRejectedValue(unauthorized())

    await expect(api.put('/api/v1/admin/settings', {})).rejects.toMatchObject({ status: 401 })
    expect(recovery).toHaveBeenCalledTimes(1)
    expect(axiosMock.request).toHaveBeenCalledTimes(2)
  })

  it('never intercepts authentication exchange endpoints', async () => {
    const recovery = vi.fn().mockResolvedValue(undefined)
    setSessionRecoveryHandler(recovery)
    axiosMock.request.mockRejectedValue(unauthorized())

    await expect(api.post('/api/v1/auth/iam/exchange')).rejects.toMatchObject({ status: 401 })
    expect(recovery).not.toHaveBeenCalled()
  })
})
