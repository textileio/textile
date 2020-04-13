// package: buckets.pb
// file: buckets.proto

import * as buckets_pb from "./buckets_pb";
import {grpc} from "@improbable-eng/grpc-web";

type APIListPath = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof buckets_pb.ListPathRequest;
  readonly responseType: typeof buckets_pb.ListPathReply;
};

type APIPushPath = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: true;
  readonly responseStream: true;
  readonly requestType: typeof buckets_pb.PushPathRequest;
  readonly responseType: typeof buckets_pb.PushPathReply;
};

type APIPullPath = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: true;
  readonly requestType: typeof buckets_pb.PullPathRequest;
  readonly responseType: typeof buckets_pb.PullPathReply;
};

type APIRemovePath = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof buckets_pb.RemovePathRequest;
  readonly responseType: typeof buckets_pb.RemovePathReply;
};

export class API {
  static readonly serviceName: string;
  static readonly ListPath: APIListPath;
  static readonly PushPath: APIPushPath;
  static readonly PullPath: APIPullPath;
  static readonly RemovePath: APIRemovePath;
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
  listPath(
    requestMessage: buckets_pb.ListPathRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: buckets_pb.ListPathReply|null) => void
  ): UnaryResponse;
  listPath(
    requestMessage: buckets_pb.ListPathRequest,
    callback: (error: ServiceError|null, responseMessage: buckets_pb.ListPathReply|null) => void
  ): UnaryResponse;
  pushPath(metadata?: grpc.Metadata): BidirectionalStream<buckets_pb.PushPathRequest, buckets_pb.PushPathReply>;
  pullPath(requestMessage: buckets_pb.PullPathRequest, metadata?: grpc.Metadata): ResponseStream<buckets_pb.PullPathReply>;
  removePath(
    requestMessage: buckets_pb.RemovePathRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: buckets_pb.RemovePathReply|null) => void
  ): UnaryResponse;
  removePath(
    requestMessage: buckets_pb.RemovePathRequest,
    callback: (error: ServiceError|null, responseMessage: buckets_pb.RemovePathReply|null) => void
  ): UnaryResponse;
}

