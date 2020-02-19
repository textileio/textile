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
  describe('Not in dev-mode, API should be on HTTPS on port 6647', () => {
    it('threadApiScheme should be HTTPS', async () => {
        api = new API({
            token: 'API_TOKEN',
            deviceId: 'DEVICE_ID',
            dev: false,
        })
        await api.start();
        var config = api.threadsConfig;
        expect(config.threadApiScheme).equals('https');
        client = new Client(api.threadsConfig);
        expect(client.config.host).contains('https');
      });
      it('threadPort should be 6647', async () => {
        api = new API({
          token: 'API_TOKEN',
          deviceId: 'DEVICE_ID',
          dev: false,
        });
        await api.start();
        var config = api.threadsConfig;
        expect(config.threadsPort).equals(6647);
        client = new Client(api.threadsConfig);
        expect(client.config.host).contains('6647');
    });
  });
});
