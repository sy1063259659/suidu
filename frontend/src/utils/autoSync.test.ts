import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createAutoSync } from './autoSync'

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: unknown) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

function setup() {
  const readVersion = vi.fn<() => Promise<string>>().mockResolvedValue('1')
  const refresh = vi.fn<() => Promise<boolean>>().mockResolvedValue(true)
  const isActive = vi.fn(() => true)
  const onError = vi.fn()
  const sync = createAutoSync({ readVersion, refresh, isActive, onError })
  return { sync, readVersion, refresh, isActive, onError }
}

describe('automatic version synchronization', () => {
  beforeEach(() => { vi.useFakeTimers() })
  afterEach(() => { vi.clearAllTimers(); vi.useRealTimers() })

  it('refreshes the initial version, skips unchanged versions, and refreshes changes', async () => {
    const { sync, readVersion, refresh } = setup()
    await sync.check()
    expect(readVersion).not.toHaveBeenCalled()
    sync.start()
    await sync.check()
    expect(refresh).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(5000)
    expect(readVersion).toHaveBeenCalledTimes(2)
    expect(refresh).toHaveBeenCalledTimes(1)
    readVersion.mockResolvedValue('2')
    await vi.advanceTimersByTimeAsync(5000)
    expect(refresh).toHaveBeenCalledTimes(2)
    sync.stop()
  })

  it('retries the same version after a skipped refresh without an immediate loop', async () => {
    const { sync, readVersion, refresh } = setup()
    refresh.mockResolvedValueOnce(false)
    sync.start()
    await sync.check()
    expect(refresh).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(4999)
    expect(readVersion).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1)
    expect(refresh).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(5000)
    expect(refresh).toHaveBeenCalledTimes(2)
  })

  it.each(['hidden', 'offline', 'logged out'])('makes no requests while %s and checks immediately on wake', async () => {
    const { sync, readVersion, refresh, isActive } = setup()
    isActive.mockReturnValue(false)
    sync.start()
    await sync.check()
    await vi.advanceTimersByTimeAsync(20000)
    expect(readVersion).not.toHaveBeenCalled()
    expect(refresh).not.toHaveBeenCalled()
    isActive.mockReturnValue(true)
    await sync.check()
    expect(readVersion).toHaveBeenCalledTimes(1)
    expect(refresh).toHaveBeenCalledTimes(1)
  })

  it('does not start a refresh if the page becomes inactive during the version request', async () => {
    const { sync, readVersion, refresh, isActive } = setup()
    const version = deferred<string>()
    readVersion.mockReturnValueOnce(version.promise)
    sync.start()
    const first = sync.check()
    await Promise.resolve()
    isActive.mockReturnValue(false)
    version.resolve('1')
    await first
    expect(refresh).not.toHaveBeenCalled()
    isActive.mockReturnValue(true)
    await sync.check()
    expect(refresh).toHaveBeenCalledTimes(1)
  })

  it('coalesces repeated wake events and schedules the next poll after completion', async () => {
    const { sync, readVersion, refresh } = setup()
    const version = deferred<string>()
    const refreshed = deferred<boolean>()
    readVersion.mockReturnValueOnce(version.promise)
    refresh.mockReturnValueOnce(refreshed.promise)
    sync.start()
    const first = sync.check()
    expect(sync.check()).toBe(first)
    await vi.advanceTimersByTimeAsync(20000)
    expect(readVersion).toHaveBeenCalledTimes(1)
    version.resolve('1')
    await Promise.resolve()
    expect(sync.check()).toBe(first)
    await vi.advanceTimersByTimeAsync(20000)
    expect(refresh).toHaveBeenCalledTimes(1)
    refreshed.resolve(true)
    await first
    expect(vi.getTimerCount()).toBe(1)
    await vi.advanceTimersByTimeAsync(4999)
    expect(readVersion).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1)
    expect(readVersion).toHaveBeenCalledTimes(2)
  })

  it('backs off failures to 30 seconds and resets the interval after success', async () => {
    const { sync, readVersion, onError } = setup()
    const failure = new Error('network')
    readVersion.mockRejectedValue(failure)
    sync.start()
    await sync.check()
    for (const delay of [10000, 20000, 30000, 30000]) {
      const calls = readVersion.mock.calls.length
      await vi.advanceTimersByTimeAsync(delay - 1)
      expect(readVersion).toHaveBeenCalledTimes(calls)
      await vi.advanceTimersByTimeAsync(1)
      expect(readVersion).toHaveBeenCalledTimes(calls + 1)
    }
    expect(onError).toHaveBeenCalledTimes(5)
    readVersion.mockResolvedValue('1')
    await vi.advanceTimersByTimeAsync(30000)
    const calls = readVersion.mock.calls.length
    await vi.advanceTimersByTimeAsync(5000)
    expect(readVersion).toHaveBeenCalledTimes(calls + 1)
  })

  it('retries refresh failures without acknowledging their version and contains reporter failures', async () => {
    const { sync, refresh, onError } = setup()
    refresh.mockRejectedValueOnce(new Error('refresh'))
    onError.mockImplementation(() => { throw new Error('reporter') })
    sync.start()
    await expect(sync.check()).resolves.toBeUndefined()
    expect(onError).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(10000)
    expect(refresh).toHaveBeenCalledTimes(2)
  })

  it('clears timers on stop and treats repeated start calls as one session', async () => {
    const { sync, readVersion } = setup()
    sync.start()
    sync.start()
    await sync.check()
    expect(readVersion).toHaveBeenCalledTimes(1)
    sync.stop()
    expect(vi.getTimerCount()).toBe(0)
    await vi.advanceTimersByTimeAsync(60000)
    await sync.check()
    expect(readVersion).toHaveBeenCalledTimes(1)
  })

  it('refreshes a previously acknowledged version after a new login session', async () => {
    const { sync, refresh } = setup()
    sync.start()
    await sync.check()
    sync.stop()
    sync.start()
    await sync.check()
    expect(refresh).toHaveBeenCalledTimes(2)
  })

  it('refreshes an unchanged version after switching the active view', async () => {
    const { sync, readVersion, refresh } = setup()
    sync.start()
    await sync.check()
    sync.invalidate()
    await sync.check()
    expect(readVersion).toHaveBeenCalledTimes(2)
    expect(refresh).toHaveBeenCalledTimes(2)
    await sync.check()
    expect(refresh).toHaveBeenCalledTimes(2)
  })

  it('does not acknowledge a version invalidated during its request', async () => {
    const { sync, readVersion, refresh } = setup()
    const version = deferred<string>()
    readVersion.mockReturnValueOnce(version.promise)
    sync.start()
    const first = sync.check()
    await Promise.resolve()
    sync.invalidate()
    expect(sync.check()).toBe(first)
    version.resolve('1')
    await first
    expect(refresh).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(5000)
    expect(readVersion).toHaveBeenCalledTimes(2)
    expect(refresh).toHaveBeenCalledTimes(2)
  })

  it('does not acknowledge a refresh invalidated while in flight or overlap its replacement', async () => {
    const { sync, refresh } = setup()
    const refreshed = deferred<boolean>()
    refresh.mockReturnValueOnce(refreshed.promise)
    sync.start()
    const first = sync.check()
    await Promise.resolve()
    await Promise.resolve()
    expect(refresh).toHaveBeenCalledTimes(1)
    sync.invalidate()
    sync.invalidate()
    expect(sync.check()).toBe(first)
    await vi.advanceTimersByTimeAsync(10000)
    expect(refresh).toHaveBeenCalledTimes(1)
    refreshed.resolve(true)
    await first
    await vi.advanceTimersByTimeAsync(5000)
    expect(refresh).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(5000)
    expect(refresh).toHaveBeenCalledTimes(2)
  })

  it('ignores a stopped version request and preserves a new session in flight', async () => {
    const { sync, readVersion, refresh } = setup()
    const oldVersion = deferred<string>()
    const newVersion = deferred<string>()
    readVersion.mockReturnValueOnce(oldVersion.promise).mockReturnValueOnce(newVersion.promise)
    sync.start()
    const oldCheck = sync.check()
    await Promise.resolve()
    sync.stop()
    sync.start()
    const newCheck = sync.check()
    await Promise.resolve()
    oldVersion.resolve('1')
    await oldCheck
    expect(refresh).not.toHaveBeenCalled()
    expect(sync.check()).toBe(newCheck)
    expect(vi.getTimerCount()).toBe(0)
    newVersion.resolve('1')
    await newCheck
    expect(refresh).toHaveBeenCalledTimes(1)
    expect(vi.getTimerCount()).toBe(1)
  })

  it('ignores an old refresh result and forces a fresh initial refresh after restart', async () => {
    const { sync, refresh } = setup()
    const oldRefresh = deferred<boolean>()
    refresh.mockReturnValueOnce(oldRefresh.promise).mockResolvedValueOnce(false)
    sync.start()
    const oldCheck = sync.check()
    await Promise.resolve()
    await Promise.resolve()
    expect(refresh).toHaveBeenCalledTimes(1)
    sync.stop()
    sync.start()
    await sync.check()
    expect(refresh).toHaveBeenCalledTimes(2)
    oldRefresh.resolve(true)
    await oldCheck
    expect(vi.getTimerCount()).toBe(1)
    await vi.advanceTimersByTimeAsync(5000)
    expect(refresh).toHaveBeenCalledTimes(3)
  })

  it('does not report old errors or schedule old timers after restart', async () => {
    const { sync, readVersion, onError } = setup()
    const oldVersion = deferred<string>()
    readVersion.mockReturnValueOnce(oldVersion.promise)
    sync.start()
    const oldCheck = sync.check()
    await Promise.resolve()
    sync.stop()
    sync.start()
    await sync.check()
    oldVersion.reject(new Error('old request'))
    await oldCheck
    expect(onError).not.toHaveBeenCalled()
    expect(vi.getTimerCount()).toBe(1)
    await vi.advanceTimersByTimeAsync(5000)
    expect(readVersion).toHaveBeenCalledTimes(3)
  })
})
