/* eslint-disable import/first */
;(global as any).WebSocket = require('isomorphic-ws')

import { ThreadID } from '@textile/threads-id'
import { grpc } from '@improbable-eng/grpc-web'
import { SignupReply } from '@textile/hub-grpc/hub_pb'
import { expect } from 'chai'
import Client from '@textile/threads-client'
import { Context } from './context'
import { Users } from './users'
import { signUp, createKey, createAPISig } from './utils'

// Settings for localhost development and testing
const addrApiurl = 'http://127.0.0.1:3007'
const addrGatewayUrl = 'http://127.0.0.1:8006'
const wrongError = new Error('wrong error!')
const sessionSecret = 'textilesession'

describe('Users...', () => {
  let ctx: Context = new Context(addrApiurl, undefined)
  const client = new Users(ctx)
  let dev: SignupReply.AsObject
  before(async () => {
    const { user } = await signUp(ctx, addrGatewayUrl, sessionSecret)
    if (user) dev = user
  })

  describe('getThread', () => {
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
      const key = await createKey(ctx.withSession(dev.session), 'ACCOUNT')
      ctx = ctx.withAPIKey(key.key)
      try {
        await client.getThread('foo', ctx)
        throw wrongError
      } catch (err) {
        expect(err).to.not.equal(wrongError)
        expect(err.code).to.equal(grpc.Code.Unauthenticated)
      }
      // Old key signature
      const sig = await createAPISig(key.secret, new Date(Date.now() - 1000 * 60))
      ctx = ctx.withAPISig(sig)
      try {
        await client.getThread('foo', ctx)
        throw wrongError
      } catch (err) {
        expect(err).to.not.equal(wrongError)
        expect(err.code).to.equal(grpc.Code.Unauthenticated)
      }
    })
    it('should handle account keys', async () => {
      const key = await createKey(ctx.withSession(dev.session), 'ACCOUNT')
      const sig = await createAPISig(key.secret) // Defaults to 1 minute from now
      ctx = ctx.withAPIKey(key.key).withAPISig(sig)
      // Not found
      try {
        await client.getThread('foo', ctx)
        throw wrongError
      } catch (err) {
        expect(err).to.not.equal(wrongError)
        expect(err.code).to.equal(grpc.Code.NotFound)
      }
      // All good
      ctx = ctx.withThreadName('foo')
      const id = ThreadID.fromRandom()
      const db = new Client(ctx)
      // // @todo: Warning, this is a hack!
      ;(db as any).config = ctx
      await db.newDB(id.toBytes())
      const res = await client.getThread('foo', ctx)
      expect(res.name).to.equal('foo')
    })

    it('should handle users keys', async () => {
      const key = await createKey(ctx.withSession(dev.session), 'USER')
      const sig = await createAPISig(key.secret) // Defaults to 1 minute from now
      ctx = ctx.withAPIKey(key.key).withAPISig(sig)
      // No token
      try {
        await client.getThread('foo', ctx)
        throw wrongError
      } catch (err) {
        expect(err).to.not.equal(wrongError)
        expect(err.code).to.equal(grpc.Code.Unauthenticated)
      }
      // Not found

      try {
        await client.getThread('foo', ctx)
        throw wrongError
      } catch (err) {
        expect(err).to.not.equal(wrongError)
        expect(err.code).to.equal(grpc.Code.NotFound)
      }
      // All good
      ctx = ctx.withThreadName('foo')
      const id = ThreadID.fromRandom()
      const db = new Client(ctx)
      // // @todo: Warning, this is a hack!
      ;(db as any).config = ctx
      await db.newDB(id.toBytes())
      const res = await client.getThread('foo', ctx)
      expect(res.name).to.equal('foo')
    })
  })
})
