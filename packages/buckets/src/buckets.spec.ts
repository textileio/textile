/* eslint-disable import/first */
;(global as any).WebSocket = require('isomorphic-ws')

import fs from 'fs'
import path from 'path'
import { isBrowser, isNode } from 'browser-or-node'
import { ThreadID } from '@textile/threads-id'
import { expect } from 'chai'
import { Client } from '@textile/threads-client'
import { InitReply } from '@textile/buckets-grpc/buckets_pb'
import { Context, Provider } from '@textile/context'
import { Buckets } from './buckets'
import { signUp } from './spec.util'

// Settings for localhost development and testing
const addrApiurl = 'http://127.0.0.1:3007'
const addrGatewayUrl = 'http://127.0.0.1:8006'
const wrongError = new Error('wrong error!')
const sessionSecret = 'textilesession'

describe('Buckets...', () => {
  const ctx: Context = new Provider(addrApiurl, undefined)
  const client = new Buckets(ctx)
  let buck: InitReply.AsObject
  let fileSize: number
  before(async () => {
    const user = await signUp(ctx, addrGatewayUrl, sessionSecret)
    const id = ThreadID.fromRandom()
    const db = new Client(ctx.withSession(user.user?.session).withThreadName('buckets'))
    await db.newDB(id, ctx.withThread(id.toString()))
  })

  it('should init a new bucket', async () => {
    // Check that we're empty
    const list = await client.list()
    expect(list).to.have.length(0)
    // Now initialize a bucket
    buck = await client.init('mybuck')
    expect(buck).to.have.ownProperty('root')
    expect(buck.root).to.have.ownProperty('key')
    expect(buck.root).to.have.ownProperty('path')
    expect(buck.root).to.have.ownProperty('createdat')
    expect(buck.root).to.have.ownProperty('updatedat')
  })

  it('should list buckets', async () => {
    const roots = await client.list()
    expect(roots).to.have.length(1)
    const root = roots[0]
    expect(root).to.have.ownProperty('key', buck.root?.key)
    expect(root).to.have.ownProperty('path', buck.root?.path)
    expect(root).to.have.ownProperty('createdat', buck.root?.createdat)
    expect(root).to.have.ownProperty('updatedat', buck.root?.updatedat)
  })

  it('should list empty bucket content at path', async () => {
    // Mostly empty
    const res = await client.listPath(buck.root?.key || '', '')
    expect(res).to.have.ownProperty('root')
    expect(res.root).to.not.be.undefined
    expect(res.item?.isdir).to.be.true
    // @todo: Should we rename itemsList to just be items?
    expect(res.item?.itemsList).to.have.length(0)
  })

  it('should push data from filesystem on node', async function () {
    if (isBrowser) return this.skip()
    const pth = path.join(__dirname, '../../..', 'testdata')
    fileSize = fs.statSync(path.join(pth, 'file1.jpg')).size
    let stream = fs.createReadStream(path.join(pth, 'file1.jpg'))
    const rootKey = buck.root?.key || ''
    let length = 0

    // Bucket path
    const res = await client.pushPath(rootKey, 'dir1/file1.jpg', stream, undefined, {
      progress: (num) => (length = num || 0),
    })
    expect(length).to.equal(fileSize)
    expect(res.path).to.not.be.undefined
    expect(res.root).to.not.be.undefined

    // Nested bucket path
    stream = fs.createReadStream(path.join(pth, 'file2.jpg'))
    const { root } = await client.pushPath(rootKey, 'path/to/file2.jpg', stream)
    expect(root).to.not.be.undefined

    // Root dir
    const rep = await client.listPath(rootKey, '')
    expect(rep.item?.isdir).to.be.true
    expect(rep.item?.itemsList).to.have.length(2)
  })

  it('should push data from file API in browser', async function () {
    if (isNode) return this.skip()
    const parts = [
      new Blob(['you construct a file...'], { type: 'text/plain' }),
      ' Same way as you do with blob',
      new Uint16Array([33]),
    ]
    // Construct a file
    const file = new File(parts, 'file1.txt')
    const rootKey = buck.root?.key || ''
    let length = 0

    // Bucket path
    const res = await client.pushPath(rootKey, 'dir1/file1.jpg', file, undefined, {
      progress: (num) => (length = num || 0),
    })
    expect(length).to.equal(54)
    expect(res.path).to.not.be.undefined
    expect(res.root).to.not.be.undefined

    // Nested bucket path
    // @note: We're reusing file here...
    const { root } = await client.pushPath(rootKey, 'path/to/file2.jpg', file)
    expect(root).to.not.be.undefined

    // Root dir
    const rep = await client.listPath(rootKey, '')
    expect(rep.item?.isdir).to.be.true
    expect(rep.item?.itemsList).to.have.length(2)
  })

  it('should list (nested) files within a bucket', async () => {
    const rootKey = buck.root?.key || ''

    // Nested dir
    let rep = await client.listPath(rootKey, 'dir1')
    expect(rep.item?.isdir).to.be.true
    expect(rep.item?.itemsList).to.have.length(1)

    // File
    rep = await client.listPath(rootKey, 'dir1/file1.jpg')
    expect(rep.item?.path.endsWith('file1.jpg')).to.be.true
    expect(rep.item?.isdir).to.be.false
  })

  it('should pull files by path and write to file on node', async function () {
    if (isBrowser) return this.skip()
    // Bucket path
    const rootKey = buck.root?.key || ''
    let length = 0

    // Bucket path
    const chunks = client.pullPath(rootKey, 'dir1/file1.jpg', undefined, {
      progress: (num) => (length = num || 0),
    })
    const pth = path.join(__dirname, '../../..', 'testdata')
    const stream = fs.createWriteStream(path.join(pth, 'output.jpg'))
    for await (const chunk of chunks) {
      stream.write(chunk)
    }
    stream.close()
    expect(length).to.equal(fileSize)
    const stored = fs.statSync(path.join(pth, 'file1.jpg'))
    const written = fs.statSync(path.join(pth, 'output.jpg'))
    // expect(stored.size).to.equal(written.size)
    fs.unlinkSync(path.join(pth, 'output.jpg'))
  })

  it('should remove files by path', async () => {
    const rootKey = buck.root?.key || ''
    await client.removePath(rootKey, 'path/to/file2.jpg')
    try {
      await client.listPath(rootKey, 'path/to/file2.jpg')
      throw wrongError
    } catch (err) {
      expect(err).to.not.equal(wrongError)
    }
    let list = await client.listPath(rootKey, '')
    expect(list.item?.itemsList).to.have.length(2)
    await client.removePath(rootKey, 'path')
    try {
      await client.listPath(rootKey, 'path')
      throw wrongError
    } catch (err) {
      expect(err).to.not.equal(wrongError)
    }
    list = await client.listPath(rootKey, '')
    expect(list.item?.itemsList).to.have.length(1)
  })

  it('should list bucket links', async () => {
    const rootKey = buck.root?.key || ''

    const rep = await client.links(rootKey)
    expect(rep.url).to.not.equal('')
    expect(rep.ipns).to.not.equal('')
  })

  it('should remove an entire bucket', async () => {
    const rootKey = buck.root?.key || ''
    const rep = await client.listPath(rootKey, 'dir1/file1.jpg')
    expect(rep).to.not.be.undefined
    await client.remove(rootKey)
    try {
      await client.listPath(rootKey, 'dir1/file1.jpg')
      throw wrongError
    } catch (err) {
      expect(err).to.not.equal(wrongError)
    }
  })
})
