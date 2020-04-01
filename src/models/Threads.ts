/**
 * @packageDocumentation
 * @module @textile/textile
 */

import { grpc } from '@improbable-eng/grpc-web'
import { Config } from '@textile/threads-client'
import axios, { AxiosRequestConfig } from 'axios'

type Session = {
  id: string
  session_id: string
}

/**
 * @internal
 * ThreadsConfig is automatically setup by the API
 * and can be used to configure a Thread Client.
 * Returned from API.threadsConfig()
 */
class ThreadsConfig extends Config {
  public host: string
  public sessionId?: string
  constructor(
    public token: string,
    public deviceId: string,
    public dev: boolean,
    public scheme: string = 'https',
    public authApi: string = 'cloud.textile.io',
    public authPort: number = 443,
    public threadApiScheme: string = 'https',
    public threadsApi: string = 'api.textile.io',
    public threadsPort: number = 6447,
    public transport: grpc.TransportFactory = grpc.WebsocketTransport(),
  ) {
    super()
    this.host = `${this.threadApiScheme}://${this.threadsApi}:${this.threadsPort}`
  }
  async start(sessionId?: string) {
    if (sessionId !== undefined) {
      this.sessionId = sessionId
      return
    }
    return await this.refreshSession()
  }
  get sessionAPI(): string {
    return `${this.scheme}://${this.authApi}:${this.authPort}`
  }
  private async refreshSession() {
    if (this.dev === true) {
      return
    }
    const setup: AxiosRequestConfig = {
      baseURL: this.sessionAPI,
      responseType: 'json',
    }
    const apiClient = axios.create(setup)
    const resp = await apiClient.post<Session>(
      '/register',
      JSON.stringify({
        token: this.token,
        device_id: this.deviceId, // eslint-disable-line
      }),
    )
    if (resp.status !== 200) {
      new Error(resp.statusText)
    }
    this.sessionId = resp.data.session_id
  }
  _wrapMetadata(values?: { [key: string]: any }): { [key: string]: any } | undefined {
    if (!this.sessionId) {
      return values
    }
    const response = values ?? {}
    if ('Authorization' in response || 'authorization' in response) {
      return response
    }
    response['Authorization'] = `Bearer ${this.sessionId}`
    return response
  }
  _wrapBrowserHeaders(values: grpc.Metadata): grpc.Metadata {
    if (!this.sessionId) {
      return values
    }
    values.set('Authorization', `Bearer ${this.sessionId}`)
    return values
  }
}

export { ThreadsConfig }
