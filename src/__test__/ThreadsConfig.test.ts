/* eslint-disable @typescript-eslint/no-non-null-assertion */
/* eslint-disable import/first */
// Some hackery to get WebSocket in the global namespace on nodejs
// @todo: Find a nicer way to do this...
;(global as any).WebSocket = require('isomorphic-ws')

import { Client } from '@textile/threads-client'
import { expect } from 'chai'
import { API } from '../index'

describe('ThreadsConfig', function() {
  let api: API;
  let client: Client;
  describe('When not in dev-mode, API should be HTTPS on port 6647', () => {
    it('threadApiScheme should be HTTPS', async () => {
        api = new API({
            token: 'API_TOKEN',
            deviceId: 'DEVICE_ID',
            dev: true,
        })
        await api.start();
        const config = api.threadsConfig;
        if (!config.dev) {
          expect(config.threadApiScheme).equals('https');
          client = new Client(api.threadsConfig);
          expect(client.config.host).contains('https');
        }
    });
    it('threadsPort should be 6647', async () => {
        const api = new API({
            token: 'API_TOKEN',
            deviceId: 'DEVICE_ID',
            dev: true,
        });
        await api.start();
        const config = api.threadsConfig;
        if (!config.dev) {
          expect(config.threadsPort).equals(6647);
          client = new Client(api.threadsConfig);
          expect(client.config.host).contains('6647');
        }
    });
  });
});
