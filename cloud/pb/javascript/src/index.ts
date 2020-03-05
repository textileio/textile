/* eslint-disable @typescript-eslint/no-non-null-assertion */
import { grpc } from '@improbable-eng/grpc-web'
import { API } from './api_pb_service'
import {
  GetProjectReply,
  GetProjectRequest,
  GetTeamReply,
  GetTeamRequest,
  ListProjectsReply,
  ListProjectsRequest,
  ListTeamsReply,
  ListTeamsRequest,
  LoginRequest,
  LoginReply,
  SwitchRequest,
  SwitchReply,
  ListBucketPathRequest,
  ListBucketPathReply,
  AddTeamRequest,
  AddTeamReply,
  AddProjectRequest,
  InviteToTeamReply,
  InviteToTeamRequest,
  WhoamiReply,
  WhoamiRequest,
  RemoveTeamRequest
} from './api_pb'

export type TextileConfig = {
  host?: string;
  token?: string;
}
/**
 * Client is a web-gRPC wrapper client for communicating with a webgRPC-enabled Textile server.
 * This client library can be used to interact with a local or remote Textile gRPC-service
 *  It is a wrapper around Textile's 'Store' API, which is defined here: https://github.com/textileio/go-threads/blob/master/api/pb/api.proto.
 */
export class Cloud {
  host: string;
  token?: string;
  
  constructor(token?: string, host?: string) {
    this.token = token ? token : undefined;
    this.host = host ? host : 'http://127.0.0.1:3007';
  }
  public async login(email: string) {
    const request = new LoginRequest();
    request.setEmail(email);
    const response = this.unary(API.Login, request) as Promise<LoginReply.AsObject>
    this.token = (await response).sessionid;
    return response;
  }

  public async listTeams() {
    const request = new ListTeamsRequest();
    return this.unary(API.ListTeams, request) as Promise<ListTeamsReply.AsObject>
  }
  public async switchTeams(teamID: string) {
    const request = new SwitchRequest();
    return this.unary(API.Switch, request, teamID) as Promise<SwitchRequest.AsObject>
  }
  public async listProjects() {
    const request = new ListProjectsRequest();
    return this.unary(API.ListProjects, request) as Promise<ListProjectsReply.AsObject>
  }
  public async listBuckets(projectID: string) {
    const request = new ListBucketPathRequest();
    request.setProject(projectID)
    return this.unary(API.ListBucketPath, request) as Promise<ListBucketPathReply.AsObject>
  }

  public async addTeam(name: string) {
    const request = new AddTeamRequest();
    request.setName(name)
    return this.unary(API.AddTeam, request) as Promise<AddTeamReply.AsObject>
  }
  public async getTeam(id: string) {
    const request = new GetTeamRequest();
    request.setId(id)
    return this.unary(API.GetTeam, request) as Promise<GetTeamReply.AsObject>
  }
  public async addProject(name: string) {
    const request = new AddProjectRequest();
    request.setName(name)
    return this.unary(API.AddProject, request) as Promise<AddProjectRequest.AsObject>
  }
  public async getProject(name: string) {
    const request = new GetProjectRequest();
    request.setName(name)
    return this.unary(API.GetProject, request) as Promise<GetProjectRequest.AsObject>
  }
  public async newInvite(email: string, teamID: string) {
    const request = new InviteToTeamRequest();
    request.setEmail(email)
    request.setId(teamID)
    return this.unary(API.InviteToTeam, request) as Promise<InviteToTeamReply.AsObject>
  }
  public async whoami() {
    const request = new WhoamiRequest();
    return this.unary(API.Whoami, request) as Promise<WhoamiReply.AsObject>
  }
  public async initProject(name: string) {
    const request = new AddProjectRequest();
    request.setName(name)
    return this.unary(API.AddProject, request)
  }
  public async removeTeam(id: string) {
    const request = new RemoveTeamRequest();
    request.setId(id)
    return this.unary(API.RemoveTeam, request)
  }

  private async unary<
    TRequest extends grpc.ProtobufMessage,
    TResponse extends grpc.ProtobufMessage,
    M extends grpc.UnaryMethodDefinition<TRequest, TResponse>
  >(methodDescriptor: M, req: TRequest, scope?: string) {
    return new Promise((resolve, reject) => {
      const meta = new grpc.Metadata()
      if (this.token) {
        meta.append('authorization', `bearer ${this.token}`)
        if (scope) {
          meta.append('x-scope', `${scope}`)
        }
      }
      console.log(meta.headersMap)
      grpc.unary(methodDescriptor, {
        request: req,
        host: this.host,
        metadata: meta,
        onEnd: res => {
          const { status, statusMessage, message } = res
          if (status === grpc.Code.OK) {
            if (message) {
              resolve(message.toObject())
            } else {
              resolve()
            }
          } else {
            reject(new Error(statusMessage))
          }
        },
      })
    })
  }
}

// eslint-disable-next-line import/no-default-export
export default Cloud
