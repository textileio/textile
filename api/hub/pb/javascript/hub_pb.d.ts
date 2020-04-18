// package: hub.pb
// file: hub.proto

import * as jspb from "google-protobuf";

export class SignupRequest extends jspb.Message {
  getUsername(): string;
  setUsername(value: string): void;

  getEmail(): string;
  setEmail(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SignupRequest.AsObject;
  static toObject(includeInstance: boolean, msg: SignupRequest): SignupRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SignupRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SignupRequest;
  static deserializeBinaryFromReader(message: SignupRequest, reader: jspb.BinaryReader): SignupRequest;
}

export namespace SignupRequest {
  export type AsObject = {
    username: string,
    email: string,
  }
}

export class SignupReply extends jspb.Message {
  getKey(): Uint8Array | string;
  getKey_asU8(): Uint8Array;
  getKey_asB64(): string;
  setKey(value: Uint8Array | string): void;

  getSession(): string;
  setSession(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SignupReply.AsObject;
  static toObject(includeInstance: boolean, msg: SignupReply): SignupReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SignupReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SignupReply;
  static deserializeBinaryFromReader(message: SignupReply, reader: jspb.BinaryReader): SignupReply;
}

export namespace SignupReply {
  export type AsObject = {
    key: Uint8Array | string,
    session: string,
  }
}

export class SigninRequest extends jspb.Message {
  getUsernameoremail(): string;
  setUsernameoremail(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SigninRequest.AsObject;
  static toObject(includeInstance: boolean, msg: SigninRequest): SigninRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SigninRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SigninRequest;
  static deserializeBinaryFromReader(message: SigninRequest, reader: jspb.BinaryReader): SigninRequest;
}

export namespace SigninRequest {
  export type AsObject = {
    usernameoremail: string,
  }
}

export class SigninReply extends jspb.Message {
  getKey(): Uint8Array | string;
  getKey_asU8(): Uint8Array;
  getKey_asB64(): string;
  setKey(value: Uint8Array | string): void;

  getSession(): string;
  setSession(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SigninReply.AsObject;
  static toObject(includeInstance: boolean, msg: SigninReply): SigninReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SigninReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SigninReply;
  static deserializeBinaryFromReader(message: SigninReply, reader: jspb.BinaryReader): SigninReply;
}

export namespace SigninReply {
  export type AsObject = {
    key: Uint8Array | string,
    session: string,
  }
}

export class SignoutRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SignoutRequest.AsObject;
  static toObject(includeInstance: boolean, msg: SignoutRequest): SignoutRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SignoutRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SignoutRequest;
  static deserializeBinaryFromReader(message: SignoutRequest, reader: jspb.BinaryReader): SignoutRequest;
}

export namespace SignoutRequest {
  export type AsObject = {
  }
}

export class SignoutReply extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SignoutReply.AsObject;
  static toObject(includeInstance: boolean, msg: SignoutReply): SignoutReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SignoutReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SignoutReply;
  static deserializeBinaryFromReader(message: SignoutReply, reader: jspb.BinaryReader): SignoutReply;
}

export namespace SignoutReply {
  export type AsObject = {
  }
}

export class CheckUsernameRequest extends jspb.Message {
  getUsername(): string;
  setUsername(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CheckUsernameRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CheckUsernameRequest): CheckUsernameRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CheckUsernameRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CheckUsernameRequest;
  static deserializeBinaryFromReader(message: CheckUsernameRequest, reader: jspb.BinaryReader): CheckUsernameRequest;
}

export namespace CheckUsernameRequest {
  export type AsObject = {
    username: string,
  }
}

export class CheckUsernameReply extends jspb.Message {
  getOk(): boolean;
  setOk(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CheckUsernameReply.AsObject;
  static toObject(includeInstance: boolean, msg: CheckUsernameReply): CheckUsernameReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CheckUsernameReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CheckUsernameReply;
  static deserializeBinaryFromReader(message: CheckUsernameReply, reader: jspb.BinaryReader): CheckUsernameReply;
}

export namespace CheckUsernameReply {
  export type AsObject = {
    ok: boolean,
  }
}

export class GetSessionRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetSessionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetSessionRequest): GetSessionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetSessionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetSessionRequest;
  static deserializeBinaryFromReader(message: GetSessionRequest, reader: jspb.BinaryReader): GetSessionRequest;
}

export namespace GetSessionRequest {
  export type AsObject = {
  }
}

export class GetSessionReply extends jspb.Message {
  getKey(): Uint8Array | string;
  getKey_asU8(): Uint8Array;
  getKey_asB64(): string;
  setKey(value: Uint8Array | string): void;

  getUsername(): string;
  setUsername(value: string): void;

  getEmail(): string;
  setEmail(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetSessionReply.AsObject;
  static toObject(includeInstance: boolean, msg: GetSessionReply): GetSessionReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetSessionReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetSessionReply;
  static deserializeBinaryFromReader(message: GetSessionReply, reader: jspb.BinaryReader): GetSessionReply;
}

export namespace GetSessionReply {
  export type AsObject = {
    key: Uint8Array | string,
    username: string,
    email: string,
  }
}

export class GetThreadRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetThreadRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetThreadRequest): GetThreadRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetThreadRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetThreadRequest;
  static deserializeBinaryFromReader(message: GetThreadRequest, reader: jspb.BinaryReader): GetThreadRequest;
}

export namespace GetThreadRequest {
  export type AsObject = {
    name: string,
  }
}

export class GetThreadReply extends jspb.Message {
  getId(): Uint8Array | string;
  getId_asU8(): Uint8Array;
  getId_asB64(): string;
  setId(value: Uint8Array | string): void;

  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetThreadReply.AsObject;
  static toObject(includeInstance: boolean, msg: GetThreadReply): GetThreadReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetThreadReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetThreadReply;
  static deserializeBinaryFromReader(message: GetThreadReply, reader: jspb.BinaryReader): GetThreadReply;
}

export namespace GetThreadReply {
  export type AsObject = {
    id: Uint8Array | string,
    name: string,
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
  getListList(): Array<GetThreadReply>;
  setListList(value: Array<GetThreadReply>): void;
  addList(value?: GetThreadReply, index?: number): GetThreadReply;

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
    listList: Array<GetThreadReply.AsObject>,
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

