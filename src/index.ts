/* eslint-disable @typescript-eslint/no-non-null-assertion */
import * as pack from '../package.json'
import { ThreadsConfig } from './ThreadsConfig'

export { ThreadsConfig }

export type APIConfig = {
  token: string
  deviceId: string
  dev?: boolean
  apiScheme?: string
  api?: string
  sessionPort?: number
  threadsPort?: number
}
export class API {
  /**
   * version is the release version.
   */
  public static version(): string {
    return pack.version
  }

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
          config.apiScheme !== (null || undefined) ? config.apiScheme : 'http',
          config.api !== (null || undefined) ? config.api : '127.0.0.1',
          config.sessionPort !== (null || undefined) ? config.sessionPort : 8006,
          config.threadsPort !== (null || undefined) ? config.threadsPort : 6007,
        )
        : new ThreadsConfig(
          config.token,
          config.deviceId,
          false,
          config.apiScheme !== (null || undefined) ? config.apiScheme : 'https',
          config.api !== (null || undefined) ? config.api : 'cloud.textile.io',
          config.sessionPort !== (null || undefined) ? config.sessionPort : 8006,
          config.threadsPort !== (null || undefined) ? config.threadsPort : 6007,
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
