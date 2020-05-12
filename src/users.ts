import log from 'loglevel'
import * as pb from '@textile/users-grpc/users_pb'
import { GetKeyReply } from '@textile/hub-grpc/hub_pb'
import { APIClient } from '@textile/users-grpc/users_pb_service'
import { ServiceError } from '@textile/hub-grpc/hub_pb_service'
import { Context } from '@textile/context'
import { ThreadID } from '@textile/threads-id'
import { Client } from '@textile/threads-client'
import { Identity, Libp2pCryptoIdentity } from '@textile/threads-core'

const logger = log.getLogger('users')

/**
 * Buckets is a web-gRPC wrapper client for communicating with the web-gRPC enabled Textile Buckets API.
 */
export class Users {
  private client: APIClient

  /**
   * Creates a new gRPC client instance for accessing the Textile Buckets API.
   * @param context The context to use for interacting with the APIs. Can be modified later.
   */
  constructor(public context: Context = new Context()) {
    this.client = new APIClient(context.host, {
      transport: context.transport,
      debug: context.debug,
    })
  }

  static async fromKey(key: GetKeyReply.AsObject, identity?: Identity, ctx?: Context) {
    const users = new Users(ctx)
    await users.context.withAPIKey(key.key).withUserKey(key)
    const id = identity ?? (await Libp2pCryptoIdentity.fromRandom())
    const client = new Client(users.context)
    await client.getToken(id)
    return users
  }

  /**
   * Returns a Thread by name.
   * @param name The name of the Thread.
   * @param ctx Context containing gRPC headers and settings.
   * These will be merged with any internal credentials.
   */
  async getThread(name: string, ctx?: Context) {
    logger.debug('get thread request')
    return new Promise<pb.GetThreadReply.AsObject>((resolve, reject) => {
      const req = new pb.GetThreadRequest()
      req.setName(name)
      this.client.getThread(
        req,
        this.context.withContext(ctx).toMetadata(),
        (err: ServiceError | null, message: pb.GetThreadReply | null) => {
          if (err) reject(err)
          const msg = message?.toObject()
          if (msg) {
            msg.id = ThreadID.fromBytes(Buffer.from(msg.id as string, 'base64')).toString()
          }
          resolve(msg)
        },
      )
    })
  }

  /**
   * Create a new Thread with name.
   * @param name The name of the Thread.
   * @param ctx Context containing gRPC headers and settings.
   * These will be merged with any internal credentials.
   */
  async createThread(name: string, ctx?: Context) {
    logger.debug('create thread request')
    const id = ThreadID.fromRandom()
    const db = new Client(this.context.withContext(ctx).withThreadName(name))
    return db.newDB(id, this.context.withThread(id))
  }

  /**
   * Returns a list of available Threads.
   * @param ctx Context containing gRPC headers and settings.
   * These will be merged with any internal credentials.
   * @note Threads can be created using the threads or threads network clients.
   */
  async listThreads(ctx?: Context) {
    logger.debug('list threads request')
    return new Promise<pb.ListThreadsReply.AsObject>((resolve, reject) => {
      const req = new pb.ListThreadsRequest()
      this.client.listThreads(
        req,
        this.context.withContext(ctx).toMetadata(),
        (err: ServiceError | null, message: pb.ListThreadsReply | null) => {
          if (err) reject(err)
          const msg = message?.toObject()
          if (msg) {
            msg.listList.forEach((thread) => {
              thread.id = ThreadID.fromBytes(Buffer.from(thread.id as string, 'base64')).toString()
            })
          }
          resolve(msg)
        },
      )
    })
  }
}
