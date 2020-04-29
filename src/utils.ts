import axios from 'axios'
import delay from 'delay'
import { SignupReply } from '@textile/hub-grpc/hub_pb'
import * as pb from '@textile/hub-grpc/hub_pb'
import { APIClient, ServiceError } from '@textile/hub-grpc/hub_pb_service'
import { Context } from './context'

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

export const signUp = (ctx: Context, addrGatewayUrl: string, sessionSecret: string) => {
  const username = createUsername()
  const email = createEmail()
  return new Promise<{ user: SignupReply.AsObject | undefined; username: string; email: string }>((resolve, reject) => {
    const req = new pb.SignupRequest()
    req.setEmail(email)
    req.setUsername(username)
    const client = new APIClient(ctx.host, { transport: ctx.transport, debug: ctx.debug })
    client.signup(req, ctx.toMetadata(), (err: ServiceError | null, message: SignupReply | null) => {
      if (err) reject(err)
      resolve({ user: message?.toObject(), username, email })
    })
    confirmEmail(addrGatewayUrl, sessionSecret).catch((err) => reject(err))
  })
}
