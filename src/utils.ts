import axios, { AxiosRequestConfig } from 'axios'
import delay from 'delay'
import { Config } from '@textile/threads-client'
import { grpc } from '@improbable-eng/grpc-web'
import { SignupReply } from '@textile/hub/hub_pb'
import { Client } from './hub'

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

export const signUp = (client: Client, addrGatewayUrl: string, sessionSecret: string) => {
  const username = createUsername()
  const email = createEmail()
  return new Promise<SignupReply.AsObject>((resolve, reject) => {
    client
      .signUp(username, email)
      .then((user) => {
        resolve(user)
      })
      .catch((err) => reject(err))
    confirmEmail(addrGatewayUrl, sessionSecret).catch((err) => reject(err))
  })
}

export const signIn = (client: Client, name: string, addrGatewayUrl: string, sessionSecret: string) => {
  return new Promise<SignupReply.AsObject>((resolve, reject) => {
    client
      .signIn(name)
      .then((user) => {
        resolve(user)
      })
      .catch((err) => reject(err))
    confirmEmail(addrGatewayUrl, sessionSecret).catch((err) => reject(err))
  })
}
