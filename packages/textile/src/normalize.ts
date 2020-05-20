/* eslint-disable @typescript-eslint/no-use-before-define */
import { Buffer } from 'buffer'

export interface File {
  path: string
  content?: AsyncIterable<Buffer | ArrayBuffer>

  [key: string]: any
}

// This function comes from https://github.com/ipfs/js-ipfs-utils/blob/master/src/files/normalise-input.js

/*
 * Transform one of:
 *
 * ```
 * Bytes (Buffer|ArrayBuffer|TypedArray) [single file]
 * Bloby (Blob|File) [single file]
 * String [single file]
 * { path, content: Bytes } [single file]
 * { path, content: Bloby } [single file]
 * { path, content: String } [single file]
 * { path, content: Iterable<Number> } [single file]
 * { path, content: Iterable<Bytes> } [single file]
 * { path, content: AsyncIterable<Bytes> } [single file]
 * Iterable<Number> [single file]
 * Iterable<Bytes> [single file]
 * Iterable<Bloby> [multiple files]
 * Iterable<String> [multiple files]
 * Iterable<{ path, content: Bytes }> [multiple files]
 * Iterable<{ path, content: Bloby }> [multiple files]
 * Iterable<{ path, content: String }> [multiple files]
 * Iterable<{ path, content: Iterable<Number> }> [multiple files]
 * Iterable<{ path, content: Iterable<Bytes> }> [multiple files]
 * Iterable<{ path, content: AsyncIterable<Bytes> }> [multiple files]
 * AsyncIterable<Bytes> [single file]
 * AsyncIterable<Bloby> [multiple files]
 * AsyncIterable<String> [multiple files]
 * AsyncIterable<{ path, content: Bytes }> [multiple files]
 * AsyncIterable<{ path, content: Bloby }> [multiple files]
 * AsyncIterable<{ path, content: String }> [multiple files]
 * AsyncIterable<{ path, content: Iterable<Number> }> [multiple files]
 * AsyncIterable<{ path, content: Iterable<Bytes> }> [multiple files]
 * AsyncIterable<{ path, content: AsyncIterable<Bytes> }> [multiple files]
 * ```
 * Into:
 *
 * ```
 * AsyncIterable<{ path, content: AsyncIterable<Buffer> }>
 * ```
 *
 * @param {Object} input
 * @return AsyncInterable<{ path, content: AsyncIterable<Buffer> }>
 */
export function normaliseInput(input: any) {
  // must give us something
  if (input === null || input === undefined) {
    throw new Error(`Unexpected input: ${input}`)
  }

  // String
  if (typeof input === 'string' || input instanceof String) {
    return (async function* () {
      // eslint-disable-line require-await
      yield toFileObject(input)
    })()
  }

  // Buffer|ArrayBuffer|TypedArray
  // Blob|File
  if (isBytes(input) || isBloby(input)) {
    return (async function* () {
      // eslint-disable-line require-await
      yield toFileObject(input)
    })()
  }

  // Iterable<?>
  if (input[Symbol.iterator]) {
    return (async function* () {
      // eslint-disable-line require-await
      const iterator = input[Symbol.iterator]()
      const first = iterator.next()
      if (first.done) return iterator

      // Iterable<Number>
      // Iterable<Bytes>
      if (Number.isInteger(first.value) || isBytes(first.value)) {
        yield toFileObject(
          (function* () {
            yield first.value
            yield* iterator
          })(),
        )
        return
      }

      // Iterable<Bloby>
      // Iterable<String>
      // Iterable<{ path, content }>
      if (isFileObject(first.value) || isBloby(first.value) || typeof first.value === 'string') {
        yield toFileObject(first.value)
        for (const obj of iterator) {
          yield toFileObject(obj)
        }
        return
      }

      throw new Error('Unexpected input: ' + typeof input)
    })()
  }

  // window.ReadableStream
  if (typeof input.getReader === 'function') {
    return (async function* () {
      for await (const obj of browserStreamToIt(input)) {
        yield toFileObject(obj)
      }
    })()
  }

  // AsyncIterable<?>
  if (input[Symbol.asyncIterator]) {
    return (async function* () {
      const iterator = input[Symbol.asyncIterator]()
      const first = await iterator.next()
      if (first.done) return iterator

      // AsyncIterable<Bytes>
      if (isBytes(first.value)) {
        yield toFileObject(
          (async function* () {
            // eslint-disable-line require-await
            yield first.value
            yield* iterator
          })(),
        )
        return
      }

      // AsyncIterable<Bloby>
      // AsyncIterable<String>
      // AsyncIterable<{ path, content }>
      if (isFileObject(first.value) || isBloby(first.value) || typeof first.value === 'string') {
        yield toFileObject(first.value)
        for await (const obj of iterator) {
          yield toFileObject(obj)
        }
        return
      }

      throw new Error('Unexpected input: ' + typeof input)
    })()
  }

  // { path, content: ? }
  // Note: Detected _after_ AsyncIterable<?> because Node.js streams have a
  // `path` property that passes this check.
  if (isFileObject(input)) {
    return (async function* () {
      // eslint-disable-line require-await
      yield toFileObject(input)
    })()
  }

  throw new Error('Unexpected input: ' + typeof input)
}

function toFileObject(input: any) {
  const obj: File = {
    path: input.path || '',
    mode: input.mode,
    mtime: input.mtime,
  }

  if (input.content) {
    obj.content = toAsyncIterable(input.content)
  } else if (!input.path) {
    // Not already a file object with path or content prop
    obj.content = toAsyncIterable(input)
  }

  return obj
}

function toAsyncIterable(input: any) {
  // Bytes | String
  if (isBytes(input) || typeof input === 'string') {
    return (async function* () {
      // eslint-disable-line require-await
      yield toBuffer(input)
    })()
  }

  // Bloby
  if (isBloby(input)) {
    return blobToAsyncGenerator(input)
  }

  // Browser stream
  if (typeof input.getReader === 'function') {
    return browserStreamToIt(input)
  }

  // Iterator<?>
  if (input[Symbol.iterator]) {
    return (async function* () {
      // eslint-disable-line require-await
      const iterator: IterableIterator<any> = input[Symbol.iterator]()
      const first = iterator.next()
      if (first.done) return iterator

      // Iterable<Number>
      if (Number.isInteger(first.value as number)) {
        yield toBuffer(
          Array.from(
            (function* () {
              yield first.value
              yield* iterator
            })(),
          ) as any,
        )
        return
      }

      // Iterable<Bytes>
      if (isBytes(first.value)) {
        yield toBuffer(first.value)
        for (const chunk of iterator) {
          yield toBuffer(chunk)
        }
        return
      }

      throw new Error('Unexpected input: ' + typeof input)
    })()
  }

  // AsyncIterable<Bytes>
  if (input[Symbol.asyncIterator]) {
    return (async function* () {
      for await (const chunk of input) {
        yield toBuffer(chunk)
      }
    })()
  }

  throw new Error(`Unexpected input: ${input}`)
}

function toBuffer(chunk: Buffer | ArrayBuffer): Buffer | ArrayBuffer {
  return isBytes(chunk) ? chunk : Buffer.from(chunk)
}

function isBytes(obj: Buffer | ArrayBuffer) {
  return Buffer.isBuffer(obj) || ArrayBuffer.isView(obj) || obj instanceof ArrayBuffer
}

function isBloby(obj: any) {
  return typeof globalThis.Blob !== 'undefined' && obj instanceof globalThis.Blob
}

// An object with a path or content property
function isFileObject(obj: any) {
  return typeof obj === 'object' && (obj.path || obj.content)
}

function blobToAsyncGenerator(blob: Blob) {
  // eslint-disable-next-line @typescript-eslint/ban-ts-ignore
  // @ts-ignore
  if (typeof blob.stream === 'function') {
    // firefox < 69 does not support blob.stream()
    // eslint-disable-next-line @typescript-eslint/ban-ts-ignore
    // @ts-ignore
    return browserStreamToIt(blob.stream())
  }

  return readBlob(blob)
}

async function* browserStreamToIt(stream: ReadableStream<Buffer>) {
  const reader = stream.getReader()

  while (true) {
    const result = await reader.read()

    if (result.done) {
      return
    }

    yield result.value
  }
}

async function* readBlob(blob: Blob, options?: any) {
  options = options || {}

  const reader = new globalThis.FileReader()
  const chunkSize = options.chunkSize || 1024 * 1024
  let offset = options.offset || 0

  const getNextChunk = () =>
    new Promise<ArrayBuffer | null>((resolve, reject) => {
      reader.onloadend = (e) => {
        const data = e.target?.result as ArrayBuffer
        resolve(data.byteLength === 0 ? null : data)
      }
      reader.onerror = reject

      const end = offset + chunkSize
      const slice = blob.slice(offset, end)
      reader.readAsArrayBuffer(slice)
      offset = end
    })

  while (true) {
    const data = await getNextChunk()

    if (data == null) {
      return
    }

    yield Buffer.from(data)
  }
}
