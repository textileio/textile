/* eslint-disable import/first */
// Some hackery to get WebSocket in the global namespace on nodejs
import { ThreadID } from '@textile/threads-id'
;(global as any).WebSocket = require('isomorphic-ws')
import { expect } from 'chai'
import { Client, Config } from '@textile/threads-client'
import { Context } from './context'
import { Buckets } from './buckets'
import { signUp } from './utils'
import { Client as HubClient } from './hub'
import { InitReply } from '@textile/buckets-grpc/buckets_pb'
import fs from 'fs'
import path from 'path'

// Settings for localhost development and testing
const addrApiurl = 'http://127.0.0.1:3007'
const addrGatewayUrl = 'http://127.0.0.1:8006'
const wrongError = new Error('wrong error!')
const sessionSecret = 'testing'

describe('Buckets...', () => {
  let ctx: Context = new Context(addrApiurl, undefined)
  const client = new Buckets(ctx)
  let buck: InitReply.AsObject
  before(async () => {
    const user = await signUp(new HubClient(ctx), addrGatewayUrl, sessionSecret)
    ctx = ctx.withSession(user.user.session).withThreadName('buckets')
    const id = ThreadID.fromRandom()
    const db = new Client(ctx)
    // @todo: Warning, this is a hack!
    ctx.host = client.serviceHost
    ;(db as any).config = ctx
    await db.newDB(id.toBytes())
    ctx = ctx.withThread(id)
  })

  it('should init a new bucket', async () => {
    // Check that we're empty
    const list = await client.list(ctx)
    expect(list).to.have.length(0)
    // Now initialize a bucket
    buck = await client.init('mybuck', ctx)
    expect(buck).to.have.ownProperty('root')
    expect(buck.root).to.have.ownProperty('key')
    expect(buck.root).to.have.ownProperty('path')
    expect(buck.root).to.have.ownProperty('createdat')
    expect(buck.root).to.have.ownProperty('updatedat')
  })

  it('should list buckets', async () => {
    const roots = await client.list(ctx)
    expect(roots).to.have.length(1)
    const root = roots[0]
    expect(root).to.have.ownProperty('key', buck.root?.key)
    expect(root).to.have.ownProperty('path', buck.root?.path)
    expect(root).to.have.ownProperty('createdat', buck.root?.createdat)
    expect(root).to.have.ownProperty('updatedat', buck.root?.updatedat)
  })

  it('should list bucket content at path', async () => {
    // Mostly empty
    const res = await client.listPath(buck.root?.key || '', '', ctx)
    expect(res).to.have.ownProperty('root')
    expect(res.root).to.not.be.undefined
    expect(res.item?.isdir).to.be.true
    // @todo: Should we rename itemsList to just be items?
    expect(res.item?.itemsList).to.have.length(0)
  })

  it('should push data from filesystem on node', async () => {
    const pth = path.join(__dirname, '..', 'testdata', 'file1.jpg')
    const stream = fs.createReadStream(pth)
    const res = await client.pushPath(buck.root?.key || '', stream, {}, ctx)
    const roots = await client.listPath(buck.root?.key || '', '', ctx)
    console.log(roots)
    const paths = await client.listPath(buck.root?.key || '', 'dir1/test.jpg', ctx)
    console.log(paths)
  })
})
