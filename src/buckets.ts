import log from 'loglevel'
import * as pb from '@textile/buckets-grpc/buckets_pb'
import { API, APIPushPath } from '@textile/buckets-grpc/buckets_pb_service'
import CID from 'cids'
import { Channel } from 'queueable'
import { Context } from './context'
import { grpc } from '@improbable-eng/grpc-web'
import { normaliseInput, File } from './normalize'
import { createUsername } from './utils'

const logger = log.getLogger('buckets')

/**
 * Buckets is a web-gRPC wrapper client for communicating with the web-gRPC enabled Textile Buckets API.
 */
export class Buckets {
  public serviceHost: string
  public rpcOptions: grpc.RpcOptions
  /**
   * Creates a new gRPC client instance for accessing the Textile Buckets API.
   * @param context The context to use for interacting with the APIs. Can be modified later.
   */
  constructor(public context: Context = new Context()) {
    this.serviceHost = context.host
    this.rpcOptions = {
      transport: context.transport,
      debug: context.debug,
    }
  }

  /**
   * Initializes a new bucket.
   * @param name Human-readable bucket name. It is only meant to help identify a bucket in a UI and is not unique.
   * @param ctx Context object containing web-gRPC headers and settings.
   */
  async init(name: string, ctx?: Context) {
    logger.debug('init request')
    const req = new pb.InitRequest()
    req.setName(name)
    const res: pb.InitReply = await this.unary(API.Init, req, ctx)
    return res.toObject()
  }

  /**
   * Returns a list of all bucket roots.
   * @param ctx Context object containing web-gRPC headers and settings.
   */
  async list(ctx?: Context) {
    logger.debug('list request')
    const req = new pb.ListRequest()
    const res: pb.ListReply = await this.unary(API.List, req, ctx)
    return res.toObject().rootsList
  }

  /**
   * Returns information about a bucket path.
   * @param key Unique (IPNS compatible) identifier key for a bucket.
   * @param path A file/object (sub)-path within a bucket.
   * @param ctx Context object containing web-gRPC headers and settings.
   */
  async listPath(key: string, path: string, ctx?: Context) {
    logger.debug('list path request')
    const req = new pb.ListPathRequest()
    req.setKey(key)
    req.setPath(path)
    const res: pb.ListPathReply = await this.unary(API.ListPath, req, ctx)
    return res.toObject()
  }

  /**
   * Removes an entire bucket. Files and directories will be unpinned.
   * @param key Unique (IPNS compatible) identifier key for a bucket.
   * @param ctx Context object containing web-gRPC headers and settings.
   */
  async remove(key: string, ctx?: Context) {
    logger.debug('remove request')
    const req = new pb.RemoveRequest()
    req.setKey(key)
    await this.unary(API.Remove, req, ctx)
    return
  }

  /**
   * Returns information about a bucket path.
   * @param key Unique (IPNS compatible) identifier key for a bucket.
   * @param path A file/object (sub)-path within a bucket.
   * @param ctx Context object containing web-gRPC headers and settings.
   */
  async removePath(key: string, path: string, ctx?: Context) {
    logger.debug('remove path request')
    const req = new pb.RemovePathRequest()
    req.setKey(key)
    req.setPath(path)
    const res: pb.RemovePathReply = await this.unary(API.RemovePath, req, ctx)
    return res.toObject()
  }

  async pushPath(key: string, input: any, opts?: { progress?: (num?: number) => void }, ctx?: Context) {
    const source: File | undefined = (await normaliseInput(input).next()).value
    const client = grpc.client<pb.PushPathRequest, pb.PushPathReply, APIPushPath>(API.PushPath, {
      host: this.serviceHost,
      transport: this.rpcOptions.transport,
      debug: true, // @todo: hardcoded for testing
    })
    client.onMessage((message) => {
      if (message.hasError()) {
        throw new Error(message.getError())
      } else if (message.hasEvent()) {
        const event = message.getEvent()?.toObject()
        if (event?.path) {
          const cid = new CID(event.path)
          const res = {
            path: {
              path: `/ipfs/${cid.toString()}`,
              cid: cid,
              root: cid,
              remainder: '',
            },
          }
          console.log(res) // @todo: once this is working properly, we'll just return this
        } else if (opts?.progress) {
          opts.progress(event?.bytes)
        }
      } else {
        throw new Error('Invalid reply')
      }
    })
    client.onEnd((code) => {
      if (code === grpc.Code.OK) {
        console.log('done')
      } else {
        throw new Error(code.toString())
      }
    })
    // @todo: hot needed
    // client.onHeaders((headers) => {
    //   console.log('headers', headers)
    // })
    if (source) {
      const head = new pb.PushPathRequest.Header()
      head.setPath('dir1/test.jpg') // @todo: hardcoded for testing
      head.setKey(key)
      const req = new pb.PushPathRequest()
      req.setHeader(head)
      client.start(this.context.withContext(ctx).toJSON())
      client.send(req)

      if (source.content) {
        for await (const chunk of source.content) {
          const part = new pb.PushPathRequest()
          part.setChunk(chunk as Buffer)
          client.send(part)
        }
        // All done, let's tell the server we're done
        client.finishSend()
      }
    }
  }

  private unary<
    R extends grpc.ProtobufMessage,
    T extends grpc.ProtobufMessage,
    M extends grpc.UnaryMethodDefinition<R, T>
  >(methodDescriptor: M, req: R, context?: Context): Promise<T> {
    // @todo: This is not totally ideal, but is cleaner for returning promises.
    // Ideally, we'd use the generated client directly, and wrap that with a promise.
    return new Promise<T>((resolve, reject) => {
      const creds = this.context.withContext(context)
      grpc.unary(methodDescriptor, {
        request: req,
        host: this.serviceHost,
        transport: this.rpcOptions.transport,
        debug: this.rpcOptions.debug,
        metadata: creds.toJSON(),
        onEnd: (res: grpc.UnaryOutput<T>) => {
          const { status, statusMessage, message } = res
          if (status === grpc.Code.OK) {
            if (message) {
              resolve(message)
            } else {
              resolve()
            }
          } else {
            reject(new Error(statusMessage))
          }
        },
      })
    })
  }
}
