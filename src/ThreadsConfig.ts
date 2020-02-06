import { grpc } from '@improbable-eng/grpc-web'
import { Config } from '@textile/threads-client'
import axios, { AxiosRequestConfig } from 'axios'

/**
 * WriteTransaction performs a mutating bulk transaction on the underlying store.
 */

type Session = {
  id: string
  session_id: string
}
export class ThreadsConfig extends Config {
  public host: string
  constructor(
    public token: string,
    public deviceId: string,
    public dev: boolean,
    public apiScheme: string = 'https',
    public api: string = 'cloud.textile.io',
    public sessionPort: number = 8006,
    public threadsPort: number = 6007,
    public transport: grpc.TransportFactory = grpc.WebsocketTransport(),
  ) {
    super()
    this.host = `${apiScheme}://${api}:${threadsPort}`
  }
  async start() {
    await this.refreshSession()
  }
  get sessionAPI(): string {
    return `${this.apiScheme}://${this.api}:${this.sessionPort}`
  }
  private async refreshSession() {
    if (this.dev) {
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
    this.session = resp.data.session_id
  }
  public _wrapMetadata(values?: { [key: string]: any }): { [key: string]: any } | undefined {
    return super._wrapMetadata(values)
  }
  public _wrapBrowserHeaders(values: grpc.Metadata): grpc.Metadata {
    return super._wrapBrowserHeaders(values)
  }
}
