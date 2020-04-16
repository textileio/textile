// package: hub.pb
// file: hub.proto

import * as jspb from "google-protobuf";

export class LoginRequest extends jspb.Message {
  getUsername(): string;
  setUsername(value: string): void;

  getEmail(): string;
  setEmail(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LoginRequest.AsObject;
  static toObject(includeInstance: boolean, msg: LoginRequest): LoginRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LoginRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LoginRequest;
  static deserializeBinaryFromReader(message: LoginRequest, reader: jspb.BinaryReader): LoginRequest;
}

export namespace LoginRequest {
  export type AsObject = {
    username: string,
    email: string,
  }
}

export class LoginReply extends jspb.Message {
  getKey(): Uint8Array | string;
  getKey_asU8(): Uint8Array;
  getKey_asB64(): string;
  setKey(value: Uint8Array | string): void;

  getUsername(): string;
  setUsername(value: string): void;

  getSession(): string;
  setSession(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LoginReply.AsObject;
  static toObject(includeInstance: boolean, msg: LoginReply): LoginReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LoginReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LoginReply;
  static deserializeBinaryFromReader(message: LoginReply, reader: jspb.BinaryReader): LoginReply;
}

export namespace LoginReply {
  export type AsObject = {
    key: Uint8Array | string,
    username: string,
    session: string,
  }
}

export class LogoutRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LogoutRequest.AsObject;
  static toObject(includeInstance: boolean, msg: LogoutRequest): LogoutRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LogoutRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LogoutRequest;
  static deserializeBinaryFromReader(message: LogoutRequest, reader: jspb.BinaryReader): LogoutRequest;
}

export namespace LogoutRequest {
  export type AsObject = {
  }
}

export class LogoutReply extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LogoutReply.AsObject;
  static toObject(includeInstance: boolean, msg: LogoutReply): LogoutReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LogoutReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LogoutReply;
  static deserializeBinaryFromReader(message: LogoutReply, reader: jspb.BinaryReader): LogoutReply;
}

export namespace LogoutReply {
  export type AsObject = {
  }
}

export class WhoamiRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): WhoamiRequest.AsObject;
  static toObject(includeInstance: boolean, msg: WhoamiRequest): WhoamiRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: WhoamiRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): WhoamiRequest;
  static deserializeBinaryFromReader(message: WhoamiRequest, reader: jspb.BinaryReader): WhoamiRequest;
}

export namespace WhoamiRequest {
  export type AsObject = {
  }
}

export class WhoamiReply extends jspb.Message {
  getKey(): Uint8Array | string;
  getKey_asU8(): Uint8Array;
  getKey_asB64(): string;
  setKey(value: Uint8Array | string): void;

  getUsername(): string;
  setUsername(value: string): void;

  getEmail(): string;
  setEmail(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): WhoamiReply.AsObject;
  static toObject(includeInstance: boolean, msg: WhoamiReply): WhoamiReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: WhoamiReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): WhoamiReply;
  static deserializeBinaryFromReader(message: WhoamiReply, reader: jspb.BinaryReader): WhoamiReply;
}

export namespace WhoamiReply {
  export type AsObject = {
    key: Uint8Array | string,
    username: string,
    email: string,
  }
}

export class ListThreadsRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListThreadsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ListThreadsRequest): ListThreadsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListThreadsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListThreadsRequest;
  static deserializeBinaryFromReader(message: ListThreadsRequest, reader: jspb.BinaryReader): ListThreadsRequest;
}

export namespace ListThreadsRequest {
  export type AsObject = {
  }
}

export class ListThreadsReply extends jspb.Message {
  clearListList(): void;
  getListList(): Array<ListThreadsReply.Thread>;
  setListList(value: Array<ListThreadsReply.Thread>): void;
  addList(value?: ListThreadsReply.Thread, index?: number): ListThreadsReply.Thread;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListThreadsReply.AsObject;
  static toObject(includeInstance: boolean, msg: ListThreadsReply): ListThreadsReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListThreadsReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListThreadsReply;
  static deserializeBinaryFromReader(message: ListThreadsReply, reader: jspb.BinaryReader): ListThreadsReply;
}

export namespace ListThreadsReply {
  export type AsObject = {
    listList: Array<ListThreadsReply.Thread.AsObject>,
  }

  export class Thread extends jspb.Message {
    getId(): Uint8Array | string;
    getId_asU8(): Uint8Array;
    getId_asB64(): string;
    setId(value: Uint8Array | string): void;

    getName(): string;
    setName(value: string): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Thread.AsObject;
    static toObject(includeInstance: boolean, msg: Thread): Thread.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Thread, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Thread;
    static deserializeBinaryFromReader(message: Thread, reader: jspb.BinaryReader): Thread;
  }

  export namespace Thread {
    export type AsObject = {
      id: Uint8Array | string,
      name: string,
    }
  }
}

export class CreateKeyRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CreateKeyRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CreateKeyRequest): CreateKeyRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CreateKeyRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CreateKeyRequest;
  static deserializeBinaryFromReader(message: CreateKeyRequest, reader: jspb.BinaryReader): CreateKeyRequest;
}

export namespace CreateKeyRequest {
  export type AsObject = {
  }
}

export class GetKeyReply extends jspb.Message {
  getKey(): string;
  setKey(value: string): void;

  getSecret(): string;
  setSecret(value: string): void;

  getValid(): boolean;
  setValid(value: boolean): void;

  getThreads(): number;
  setThreads(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetKeyReply.AsObject;
  static toObject(includeInstance: boolean, msg: GetKeyReply): GetKeyReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetKeyReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetKeyReply;
  static deserializeBinaryFromReader(message: GetKeyReply, reader: jspb.BinaryReader): GetKeyReply;
}

export namespace GetKeyReply {
  export type AsObject = {
    key: string,
    secret: string,
    valid: boolean,
    threads: number,
  }
}

export class InvalidateKeyRequest extends jspb.Message {
  getKey(): string;
  setKey(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): InvalidateKeyRequest.AsObject;
  static toObject(includeInstance: boolean, msg: InvalidateKeyRequest): InvalidateKeyRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: InvalidateKeyRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): InvalidateKeyRequest;
  static deserializeBinaryFromReader(message: InvalidateKeyRequest, reader: jspb.BinaryReader): InvalidateKeyRequest;
}

export namespace InvalidateKeyRequest {
  export type AsObject = {
    key: string,
  }
}

export class InvalidateKeyReply extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): InvalidateKeyReply.AsObject;
  static toObject(includeInstance: boolean, msg: InvalidateKeyReply): InvalidateKeyReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: InvalidateKeyReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): InvalidateKeyReply;
  static deserializeBinaryFromReader(message: InvalidateKeyReply, reader: jspb.BinaryReader): InvalidateKeyReply;
}

export namespace InvalidateKeyReply {
  export type AsObject = {
  }
}

export class ListKeysRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListKeysRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ListKeysRequest): ListKeysRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListKeysRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListKeysRequest;
  static deserializeBinaryFromReader(message: ListKeysRequest, reader: jspb.BinaryReader): ListKeysRequest;
}

export namespace ListKeysRequest {
  export type AsObject = {
  }
}

export class ListKeysReply extends jspb.Message {
  clearListList(): void;
  getListList(): Array<GetKeyReply>;
  setListList(value: Array<GetKeyReply>): void;
  addList(value?: GetKeyReply, index?: number): GetKeyReply;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListKeysReply.AsObject;
  static toObject(includeInstance: boolean, msg: ListKeysReply): ListKeysReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListKeysReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListKeysReply;
  static deserializeBinaryFromReader(message: ListKeysReply, reader: jspb.BinaryReader): ListKeysReply;
}

export namespace ListKeysReply {
  export type AsObject = {
    listList: Array<GetKeyReply.AsObject>,
  }
}

export class CreateOrgRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CreateOrgRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CreateOrgRequest): CreateOrgRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CreateOrgRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CreateOrgRequest;
  static deserializeBinaryFromReader(message: CreateOrgRequest, reader: jspb.BinaryReader): CreateOrgRequest;
}

export namespace CreateOrgRequest {
  export type AsObject = {
    name: string,
  }
}

export class GetOrgRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetOrgRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetOrgRequest): GetOrgRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetOrgRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetOrgRequest;
  static deserializeBinaryFromReader(message: GetOrgRequest, reader: jspb.BinaryReader): GetOrgRequest;
}

export namespace GetOrgRequest {
  export type AsObject = {
  }
}

export class GetOrgReply extends jspb.Message {
  getKey(): Uint8Array | string;
  getKey_asU8(): Uint8Array;
  getKey_asB64(): string;
  setKey(value: Uint8Array | string): void;

  getName(): string;
  setName(value: string): void;

  clearMembersList(): void;
  getMembersList(): Array<GetOrgReply.Member>;
  setMembersList(value: Array<GetOrgReply.Member>): void;
  addMembers(value?: GetOrgReply.Member, index?: number): GetOrgReply.Member;

  getCreatedat(): number;
  setCreatedat(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetOrgReply.AsObject;
  static toObject(includeInstance: boolean, msg: GetOrgReply): GetOrgReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetOrgReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetOrgReply;
  static deserializeBinaryFromReader(message: GetOrgReply, reader: jspb.BinaryReader): GetOrgReply;
}

export namespace GetOrgReply {
  export type AsObject = {
    key: Uint8Array | string,
    name: string,
    membersList: Array<GetOrgReply.Member.AsObject>,
    createdat: number,
  }

  export class Member extends jspb.Message {
    getKey(): Uint8Array | string;
    getKey_asU8(): Uint8Array;
    getKey_asB64(): string;
    setKey(value: Uint8Array | string): void;

    getUsername(): string;
    setUsername(value: string): void;

    getRole(): string;
    setRole(value: string): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Member.AsObject;
    static toObject(includeInstance: boolean, msg: Member): Member.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Member, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Member;
    static deserializeBinaryFromReader(message: Member, reader: jspb.BinaryReader): Member;
  }

  export namespace Member {
    export type AsObject = {
      key: Uint8Array | string,
      username: string,
      role: string,
    }
  }
}

export class ListOrgsRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListOrgsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ListOrgsRequest): ListOrgsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListOrgsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListOrgsRequest;
  static deserializeBinaryFromReader(message: ListOrgsRequest, reader: jspb.BinaryReader): ListOrgsRequest;
}

export namespace ListOrgsRequest {
  export type AsObject = {
  }
}

export class ListOrgsReply extends jspb.Message {
  clearListList(): void;
  getListList(): Array<GetOrgReply>;
  setListList(value: Array<GetOrgReply>): void;
  addList(value?: GetOrgReply, index?: number): GetOrgReply;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListOrgsReply.AsObject;
  static toObject(includeInstance: boolean, msg: ListOrgsReply): ListOrgsReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListOrgsReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListOrgsReply;
  static deserializeBinaryFromReader(message: ListOrgsReply, reader: jspb.BinaryReader): ListOrgsReply;
}

export namespace ListOrgsReply {
  export type AsObject = {
    listList: Array<GetOrgReply.AsObject>,
  }
}

export class RemoveOrgRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): RemoveOrgRequest.AsObject;
  static toObject(includeInstance: boolean, msg: RemoveOrgRequest): RemoveOrgRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: RemoveOrgRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): RemoveOrgRequest;
  static deserializeBinaryFromReader(message: RemoveOrgRequest, reader: jspb.BinaryReader): RemoveOrgRequest;
}

export namespace RemoveOrgRequest {
  export type AsObject = {
  }
}

export class RemoveOrgReply extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): RemoveOrgReply.AsObject;
  static toObject(includeInstance: boolean, msg: RemoveOrgReply): RemoveOrgReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: RemoveOrgReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): RemoveOrgReply;
  static deserializeBinaryFromReader(message: RemoveOrgReply, reader: jspb.BinaryReader): RemoveOrgReply;
}

export namespace RemoveOrgReply {
  export type AsObject = {
  }
}

export class InviteToOrgRequest extends jspb.Message {
  getEmail(): string;
  setEmail(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): InviteToOrgRequest.AsObject;
  static toObject(includeInstance: boolean, msg: InviteToOrgRequest): InviteToOrgRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: InviteToOrgRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): InviteToOrgRequest;
  static deserializeBinaryFromReader(message: InviteToOrgRequest, reader: jspb.BinaryReader): InviteToOrgRequest;
}

export namespace InviteToOrgRequest {
  export type AsObject = {
    email: string,
  }
}

export class InviteToOrgReply extends jspb.Message {
  getToken(): string;
  setToken(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): InviteToOrgReply.AsObject;
  static toObject(includeInstance: boolean, msg: InviteToOrgReply): InviteToOrgReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: InviteToOrgReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): InviteToOrgReply;
  static deserializeBinaryFromReader(message: InviteToOrgReply, reader: jspb.BinaryReader): InviteToOrgReply;
}

export namespace InviteToOrgReply {
  export type AsObject = {
    token: string,
  }
}

export class LeaveOrgRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LeaveOrgRequest.AsObject;
  static toObject(includeInstance: boolean, msg: LeaveOrgRequest): LeaveOrgRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LeaveOrgRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LeaveOrgRequest;
  static deserializeBinaryFromReader(message: LeaveOrgRequest, reader: jspb.BinaryReader): LeaveOrgRequest;
}

export namespace LeaveOrgRequest {
  export type AsObject = {
  }
}

export class LeaveOrgReply extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LeaveOrgReply.AsObject;
  static toObject(includeInstance: boolean, msg: LeaveOrgReply): LeaveOrgReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LeaveOrgReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LeaveOrgReply;
  static deserializeBinaryFromReader(message: LeaveOrgReply, reader: jspb.BinaryReader): LeaveOrgReply;
}

export namespace LeaveOrgReply {
  export type AsObject = {
  }
}

