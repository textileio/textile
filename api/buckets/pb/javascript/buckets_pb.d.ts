// package: buckets.pb
// file: buckets.proto

import * as jspb from "google-protobuf";

export class Root extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getPath(): string;
  setPath(value: string): void;

  getCreatedat(): number;
  setCreatedat(value: number): void;

  getUpdatedat(): number;
  setUpdatedat(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Root.AsObject;
  static toObject(includeInstance: boolean, msg: Root): Root.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Root, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Root;
  static deserializeBinaryFromReader(message: Root, reader: jspb.BinaryReader): Root;
}

export namespace Root {
  export type AsObject = {
    name: string,
    path: string,
    createdat: number,
    updatedat: number,
  }
}

export class ListPathRequest extends jspb.Message {
  getPath(): string;
  setPath(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListPathRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ListPathRequest): ListPathRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListPathRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListPathRequest;
  static deserializeBinaryFromReader(message: ListPathRequest, reader: jspb.BinaryReader): ListPathRequest;
}

export namespace ListPathRequest {
  export type AsObject = {
    path: string,
  }
}

export class ListPathReply extends jspb.Message {
  hasItem(): boolean;
  clearItem(): void;
  getItem(): ListPathReply.Item | undefined;
  setItem(value?: ListPathReply.Item): void;

  hasRoot(): boolean;
  clearRoot(): void;
  getRoot(): Root | undefined;
  setRoot(value?: Root): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListPathReply.AsObject;
  static toObject(includeInstance: boolean, msg: ListPathReply): ListPathReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListPathReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListPathReply;
  static deserializeBinaryFromReader(message: ListPathReply, reader: jspb.BinaryReader): ListPathReply;
}

export namespace ListPathReply {
  export type AsObject = {
    item?: ListPathReply.Item.AsObject,
    root?: Root.AsObject,
  }

  export class Item extends jspb.Message {
    getName(): string;
    setName(value: string): void;

    getPath(): string;
    setPath(value: string): void;

    getSize(): number;
    setSize(value: number): void;

    getIsdir(): boolean;
    setIsdir(value: boolean): void;

    clearItemsList(): void;
    getItemsList(): Array<ListPathReply.Item>;
    setItemsList(value: Array<ListPathReply.Item>): void;
    addItems(value?: ListPathReply.Item, index?: number): ListPathReply.Item;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Item.AsObject;
    static toObject(includeInstance: boolean, msg: Item): Item.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Item, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Item;
    static deserializeBinaryFromReader(message: Item, reader: jspb.BinaryReader): Item;
  }

  export namespace Item {
    export type AsObject = {
      name: string,
      path: string,
      size: number,
      isdir: boolean,
      itemsList: Array<ListPathReply.Item.AsObject>,
    }
  }
}

export class PushPathRequest extends jspb.Message {
  hasHeader(): boolean;
  clearHeader(): void;
  getHeader(): PushPathRequest.Header | undefined;
  setHeader(value?: PushPathRequest.Header): void;

  hasChunk(): boolean;
  clearChunk(): void;
  getChunk(): Uint8Array | string;
  getChunk_asU8(): Uint8Array;
  getChunk_asB64(): string;
  setChunk(value: Uint8Array | string): void;

  getPayloadCase(): PushPathRequest.PayloadCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): PushPathRequest.AsObject;
  static toObject(includeInstance: boolean, msg: PushPathRequest): PushPathRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: PushPathRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): PushPathRequest;
  static deserializeBinaryFromReader(message: PushPathRequest, reader: jspb.BinaryReader): PushPathRequest;
}

export namespace PushPathRequest {
  export type AsObject = {
    header?: PushPathRequest.Header.AsObject,
    chunk: Uint8Array | string,
  }

  export class Header extends jspb.Message {
    getPath(): string;
    setPath(value: string): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Header.AsObject;
    static toObject(includeInstance: boolean, msg: Header): Header.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Header, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Header;
    static deserializeBinaryFromReader(message: Header, reader: jspb.BinaryReader): Header;
  }

  export namespace Header {
    export type AsObject = {
      path: string,
    }
  }

  export enum PayloadCase {
    PAYLOAD_NOT_SET = 0,
    HEADER = 1,
    CHUNK = 2,
  }
}

export class PushPathReply extends jspb.Message {
  hasEvent(): boolean;
  clearEvent(): void;
  getEvent(): PushPathReply.Event | undefined;
  setEvent(value?: PushPathReply.Event): void;

  hasError(): boolean;
  clearError(): void;
  getError(): string;
  setError(value: string): void;

  getPayloadCase(): PushPathReply.PayloadCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): PushPathReply.AsObject;
  static toObject(includeInstance: boolean, msg: PushPathReply): PushPathReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: PushPathReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): PushPathReply;
  static deserializeBinaryFromReader(message: PushPathReply, reader: jspb.BinaryReader): PushPathReply;
}

export namespace PushPathReply {
  export type AsObject = {
    event?: PushPathReply.Event.AsObject,
    error: string,
  }

  export class Event extends jspb.Message {
    getName(): string;
    setName(value: string): void;

    getPath(): string;
    setPath(value: string): void;

    getBytes(): number;
    setBytes(value: number): void;

    getSize(): string;
    setSize(value: string): void;

    hasRoot(): boolean;
    clearRoot(): void;
    getRoot(): Root | undefined;
    setRoot(value?: Root): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Event.AsObject;
    static toObject(includeInstance: boolean, msg: Event): Event.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Event, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Event;
    static deserializeBinaryFromReader(message: Event, reader: jspb.BinaryReader): Event;
  }

  export namespace Event {
    export type AsObject = {
      name: string,
      path: string,
      bytes: number,
      size: string,
      root?: Root.AsObject,
    }
  }

  export enum PayloadCase {
    PAYLOAD_NOT_SET = 0,
    EVENT = 1,
    ERROR = 2,
  }
}

export class PullPathRequest extends jspb.Message {
  getPath(): string;
  setPath(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): PullPathRequest.AsObject;
  static toObject(includeInstance: boolean, msg: PullPathRequest): PullPathRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: PullPathRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): PullPathRequest;
  static deserializeBinaryFromReader(message: PullPathRequest, reader: jspb.BinaryReader): PullPathRequest;
}

export namespace PullPathRequest {
  export type AsObject = {
    path: string,
  }
}

export class PullPathReply extends jspb.Message {
  getChunk(): Uint8Array | string;
  getChunk_asU8(): Uint8Array;
  getChunk_asB64(): string;
  setChunk(value: Uint8Array | string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): PullPathReply.AsObject;
  static toObject(includeInstance: boolean, msg: PullPathReply): PullPathReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: PullPathReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): PullPathReply;
  static deserializeBinaryFromReader(message: PullPathReply, reader: jspb.BinaryReader): PullPathReply;
}

export namespace PullPathReply {
  export type AsObject = {
    chunk: Uint8Array | string,
  }
}

export class RemovePathRequest extends jspb.Message {
  getPath(): string;
  setPath(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): RemovePathRequest.AsObject;
  static toObject(includeInstance: boolean, msg: RemovePathRequest): RemovePathRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: RemovePathRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): RemovePathRequest;
  static deserializeBinaryFromReader(message: RemovePathRequest, reader: jspb.BinaryReader): RemovePathRequest;
}

export namespace RemovePathRequest {
  export type AsObject = {
    path: string,
  }
}

export class RemovePathReply extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): RemovePathReply.AsObject;
  static toObject(includeInstance: boolean, msg: RemovePathReply): RemovePathReply.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: RemovePathReply, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): RemovePathReply;
  static deserializeBinaryFromReader(message: RemovePathReply, reader: jspb.BinaryReader): RemovePathReply;
}

export namespace RemovePathReply {
  export type AsObject = {
  }
}

