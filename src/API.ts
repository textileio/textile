/* eslint-disable @typescript-eslint/no-non-null-assertion */
import { ThreadsConfig, APIConfig } from './Models'

class API {
  /**
   * threadsConfig is the (private) threads config.
   */
  private _threadsConfig: ThreadsConfig

  constructor(config: APIConfig) {
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

  async start(sessionId?: string) {
    await this._threadsConfig.start(sessionId)
    return this
  }

  get sessionId(): string | undefined {
    return this._threadsConfig.sessionId
  }

  get threadsConfig(): ThreadsConfig {
    return this._threadsConfig
  }
}

// eslint-disable-next-line import/no-default-export
export default API
