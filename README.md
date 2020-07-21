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

Go to [the docs](https://docs.textile.io/) for more about the motivations behind Textile.

## Install

This repo contains two service daemons with CLIs and a Buckets Library for building local-first apps and services.

### The Hub

#### `hubd`

-   **Prebuilt package**: See [release assets](https://github.com/textileio/textile/releases/latest)
-   **Docker image**: See the `latest` tag on [Docker Hub](https://hub.docker.com/r/textile/textile/tags)
-   **Build from the source**:

```
go get github.com/textileio/textile
go install github.com/textileio/textile/cmd/hubd
```

#### `hub`

-   **Prebuilt package**: See [release assets](https://github.com/textileio/textile/releases/latest)
-   **Build from the source**: 

```
go get github.com/textileio/textile
go install github.com/textileio/textile/cmd/hub
```

**Note**: `hub` _includes_ `buck` as a subcommand: `hub buck`. This is because `hubd` hosts `buckd`, along with other services.

`hub` is built in part on the gRPC client, which can be installed in an existing project with `go get`.

```
go get github.com/textileio/textile/api/hub/client
```

### Buckets

#### `buckd`

-   **Prebuilt package**: See [release assets](https://github.com/textileio/textile/releases/latest)
-   **Docker image**: See the `buckets` tag on [Docker Hub](https://hub.docker.com/r/textile/textile/tags)
-   **Build from the source**: 

```
go get github.com/textileio/textile
go install github.com/textileio/textile/cmd/buckd
```

#### `buck`

-   **Prebuilt package**: See [release assets](https://github.com/textileio/textile/releases/latest)
-   **Build from the source**: 

```
go get github.com/textileio/textile
go install github.com/textileio/textile/cmd/buck
```

`buck` is built in part on the gRPC client, which can be used in an existing project with `go get`.

```
go get github.com/textileio/textile/api/buckets/client
```

### The Buckets Library

```
go get github.com/textileio/textile/buckets/local
```

## Getting Started

### The Hub

The Hub daemon (`hubd`), a.k.a. _The Hub_, is a hosted wrapper around other Textile services that includes developer accounts for individuals and organizations. You are free to run your own, but we encourage the use of the official [Textile Hub](https://docs.textile.io/hub/).

The layout of the `hub` client CLI mirrors the services wrapped by `hubd`:

-   `hub threads` provides limited access to ThreadDB.
-   `hub buck` provides access to Buckets by wrapping the standalone `buck` CLI.
-   `hub buck archive` provides limited access to The Hub's hosted Powergate instance, and the Filecoin network.

Try `hub --help` for more usage.

```
The Hub Client.

Usage:
  hub [command]

Available Commands:
  buck        Manage an object storage bucket
  destroy     Destroy your account
  help        Help about any command
  init        Initialize account
  keys        API key management
  login       Login
  logout      Logout
  orgs        Org management
  threads     Thread management
  whoami      Show current user

Flags:
      --api string       API target (default "api.textile.io:443")
  -h, --help             help for hub
  -o, --org string       Org username
  -s, --session string   User session token

Use "hub [command] --help" for more information about a command.
```

Read more about The Hub, including how to [create an account](https://docs.textile.io/hub/accounts/#account-setup), in the [docs](https://docs.textile.io/hub/).

### Running Buckets

Much like [`threadsd`](https://github.com/textileio/go-threads/tree/master/threadsd), the `buckd` daemon can be run as a server or alongside desktop apps or command-line tools. The easiest way to run `buckd` is by using the provided Docker Compose files. If you're new to Docker and/or Docker Compose, get started [here](https://docs.docker.com/compose/gettingstarted/). Once you're all setup, you should have `docker-compose` in your `PATH`.

Create an `.env` file and add the following values:  

```
REPO_PATH=~/myrepo
BUCK_LOG_DEBUG=true
```

Copy [this compose file](https://github.com/textileio/textile/blob/master/cmd/buckd/docker-compose.yml) and run it with the following command.

```
docker-compose -f docker-compose.yml up 
```

Congrats! Now you have Buckets running locally.

Notice that the compose file also starts an IPFS node. You could point `buckd` to a different (possibly remote) IPFS node by setting the `BUCK_ADDR_IPFS_API` variable to a different multiaddress.  

By default, this approach does not start [Powergate](https://github.com/textileio/powergate). If you do, be sure to set the `BUCK_ADDR_POWERGATE_API` variable to the multiaddress of your Powergate. Buckets must be configured with Powergate to enable Filecoin archiving with `buck archive`.

### Creating a bucket

Since `hub buck` and `buck` are functionally identical, this section will focus on `buck` and the Buckets Library using a locally running `buckd`.

Textile Buckets function a bit like S3 buckets and a bit like `git`. To get started, initialize a new bucket.

```
mkdir mybucket && cd mybucket
buck init
```

When prompted, give your new bucket a name. You'll then be asked if you want the contents encrypted.

Bucket encryption (AES-CTR + AES-512 HMAC) happens entirely within the Buckets daemon, meaning that your data is encrypted on the way in, and decrypted on the way out. This type of encryption has two goals:

- Obfuscate bucket data / files (the normal goal of encryption)
- Obfuscate directory structure, which amounts to encrypting [IPLD](https://ipld.io/) nodes and their links.

As a result of these goals, encrypted buckets are referred to as _private buckets_. Read more about bucket encryption [here](https://docs.textile.io/buckets/#encryption).

You should now see two links for the new bucket on the locally running gateway.

```
> http://127.0.0.1:8006/thread/bafkq3ocmdkrljadlgybtvocytpdw4hbnzygxecxehdp7pfj32lxp34a/buckets/bafzbeifyzfm3kosie25s5qthvvcjrr42ivd7doqhwvu5m4ks7uqv4j5lyi Thread link
> http://127.0.0.1:8006/ipns/bafzbeifyzfm3kosie25s5qthvvcjrr42ivd7doqhwvu5m4ks7uqv4j5lyi IPNS link (propagation can be slow)
> Success! Initialized /path/to/mybucket as a new empty bucket
```

The first URL is the ThreadDB link to the instance in the DB collection. Buckets are built on ThreadDB. Internally, a collection named `buckets` is created. Each new instance in this collection amounts to a new bucket. However, when you visit this link, you'll notice a custom file browser. This is because the gateway considers the built-in `buckets` collection a special case. You can still view the raw ThreadDB instance by appending `?json=true` to the URL.

The second URL is the bucket's unique IPNS address, which is auto-updated whenever files are added or deleted.

If you have configured the Buckets daemon with DNS settings, you will see a third URL that links to the bucket's WWW address, where it is rendered like as a static website / client-side application. See `buckd --help` for more info.

**Note**: If your bucket is private (encrypted), these links will 404 on gateway. This behavior is temporary while ThreadDB ACLs are still under development.

`buck init` created a configuration folder in `mybucket` called `.textile`. This folder is somewhat like a `.git` folder, as it contains information about the bucket's remote address and local state.

The value in `.textile/config.yml` will look something like,

```
key: bafzbeifyzfm3kosie25s5qthvvcjrr42ivd7doqhwvu5m4ks7uqv4j5lyi
thread: bafkq3ocmdkrljadlgybtvocytpdw4hbnzygxecxehdp7pfj32lxp34a
```

Where `key` is the bucket's unique key, and `thread` is it's ThreadDB ID.

Additionally, `.textile/repo` contains a repository describing the current file structure, which is used to stage changes against the remote.

### Adding files and folders to a bucket

Bucket files and folders are content-addressed by Cids. Check out [the spec](https://github.com/multiformats/cid) if you're unfamiliar with Cids.

New files are staged as additions:

```
echo "hello world" > hello.txt
buck status
> new file:  hello.txt
```

Use `push` to sync the change.

```
+ hello.txt: bafkreifjjcie6lypi6ny7amxnfftagclbuxndqonfipmb64f2km2devei4
> bafybeihm4zrnrsdroazwsvk3i65ooqzdftaugdkjiedr6ocq65u3ap4wni
```

The output shows the Cid of the added file and the bucket's new root Cid.

`push` will sync all types of file changes: _Additions, modifications, and deletions_.

### Recreating an existing bucket

### Creating a bucket from an existing Cid

### Resetting bucket contents

### Watching a bucket for changes

### Protect a file with a password

### Creating a Filecoin bucket archive

### Multi-writer buckets

### Deleting a bucket

### Using the Buckets Library

The `buckets/local` library powers both the `buck` and `hub buck` commands.

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
