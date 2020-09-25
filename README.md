# textile

[![Made by Textile](https://img.shields.io/badge/made%20by-Textile-informational.svg?style=popout-square)](https://textile.io)
[![Chat on Slack](https://img.shields.io/badge/slack-slack.textile.io-informational.svg?style=popout-square)](https://slack.textile.io)
[![GitHub license](https://img.shields.io/github/license/textileio/textile.svg?style=popout-square)](./LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/textileio/textile?style=flat-square)](https://goreportcard.com/report/github.com/textileio/textile?style=flat-square)
[![GitHub action](https://github.com/textileio/textile/workflows/Tests/badge.svg?style=popout-square)](https://github.com/textileio/textile/actions)
[![standard-readme compliant](https://img.shields.io/badge/readme%20style-standard-brightgreen.svg?style=popout-square)](https://github.com/RichardLitt/standard-readme)

> Textile hub services and buckets lib

Textile connects and extends [Libp2p](https://libp2p.io/), [IPFS](https://ipfs.io/), and [Filecoin](https://filecoin.io/). Three interoperable technologies makeup Textile:

* [**ThreadDB**](https://github.com/textileio/go-threads): A server-less p2p database built on Libp2p
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
  * [Sharing bucket files and folders](#sharing-bucket-files-and-folders)
  * [Creating a Filecoin bucket archive](#creating-a-filecoin-bucket-archive)
  * [Multi-writer buckets](#multi-writer-buckets)
  * [Deleting a bucket](#deleting-a-bucket)
  * [Using the Buckets Library](#using-the-buckets-library)
    * [Creating a bucket](#creating-a-bucket-1)
    * [Getting an existing bucket](#getting-an-existing-bucket)
    * [Pushing local changes](#pushing-local-changes)
    * [Pulling remote changes](#pulling-remote-changes)
  * [Using the Mail Library](#using-the-mail-library)
    * [Creating a mailbox](#creating-a-mailbox)
    * [Getting an existing mailbox](#getting-an-existing-mailbox)
    * [Sending a message](#sending-a-message)
    * [Watching for new messages](#watching-for-new-messages)
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
git clone https://github.com/textileio/textile
cd textile
go get ./cmd/hubd
```

#### `hub`

-   **Prebuilt package**: See [release assets](https://github.com/textileio/textile/releases/latest)
-   **Build from the source**: 

```
git clone https://github.com/textileio/textile
cd textile
go get ./cmd/hub
```

**Note**: `hub` _includes_ `buck` as a subcommand: `hub buck`. This is because `hubd` hosts `buckd`, along with other services.

`hub` is built in part on the [gRPC client](https://pkg.go.dev/github.com/textileio/textile/v2/api/hub/client), which can be imported to an existing project:

```
import "github.com/textileio/textile/v2/api/hub/client"
```

### Buckets

#### `buckd`

-   **Prebuilt package**: See [release assets](https://github.com/textileio/textile/releases/latest)
-   **Docker image**: See the `buckets` tag on [Docker Hub](https://hub.docker.com/r/textile/textile/tags)
-   **Build from the source**: 

```
git clone https://github.com/textileio/textile
cd textile
go get ./cmd/buckd
```

#### `buck`

-   **Prebuilt package**: See [release assets](https://github.com/textileio/textile/releases/latest)
-   **Build from the source**: 

```
git clone https://github.com/textileio/textile
cd textile
go get ./cmd/buck
```

`buck` is built in part on the [gRPC client](https://pkg.go.dev/github.com/textileio/textile/v2/api/buckets/client), which can be imported in an existing project:

```
import "github.com/textileio/textile/v2/api/buckets/client"
```

### The Buckets Library

```
import "github.com/textileio/textile/v2/buckets/local"
```

The full spec is available [here](https://pkg.go.dev/github.com/textileio/textile/v2/buckets/local).

## Getting Started

### The Hub

The Hub daemon (`hubd`), a.k.a. _The Hub_, is a hosted wrapper around other Textile services that includes developer accounts for individuals and organizations. You are free to run your own, but we encourage the use of the official [Textile Hub](https://docs.textile.io/hub/).

The layout of the `hub` client CLI mirrors the services wrapped by `hubd`:

-   `hub threads` provides limited access to ThreadDB.
-   `hub buck` provides access to Buckets (`buckd`) by wrapping the standalone `buck` CLI.
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
  pow         Interact with Powergate
  threads     Thread management
  update      Update the hub CLI
  version     Show current version
  whoami      Show current user

Flags:
      --api string       API target (default "api.hub.textile.io:443")
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

Congrats! Now you have Buckets running locally.

The Docker Compose file starts an IPFS node, which is used to pin bucket files and folders. You could point `buckd` to a different (possibly remote) IPFS node by setting the `BUCK_ADDR_IPFS_API` variable to a different multiaddress.  

By default, this approach does not start [Powergate](https://github.com/textileio/powergate). If you do, be sure to set the `BUCK_ADDR_POWERGATE_API` variable to the multiaddress of your Powergate. `buckd` must be configured with Powergate to enable Filecoin archiving with `buck archive`.

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

If you have configured the daemon with DNS settings, you will see a third URL that links to the bucket's WWW address, where it is rendered as a static website / client-side application. See `buckd --help` for more info.

**Important**: If your bucket is private (encrypted), an access token (JWT) will be appended to these links. This token represents your _identity_ across ***all buckets*** and should not be shared without caution.

`buck init` created a configuration folder in `mybucket` called `.textile`. This folder is somewhat like a `.git` folder, as it contains information about the bucket's remote address and local state.

`.textile/config.yml` will look something like,

```
key: bafzbeifyzfm3kosie25s5qthvvcjrr42ivd7doqhwvu5m4ks7uqv4j5lyi
thread: bafkq3ocmdkrljadlgybtvocytpdw4hbnzygxecxehdp7pfj32lxp34a
```

Where `key` is the bucket's unique key, and `thread` is it's ThreadDB ID.

Additionally, `.textile/repo` contains a repository describing the current file structure, which is used to stage changes against the remote.

### Creating a private bucket

Bucket encryption (AES-CTR + AES-512 HMAC) happens entirely within the `buckd`, meaning your data gets encrypted on the way in, and decrypted on the way out. This type of encryption has two goals:

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

`buck status` is powered by DAG-based diffing. Much like `git`, this allows buck to only push and pull _changes_. Read more about bucket diffing in the [docs](https://docs.textile.io/buckets/#diffing-and-synching), or check out this [in-depth blog post](https://blog.textile.io/buckets-diffing-syncing-archiving/).

Use `push` to sync the change.

```
buck push
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

**Note**: If `buckd` was running inside The Hub (`hubd`), you would be able to choose from buckets belonging to your Organizations and well as your individual Developer account by using the `--org` flag. Read more about Hub Accounts and Organizations [here](https://docs.textile.io/hub/accounts/).

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

Use the recursvie flag `-r` with `ipfs add`.

```
ipfs add -r .
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
```

After adding the entire directory, we see the root Cid is `QmcvkGF2t8Z94UqhdtdFRokGoqypbGyKkzRPVF4owmjVrE`. Let's create the bucket using this Cid.

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

Currently, UnixFS in `go-ipfs` uses Cid version 0, which is why we see all these old-style Cids started with `Qm`. Of course, you can also use UnixFS directories that use Cid version 1.

Similar to initializing a new bucket from an existing Cid, `buck add` allows you to _add_ and/or _merge in_ an existing UnixFS directory to an _existing bucket_. Like adding new files locally, this works by pulling down the UnixFS content from the IPFS network into the local bucket. Sync the changes with `buck push` as normal.

Pulling an existing UnixFS directory into a new or existing private bucket is also possible. Just opt-in to encryption during initialization as normal. `buckd` will recursively encrypt (without duplicating) the Cid's IPLD file and directory nodes as they are pulled into the new bucket.

### Exploring bucket contents

Use `buck ls [path]` to explore bucket contents. Omitting `[path]` will list the top-level directory.

```
buck ls

  NAME          SIZE     DIR    OBJECTS  CID
  .textileseed  32       false  n/a      bafkreiezexkrnk7yew6glm6sulhur66bbecc2aeaitf7uz4ymmp442lepu
  a             3726     true   3        QmSujVHvG8Y3Jv21AbMFNQPphjyqNamh6cvdyXSD1jAtSZ
  b             3191     true   2        QmQdAtg5NkwkvLtTbka3eci58UGj3m9AehC2sbksGSbjPZ
  c             1537626  true   2        QmcjtVAF9PQfMKTc57vcvZeBrzww3TLxPcQfUQW7cXXLJL
```

Use `[path]` to drill into directories, e.g.,

```
buck ls a

  NAME     SIZE  DIR    OBJECTS  CID
  bar.txt  517   false  n/a      QmcDkcMJXZsNnExehsE1Yh6SRWucHa9ruVT82gpL83431W
  foo.txt  557   false  n/a      QmYiUq2U6euWnKag23wFppG12hon4EBDswdoe4MwrKzDBn
  one      2502  true   3        QmWh2Wx9Lec4wbEvFbsq4HmYjFmgUFtxNJ8wEVwXjhJ2uk
```

`buck cat` functions a lot like `ls`, but cats file contents to stdout.

### Resetting bucket contents

Similar to a `git reset --hard`, you can use `buck pull --hard` to discard local changes that have not been pushed.

Continuing with the bucket above, add, modify, and/or delete some files. `buck status` should show your staged changes.

```
buck status
> modified:  a/bar.txt
> deleted:   a/one/baz.txt
> new file:  b/one/three/car.txt
> deleted:   b/foo.txt
```

Normally, `buck pull` will move your local changes to temporary `.buckpatch` files, apply the remote / upstream changes, then reapply your local changes. However, the `--hard` flag will prune all local changes, resetting the local bucket contents to match the remote exactly.

```
buck pull --hard
+ a/one/baz.txt: QmXrd35ja3kknnmgj5kyDM74jfG8GLJJQGtRpEQpXCLTR3
+ b/foo.txt: QmYiQAk1seXrmuQkpGE83AxJyNZDK1RNSaLyp3Z4r1zsrB
+ a/bar.txt: QmcDkcMJXZsNnExehsE1Yh6SRWucHa9ruVT82gpL83431W
- b/one/three/car.txt
> QmTz6HoC18QQqAEtYhfLc4Fse3LPbSCKV8vouvE88MKjFj
```

Now `buck status` will report `> Everything up-to-date`.

Try `buck pull --help` for more options when pulling the remote.

### Watching a bucket for changes

So far we've seen how a bucket can change locally, but the remote can also change. This could happen for a couple reasons:

* Changes are pushed from a different bucket copy against the same `buckd`.
* Changes are pushed from a different `buckd` at the ThreadDB layer. This is known as a multi-writer scenario. See [Multi-writer buckets](#multi-writer-buckets) for more.

In either case, it is possible to listen for and apply the remote changes using `buck watch`. This will also watch for local changes and auto-push them to the remote. In this way, multiple copies of the same bucket can be kept in sync.

`watch` will block until it's cancelled with a Ctrl-C.

```
buck watch
> Success! Watching /path/to/mybucket for changes...
```

`watch` will survive network interruptions, reconnecting when possible.

```
> Not connected. Trying to connect...
> Not connected. Trying to connect...
> Not connected. Trying to connect...
> Success! Watching /path/to/mybucket for changes...
```

While `watch` is active, file and folders dropped into the bucket will be automatically pushed.

### Protecting a file with a password

Private buckets handle encryption entirely within `buckd`, but you can use an additional client-side encryption layer with `buck encrypt` to password protect files. This encryption is also AES-CTR + AES-512 HMAC, which means you can efficiently encrypt large file streams. However, unlike bucket-wide encryption in private buckets, client-side encryption is only available for files, not IPLD directory nodes.

Let's create an encrypted version of the `hello.txt` file.

```
buck encrypt hello.txt supersecret > secret.txt
```

`encrypt` writes to stdout. So, here we redirect the output to a new file called `secret.txt`. [scrypt](https://pkg.go.dev/golang.org/x/crypto/scrypt?tab=doc) is used to derive the AES and HMAC keys from a password. This carries the normal tradeoff: _The encryption is only as good as the password_. Also, as with all client-side encryption, you must also store or otherwise remember the password!

`encrypt` only works on local files. You'll have to use `push` to sync the new file to the remote.

```
buck push --yes
+ secret.txt: bafkreiayymufgaut3wrfbzfdxiacxn64mxijj54g2osyk7qnco54iftovi
> bafybeidhffwg5ucwktn7iwyvnkhxpz7b2yrh643bo74cjvsbquzpdgpcd4
```

`decrypt`, on the other hand, works on remote files. So, after pushing `secret.txt`, we can decrypt it (if we can remember the password) and write the plaintext to stdout.

```
buck decrypt secret.txt supersecret
hello world
```

Looks like it worked!

### Sharing bucket files and folders

Bucket contents can be shared with other Hub accounts and users using the `buck roles` command. Each file and folder in a bucket maintains a set of public-key based access roles: `None`, `Reader`, `Writer`, and `Admin`. Only the `Admin` role can add and remove files and folders from a shared path. See `hub buck roles grant --help` for more about each role. For most applications, access roles only makes sense in the context of the Hub.

By default, public buckets have two roles located at the top-level path:

```
hub buck roles ls

  IDENTITY                                                     ROLE
  *                                                            Reader
  bbaareibzpb44ahd7oieqevvlqajidd4jajcvx2vdvti6bpw5wkqolwwerm  Admin

> Found 2 access roles
```

Since access roles are inherited down a bucket path, the single admin role grants the owner full access to all current and future files and folders. The default (`*`) `Read` role indicates that the entire bucket is open to the world. This is merely a reflection of the fact that the underlying UnixFS directory behind public (non-encrypted) buckets are discoverable on the IPFS Network.

Private buckets are not open to the world and are created with only the single admin role. However, we can still grant default (`*`) `Read` access to individual files, folders, or the entire bucket posteriori.

```
hub buck roles grant "*" myfolder
Use the arrow keys to navigate: ↓ ↑ → ←
? Select a role:
  None
  ▸ Reader
  Writer
  Admin
```

We can now see a new role added to `myfolder`.

```
 hub buck roles ls myfolder

  IDENTITY  ROLE
  *         Reader

> Found 1 access roles
```

Similarly, grant the `None` role to revoke access.

Manipulating access roles for a single Hub account or user (public key) can be cumbersome with the `buck` CLI. Applications in need of this level of granular access control should do so programmatically using the [Go client](https://pkg.go.dev/github.com/textileio/textile/v2/api/buckets/client), [JavaScript client](https://textileio.github.io/js-hub/docs/hub.buckets).

### Creating a Filecoin bucket archive

Bucket archiving requires a Powergate to be running in `buckd`. If you're curious how to do this, take a look at [this Docker Compose file](https://github.com/textileio/textile/blob/master/integrationtest/pg/docker-compose.yml).

Let's try archiving the bucket from the [Creating a bucket](#creating-a-bucket) section.

```
buck archive
> Warning! Archives are currently saved on an experimental test network. They may be lost at any time.
? Proceed? [y/N]
```

Please take note of the warning. Archiving should be considered experimental since Filecoin `mainnet` has not yet launched, and Powergate will either be running a `localnet` or `testnet`.

You should see a success message if you proceed.

```
> Success! Archive queued successfully
```

This means that archiving has been initiated. It may take some time to complete...

```
buck archive status
> Archive is currently executing, grab a coffee and be patient...
```

Use the `archive status` command with `-w` to watch the progress of your archive as it moves through the Filecoin market deal stages.

```
buck archive status -w
> Archive is currently executing, grab a coffee and be patient...
>    Pushing new configuration...
>    Configuration saved successfully
>    Executing job 1006707f-efa8-48c2-98af-a1b320a59780...
>    Ensuring Hot-Storage satisfies the configuration...
>    No actions needed in Hot Storage.
>    Hot-Storage execution ran successfully.
>    Ensuring Cold-Storage satisfies the configuration...
>    Current replication factor is lower than desired, making 10 new deals...
>    Calculating piece size...
>    Estimated piece size is 256 bytes.
>    Proposing deal to miner t01459 with 0 fil per epoch...
>    Proposing deal to miner t0117734 with 500000000 fil per epoch...
>    Proposing deal to miner t0120993 with 500000000 fil per epoch...
>    Proposing deal to miner t0120642 with 500000000 fil per epoch...
>    Proposing deal to miner t0121477 with 500000000 fil per epoch...
>    Proposing deal to miner t0119390 with 500000000 fil per epoch...
>    Proposing deal to miner t0101180 with 10000000 fil per epoch...
>    Proposing deal to miner t0117803 with 500000000 fil per epoch...
>    Proposing deal to miner t0121852 with 500000000 fil per epoch...
>    Proposing deal to miner t0119822 with 500000000 fil per epoch...
>    Watching deals unfold...
>    Deal with miner t0117803 changed state to StorageDealClientFunding
>    Deal with miner t0121852 changed state to StorageDealClientFunding
>    Deal with miner t0121477 changed state to StorageDealClientFunding
>    Deal with miner t0101180 changed state to StorageDealClientFunding
>    Deal with miner t0119822 changed state to StorageDealClientFunding
>    Deal with miner t0119390 changed state to StorageDealClientFunding
>    Deal with miner t0120642 changed state to StorageDealClientFunding
>    Deal with miner t0117734 changed state to StorageDealClientFunding
>    Deal with miner t01459 changed state to StorageDealClientFunding
>    Deal with miner t0120993 changed state to StorageDealClientFunding
>    Deal with miner t0121477 changed state to StorageDealWaitingForDataRequest
>    Deal with miner t0119822 changed state to StorageDealWaitingForDataRequest
>    Deal with miner t0117734 changed state to StorageDealWaitingForDataRequest
>    Deal with miner t0121852 changed state to StorageDealWaitingForDataRequest
>    Deal with miner t01459 changed state to StorageDealWaitingForDataRequest
>    Deal with miner t0120642 changed state to StorageDealWaitingForDataRequest
>    Deal with miner t0120993 changed state to StorageDealWaitingForDataRequest
>    Deal with miner t0117803 changed state to StorageDealWaitingForDataRequest
>    Deal with miner t0101180 changed state to StorageDealWaitingForDataRequest
>    Deal with miner t0119390 changed state to StorageDealWaitingForDataRequest
>    Deal with miner t01459 changed state to StorageDealProposalAccepted
>    Deal with miner t01459 changed state to StorageDealSealing
```

The output will look something like the above. With a little luck, you will start seeing some successful storage deals.

Bucket archiving allows you to leverage the purely decentralized nature of Filecoin in your buckets. Check out [this video](https://www.youtube.com/watch?v=jiBUxIi1zko&feature=emb_title) from a [blog post](https://blog.textile.io/buckets-diffing-syncing-archiving/) demonstrating Filecoin bucket recovery using the [Lotus client](https://github.com/filecoin-project/lotus).

### Multi-writer buckets

Multi-writer buckets leverage the distributed nature of ThreadDB by allowing multiple identities to write to the same bucket hosted by different Libp2p hosts. Since buckets are ThreadDB collection _instances_, this is no different than normal ThreadDB peer collaboration.

To-do: Demonstrate joining a bucket from a ThreadDB invite.

### Deleting a bucket

Deleting a bucket is easy... and permanent! `buck destroy` will delete your local bucket as well as the remote, making it unrecoverable with `buck init --existing`.

### Using the Buckets Library

The `buckets/local` library powers both the `buck` and `hub buck` CLIs. Everything possible in `buck`, from bucket diffing, pushing, pulling, watching, archiving, etc., is available to you in existing projects by importing the Buckets Library.

```
go get github.com/textileio/textile/v2/buckets/local
```

Visit the [GoDoc](https://pkg.go.dev/github.com/textileio/textile/v2/buckets/local) for a complete list of methods and more usage descriptions.

#### Creating a bucket

Create a new bucket by constructing a configuration object. Only `Path` is required.

```
// Setup the buckets lib
buckets := local.NewBuckets(cmd.NewClients("api.textile.io:443", false), local.DefaultConfConfig())

// Create a new bucket with config
mybuck, err := buckets.NewBucket(context.Background(), local.Config{
    Path: "path/to/bucket/folder"
})

// Check current status
diff, err := mybuck.DiffLocal() // diff contains staged changes
```

`buckets.NewBucket` will write a local config file and data repo.

See `local.WithName`, `local.WithPrivate`, `local.WithCid`, `local.WithExistingPathEvents` for more options when creating buckets.

To create a bucket from an existing remote, use its thread ID and instance ID (bucket `key`) in the config.

#### Getting an existing bucket

`GetLocalBucket` returns the bucket at path.

```
mybuck, err := buckets.GetLocalBucket(context.Background(), "path/to/bucket/folder")
```

#### Pushing local files

`PushLocal` pushes all staged changes to the remote and returns the new local and remote root Cids. These roots will only be different if the bucket is private (the remote is encrypted).

```
newRoots, err := mybuck.PushLocal()
```

See `local.PathOption` for more options when pushing.

#### Pulling remote changes

`PullRemote` pulls all remote changes locally and returns the new root Cids.

```
newRoots, err := mybuck.PullRemote()
```

See `local.PathOption` for more options when pulling.

### Using the Mail Library

The `mail/local` library provides mechanisms for sending and receiving messages between Hub users. Mailboxes are built on ThreadDB.

```
go get github.com/textileio/textile/v2/mail/local
```

Visit the [GoDoc](https://pkg.go.dev/github.com/textileio/textile/v2/mail/local) for a complete list of methods and more usage descriptions.

#### Creating a mailbox

Like creating a bucket, create a new mailbox by constructing a configuration object. All fields are required.

```
// Setup the mail lib
mail := local.NewMail(cmd.NewClients("api.textile.io:443", true), local.DefaultConfConfig())

// Create a libp2p identity (this can be any thread.Identity)
privKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
id := thread.NewLibp2pIdentity(privKey)

// Create a new mailbox with config
mailbox, err := mail.NewMailbox(context.Background(), local.Config{
    Path: "path/to/mail/folder", // Usually a global location like ~/.textile/mail
    Identity: id,
    APIKey: <API_SECRET>,
    APISecret: <API_KEY>,
})
```

`APIKey` and `APISecret` are User Group API Keys. Read more about [creating API Keys](https://docs.textile.io/hub/app-apis/#creating-user-group-keys).

To recreate a user's mailbox, specify the same identity and API Key in the config.

#### Getting an existing mailbox

`GetLocalMailbox` returns the mailbox at path.

```
mailbox, err := mail.GetLocalMailbox(context.Background(), "path/to/mailbox/folder")
```

#### Sending a message

When a mailbox sends a message to another mailbox, the message is encrypted for the recipient's inbox _and_ for the senders sentbox. This allows both parties to control the message's lifecycle.

```
// Create two mailboxes (for most applications, this would not happen on the same machine)
box1, err := mail.NewMailbox(context.Background(), local.Config{...})
box2, err := mail.NewMailbox(context.Background(), local.Config{...})

// Send a message from the first mailbox to the second
message, err := box1.SendMessage(context.Background(), box2.Identity().GetPublic(), []byte("howdy"))

// List the recipient's inbox
inbox, err := box2.ListInboxMessages(context.Background())

// Open decrypts the message body
body, err := inbox[0].Open(context.Background(), box2.Identity())

// Mark the message as read
err = box2.ReadInboxMessage(context.Background(), inbox[0].ID)
```

#### Watching for new messages

Applications may watch for mailbox events in the inbox and/or sentbox.

```
// Handle mailbox events as they arrive
events := make(chan MailboxEvent)
defer close(events)
go func() {
    for e := range events {
        switch e.Type {
        case NewMessage:
            // handle new message
        case MessageRead:
            // handle message read (inbox only)
        case MessageDeleted:
            // handle message deleted
        }
    }
}()

// Start watching (the third param indicates we want to keep watching when offline)
state, err := mailbox.WatchInbox(context.Background(), events, true)
for s := range state {
    // handle connectivity state
}
```

Similarly, use `WatchSentbox` to watch a sentbox.

## Developing

The easiest way to develop against `hubd` or `buckd` is to use the Docker Compose files found in `cmd`. The `-dev` flavored files do not persist repos via Docker Volumes, which may be desirable in some cases.

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
