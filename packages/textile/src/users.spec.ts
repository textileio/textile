/* eslint-disable import/first */
;(global as any).WebSocket = require('isomorphic-ws')

import path from 'path'
import fs from 'fs'
import { ThreadID } from '@textile/threads-id'
import { grpc } from '@improbable-eng/grpc-web'
import { SignupReply } from '@textile/hub-grpc/hub_pb'
import { expect } from 'chai'
import { Libp2pCryptoIdentity } from '@textile/threads-core'
import { Context } from '@textile/context'
import { isBrowser } from 'browser-or-node'
import { signUp, createKey, createAPISig } from './utils'
import { Buckets } from './buckets'
import { Client } from './users'

// Settings for localhost development and testing
const addrApiurl = 'http://127.0.0.1:3007'
const addrGatewayUrl = 'http://127.0.0.1:8006'
const wrongError = new Error('wrong error!')
const sessionSecret = 'textilesession'

describe('Users...', () => {
  describe('getThread', () => {
    const ctx: Context = new Context(addrApiurl, undefined)
    const client = new Client(ctx)
    let dev: SignupReply.AsObject
    before(async function () {
      this.timeout(3000)
      const { user } = await signUp(ctx, addrGatewayUrl, sessionSecret)
      if (user) dev = user
    })
    it('should handle bad keys', async () => {
      // No key
      try {
        await client.getThread('foo', ctx)
        throw wrongError
      } catch (err) {
        expect(err).to.not.equal(wrongError)
        expect(err.code).to.equal(grpc.Code.Unauthenticated)
      }
      // No key signature
      const tmp = new Context(addrApiurl).withSession(dev.session)
      const key = await createKey(tmp, 'ACCOUNT')
      try {
        await client.getThread('foo', ctx.withAPIKey(key.key))
        throw wrongError
      } catch (err) {
        expect(err).to.not.equal(wrongError)
        expect(err.code).to.equal(grpc.Code.Unauthenticated)
      }
      // Old key signature
      const sig = await createAPISig(key.secret, new Date(Date.now() - 1000 * 60))
      try {
        await client.getThread('foo', ctx.withAPISig(sig))
        throw wrongError
      } catch (err) {
        expect(err).to.not.equal(wrongError)
        expect(err.code).to.equal(grpc.Code.Unauthenticated)
      }
    })
    it('should handle account keys', async () => {
      const key = await createKey(ctx.withSession(dev.session), 'ACCOUNT')
      await ctx.withAPIKey(key.key).withUserKey(key)
      // Not found
      try {
        await client.getThread('foo')
        throw wrongError
      } catch (err) {
        expect(err).to.not.equal(wrongError)
        expect(err.code).to.equal(grpc.Code.NotFound)
      }
      // All good
      const id = ThreadID.fromRandom()
      const db = new Client(ctx)
      await db.newDB(id, ctx.withThreadName('foo'))
      const res = await client.getThread('foo')
      expect(res.name).to.equal('foo')
    })

    it('should handle users keys', async () => {
      // Reset client context (just for the tests)
      const ctx = new Context(addrApiurl)
      client.context = ctx
      const tmp = new Context(addrApiurl).withSession(dev.session)
      const key = await createKey(tmp, 'USER')
      await ctx.withAPIKey(key.key).withUserKey(key)
      // No token
      try {
        await client.getThread('foo')
        throw wrongError
      } catch (err) {
        expect(err).to.not.equal(wrongError)
        expect(err.code).to.equal(grpc.Code.Unauthenticated)
      }
      // Not found
      const db = new Client(ctx)
      const identity = await Libp2pCryptoIdentity.fromRandom()
      await db.getToken(identity)
      try {
        await client.getThread('foo')
        throw wrongError
      } catch (err) {
        expect(err).to.not.equal(wrongError)
        expect(err.code).to.equal(grpc.Code.NotFound)
      }
      // All good
      const id = ThreadID.fromRandom()
      await db.newDB(id, ctx.withThreadName('foo'))
      const res = await client.getThread('foo')
      expect(res.name).to.equal('foo')
    })
  })

  describe('listThreads', () => {
    const ctx: Context = new Context(addrApiurl, undefined)
    const client = new Client(ctx)
    let dev: SignupReply.AsObject
    before(async function () {
      this.timeout(3000)
      const { user } = await signUp(ctx, addrGatewayUrl, sessionSecret)
      if (user) dev = user
    })
    it('should handle bad keys', async () => {
      // No key
      try {
        await client.listThreads()
        throw wrongError
      } catch (err) {
        expect(err).to.not.equal(wrongError)
        expect(err.code).to.equal(grpc.Code.Unauthenticated)
      }
      // No key signature
      const tmp = new Context(addrApiurl).withSession(dev.session)
      const key = await createKey(tmp, 'ACCOUNT')
      try {
        await client.listThreads(ctx.withAPIKey(key.key))
        throw wrongError
      } catch (err) {
        expect(err).to.not.equal(wrongError)
        expect(err.code).to.equal(grpc.Code.Unauthenticated)
      }
      // Old key signature
      const sig = await createAPISig(key.secret, new Date(Date.now() - 1000 * 60))
      try {
        await client.listThreads(ctx.withAPISig(sig))
        throw wrongError
      } catch (err) {
        expect(err).to.not.equal(wrongError)
        expect(err.code).to.equal(grpc.Code.Unauthenticated)
      }
    })
    it('should handle account keys', async () => {
      const key = await createKey(ctx.withSession(dev.session), 'ACCOUNT')
      await ctx.withAPIKey(key.key).withUserKey(key)
      // Empty
      let res = await client.listThreads()
      expect(res.listList).to.have.length(0)
      // Got one
      const id = ThreadID.fromRandom()
      const db = new Client(ctx)
      await db.newDB(id)
      res = await client.listThreads()
      expect(res.listList).to.have.length(1)
    })

    it('should handle users keys', async () => {
      // Reset client context (just for the tests)
      const ctx = new Context(addrApiurl)
      client.context = ctx
      const tmp = new Context(addrApiurl).withSession(dev.session)
      const key = await createKey(tmp, 'USER')
      await ctx.withAPIKey(key.key).withUserKey(key)
      // No token
      try {
        await client.listThreads()
        throw wrongError
      } catch (err) {
        expect(err).to.not.equal(wrongError)
        expect(err.code).to.equal(grpc.Code.Unauthenticated)
      }
      // Empty
      const db = new Client(ctx)
      const identity = await Libp2pCryptoIdentity.fromRandom()
      await db.getToken(identity)
      let res = await client.listThreads()
      expect(res.listList).to.have.length(0)
      // Got one
      const id = ThreadID.fromRandom()
      await db.newDB(id, ctx.withThreadName('foo'))
      res = await client.listThreads()
      expect(res.listList).to.have.length(1)
      expect(res.listList[0].name).to.equal('foo')
    })
  })

  describe('Buckets and accounts', () => {
    context('a developer', () => {
      const ctx: Context = new Context(addrApiurl, undefined)
      let dev: SignupReply.AsObject
      it('should sign-up, create an API key, and sign it for the requests', async () => {
        // @note This should be done using the cli
        const { user } = await signUp(ctx, addrGatewayUrl, sessionSecret)
        if (user) dev = user
        // @note This should be done using the cli
        ctx.withSession(dev.session)
        const key = await createKey(ctx, 'ACCOUNT')
        await ctx.withAPIKey(key.key).withUserKey(key)
        expect(ctx.toJSON()).to.have.ownProperty('x-textile-api-sig')
      }).timeout(3000)
      it('should then create a db for the bucket', async () => {
        const db = new Client(ctx)
        const id = ThreadID.fromRandom()
        await db.newDB(id, ctx.withThreadName('my-buckets'))
        expect(ctx.toJSON()).to.have.ownProperty('x-textile-thread-name')
      })
      it('should then initialize a new bucket in the db and push to it', async function () {
        if (isBrowser) return this.skip()
        // Initialize a new bucket in the db
        const buckets = new Buckets(ctx)
        const buck = await buckets.init('mybuck')
        expect(buck.root?.name).to.equal('mybuck')

        // Finally, push a file to the bucket.
        const pth = path.join(__dirname, '../../..', 'testdata')
        const stream = fs.createReadStream(path.join(pth, 'file1.jpg'))
        const rootKey = buck.root?.key || ''
        const { root } = await buckets.pushPath(rootKey, 'dir1/file1.jpg', stream)
        expect(root).to.not.be.undefined

        // We should have a thread named "my-buckets"
        const users = new Client(ctx)
        const res = await users.getThread('my-buckets')
        expect(res.id).to.deep.equal(ctx.toJSON()['x-textile-thread'])
      })
    })
    context('a developer with a user', function () {
      const ctx: Context = new Context(addrApiurl, undefined)
      let dev: SignupReply.AsObject
      let users: Client
      it('should sign-up, create an API key, and a new user', async function () {
        // @note This should be done using the cli
        const { user } = await signUp(ctx, addrGatewayUrl, sessionSecret)
        if (user) dev = user
        // @note This should be done using the cli
        // This time they create a user key
        const key = await createKey(ctx.withSession(dev.session), 'USER')

        // This should automatically generate a user identity and validate keys, though we use a random ident
        // for demo purposes here to show that it can also use custom identities
        const identity = await Libp2pCryptoIdentity.fromRandom()
        // We also explicitly specify a custom context here, which could be omitted as it uses reasonable defaults
        const userContext = await new Context(addrApiurl).withUserKey(key)
        // In the app, we simply create a new user from the provided user key, signing is done automatically
        users = new Client(userContext)
        await users.getToken(identity)
        expect(users.context.toJSON()).to.have.ownProperty('x-textile-api-sig')
      }).timeout(3000)

      it('should then create a db for the bucket', async function () {
        await users.newDB(ThreadID.fromRandom(), users.context.withThreadName('my-buckets'))
        expect(users.context.toJSON()).to.have.ownProperty('x-textile-thread-name')
      })

      it('should then initialize a new bucket in the db and push to it', async function () {
        if (isBrowser) return this.skip()
        // Initialize a new bucket in the db from the user context
        const buckets = new Buckets(users.context)
        const buck = await buckets.init('mybuck')
        expect(buck.root?.name).to.equal('mybuck')

        // Finally, push a file to the bucket.
        const pth = path.join(__dirname, '../../..', 'testdata')
        const stream = fs.createReadStream(path.join(pth, 'file1.jpg'))
        const rootKey = buck.root?.key || ''
        const { root } = await buckets.pushPath(rootKey, 'dir1/file1.jpg', stream)
        expect(root).to.not.be.undefined

        // We should have a thread named "my-buckets"
        const res = await users.getThread('my-buckets')
        expect(res.id).to.deep.equal(users.context.toJSON()['x-textile-thread'])

        // The dev should see that the key was used to create one thread
        // @todo: Use the cli to list keys
      })
    })
  })
})
