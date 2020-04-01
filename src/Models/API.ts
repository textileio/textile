/**
 * @packageDocumentation
 * @module API
 */

/**
 * Textile API config parameters.
 */
export interface Config {
  /** Your Textile API token. */
  token: string
  /** A unique ID for this Thread user. */
  deviceId: string
  /** Run your app against a local Threads daemon. */
  dev?: boolean
  /** @internal */
  scheme?: string
  /** @internal */
  authApi?: string
  /** @internal */
  authPort?: number
  /** @internal */
  threadApiScheme?: string
  /** @internal */
  threadsApi?: string
  /** @internal */
  threadsPort?: number
}
