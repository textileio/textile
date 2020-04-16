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

