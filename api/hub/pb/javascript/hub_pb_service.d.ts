// package: hub.pb
// file: hub.proto

import * as hub_pb from "./hub_pb";
import {grpc} from "@improbable-eng/grpc-web";

type APISignup = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof hub_pb.SignupRequest;
  readonly responseType: typeof hub_pb.SignupReply;
};

type APISignin = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof hub_pb.SigninRequest;
  readonly responseType: typeof hub_pb.SigninReply;
};

type APISignout = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof hub_pb.SignoutRequest;
  readonly responseType: typeof hub_pb.SignoutReply;
};

type APIGetSessionInfo = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof hub_pb.GetSessionInfoRequest;
  readonly responseType: typeof hub_pb.GetSessionInfoReply;
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

type APIIsUsernameAvailable = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof hub_pb.IsUsernameAvailableRequest;
  readonly responseType: typeof hub_pb.IsUsernameAvailableReply;
};

type APIIsOrgNameAvailable = {
  readonly methodName: string;
  readonly service: typeof API;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof hub_pb.IsOrgNameAvailableRequest;
  readonly responseType: typeof hub_pb.IsOrgNameAvailableReply;
};

export class API {
  static readonly serviceName: string;
  static readonly Signup: APISignup;
  static readonly Signin: APISignin;
  static readonly Signout: APISignout;
  static readonly GetSessionInfo: APIGetSessionInfo;
  static readonly CreateKey: APICreateKey;
  static readonly ListKeys: APIListKeys;
  static readonly InvalidateKey: APIInvalidateKey;
  static readonly CreateOrg: APICreateOrg;
  static readonly GetOrg: APIGetOrg;
  static readonly ListOrgs: APIListOrgs;
  static readonly RemoveOrg: APIRemoveOrg;
  static readonly InviteToOrg: APIInviteToOrg;
  static readonly LeaveOrg: APILeaveOrg;
  static readonly IsUsernameAvailable: APIIsUsernameAvailable;
  static readonly IsOrgNameAvailable: APIIsOrgNameAvailable;
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
  signup(
    requestMessage: hub_pb.SignupRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: hub_pb.SignupReply|null) => void
  ): UnaryResponse;
  signup(
    requestMessage: hub_pb.SignupRequest,
    callback: (error: ServiceError|null, responseMessage: hub_pb.SignupReply|null) => void
  ): UnaryResponse;
  signin(
    requestMessage: hub_pb.SigninRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: hub_pb.SigninReply|null) => void
  ): UnaryResponse;
  signin(
    requestMessage: hub_pb.SigninRequest,
    callback: (error: ServiceError|null, responseMessage: hub_pb.SigninReply|null) => void
  ): UnaryResponse;
  signout(
    requestMessage: hub_pb.SignoutRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: hub_pb.SignoutReply|null) => void
  ): UnaryResponse;
  signout(
    requestMessage: hub_pb.SignoutRequest,
    callback: (error: ServiceError|null, responseMessage: hub_pb.SignoutReply|null) => void
  ): UnaryResponse;
  getSessionInfo(
    requestMessage: hub_pb.GetSessionInfoRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: hub_pb.GetSessionInfoReply|null) => void
  ): UnaryResponse;
  getSessionInfo(
    requestMessage: hub_pb.GetSessionInfoRequest,
    callback: (error: ServiceError|null, responseMessage: hub_pb.GetSessionInfoReply|null) => void
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
  isUsernameAvailable(
    requestMessage: hub_pb.IsUsernameAvailableRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: hub_pb.IsUsernameAvailableReply|null) => void
  ): UnaryResponse;
  isUsernameAvailable(
    requestMessage: hub_pb.IsUsernameAvailableRequest,
    callback: (error: ServiceError|null, responseMessage: hub_pb.IsUsernameAvailableReply|null) => void
  ): UnaryResponse;
  isOrgNameAvailable(
    requestMessage: hub_pb.IsOrgNameAvailableRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: hub_pb.IsOrgNameAvailableReply|null) => void
  ): UnaryResponse;
  isOrgNameAvailable(
    requestMessage: hub_pb.IsOrgNameAvailableRequest,
    callback: (error: ServiceError|null, responseMessage: hub_pb.IsOrgNameAvailableReply|null) => void
  ): UnaryResponse;
}

