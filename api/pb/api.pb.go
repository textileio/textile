// Code generated by protoc-gen-go. DO NOT EDIT.
// source: api.proto

package api_pb

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type LoginRequest struct {
	Email                string   `protobuf:"bytes,1,opt,name=email,proto3" json:"email,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *LoginRequest) Reset()         { *m = LoginRequest{} }
func (m *LoginRequest) String() string { return proto.CompactTextString(m) }
func (*LoginRequest) ProtoMessage()    {}
func (*LoginRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_api_e1dcc96785f2904a, []int{0}
}
func (m *LoginRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_LoginRequest.Unmarshal(m, b)
}
func (m *LoginRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_LoginRequest.Marshal(b, m, deterministic)
}
func (dst *LoginRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_LoginRequest.Merge(dst, src)
}
func (m *LoginRequest) XXX_Size() int {
	return xxx_messageInfo_LoginRequest.Size(m)
}
func (m *LoginRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_LoginRequest.DiscardUnknown(m)
}

var xxx_messageInfo_LoginRequest proto.InternalMessageInfo

func (m *LoginRequest) GetEmail() string {
	if m != nil {
		return m.Email
	}
	return ""
}

type LoginReply struct {
	ID                   string   `protobuf:"bytes,1,opt,name=ID,proto3" json:"ID,omitempty"`
	Token                string   `protobuf:"bytes,2,opt,name=token,proto3" json:"token,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *LoginReply) Reset()         { *m = LoginReply{} }
func (m *LoginReply) String() string { return proto.CompactTextString(m) }
func (*LoginReply) ProtoMessage()    {}
func (*LoginReply) Descriptor() ([]byte, []int) {
	return fileDescriptor_api_e1dcc96785f2904a, []int{1}
}
func (m *LoginReply) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_LoginReply.Unmarshal(m, b)
}
func (m *LoginReply) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_LoginReply.Marshal(b, m, deterministic)
}
func (dst *LoginReply) XXX_Merge(src proto.Message) {
	xxx_messageInfo_LoginReply.Merge(dst, src)
}
func (m *LoginReply) XXX_Size() int {
	return xxx_messageInfo_LoginReply.Size(m)
}
func (m *LoginReply) XXX_DiscardUnknown() {
	xxx_messageInfo_LoginReply.DiscardUnknown(m)
}

var xxx_messageInfo_LoginReply proto.InternalMessageInfo

func (m *LoginReply) GetID() string {
	if m != nil {
		return m.ID
	}
	return ""
}

func (m *LoginReply) GetToken() string {
	if m != nil {
		return m.Token
	}
	return ""
}

type AddProjectRequest struct {
	Name                 string   `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	GroupID              string   `protobuf:"bytes,2,opt,name=groupID,proto3" json:"groupID,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *AddProjectRequest) Reset()         { *m = AddProjectRequest{} }
func (m *AddProjectRequest) String() string { return proto.CompactTextString(m) }
func (*AddProjectRequest) ProtoMessage()    {}
func (*AddProjectRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_api_e1dcc96785f2904a, []int{2}
}
func (m *AddProjectRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_AddProjectRequest.Unmarshal(m, b)
}
func (m *AddProjectRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_AddProjectRequest.Marshal(b, m, deterministic)
}
func (dst *AddProjectRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AddProjectRequest.Merge(dst, src)
}
func (m *AddProjectRequest) XXX_Size() int {
	return xxx_messageInfo_AddProjectRequest.Size(m)
}
func (m *AddProjectRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_AddProjectRequest.DiscardUnknown(m)
}

var xxx_messageInfo_AddProjectRequest proto.InternalMessageInfo

func (m *AddProjectRequest) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *AddProjectRequest) GetGroupID() string {
	if m != nil {
		return m.GroupID
	}
	return ""
}

type AddProjectReply struct {
	ID                   string   `protobuf:"bytes,1,opt,name=ID,proto3" json:"ID,omitempty"`
	StoreID              string   `protobuf:"bytes,2,opt,name=storeID,proto3" json:"storeID,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *AddProjectReply) Reset()         { *m = AddProjectReply{} }
func (m *AddProjectReply) String() string { return proto.CompactTextString(m) }
func (*AddProjectReply) ProtoMessage()    {}
func (*AddProjectReply) Descriptor() ([]byte, []int) {
	return fileDescriptor_api_e1dcc96785f2904a, []int{3}
}
func (m *AddProjectReply) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_AddProjectReply.Unmarshal(m, b)
}
func (m *AddProjectReply) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_AddProjectReply.Marshal(b, m, deterministic)
}
func (dst *AddProjectReply) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AddProjectReply.Merge(dst, src)
}
func (m *AddProjectReply) XXX_Size() int {
	return xxx_messageInfo_AddProjectReply.Size(m)
}
func (m *AddProjectReply) XXX_DiscardUnknown() {
	xxx_messageInfo_AddProjectReply.DiscardUnknown(m)
}

var xxx_messageInfo_AddProjectReply proto.InternalMessageInfo

func (m *AddProjectReply) GetID() string {
	if m != nil {
		return m.ID
	}
	return ""
}

func (m *AddProjectReply) GetStoreID() string {
	if m != nil {
		return m.StoreID
	}
	return ""
}

func init() {
	proto.RegisterType((*LoginRequest)(nil), "api.pb.LoginRequest")
	proto.RegisterType((*LoginReply)(nil), "api.pb.LoginReply")
	proto.RegisterType((*AddProjectRequest)(nil), "api.pb.AddProjectRequest")
	proto.RegisterType((*AddProjectReply)(nil), "api.pb.AddProjectReply")
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// APIClient is the client API for API service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type APIClient interface {
	Login(ctx context.Context, in *LoginRequest, opts ...grpc.CallOption) (API_LoginClient, error)
	AddProject(ctx context.Context, in *AddProjectRequest, opts ...grpc.CallOption) (*AddProjectReply, error)
}

type aPIClient struct {
	cc *grpc.ClientConn
}

func NewAPIClient(cc *grpc.ClientConn) APIClient {
	return &aPIClient{cc}
}

func (c *aPIClient) Login(ctx context.Context, in *LoginRequest, opts ...grpc.CallOption) (API_LoginClient, error) {
	stream, err := c.cc.NewStream(ctx, &_API_serviceDesc.Streams[0], "/api.pb.API/Login", opts...)
	if err != nil {
		return nil, err
	}
	x := &aPILoginClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type API_LoginClient interface {
	Recv() (*LoginReply, error)
	grpc.ClientStream
}

type aPILoginClient struct {
	grpc.ClientStream
}

func (x *aPILoginClient) Recv() (*LoginReply, error) {
	m := new(LoginReply)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *aPIClient) AddProject(ctx context.Context, in *AddProjectRequest, opts ...grpc.CallOption) (*AddProjectReply, error) {
	out := new(AddProjectReply)
	err := c.cc.Invoke(ctx, "/api.pb.API/AddProject", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// APIServer is the server API for API service.
type APIServer interface {
	Login(*LoginRequest, API_LoginServer) error
	AddProject(context.Context, *AddProjectRequest) (*AddProjectReply, error)
}

func RegisterAPIServer(s *grpc.Server, srv APIServer) {
	s.RegisterService(&_API_serviceDesc, srv)
}

func _API_Login_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(LoginRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(APIServer).Login(m, &aPILoginServer{stream})
}

type API_LoginServer interface {
	Send(*LoginReply) error
	grpc.ServerStream
}

type aPILoginServer struct {
	grpc.ServerStream
}

func (x *aPILoginServer) Send(m *LoginReply) error {
	return x.ServerStream.SendMsg(m)
}

func _API_AddProject_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(AddProjectRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(APIServer).AddProject(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.pb.API/AddProject",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(APIServer).AddProject(ctx, req.(*AddProjectRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _API_serviceDesc = grpc.ServiceDesc{
	ServiceName: "api.pb.API",
	HandlerType: (*APIServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "AddProject",
			Handler:    _API_AddProject_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Login",
			Handler:       _API_Login_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "api.proto",
}

func init() { proto.RegisterFile("api.proto", fileDescriptor_api_e1dcc96785f2904a) }

var fileDescriptor_api_e1dcc96785f2904a = []byte{
	// 262 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x4c, 0x2c, 0xc8, 0xd4,
	0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x03, 0x33, 0x93, 0x94, 0x54, 0xb8, 0x78, 0x7c, 0xf2,
	0xd3, 0x33, 0xf3, 0x82, 0x52, 0x0b, 0x4b, 0x53, 0x8b, 0x4b, 0x84, 0x44, 0xb8, 0x58, 0x53, 0x73,
	0x13, 0x33, 0x73, 0x24, 0x18, 0x15, 0x18, 0x35, 0x38, 0x83, 0x20, 0x1c, 0x25, 0x23, 0x2e, 0x2e,
	0xa8, 0xaa, 0x82, 0x9c, 0x4a, 0x21, 0x3e, 0x2e, 0x26, 0x4f, 0x17, 0xa8, 0x02, 0x26, 0x4f, 0x17,
	0x90, 0x9e, 0x92, 0xfc, 0xec, 0xd4, 0x3c, 0x09, 0x26, 0x88, 0x1e, 0x30, 0x47, 0xc9, 0x91, 0x4b,
	0xd0, 0x31, 0x25, 0x25, 0xa0, 0x28, 0x3f, 0x2b, 0x35, 0xb9, 0x04, 0x66, 0xbc, 0x10, 0x17, 0x4b,
	0x5e, 0x62, 0x6e, 0x2a, 0x54, 0x33, 0x98, 0x2d, 0x24, 0xc1, 0xc5, 0x9e, 0x5e, 0x94, 0x5f, 0x5a,
	0xe0, 0xe9, 0x02, 0x35, 0x00, 0xc6, 0x55, 0xb2, 0xe6, 0xe2, 0x47, 0x36, 0x02, 0x9b, 0xdd, 0x12,
	0x5c, 0xec, 0xc5, 0x25, 0xf9, 0x45, 0xa9, 0x08, 0xcd, 0x50, 0xae, 0x51, 0x03, 0x23, 0x17, 0xb3,
	0x63, 0x80, 0xa7, 0x90, 0x29, 0x17, 0x2b, 0xd8, 0xed, 0x42, 0x22, 0x7a, 0x10, 0x3f, 0xeb, 0x21,
	0x7b, 0x58, 0x4a, 0x08, 0x4d, 0xb4, 0x20, 0xa7, 0x52, 0x89, 0xc1, 0x80, 0x51, 0xc8, 0x89, 0x8b,
	0x0b, 0x61, 0xb7, 0x90, 0x24, 0x4c, 0x15, 0x86, 0x97, 0xa4, 0xc4, 0xb1, 0x49, 0x81, 0x4d, 0x71,
	0xd2, 0xe6, 0x12, 0xcf, 0xcc, 0xd7, 0x2b, 0x49, 0xad, 0x28, 0xc9, 0xcc, 0x49, 0xd5, 0x4b, 0x2f,
	0x2a, 0x48, 0x8e, 0x87, 0x72, 0x9c, 0xd8, 0x43, 0x20, 0x8c, 0x00, 0xc6, 0x45, 0x4c, 0x2c, 0x21,
	0x11, 0x21, 0x3e, 0x49, 0x6c, 0xe0, 0x88, 0x31, 0x06, 0x04, 0x00, 0x00, 0xff, 0xff, 0x7c, 0xa0,
	0xd1, 0x47, 0xa5, 0x01, 0x00, 0x00,
}
