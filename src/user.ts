// import { Context } from './context'
//
// /**
//  * Returns a Thread by name.
//  * @param name The name of the Thread.
//  * @param credentials Context containing gRPC headers and settings.
//  * These will be merged with any internal credentials.
//  */
// async getThread(name: string, credentials?: Context) {
//   logger.debug('get thread request')
//   const req = new pb.GetThreadRequest()
//   req.setName(name)
//   const res: pb.GetThreadReply = await this.unary(API.GetThread, req, credentials)
//   return res.toObject()
// }
//
// /**
//  * Returns a list of available Threads.
//  * @param credentials Context containing gRPC headers and settings.
//  * These will be merged with any internal credentials.
//  * @note Threads can be created using the threads or threads network clients.
//  */
// async listThreads(credentials?: Context) {
//   logger.debug('list thread request')
//   const req = new pb.ListThreadsRequest()
//   const res: pb.ListThreadsReply = await this.unary(API.ListThreads, req, credentials)
//   return res.getListList().map((thread) => thread.toObject())
// }
