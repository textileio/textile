/* eslint-disable import/first */
import { grpc } from '@improbable-eng/grpc-web'
;(global as any).WebSocket = require('isomorphic-ws')
// Some hackery to get WebSocket in the global namespace on nodejs
import axios from 'axios'
import { expect } from 'chai'
import { SignupReply } from '@textile/hub/hub_pb'
import { Client } from './hub'
import { Client as ThreadsClient, Config } from '@textile/threads-client'
import { createUsername, createEmail, confirmEmail, signIn, signUp } from './utils'
import { Credentials } from './credentials'
import { ThreadID } from '@textile/threads-id'

// Settings for localhost development and testing
const addrApiurl = 'http://127.0.0.1:3007'
const addrGatewayUrl = 'http://127.0.0.1:8006'
const wrongError = new Error('wrong error!')
const sessionSecret = 'testing'

describe('Hub Client...', () => {
  let client: Client
  before(async () => {
    const config = new Credentials().withHost(addrApiurl)
    client = new Client(config)
  })
  describe('Account creation...', () => {
    const username = createUsername()
    const email = createEmail()
    let user: SignupReply.AsObject
    it('should sign up', (done) => {
      client
        .signUp(username, email)
        .then((u) => {
          user = u
          expect(user.key).to.not.be.undefined
          expect(user.session).to.not.be.undefined
          done()
        })
        .catch(done)
      confirmEmail(addrGatewayUrl, sessionSecret)
    })

    it('should sign out', async () => {
      try {
        // Without session
        await client.signOut()
        throw wrongError
      } catch (err) {
        expect(err).to.not.equal(wrongError)
      }
      // With session
      const creds = new Credentials().withSession(user.session)
      const res = await client.signOut(creds)
      expect(res).to.be.undefined
    })

    it('should sign in', async () => {
      let counts = 0
      // Sign in first (previous test signed out)
      user = await signIn(client, username, addrGatewayUrl, sessionSecret)
      expect(user.key).to.not.be.undefined
      expect(user.session).to.not.be.undefined
      const creds = new Credentials().withSession(user.session)
      // Sign back out before sign in again
      await client.signOut(creds)
      user = await signIn(client, username, addrGatewayUrl, sessionSecret)
      expect(user.key).to.not.be.undefined
      expect(user.session).to.not.be.undefined
    })
    it('should get session info', async () => {
      try {
        // Without session
        await client.getSessionInfo()
        throw wrongError
      } catch (err) {
        expect(err).to.not.equal(wrongError)
      }
      // With session
      const creds = new Credentials().withSession(user.session)
      const res = await client.getSessionInfo(creds)
      expect(res.key).to.equal(user.key)
      expect(res.username).to.equal(username)
      expect(res.email).to.equal(email)
    })
  })
  describe('Thread access...', () => {
    let user: SignupReply.AsObject

    beforeEach(async () => {
      user = (await signUp(client, addrGatewayUrl, sessionSecret)).user
    })

    context('Threads...', () => {
      it('should get thread', async () => {
        try {
          // Without session
          await client.getThread('foo')
          throw wrongError
        } catch (err) {
          expect(err).to.not.equal(wrongError)
        }
        // With session
        const creds = new Credentials().withSession(user.session).withThreadName('foo').withHost(addrApiurl)

        try {
          // Without existing
          await client.getThread('foo', creds)
          throw wrongError
        } catch (err) {
          expect(err).to.not.equal(wrongError)
        }
        // With existing
        const threads = new ThreadsClient(creds)
        // @todo: Warning, this is a hack!
        ;(threads as any).config = creds
        await threads.newDB(ThreadID.fromRandom().toBytes())

        const res = await client.getThread('foo', creds)
        expect(res).to.not.be.undefined
        expect(res).to.have.ownProperty('name', 'foo')
      })

      it('should list threads', async () => {
        try {
          // Without session
          await client.listThreads()
          throw wrongError
        } catch (err) {
          expect(err).to.not.equal(wrongError)
        }
        // With session
        const creds = new Credentials().withSession(user.session).withHost(addrApiurl)

        // Without existing
        let list = await client.listThreads(creds)
        expect(list).to.have.length(0)

        // With existing
        const threads = new ThreadsClient(creds)
        // @todo: Warning, this is a hack!
        ;(threads as any).config = creds
        await threads.newDB(ThreadID.fromRandom().toBytes())
        await threads.newDB(ThreadID.fromRandom().toBytes())

        list = await client.listThreads(creds)
        expect(list).to.have.length(2)
      })
    })

    context('Keys...', () => {
      it('should create keys', async () => {
        try {
          // Without session
          await client.createKey()
          throw wrongError
        } catch (err) {
          expect(err).to.not.equal(wrongError)
        }
        // With session
        const creds = new Credentials().withSession(user.session).withHost(addrApiurl)
        const key = await client.createKey(creds)
        expect(key).to.have.ownProperty('key')
        expect(key).to.have.ownProperty('secret')
      })

      it('should invalidate keys', async () => {
        const creds = new Credentials().withSession(user.session).withHost(addrApiurl)
        const key = await client.createKey(creds)
        try {
          // Without session
          await client.invalidateKey(key.key)
          throw wrongError
        } catch (err) {
          expect(err).to.not.equal(wrongError)
        }
        // With session
        await client.invalidateKey(key.key, creds)
        const list = await client.listKeys(creds)
        expect(list).to.have.length(1)
        expect(list[0]).to.have.ownProperty('valid', false)
      })

      it('should list keys', async () => {
        const creds = new Credentials().withSession(user.session).withHost(addrApiurl)
        // Empty
        let list = await client.listKeys(creds)
        expect(list).to.have.length(0)

        await client.createKey(creds)
        await client.createKey(creds)

        // Not empty
        list = await client.listKeys(creds)
        expect(list).to.have.length(2)
      })
    })

    context('Orgs...', () => {
      it('should create and org', async () => {
        const name = createUsername()
        try {
          // Without session
          await client.createOrg(name)
          throw wrongError
        } catch (err) {
          expect(err).to.not.equal(wrongError)
        }
        // With session
        const creds = new Credentials().withSession(user.session).withHost(addrApiurl)
        const key = await client.createOrg(name, creds)
        expect(key).to.have.ownProperty('key')
        expect(key).to.have.ownProperty('name')
      })

      it('should get org', async () => {
        const name = createUsername()
        const creds = new Credentials().withSession(user.session).withHost(addrApiurl)
        const org = await client.createOrg(name, creds)
        try {
          // Bad org
          await client.getOrg(creds.withOrg('bad'))
          throw wrongError
        } catch (err) {
          expect(err).to.not.equal(wrongError)
        }
        // Good org
        const got = await client.getOrg(creds.withOrg(org.name))
        expect(got.key).to.equal(org.key)
      })

      it('should list orgs', async () => {
        const creds = new Credentials().withSession(user.session).withHost(addrApiurl)
        // Empty
        let list = await client.listOrgs(creds)
        expect(list).to.have.length(0)

        await client.createOrg('My Org 1', creds)
        await client.createOrg('My Org 2', creds)

        // Not empty
        list = await client.listOrgs(creds)
        expect(list).to.have.length(2)
      })

      it('should remove orgs', async () => {
        const name = createUsername()
        const creds = new Credentials().withSession(user.session).withHost(addrApiurl)
        const org = await client.createOrg(name, creds)
        try {
          // Bad org
          await client.removeOrg(creds.withOrg('bad'))
          throw wrongError
        } catch (err) {
          expect(err).to.not.equal(wrongError)
        }

        try {
          // Without session
          const creds = new Credentials().withHost(addrApiurl).withOrg(org.name)
          await client.removeOrg(creds)
          throw wrongError
        } catch (err) {
          expect(err).to.not.equal(wrongError)
        }
        // Good org
        await client.removeOrg(creds.withOrg(org.name))
        try {
          await client.getOrg(creds.withOrg(org.name))
          throw wrongError
        } catch (err) {
          expect(err).to.not.equal(wrongError)
        }
      })

      it('should invite to org', async () => {
        const name = createUsername()
        const creds = new Credentials().withSession(user.session).withHost(addrApiurl)
        const org = await client.createOrg(name, creds)
        try {
          // Bad email
          await client.inviteToOrg('jane', creds.withOrg(name))
          throw wrongError
        } catch (err) {
          expect(err).to.not.equal(wrongError)
        }
        // Good email
        const invite = await client.inviteToOrg(createEmail(), creds.withOrg(name))
        expect(invite).to.have.ownProperty('token')
        const res = await axios.get(`${addrGatewayUrl}/consent/${invite.token}`)
        expect(res.status).to.equal(200)
      })

      it('should leave an org', async () => {
        const name = createUsername()
        const creds = new Credentials().withSession(user.session).withHost(addrApiurl)
        const org = await client.createOrg(name, creds)
        try {
          // As owner
          await client.leaveOrg(creds.withOrg(name))
          throw wrongError
        } catch (err) {
          expect(err).to.not.equal(wrongError)
        }
        const user2 = await signUp(client, addrGatewayUrl, sessionSecret)
        const creds2 = new Credentials().withSession(user2.user.session).withHost(addrApiurl)
        try {
          // As non-member
          await client.leaveOrg(creds2.withOrg(org.name))
          throw wrongError
        } catch (err) {
          expect(err).to.not.equal(wrongError)
        }

        const invite = await client.inviteToOrg(user2.email, creds.withOrg(org.name))
        await axios.get(`${addrGatewayUrl}/consent/${invite.token}`)
        // As member
        // @todo: Figure out why this throws a 'Email address is not valid' Error?
        // const res = await client.leaveOrg(creds2.withOrg(org.name))
        // expect(res).to.be.undefined
      })
    })
    describe('Utils...', () => {
      it('should check that a user is available', async () => {
        const username = createUsername()
        const available = await client.isUsernameAvailable(username)
        expect(available).to.be.true
        const user = await signUp(client, addrGatewayUrl, sessionSecret)
        try {
          await client.isUsernameAvailable(user.username)
          throw wrongError
        } catch (err) {
          expect(err).to.not.equal(wrongError)
        }
      })

      it('should check that an org name is available', async () => {
        const user = await signUp(client, addrGatewayUrl, sessionSecret)
        const creds = new Credentials().withSession(user.user.session).withHost(addrApiurl)

        const name = 'My awesome org!'
        const res = await client.isOrgNameAvailable(name, creds)
        expect(res).to.have.ownProperty('slug', 'My-awesome-org')

        const org = await client.createOrg(name, creds)
        expect(org.slug).to.equal(res.slug)

        try {
          await client.isOrgNameAvailable(name, creds)
          throw wrongError
        } catch (err) {
          expect(err).to.not.equal(wrongError)
        }
      })
    })
  })
})
