# textile

[![Made by Textile](https://img.shields.io/badge/made%20by-Textile-informational.svg?style=popout-square)](https://textile.io)
[![Chat on Slack](https://img.shields.io/badge/slack-slack.textile.io-informational.svg?style=popout-square)](https://slack.textile.io)
[![GitHub license](https://img.shields.io/github/license/textileio/textile.svg?style=popout-square)](./LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/textileio/textile?style=flat-square)](https://goreportcard.com/report/github.com/textileio/textile?style=flat-square)
[![GitHub action](https://github.com/textileio/textile/workflows/Tests/badge.svg?style=popout-square)](https://github.com/textileio/textile/actions)
[![standard-readme compliant](https://img.shields.io/badge/readme%20style-standard-brightgreen.svg?style=popout-square)](https://github.com/RichardLitt/standard-readme)

> Textile services and buckets lib written in Go

Textile connects and extends [Libp2p](https://libp2p.io/), [IPFS](https://ipfs.io/), and [Filecoin](https://filecoin.io/). Three interoperable technologies makeup Textile:
-   [**ThreadDB**](https://github.com/textileio/go-threads): A serverless p2p database built on Libp2p
-   [**Powergate**](https://github.com/textileio/powergate): File storage built on Filecoin and IPFS
-   [**Buckets**](https://github.com/textileio/textile/tree/master/buckets): File and dynamic directory storage built on ThreadDB, Powergate, and [UnixFS](https://github.com/ipfs/go-unixfs).

Join us on our [public Slack channel](https://slack.textile.io/) for news, discussions, and status updates. [Check out our blog](https://medium.com/textileio) for the latest posts and announcements.

## Table of Contents

-   [Security](#security)
-   [Background](#background) 
-   [Install](#install)
-   [Getting Started](#getting-started)
-   [Contributing](#contributing)
-   [Developing](#developing)
-   [Changelog](#changelog)
-   [License](#license)

## Security

Textile is still under heavy development and no part of it should be used before a thorough review of the underlying code and an understanding APIs and protocols may change rapidly. There may be coding mistakes, and the underlying protocols may contain design flaws. Please [let us know](mailto:contact@textile.io) immediately if you have discovered a security vulnerability.

Please also read the [security note](https://github.com/ipfs/go-ipfs#security-issues) for [go-ipfs](https://github.com/ipfs/go-ipfs).

## Background

Go to [the docs](https://docs.textile.io/) for more about Textile.

## Install

This repo contains two service daemons with CLIs and a Bucket library for building local-first apps and services in Go.

### Buckets daemon: `buckd`

Much like [`threadsd`](https://github.com/textileio/go-threads/tree/master/threadsd), the `buckd` daemon can be run as a server or alongside desktop apps or command-line tools. 

-   **Prebuilt package**: See [release assets](https://github.com/textileio/textile/releases/latest)
-   **Docker image**: See the `buckets` tag on [Docker Hub](https://hub.docker.com/r/textile/textile/tags)
-   **Build from the source**: 

```
go get github.com/textileio/textile
go install github.com/textileio/textile/cmd/buckd
```

For usage, try `buckd --help`. 

### Buckets CLI: `buck`

This CLI is a front end to `buckd`. For usage, try `buck --help`.

`buck` is built in part on the gRPC client, which can be installed in an existing Go project with `go get`.

```
go get github.com/textileio/textile/api/buckets/client
```

### Hub deamon: `hubd`

The `hubd` daemon is a hosted portal to ThreadDB, Powergate, and Buckets that includes developer accounts for individuals and organizations. The official Textile Hub can be found at https://hub.textile.io.

-   **Prebuilt package**: See [release assets](https://github.com/textileio/textile/releases/latest)
-   **Docker image**: See the `latest` tag on [Docker Hub](https://hub.docker.com/r/textile/textile/tags)
-   **Build from the source**:

```
go get github.com/textileio/textile
go install github.com/textileio/textile/cmd/hubd
```

For usage, try `hubd --help`.

### Hub CLI: `hub`

This CLI is a front end to `hubd`. For usage, try `hub --help`.

Note: `hub` _includes_ the `buck` CLI as a subcommand: `hub buck`. This is because `hubd` hosts `buckd`, along with other services.

`hub` is built in part on the gRPC client, which can be installed in an existing Go project with `go get`.

```
go get github.com/textileio/textile/api/hub/client
```

### Local Buckets lib 

The `buckets/local` library powers both the `buck` and `hub buck` commands. Use `go get` to install in an existing Go project.

```
go get github.com/textileio/textile/buckets/local
```

## Getting Started

### Creating an account on The Hub
### Adding files and folders to a bucket
### Local-first bucket syncing

## Developing

## Contributing

Pull requests and bug reports are very welcome ❤️

This repository falls under the Textile [Code of Conduct](./CODE_OF_CONDUCT.md).

Feel free to get in touch by:
-   [Opening an issue](https://github.com/textileio/textile/issues/new)
-   Joining the [public Slack channel](https://slack.textile.io/)
-   Sending an email to contact@textile.io

## Changelog

A changelog is published along with each [release](https://github.com/textileio/textile/releases).

## License

[MIT](LICENSE)
