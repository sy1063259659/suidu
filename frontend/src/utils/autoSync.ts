export interface AutoSyncOptions {
  readVersion: () => Promise<string>
  refresh: () => Promise<boolean>
  isActive: () => boolean
  onError?: (error: unknown) => void
  intervalMs?: number
  maxBackoffMs?: number
}

interface SyncSession {
  timer?: ReturnType<typeof setTimeout>
  inFlight?: Promise<void>
  version?: string
  invalidation: number
  delay: number
}

/** Poll a cheap watermark and refresh only after a change (including the first read). */
export function createAutoSync(options: AutoSyncOptions) {
  const interval = options.intervalMs ?? 5000
  const maxBackoff = Math.max(interval, options.maxBackoffMs ?? 30000)
  let session: SyncSession | undefined

  function clearTimer(current: SyncSession) {
    if (current.timer !== undefined) {
      clearTimeout(current.timer)
      current.timer = undefined
    }
  }

  async function poll(current: SyncSession) {
    const invalidation = current.invalidation
    try {
      if (session !== current || !options.isActive()) return
      const version = await options.readVersion()
      if (session !== current || !options.isActive()) return
      if (current.version !== version) {
        // A skipped refresh must not acknowledge the watermark: retry next time.
        const refreshed = await options.refresh()
        if (session !== current) return
        if (refreshed && current.invalidation === invalidation) current.version = version
      }
      current.delay = interval
    } catch (error) {
      if (session !== current) return
      current.delay = Math.min(current.delay * 2, maxBackoff)
      // Reporting errors must never turn a background check into an unhandled rejection.
      try { options.onError?.(error) } catch { /* Ignore errors in the reporter. */ }
    } finally {
      if (session === current) {
        current.inFlight = undefined
        current.timer = setTimeout(() => { void check() }, current.delay)
      }
    }
  }

  function check(): Promise<void> {
    const current = session
    if (!current) return Promise.resolve()
    if (current.inFlight) return current.inFlight
    clearTimer(current)
    // Set inFlight before calling application callbacks, including synchronous ones.
    current.inFlight = Promise.resolve().then(() => poll(current))
    return current.inFlight
  }

  function start() {
    if (session) return
    session = { delay: interval, invalidation: 0 }
    void check()
  }

  function invalidate() {
    if (!session) return
    session.version = undefined
    // A view change must also invalidate the acknowledgement of a refresh in flight.
    session.invalidation += 1
  }

  function stop() {
    if (session) clearTimer(session)
    // Identity invalidates every continuation from a previous login, even for the same user.
    session = undefined
  }

  return { start, stop, check, invalidate }
}
