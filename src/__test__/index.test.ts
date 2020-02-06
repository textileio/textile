/* eslint-disable @typescript-eslint/no-non-null-assertion */
/* eslint-disable import/first */
// Some hackery to get WebSocket in the global namespace on nodejs
// @todo: Find a nicer way to do this...
;(global as any).WebSocket = require('isomorphic-ws')

import { expect } from 'chai'
import { API } from '../index'

describe('API', function() {
  let api: API
  describe('create new instance', () => {
    it('it should create a new API instance', async () => {
      api = new API({
        token: 'b8053ec8-456c-47eb-9ee9-99fef7ae812f',
        deviceId: '77b7b712-5ec4-42f2-b3c4-57b22f0cf057',
        dev: true,
        api: '127.0.0.1',
        apiScheme: 'http',
      })
      api.start()
      // api = new API({
      //   token: '<app token>',
      //   deviceId: '<user id>',
      //   dev: true,
      //   api: '127.0.0.1',
      //   apiScheme: 'http',
      // })
      expect(api).to.not.be.undefined
    })
    it('create new store', async () => {
      const store = await api.threadsClient.newStore()
      expect(store).to.not.be.undefined
    })
  })
})
