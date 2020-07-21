# textile

[![Made by Textile](https://img.shields.io/badge/made%20by-Textile-informational.svg?style=popout-square)](https://textile.io)
[![Chat on Slack](https://img.shields.io/badge/slack-slack.textile.io-informational.svg?style=popout-square)](https://slack.textile.io)
[![GitHub license](https://img.shields.io/github/license/textileio/textile.svg?style=popout-square)](./LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/textileio/textile?style=flat-square)](https://goreportcard.com/report/github.com/textileio/textile?style=flat-square)
[![GitHub action](https://github.com/textileio/textile/workflows/Tests/badge.svg?style=popout-square)](https://github.com/textileio/textile/actions)
[![standard-readme compliant](https://img.shields.io/badge/readme%20style-standard-brightgreen.svg?style=popout-square)](https://github.com/RichardLitt/standard-readme)

> Textile services and buckets lib written in Go

Textile connects and extends [Libp2p](https://libp2p.io/), [IPFS](https://ipfs.io/), and [Filecoin](https://filecoin.io/). Three interoperable technologies makeup Textile:

* [**ThreadDB**](https://github.com/textileio/go-threads): A serverless p2p database built on Libp2p
* [**Powergate**](https://github.com/textileio/powergate): File storage built on Filecoin and IPFS
* [**Buckets**](https://github.com/textileio/textile/tree/master/buckets): File and dynamic directory storage built on ThreadDB, Powergate, and [UnixFS](https://github.com/ipfs/go-unixfs).

Join us on our [public Slack channel](https://slack.textile.io/) for news, discussions, and status updates. [Check out our blog](https://medium.com/textileio) for the latest posts and announcements.

## Table of Contents

* [Table of Contents](#table-of-contents)
* [Security](#security)
* [Background](#background)
* [Install](#install)
  * [The Hub](#the-hub)
    * [hubd](#hubd)
    * [hub](#hub)
  * [Buckets](#buckets)
    * [buckd](#buckd)
    * [buck](#buck)
  * [The Buckets Library](#the-buckets-library)
* [Getting Started](#getting-started)
  * [The Hub](#the-hub-1)
  * [Running Buckets](#running-buckets)
  * [Creating a bucket](#creating-a-bucket)
  * [Creating a private bucket](#creating-a-private-bucket)
  * [Adding files and folders to a bucket](#adding-files-and-folders-to-a-bucket)
  * [Recreating an existing bucket](#recreating-an-existing-bucket)
  * [Creating a bucket from an existing Cid](#creating-a-bucket-from-an-existing-cid)
  * [Exploring bucket contents](#exploring-bucket-contents)
  * [Resetting bucket contents](#resetting-bucket-contents)
  * [Watching a bucket for changes](#watching-a-bucket-for-changes)
  * [Protecting a file with a password](#protecting-a-file-with-a-password)
  * [Creating a Filecoin bucket archive](#creating-a-filecoin-bucket-archive)
  * [Multi-writer buckets](#multi-writer-buckets)
  * [Deleting a bucket](#deleting-a-bucket)
  * [Using the Buckets Library](#using-the-buckets-library)
* [Developing](#developing)
* [Contributing](#contributing)
* [Changelog](#changelog)
* [License](#license)

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

Much like [`threadsd`](https://github.com/textileio/go-threads/tree/master/threadsd), the `buckd` daemon can be run as a server or alongside desktop apps or command-line tools. The easiest way to run `buckd` is by using the provided Docker Compose files. If you're new to Docker and/or Docker Compose, get started [here](https://docs.docker.com/compose/gettingstarted/). Once you are setup, you should have `docker-compose` in your `PATH`.

Create an `.env` file and add the following values:  

```
REPO_PATH=~/myrepo
BUCK_LOG_DEBUG=true
```

Copy [this compose file](https://github.com/textileio/textile/blob/master/cmd/buckd/docker-compose.yml) and run it with the following command.

```
docker-compose -f docker-compose.yml up 
```

Congrats! You now have Buckets running locally.

The Docker Compose file starts an IPFS node, which is used to pin bucket files and folders. You could point `buckd` to a different (possibly remote) IPFS node by setting the `BUCK_ADDR_IPFS_API` variable to a different multiaddress.  

By default, this approach does not start [Powergate](https://github.com/textileio/powergate). If you do, be sure to set the `BUCK_ADDR_POWERGATE_API` variable to the multiaddress of your Powergate. Buckets must be configured with Powergate to enable Filecoin archiving with `buck archive`.

### Creating a bucket

Since `hub buck` and `buck` are functionally identical, this section will focus on `buck` and the Buckets Library using a locally running `buckd`.

First off, take a look at `buck --help`.

```
The Bucket Client.

Manages files and folders in an object storage bucket.

Usage:
  buck [command]

Available Commands:
  add         Add adds a UnixFs DAG locally at path
  archive     Create a Filecoin archive
  cat         Cat bucket objects at path
  decrypt     Decrypt bucket objects at path with password
  destroy     Destroy bucket and all objects
  encrypt     Encrypt file with a password
  help        Help about any command
  init        Initialize a new or existing bucket
  links       Show links to where this bucket can be accessed
  ls          List top-level or nested bucket objects
  pull        Pull bucket object changes
  push        Push bucket object changes
  root        Show bucket root CIDs
  status      Show bucket object changes
  watch       Watch auto-pushes local changes to the remote

Flags:
      --api string   API target (default "127.0.0.1:3006")
  -h, --help         help for buck

Use "buck [command] --help" for more information about a command.
```

A Textile bucket functions a bit like an S3 bucket. It's a virtual filesystem where you can push, pull, list, and cat files. You can share them via web links or render the whole thing as a website or web app. They also function a bit like a Git repository. The point of entry is from a folder on your local machine that is synced to a _remote_.

To get started, initialize a new bucket.

```
mkdir mybucket && cd mybucket
buck init
```

When prompted, give your bucket a name and either opt-in or decline bucket encyption (see [Creating a private bucket](#creating-a-private-bucket) for more about bucket encryption).

You should now see two links for the new bucket on the locally running gateway.

```
> http://127.0.0.1:8006/thread/bafkq3ocmdkrljadlgybtvocytpdw4hbnzygxecxehdp7pfj32lxp34a/buckets/bafzbeifyzfm3kosie25s5qthvvcjrr42ivd7doqhwvu5m4ks7uqv4j5lyi Thread link
> http://127.0.0.1:8006/ipns/bafzbeifyzfm3kosie25s5qthvvcjrr42ivd7doqhwvu5m4ks7uqv4j5lyi IPNS link (propagation can be slow)
> Success! Initialized /path/to/mybucket as a new empty bucket
```

The first URL is the link to the ThreadDB instance. Internally, a collection named `buckets` is created. Each new instance in this collection amounts to a new bucket. However, when you visit this link, you'll notice a custom file browser. This is because the gateway considers the built-in `buckets` collection a special case. You can still view the raw ThreadDB instance by appending `?json=true` to the URL.

The second URL is the bucket's unique IPNS address, which is auto-updated when you add, modify, or delete files.

If you have configured the daemon with DNS settings, you will see a third URL that links to the bucket's WWW address, where it is rendered like as a static website / client-side application. See `buckd --help` for more info.

**Note**: If your bucket is private (encrypted), these links will 404 on the gateway. This behavior is temporary while ThreadDB ACLs are still under development.

`buck init` created a configuration folder in `mybucket` called `.textile`. This folder is somewhat like a `.git` folder, as it contains information about the bucket's remote address and local state.

`.textile/config.yml` will look something like,

```
key: bafzbeifyzfm3kosie25s5qthvvcjrr42ivd7doqhwvu5m4ks7uqv4j5lyi
thread: bafkq3ocmdkrljadlgybtvocytpdw4hbnzygxecxehdp7pfj32lxp34a
```

Where `key` is the bucket's unique key, and `thread` is it's ThreadDB ID.

Additionally, `.textile/repo` contains a repository describing the current file structure, which is used to stage changes against the remote.

### Creating a private bucket

Bucket encryption (AES-CTR + AES-512 HMAC) happens entirely within the Buckets daemon, meaning your data gets encrypted on the way in, and decrypted on the way out. This type of encryption has two goals:

- Obfuscate bucket data / files (the normal goal of encryption)
- Obfuscate directory structure, which amounts to encrypting [IPLD](https://ipld.io/) nodes and their links.

As a result of these goals, we refer to encrypted buckets as _private buckets_. Read more about bucket encryption [here](https://docs.textile.io/buckets/#encryption).

To create a new private bucket, use the `--private` flag with `buck` init or respond `y` when prompted.

In addition to bucket-level encryption, you can also [protect a file with a password](#protecting-a-file-with-a-password).

### Adding files and folders to a bucket

Bucket files and folders are content-addressed by Cids. Check out [the spec](https://github.com/multiformats/cid) if you're unfamiliar with Cids.

`buck` stages new files as additions:

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

It's often useful to recreate a bucket from the remote. This is somewhat like re-cloning a Git repo. This can be done in a different location on the same machine, or, if `buckd` has a public IP address, from a completely different machine.

Let's recreate the bucket from the previous step in a new directory outside of the original bucket.

```
mkdir mybucket2 && cd mybucket2
buck init --existing
```

The `--existing` flag allows for interactively selecting an existing bucket to initialize from.

```
? Which exiting bucket do you want to init from?:
  ▸ MyBucket bafzbeifyzfm3kosie25s5qthvvcjrr42ivd7doqhwvu5m4ks7uqv4j5lyi
```

At this point, there's only one bucket to choose from.

```
> Selected bucket MyBucket
+ hello.txt: bafkreifjjcie6lypi6ny7amxnfftagclbuxndqonfipmb64f2km2devei4
+ .textileseed: bafkreifbdzttoqsch5j66hfmcbsic6qvwrikibgzfbg3tn7rc3j63ukk3u
> Your bucket links:
> http://127.0.0.1:8006/thread/bafkq3ocmdkrljadlgybtvocytpdw4hbnzygxecxehdp7pfj32lxp34a/buckets/bafzbeifyzfm3kosie25s5qthvvcjrr42ivd7doqhwvu5m4ks7uqv4j5lyi Thread link
> http://127.0.0.1:8006/ipns/bafzbeifyzfm3kosie25s5qthvvcjrr42ivd7doqhwvu5m4ks7uqv4j5lyi IPNS link (propagation can be slow)
> Success! Initialized /path/to/mybucket2 from an existing bucket
```

Just as before, the output shows the bucket's remote links. However, in this case `init` also pulled down the content.

**Note**: `.textileseed` is used to randomize a bucket's top level Cid and cannot be modified.

The `--existing` flag is really just a helper that sets the `--thread` and `--key` flags, which match the config values we saw earlier. We could have used those flags directly to achieve the same result.

```
buck init --thread bafkq3ocmdkrljadlgybtvocytpdw4hbnzygxecxehdp7pfj32lxp34a --key bafzbeifyzfm3kosie25s5qthvvcjrr42ivd7doqhwvu5m4ks7uqv4j5lyi
```

Lastly, we could have just copied `.textile/config.yml` to a new directory and used `buck pull` to pull down the existing content.

### Creating a bucket from an existing Cid

Sometimes it's useful to create a bucket from a [UnixFS](https://github.com/ipfs/go-unixfs) directory that is already on the IPFS network.

We can simulate this scenario by adding a local folder to IPFS and then using its root Cid to create a bucket with the `--cid` flag. Here's a local directory.

```
.
├── a
│   ├── bar.txt
│   ├── foo.txt
│   └── one
│       ├── baz.txt
│       ├── buz.txt
│       └── two
│           ├── boo.txt
│           └── fuz.txt
├── b
│   ├── foo.txt
│   └── one
│       ├── baz.txt
│       ├── muz.txt
│       ├── three
│       │   └── far.txt
│       └── two
│           └── fuz.txt
└── c
    ├── one.jpg
    └── two.jpg
```

Add the entire thing to IPFS (`-r` for recursive).

```
ipfs add -r .                                                                      10:38:53
added QmcDkcMJXZsNnExehsE1Yh6SRWucHa9ruVT82gpL83431W mydir/a/bar.txt
added QmYiUq2U6euWnKag23wFppG12hon4EBDswdoe4MwrKzDBn mydir/a/foo.txt
added QmXrd35ja3kknnmgj5kyDM74jfG8GLJJQGtRpEQpXCLTR3 mydir/a/one/baz.txt
added QmSWJvCzotB3CbdxVu8mBvmLqpSuEQgUoJHTFy1azRfwhT mydir/a/one/buz.txt
added QmT6h1eaBV74Sh75upE7ugFLkBnmyGr3WsQ8w8yx5NjgPV mydir/a/one/two/boo.txt
added QmTdg1b5eWEx4zJtrgvew1inkkZ29fp9mbQ4uHyKurW8Ub mydir/a/one/two/fuz.txt
added QmYiQAk1seXrmuQkpGE83AxJyNZDK1RNSaLyp3Z4r1zsrB mydir/b/foo.txt
added QmXrd35ja3kknnmgj5kyDM74jfG8GLJJQGtRpEQpXCLTR3 mydir/b/one/baz.txt
added QmSWJvCzotB3CbdxVu8mBvmLqpSuEQgUoJHTFy1azRfwhT mydir/b/one/muz.txt
added QmYs12A3CGSTHX4QrsvBe2AvLHEThrapXoTFQpyh8AzpFa mydir/b/one/three/far.txt
added QmTdg1b5eWEx4zJtrgvew1inkkZ29fp9mbQ4uHyKurW8Ub mydir/b/one/two/fuz.txt
added QmaLpwNPwftSQY3w4ZtMfZ8k38D5EgK2bcDuU4UwzREJpi mydir/c/one.jpg
added QmYLiWv2WXQd1m8YyHx4dMoj8B3Kuiuu7pCCoYibkqKyVj mydir/c/two.jpg
added QmT5YXeCfbMuVjanbHjQhECUQSACJLecfmjRBZHvmu5FDU mydir/a/one/two
added QmWh2Wx9Lec4wbEvFbsq4HmYjFmgUFtxNJ8wEVwXjhJ2uk mydir/a/one
added QmSujVHvG8Y3Jv21AbMFNQPphjyqNamh6cvdyXSD1jAtSZ mydir/a
added QmUGSorWDy2JiKYvQuJzEb4TnYDuDNLcdFyR6NhMwnwdvy mydir/b/one/three
added QmWvX7UVexbjXJtxKMyMSgGpPesFQD7teNTqUcDsP2mzW6 mydir/b/one/two
added QmPyMD67EgSZS1WpvgudHkxbA5zgjqmse8srPpFb9sVefT mydir/b/one
added QmQdAtg5NkwkvLtTbka3eci58UGj3m9AehC2sbksGSbjPZ mydir/b
added QmcjtVAF9PQfMKTc57vcvZeBrzww3TLxPcQfUQW7cXXLJL mydir/c
added QmcvkGF2t8Z94UqhdtdFRokGoqypbGyKkzRPVF4owmjVrE mydir
 1.47 MiB / 1.47 MiB [=========================================================================] 100.00%
```

The root Cid of `mydir` is `QmcvkGF2t8Z94UqhdtdFRokGoqypbGyKkzRPVF4owmjVrE`. Let's create the bucket.

```
buck init --cid QmcvkGF2t8Z94UqhdtdFRokGoqypbGyKkzRPVF4owmjVrE
```

The files behind the Cid will be pulled into the new bucket.

```
+ a/bar.txt: QmcDkcMJXZsNnExehsE1Yh6SRWucHa9ruVT82gpL83431W
+ a/foo.txt: QmYiUq2U6euWnKag23wFppG12hon4EBDswdoe4MwrKzDBn
+ a/one/two/fuz.txt: QmTdg1b5eWEx4zJtrgvew1inkkZ29fp9mbQ4uHyKurW8Ub
+ a/one/baz.txt: QmXrd35ja3kknnmgj5kyDM74jfG8GLJJQGtRpEQpXCLTR3
+ c/two.jpg: QmYLiWv2WXQd1m8YyHx4dMoj8B3Kuiuu7pCCoYibkqKyVj
+ b/foo.txt: QmYiQAk1seXrmuQkpGE83AxJyNZDK1RNSaLyp3Z4r1zsrB
+ a/one/buz.txt: QmSWJvCzotB3CbdxVu8mBvmLqpSuEQgUoJHTFy1azRfwhT
+ a/one/two/boo.txt: QmT6h1eaBV74Sh75upE7ugFLkBnmyGr3WsQ8w8yx5NjgPV
+ b/one/muz.txt: QmSWJvCzotB3CbdxVu8mBvmLqpSuEQgUoJHTFy1azRfwhT
+ b/one/three/far.txt: QmYs12A3CGSTHX4QrsvBe2AvLHEThrapXoTFQpyh8AzpFa
+ b/one/baz.txt: QmXrd35ja3kknnmgj5kyDM74jfG8GLJJQGtRpEQpXCLTR3
+ b/one/two/fuz.txt: QmTdg1b5eWEx4zJtrgvew1inkkZ29fp9mbQ4uHyKurW8Ub
+ c/one.jpg: QmaLpwNPwftSQY3w4ZtMfZ8k38D5EgK2bcDuU4UwzREJpi
> Your bucket links:
> http://127.0.0.1:8006/thread/bafk3k3itq2rsybcvhf6wuvumruw3j6cw7ixhrtx4ek45qgvp3e7u2xa/buckets/bafzbeiawo6ghgsqjlorii4wghdl4tzz54x2kiwtcgtaq7b3h5gta2yok2i Thread link
> http://127.0.0.1:8006/ipns/bafzbeiawo6ghgsqjlorii4wghdl4tzz54x2kiwtcgtaq7b3h5gta2yok2i IPNS link (propagation can be slow)
> Success! Initialized /path/to/mybucket3 as a new bootstrapped bucket
```

Notice also that you can still leverage bucket encryption when creating a bucket from an existing Cid. In this case, Buckets will recursively encrypt the Cid's IPLD file and directory nodes as it's pulled into the new bucket.

### Exploring bucket contents

Use `buck ls [path]` to explore bucket contents. Omitting `[path]` will list the top-level directory.

```
  NAME          SIZE     DIR    OBJECTS  CID
  .textileseed  32       false  n/a      bafkreiezexkrnk7yew6glm6sulhur66bbecc2aeaitf7uz4ymmp442lepu
  a             3726     true   3        QmSujVHvG8Y3Jv21AbMFNQPphjyqNamh6cvdyXSD1jAtSZ
  b             3191     true   2        QmQdAtg5NkwkvLtTbka3eci58UGj3m9AehC2sbksGSbjPZ
  c             1537626  true   2        QmcjtVAF9PQfMKTc57vcvZeBrzww3TLxPcQfUQW7cXXLJL
```

Use `[path]` to drill into directories, e.g., `buck ls a`:

```
  NAME     SIZE  DIR    OBJECTS  CID
  bar.txt  517   false  n/a      QmcDkcMJXZsNnExehsE1Yh6SRWucHa9ruVT82gpL83431W
  foo.txt  557   false  n/a      QmYiUq2U6euWnKag23wFppG12hon4EBDswdoe4MwrKzDBn
  one      2502  true   3        QmWh2Wx9Lec4wbEvFbsq4HmYjFmgUFtxNJ8wEVwXjhJ2uk
```

`buck cat` functions a lot like `ls`, but cats file contents to stdout.

### Resetting bucket contents

### Watching a bucket for changes

### Protecting a file with a password

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
