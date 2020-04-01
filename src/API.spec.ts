/* eslint-disable @typescript-eslint/no-non-null-assertion */
/* eslint-disable import/first */
// Some hackery to get WebSocket in the global namespace on nodejs
// @todo: Find a nicer way to do this...
;(global as any).WebSocket = require('isomorphic-ws')

import { Client } from '@textile/threads-client'
import { expect } from 'chai'
import { API } from './API'

describe('API', function() {
  let api: API
  let client: Client
  describe('create new instance', () => {
    it('it should create a new API instance', async () => {
      api = new API({
        token: 'API_TOKEN',
        deviceId: 'DEVICE_ID',
        dev: true,
      })
      await api.start()
      expect(api).to.not.be.undefined
    })
    it('it should create a new Client instance', async () => {
      client = new Client(api.threadsConfig)
      expect(client).to.not.be.undefined
    })
    it('create new store', async () => {
      const store = await client.newStore()
      expect(store).to.not.be.undefined
    })
  })
})
