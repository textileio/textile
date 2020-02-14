# js-textile

[![Made by Textile](https://img.shields.io/badge/made%20by-Textile-informational.svg?style=popout-square)](https://textile.io)
[![Chat on Slack](https://img.shields.io/badge/slack-slack.textile.io-informational.svg?style=popout-square)](https://slack.textile.io)
[![Threads version](https://img.shields.io/badge/dynamic/json?style=popout-square&color=3527ff&label=Threads&prefix=v&query=%24.dependencies%5B%27%40textile%2Fthreads-client-grpc%27%5D.version&url=https%3A%2F%2Fraw.githubusercontent.com%2Ftextileio%2Fjs-textile%2Fmaster%2Fpackage-lock.json)](https://github.com/textileio/go-threads)
[![GitHub license](https://img.shields.io/github/license/textileio/js-textile.svg?style=popout-square)](./LICENSE)
[![Build status](https://img.shields.io/github/workflow/status/textileio/js-textile/lint_test/master.svg?style=popout-square)](https://github.com/textileio/js-textile/actions?query=branch%3Amaster)

> JS lib for interacting with Textile APIs

Go to [the docs](https://docs.textile.io/) for more about Textile.

Join us on our [public Slack channel](https://slack.textile.io/) for news, discussions, and status updates. [Check out our blog](https://medium.com/textileio) for the latest posts and announcements.

## Table of Contents

-   [Install](#install)
-   [Usage](#usage)
-   [Contributing](#contributing)
-   [Changelog](#changelog)
-   [License](#license)

## Install

`npm install @textile/textile`

## Usage

`@textile/textile` provides access to Textile APIs in apps based on a Project Token. For details on getting an app token, see [textileio/textile](https://github.com/textileio/textile) or join the [Textile Slack](https://slack.textile.io).

### Threads APIs

Textile provides remote threads APIs your app can use.

- token: a project token from your textile project
- deviceId: a unique ID (uuid) for this user in your app

```js
import {API} from '@textile/textile'
import {Client} from '@textile/threads-client'

const api = new API({
    token: '<project token>',
    deviceId: '<user id>'
})
await api.start()

const client = new Client(api.threadsConfig)
const newStore = client.newStore()
```

### Developing threads with local deamon

Requires you run the Threads daemon (`threadsd`) on localhost. See [instructions here](https://github.com/textileio/go-threads).

```js
import {API} from '@textile/textile'
import {Client} from '@textile/threads-client'

const api = new API({
    token: '<project token>',
    deviceId: '<user id>',
    dev: true
})
await api.start()

const client = new Client(api.threadsConfig)
const newStore = client.newStore()
```

## Contributing

This project is a work in progress. As such, there's a few things you can do right now to help out:

-   **Ask questions**! We'll try to help. Be sure to drop a note (on the above issue) if there is anything you'd like to work on and we'll update the issue to let others know. Also [get in touch](https://slack.textile.io) on Slack.
-   **Open issues**, [file issues](https://github.com/textileio/js-textile/issues), submit pull requests!
-   **Perform code reviews**. More eyes will help a) speed the project along b) ensure quality and c) reduce possible future bugs.
-   **Take a look at the code**. Contributions here that would be most helpful are **top-level comments** about how it should look based on your understanding. Again, the more eyes the better.
-   **Add tests**. There can never be enough tests.

Before you get started, be sure to read our [contributors guide](./CONTRIBUTING.md) and our [contributor covenant code of conduct](./CODE_OF_CONDUCT.md).

## Changelog

[Changelog is published to Releases.](https://github.com/textileio/js-textile/releases)

## License

[MIT](LICENSE)
