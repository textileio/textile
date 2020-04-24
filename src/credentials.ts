import { grpc } from '@improbable-eng/grpc-web'
import { ThreadID } from '@textile/threads-id'
import { Config } from '@textile/threads-client'

type HostString = 'https://hub.textile.io:443' | 'https://hub.staging.textile.io:443' | 'http://127.0.0.1:3007' | string
const defaultHost: HostString = 'https://hub.textile.io:443'

export interface CredentialContext {
  /**
   * Thread name. Specifies a mapping between human-readable name and a Thread ID.
   */
  ['x-textile-thread-name']?: string
  /**
   * Thread ID as a string. Should be generated with `ThreadID.toString()` method.
   */
  ['x-textile-thread']?: string
  /**
   * Session key. Used for various session contexts.
   */
  ['x-textile-session']?: string

  /**
   * Org slug/name. Used for various org session operations.
   */
  ['x-textile-org']?: string
}

export class Credentials implements Config {
  public host: HostString
  public transport: grpc.TransportFactory
  private _context: Partial<Record<keyof CredentialContext, any>> = {}
  constructor(
    // To comply with Config interface
    host: HostString = defaultHost,
    // To comply with Config interface
    transport: grpc.TransportFactory = grpc.WebsocketTransport(),
  ) {
    this.host = host
    this.transport = transport
    // Setup defaults
    return this.withHost(host).withTransport(transport)
  }

  withHost(value: HostString) {
    this.host = value
    return this
  }

  withTransport(value: grpc.TransportFactory) {
    this.transport = value
    return this
  }

  withSession(value: string) {
    this._context['x-textile-session'] = value
    return this
  }

  withThread(value: ThreadID) {
    this._context['x-textile-thread'] = value.toString()
    return this
  }

  withThreadName(value: string) {
    this._context['x-textile-thread-name'] = value
    return this
  }

  withOrg(value: string) {
    this._context['x-textile-org'] = value
    return this
  }

  toJSON() {
    return this._context
  }

  // @todo: Drop these in favor of toJSON()?
  _wrapMetadata(values?: Record<string, any>) {
    return { ...values, ...this.toJSON() }
  }

  // @todo: Drop this in favor of toJSON()?
  _wrapBrowserHeaders(values: grpc.Metadata): grpc.Metadata {
    return values
  }
}
