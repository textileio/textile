/* eslint-disable @typescript-eslint/no-non-null-assertion */
import * as uuid from 'uuid'
import { Client, Config as ClientConfig } from '@textile/threads-client'
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

  /**
   * threadsClient is the (private) threads client.
   */
  private _threadsClient?: Client

  constructor(config: APIConfig) {
    // prettier-ignore
    this._threadsConfig =
      config.dev == false
        ? new ThreadsConfig(
          config.token,
          config.deviceId,
          !!config.dev,
          config.apiScheme !== null ? config.apiScheme : 'https',
          config.api !== null ? config.api : 'cloud.textile.io',
          config.sessionPort !== null ? config.sessionPort : 8006,
          config.threadsPort !== null ? config.threadsPort : 6007,
        )
        : new ThreadsConfig(
          config.token,
          config.deviceId,
          !!config.dev,
          config.apiScheme !== null ? config.apiScheme : 'http',
          config.api !== null ? config.api : '127.0.0.1',
          config.sessionPort !== null ? config.sessionPort : 8006,
          config.threadsPort !== null ? config.threadsPort : 6007,
        )
  }

  async start() {
    this._threadsConfig.start()
  }

  get threadsClient(): Client {
    if (!this._threadsClient) {
      this._threadsConfig.start()
      this._threadsClient = new Client(this._threadsConfig)
    }
    return this._threadsClient
  }
  get threadsConfig(): ThreadsConfig {
    return this._threadsConfig
  }
}

// eslint-disable-next-line import/no-default-export
export default API
