// package: users.pb
// file: users.proto

import * as users_pb from "./users_pb";
import {grpc} from "@improbable-eng/grpc-web";

type APIGetThread = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof users_pb.GetThreadRequest;
  readonly responseType: typeof users_pb.GetThreadReply;
};

type APIListThreads = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof users_pb.ListThreadsRequest;
  readonly responseType: typeof users_pb.ListThreadsReply;
};

export class API {
  static readonly serviceName: string;
  static readonly GetThread: APIGetThread;
  static readonly ListThreads: APIListThreads;
}

export type ServiceError = { message: string, code: number; metadata: grpc.Metadata }
export type Status = { details: string, code: number; metadata: grpc.Metadata }

interface UnaryResponse {
  cancel(): void;
}
interface ResponseStream<T> {
  cancel(): void;
  on(type: 'data', handler: (message: T) => void): ResponseStream<T>;
  on(type: 'end', handler: (status?: Status) => void): ResponseStream<T>;
  on(type: 'status', handler: (status: Status) => void): ResponseStream<T>;
}
interface RequestStream<T> {
  write(message: T): RequestStream<T>;
  end(): void;
  cancel(): void;
  on(type: 'end', handler: (status?: Status) => void): RequestStream<T>;
  on(type: 'status', handler: (status: Status) => void): RequestStream<T>;
}
interface BidirectionalStream<ReqT, ResT> {
  write(message: ReqT): BidirectionalStream<ReqT, ResT>;
  end(): void;
  cancel(): void;
  on(type: 'data', handler: (message: ResT) => void): BidirectionalStream<ReqT, ResT>;
  on(type: 'end', handler: (status?: Status) => void): BidirectionalStream<ReqT, ResT>;
  on(type: 'status', handler: (status: Status) => void): BidirectionalStream<ReqT, ResT>;
}

export class APIClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  getThread(
    requestMessage: users_pb.GetThreadRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: users_pb.GetThreadReply|null) => void
  ): UnaryResponse;
  getThread(
    requestMessage: users_pb.GetThreadRequest,
    callback: (error: ServiceError|null, responseMessage: users_pb.GetThreadReply|null) => void
  ): UnaryResponse;
  listThreads(
    requestMessage: users_pb.ListThreadsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: users_pb.ListThreadsReply|null) => void
  ): UnaryResponse;
  listThreads(
    requestMessage: users_pb.ListThreadsRequest,
    callback: (error: ServiceError|null, responseMessage: users_pb.ListThreadsReply|null) => void
  ): UnaryResponse;
}

