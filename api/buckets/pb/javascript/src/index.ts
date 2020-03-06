/* eslint-disable @typescript-eslint/no-non-null-assertion */
import { grpc } from '@improbable-eng/grpc-web'
import { API } from './buckets_pb_service'
import {
  ListBucketPathRequest,
  ListBucketPathReply,
} from './buckets_pb'

export type TextileConfig = {
  host?: string;
  token?: string;
}
/**
 * Client is a web-gRPC wrapper client for communicating with a webgRPC-enabled Textile server.
 * This client library can be used to interact with a local or remote Textile gRPC-service
 *  It is a wrapper around Textile's 'Store' API, which is defined here: https://github.com/textileio/go-threads/blob/master/api/pb/api.proto.
 */
export class Buckets {
  host: string;
  token?: string;
  
  constructor(token?: string, host?: string) {
    this.token = token ? token : undefined;
    this.host = host ? host : 'http://127.0.0.1:3007';
  }

  public async listBuckets() {
    const request = new ListBucketPathRequest();
    return this.unary(API.ListBucketPath, request) as Promise<ListBucketPathReply.AsObject>
  }

  private async unary<
    TRequest extends grpc.ProtobufMessage,
    TResponse extends grpc.ProtobufMessage,
    M extends grpc.UnaryMethodDefinition<TRequest, TResponse>
  >(methodDescriptor: M, req: TRequest, scope?: string) {
    return new Promise((resolve, reject) => {
      const meta = new grpc.Metadata()
      if (this.token) {
        meta.append('authorization', `bearer ${this.token}`)
        if (scope) {
          meta.append('x-scope', `${scope}`)
        }
      }
      grpc.unary(methodDescriptor, {
        request: req,
        host: this.host,
        metadata: meta,
        onEnd: res => {
          const { status, statusMessage, message } = res
          if (status === grpc.Code.OK) {
            if (message) {
              resolve(message.toObject())
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

// eslint-disable-next-line import/no-default-export
export default Cloud
