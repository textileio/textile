// package: hub.pb
// file: hub.proto

import * as hub_pb from "./hub_pb";
import {grpc} from "@improbable-eng/grpc-web";

type APILogin = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof hub_pb.LoginRequest;
  readonly responseType: typeof hub_pb.LoginReply;
};

type APILogout = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof hub_pb.LogoutRequest;
  readonly responseType: typeof hub_pb.LogoutReply;
};

type APIWhoami = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof hub_pb.WhoamiRequest;
  readonly responseType: typeof hub_pb.WhoamiReply;
};

type APIGetPrimaryThread = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof hub_pb.GetPrimaryThreadRequest;
  readonly responseType: typeof hub_pb.GetPrimaryThreadReply;
};

type APISetPrimaryThread = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof hub_pb.SetPrimaryThreadRequest;
  readonly responseType: typeof hub_pb.SetPrimaryThreadReply;
};

type APIListThreads = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof hub_pb.ListThreadsRequest;
  readonly responseType: typeof hub_pb.ListThreadsReply;
};

type APICreateKey = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof hub_pb.CreateKeyRequest;
  readonly responseType: typeof hub_pb.GetKeyReply;
};

type APIListKeys = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof hub_pb.ListKeysRequest;
  readonly responseType: typeof hub_pb.ListKeysReply;
};

type APIInvalidateKey = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof hub_pb.InvalidateKeyRequest;
  readonly responseType: typeof hub_pb.InvalidateKeyReply;
};

type APICreateOrg = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof hub_pb.CreateOrgRequest;
  readonly responseType: typeof hub_pb.GetOrgReply;
};

type APIGetOrg = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof hub_pb.GetOrgRequest;
  readonly responseType: typeof hub_pb.GetOrgReply;
};

type APIListOrgs = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof hub_pb.ListOrgsRequest;
  readonly responseType: typeof hub_pb.ListOrgsReply;
};

type APIRemoveOrg = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof hub_pb.RemoveOrgRequest;
  readonly responseType: typeof hub_pb.RemoveOrgReply;
};

type APIInviteToOrg = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof hub_pb.InviteToOrgRequest;
  readonly responseType: typeof hub_pb.InviteToOrgReply;
};

type APILeaveOrg = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof hub_pb.LeaveOrgRequest;
  readonly responseType: typeof hub_pb.LeaveOrgReply;
};

export class API {
  static readonly serviceName: string;
  static readonly Login: APILogin;
  static readonly Logout: APILogout;
  static readonly Whoami: APIWhoami;
  static readonly GetPrimaryThread: APIGetPrimaryThread;
  static readonly SetPrimaryThread: APISetPrimaryThread;
  static readonly ListThreads: APIListThreads;
  static readonly CreateKey: APICreateKey;
  static readonly ListKeys: APIListKeys;
  static readonly InvalidateKey: APIInvalidateKey;
  static readonly CreateOrg: APICreateOrg;
  static readonly GetOrg: APIGetOrg;
  static readonly ListOrgs: APIListOrgs;
  static readonly RemoveOrg: APIRemoveOrg;
  static readonly InviteToOrg: APIInviteToOrg;
  static readonly LeaveOrg: APILeaveOrg;
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
  login(
    requestMessage: hub_pb.LoginRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: hub_pb.LoginReply|null) => void
  ): UnaryResponse;
  login(
    requestMessage: hub_pb.LoginRequest,
    callback: (error: ServiceError|null, responseMessage: hub_pb.LoginReply|null) => void
  ): UnaryResponse;
  logout(
    requestMessage: hub_pb.LogoutRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: hub_pb.LogoutReply|null) => void
  ): UnaryResponse;
  logout(
    requestMessage: hub_pb.LogoutRequest,
    callback: (error: ServiceError|null, responseMessage: hub_pb.LogoutReply|null) => void
  ): UnaryResponse;
  whoami(
    requestMessage: hub_pb.WhoamiRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: hub_pb.WhoamiReply|null) => void
  ): UnaryResponse;
  whoami(
    requestMessage: hub_pb.WhoamiRequest,
    callback: (error: ServiceError|null, responseMessage: hub_pb.WhoamiReply|null) => void
  ): UnaryResponse;
  getPrimaryThread(
    requestMessage: hub_pb.GetPrimaryThreadRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: hub_pb.GetPrimaryThreadReply|null) => void
  ): UnaryResponse;
  getPrimaryThread(
    requestMessage: hub_pb.GetPrimaryThreadRequest,
    callback: (error: ServiceError|null, responseMessage: hub_pb.GetPrimaryThreadReply|null) => void
  ): UnaryResponse;
  setPrimaryThread(
    requestMessage: hub_pb.SetPrimaryThreadRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: hub_pb.SetPrimaryThreadReply|null) => void
  ): UnaryResponse;
  setPrimaryThread(
    requestMessage: hub_pb.SetPrimaryThreadRequest,
    callback: (error: ServiceError|null, responseMessage: hub_pb.SetPrimaryThreadReply|null) => void
  ): UnaryResponse;
  listThreads(
    requestMessage: hub_pb.ListThreadsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: hub_pb.ListThreadsReply|null) => void
  ): UnaryResponse;
  listThreads(
    requestMessage: hub_pb.ListThreadsRequest,
    callback: (error: ServiceError|null, responseMessage: hub_pb.ListThreadsReply|null) => void
  ): UnaryResponse;
  createKey(
    requestMessage: hub_pb.CreateKeyRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: hub_pb.GetKeyReply|null) => void
  ): UnaryResponse;
  createKey(
    requestMessage: hub_pb.CreateKeyRequest,
    callback: (error: ServiceError|null, responseMessage: hub_pb.GetKeyReply|null) => void
  ): UnaryResponse;
  listKeys(
    requestMessage: hub_pb.ListKeysRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: hub_pb.ListKeysReply|null) => void
  ): UnaryResponse;
  listKeys(
    requestMessage: hub_pb.ListKeysRequest,
    callback: (error: ServiceError|null, responseMessage: hub_pb.ListKeysReply|null) => void
  ): UnaryResponse;
  invalidateKey(
    requestMessage: hub_pb.InvalidateKeyRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: hub_pb.InvalidateKeyReply|null) => void
  ): UnaryResponse;
  invalidateKey(
    requestMessage: hub_pb.InvalidateKeyRequest,
    callback: (error: ServiceError|null, responseMessage: hub_pb.InvalidateKeyReply|null) => void
  ): UnaryResponse;
  createOrg(
    requestMessage: hub_pb.CreateOrgRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: hub_pb.GetOrgReply|null) => void
  ): UnaryResponse;
  createOrg(
    requestMessage: hub_pb.CreateOrgRequest,
    callback: (error: ServiceError|null, responseMessage: hub_pb.GetOrgReply|null) => void
  ): UnaryResponse;
  getOrg(
    requestMessage: hub_pb.GetOrgRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: hub_pb.GetOrgReply|null) => void
  ): UnaryResponse;
  getOrg(
    requestMessage: hub_pb.GetOrgRequest,
    callback: (error: ServiceError|null, responseMessage: hub_pb.GetOrgReply|null) => void
  ): UnaryResponse;
  listOrgs(
    requestMessage: hub_pb.ListOrgsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: hub_pb.ListOrgsReply|null) => void
  ): UnaryResponse;
  listOrgs(
    requestMessage: hub_pb.ListOrgsRequest,
    callback: (error: ServiceError|null, responseMessage: hub_pb.ListOrgsReply|null) => void
  ): UnaryResponse;
  removeOrg(
    requestMessage: hub_pb.RemoveOrgRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: hub_pb.RemoveOrgReply|null) => void
  ): UnaryResponse;
  removeOrg(
    requestMessage: hub_pb.RemoveOrgRequest,
    callback: (error: ServiceError|null, responseMessage: hub_pb.RemoveOrgReply|null) => void
  ): UnaryResponse;
  inviteToOrg(
    requestMessage: hub_pb.InviteToOrgRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: hub_pb.InviteToOrgReply|null) => void
  ): UnaryResponse;
  inviteToOrg(
    requestMessage: hub_pb.InviteToOrgRequest,
    callback: (error: ServiceError|null, responseMessage: hub_pb.InviteToOrgReply|null) => void
  ): UnaryResponse;
  leaveOrg(
    requestMessage: hub_pb.LeaveOrgRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: hub_pb.LeaveOrgReply|null) => void
  ): UnaryResponse;
  leaveOrg(
    requestMessage: hub_pb.LeaveOrgRequest,
    callback: (error: ServiceError|null, responseMessage: hub_pb.LeaveOrgReply|null) => void
  ): UnaryResponse;
}

