import { grpc } from '@improbable-eng/grpc-web'
import { ThreadID } from '@textile/threads-id'
import { Config } from '@textile/threads-client'

type HostString =
  | 'https://api.textile.io:3447'
  | 'https://api.staging.textile.io:3447'
  | 'http://127.0.0.1:3007'
  | string
export const defaultHost: HostString = 'https://api.textile.io:3447'

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

  /**
   * API key. Used for user authentication.
   */
  ['x-textile-api-key']?: string

  /**
   * Authorization token for interacting with remote APIs,
   */
  authorization?: string

  // @todo: Add docs here
  ['x-textile-api-sig']?: string
  ['x-textile-api-sig-msg']?: string

  /**
   * The service host address/url. Defaults to https://hub.textile.io.
   */
  host?: HostString
  /**
   * The transport to use for gRPC calls. Defaults to web-sockets.
   */
  transport?: grpc.TransportFactory

  /**
   * Whether to enable debugging output during gRPC calls.
   */
  debug?: boolean
}

/**
 * Context provides immutable context management for gRPC credentials and config settings.
 */
export class Context implements Config {
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
    this._context['host'] = host
    this._context['transport'] = transport
    this._context['debug'] = debug
  }

  get host() {
    return this._context['host']
  }

  get transport() {
    return this._context['transport']
  }

  get debug() {
    return this._context['debug']
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

  withToken(value?: string) {
    if (value === undefined) return this
    return Context.fromJSON({ ...this._context, ['authorization']: `bearer ${value}` })
  }

  withAPIKey(value?: string) {
    if (value === undefined) return this
    return Context.fromJSON({ ...this._context, ['x-textile-api-key']: value })
  }

  withAPISig(value?: { sig: string; msg: string }) {
    if (value === undefined) return this
    const { sig, msg } = value
    return Context.fromJSON({ ...this._context, ['x-textile-api-sig-msg']: msg, ['x-textile-api-sig']: sig })
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
