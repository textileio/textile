import axios from 'axios'
import delay from 'delay'
import { HMAC } from 'fast-sha256'
import multibase from 'multibase'
import * as pb from '@textile/hub-grpc/hub_pb'
import { APIClient, ServiceError } from '@textile/hub-grpc/hub_pb_service'
import { Context } from '@textile/context'

export const createUsername = (size = 12) => {
  return Array(size)
    .fill(0)
    .map(() => Math.random().toString(36).charAt(2))
    .join('')
}

export const createEmail = () => {
  return `${createUsername()}@doe.com`
}

export const confirmEmail = async (gurl: string, secret: string) => {
  await delay(500)
  const resp = await axios.get(`${gurl}/confirm/${secret}`)
  if (resp.status !== 200) {
    throw new Error(resp.statusText)
  }
  return true
}

export const createKey = (ctx: Context, kind: keyof pb.KeyTypeMap) => {
  return new Promise<pb.GetKeyReply.AsObject>((resolve, reject) => {
    const req = new pb.CreateKeyRequest()
    req.setType(pb.KeyType[kind])
    const client = new APIClient(ctx.host, { transport: ctx.transport, debug: ctx.debug })
    client.createKey(req, ctx.toMetadata(), (err: ServiceError | null, message: pb.GetKeyReply | null) => {
      if (err) reject(err)
      resolve(message?.toObject())
    })
  })
}

export const signUp = (ctx: Context, addrGatewayUrl: string, sessionSecret: string) => {
  const username = createUsername()
  const email = createEmail()
  return new Promise<{ user: pb.SignupReply.AsObject | undefined; username: string; email: string }>(
    (resolve, reject) => {
      const req = new pb.SignupRequest()
      req.setEmail(email)
      req.setUsername(username)
      const client = new APIClient(ctx.host, { transport: ctx.transport, debug: ctx.debug })
      client.signup(req, ctx.toMetadata(), (err: ServiceError | null, message: pb.SignupReply | null) => {
        if (err) reject(err)
        resolve({ user: message?.toObject(), username, email })
      })
      confirmEmail(addrGatewayUrl, sessionSecret).catch((err) => reject(err))
    },
  )
}

export const createAPISig = async (secret: string, date: Date = new Date(Date.now() + 1000 * 60)) => {
  const sec = multibase.decode(secret)
  const msg = (date ?? new Date()).toISOString()
  const hash = new HMAC(sec)
  const mac = hash.update(Buffer.from(msg)).digest()
  const sig = multibase.encode('base32', Buffer.from(mac)).toString()
  return { sig, msg }
}
