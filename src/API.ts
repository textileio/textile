/**
 * @packageDocumentation
 * @module API
 */

import { Config } from './models/API'
import { ThreadsConfig } from './models/Threads'

export { Config }

/**
 * API is the primary interface to the Textile API
 */
export class API {
  private _threadsConfig: ThreadsConfig

  /**
   * New API class constructor.
   * @param config Textile API config object.
   */
  constructor(config: Config) {
    // prettier-ignore
    this._threadsConfig =
      config.dev === true
        ? new ThreadsConfig(
          config.token,
          config.deviceId,
          true,
          config.scheme !== (null || undefined) ? config.scheme : 'http',
          config.authApi !== (null || undefined) ? config.authApi : '127.0.0.1',
          config.authPort !== (null || undefined) ? config.authPort : 8006,
          config.threadApiScheme !== (null || undefined) ? config.threadApiScheme : 'http',
          config.threadsApi !== (null || undefined) ? config.threadsApi : '127.0.0.1',
          config.threadsPort !== (null || undefined) ? config.threadsPort : 6007,
        )
        : new ThreadsConfig(
          config.token,
          config.deviceId,
          false,
          config.scheme !== (null || undefined) ? config.scheme : 'https',
          config.authApi !== (null || undefined) ? config.authApi : 'cloud.textile.io',
          config.authPort !== (null || undefined) ? config.authPort : 443,
          config.threadApiScheme !== (null || undefined) ? config.threadApiScheme : 'https',
          config.threadsApi !== (null || undefined) ? config.threadsApi : 'api.textile.io',
          config.threadsPort !== (null || undefined) ? config.threadsPort : 6447,
        )
  }

  /**
   * Start must be called if no sessionId is known.
   * After Start is called, the app should store sessionId
   * for reuse using start(existingSession).
   * @param sessionId Set to reuse an existing session.
   */
  async start(sessionId?: string) {
    await this._threadsConfig.start(sessionId)
    return this
  }

  /**
   * Get the existing session ID after successfully running start.
   */
  get sessionId(): string | undefined {
    return this._threadsConfig.sessionId
  }

  /**
   * Export the threadConfig.
   */
  get threadsConfig(): ThreadsConfig {
    return this._threadsConfig
  }
}
