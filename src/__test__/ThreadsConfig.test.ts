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
  describe('In dev-mode, API should be on HTTP on port 6007', () => {
    it('threadApiScheme should be HTTP', async () => {
        api = new API({
            token: 'API_TOKEN',
            deviceId: 'DEVICE_ID',
            dev: true,
        })
        await api.start();
        var config = api.threadsConfig;
        expect(config.threadApiScheme).equals('http');
        client = new Client(api.threadsConfig);
        expect(client.config.host).contains('http');
      });
    it('threadPort should be 6007', async () => {
        api = new API({
          token: 'API_TOKEN',
          deviceId: 'DEVICE_ID',
          dev: true,
        });
        await api.start();
        var config = api.threadsConfig;
        expect(config.threadsPort).equals(6007);
        client = new Client(api.threadsConfig);
        expect(client.config.host).contains('6007');
    });
  });
  describe('Not in dev-mode, API should be on HTTPS on port 6447', () => {
    it('threadApiScheme should be HTTPS', async () => {
        api = new API({
            token: '54e24fc3-fda5-478a-b1f7-040ea5aaab33',
            deviceId: 'c8e5b368-6d08-4215-81b3-eb2522e2df32',
            dev: false,
        })
        await api.start();
        var config = api.threadsConfig;
        client = new Client(config);
        expect(config.threadApiScheme).equals('https');
        expect(client.config.host).contains('https');
      });
    it('threadPort should be 6447', async () => {
        api = new API({
          token: '54e24fc3-fda5-478a-b1f7-040ea5aaab33',
          deviceId: 'c8e5b368-6d08-4215-81b3-eb2522e2df32',
          dev: false,
        });
        await api.start();
        var config = api.threadsConfig;
        client = new Client(config);
        expect(config.threadsPort).equals(6447);
        expect(client.config.host).contains('6447');
    });
  });
});
