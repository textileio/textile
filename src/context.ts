import { grpc } from '@improbable-eng/grpc-web'
import { ThreadID } from '@textile/threads-id'
import { Config } from '@textile/threads-client'

type HostString = 'https://hub.textile.io:443' | 'https://hub.staging.textile.io:443' | 'http://127.0.0.1:3007' | string
export const defaultHost: HostString = 'https://hub.textile.io:443'

export interface ContextKeys {
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

/**
 * Context provides immutable context management for gRPC credentials and config settings.
 */
export class Context implements Config {
  /**
   * The service host address/url. Defaults to https://hub.textile.io.
   */
  public host: HostString
  /**
   * The transport to use for gRPC calls. Defaults to web-sockets.
   */
  public transport: grpc.TransportFactory
  /**
   * Whether to enable debugging output during gRPC calls.
   */
  debug?: boolean

  // Internal context variables
  private _context: Partial<Record<keyof ContextKeys, any>> = {}

  constructor(
    // To comply with Config interface
    host: HostString = defaultHost,
    // To comply with Config interface
    transport: grpc.TransportFactory = grpc.WebsocketTransport(),
    // For testing and debugging purposes.
    debug = false,
  ) {
    this.host = host
    this.transport = transport
    this.debug = debug
  }

  withSession(value?: string) {
    if (value === undefined) return this
    return Context.fromJSON({ ...this._context, ['x-textile-session']: value })
  }

  withThread(value?: ThreadID) {
    if (value === undefined) return this
    return Context.fromJSON({ ...this._context, ['x-textile-thread']: value.toString() })
  }

  withThreadName(value?: string) {
    if (value === undefined) return this
    return Context.fromJSON({ ...this._context, ['x-textile-thread-name']: value })
  }

  withOrg(value?: string) {
    if (value === undefined) return this
    return Context.fromJSON({ ...this._context, ['x-textile-org']: value })
  }

  withContext(value?: Context) {
    if (value === undefined) return this
    return Context.fromJSON({ ...this._context, ...value._context })
  }

  toJSON() {
    return this._context
  }

  toMetadata() {
    return new grpc.Metadata(this.toJSON())
  }

  static fromJSON(json: ContextKeys) {
    const ctx = new Context()
    ctx._context = json
    return ctx
  }

  // @todo: Drop these in favor of toJSON() once other clients make the switch
  _wrapMetadata(values?: Record<string, any>) {
    return { ...values, ...this.toJSON() }
  }

  // @todo: Drop these in favor of toJSON() once other clients make the switch
  _wrapBrowserHeaders(values: grpc.Metadata): grpc.Metadata {
    return values
  }
}
