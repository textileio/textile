// package: users.pb
// file: users.proto

import * as jspb from "google-protobuf";

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

