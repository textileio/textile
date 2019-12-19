// Code generated by protoc-gen-go. DO NOT EDIT.
// source: api.proto

package api_pb

import (
	context "context"
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

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
	return fileDescriptor_00212fb1f9d3bf1c, []int{0}
}

func (m *LoginRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_LoginRequest.Unmarshal(m, b)
}
func (m *LoginRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_LoginRequest.Marshal(b, m, deterministic)
}
func (m *LoginRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_LoginRequest.Merge(m, src)
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
	return fileDescriptor_00212fb1f9d3bf1c, []int{1}
}

func (m *LoginReply) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_LoginReply.Unmarshal(m, b)
}
func (m *LoginReply) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_LoginReply.Marshal(b, m, deterministic)
}
func (m *LoginReply) XXX_Merge(src proto.Message) {
	xxx_messageInfo_LoginReply.Merge(m, src)
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

func init() {
	proto.RegisterType((*LoginRequest)(nil), "api.pb.LoginRequest")
	proto.RegisterType((*LoginReply)(nil), "api.pb.LoginReply")
}

func init() { proto.RegisterFile("api.proto", fileDescriptor_00212fb1f9d3bf1c) }

var fileDescriptor_00212fb1f9d3bf1c = []byte{
	// 180 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x4c, 0x2c, 0xc8, 0xd4,
	0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x03, 0x33, 0x93, 0x94, 0x54, 0xb8, 0x78, 0x7c, 0xf2,
	0xd3, 0x33, 0xf3, 0x82, 0x52, 0x0b, 0x4b, 0x53, 0x8b, 0x4b, 0x84, 0x44, 0xb8, 0x58, 0x53, 0x73,
	0x13, 0x33, 0x73, 0x24, 0x18, 0x15, 0x18, 0x35, 0x38, 0x83, 0x20, 0x1c, 0x25, 0x23, 0x2e, 0x2e,
	0xa8, 0xaa, 0x82, 0x9c, 0x4a, 0x21, 0x3e, 0x2e, 0x26, 0x4f, 0x17, 0xa8, 0x02, 0x26, 0x4f, 0x17,
	0x90, 0x9e, 0x92, 0xfc, 0xec, 0xd4, 0x3c, 0x09, 0x26, 0x88, 0x1e, 0x30, 0xc7, 0xc8, 0x86, 0x8b,
	0xd9, 0x31, 0xc0, 0x53, 0xc8, 0x94, 0x8b, 0x15, 0xac, 0x55, 0x48, 0x44, 0x0f, 0x62, 0xa5, 0x1e,
	0xb2, 0x7d, 0x52, 0x42, 0x68, 0xa2, 0x05, 0x39, 0x95, 0x4a, 0x0c, 0x06, 0x8c, 0x4e, 0xda, 0x5c,
	0xe2, 0x99, 0xf9, 0x7a, 0x25, 0xa9, 0x15, 0x25, 0x99, 0x39, 0xa9, 0x7a, 0xe9, 0x45, 0x05, 0xc9,
	0xf1, 0x50, 0x8e, 0x13, 0x7b, 0x08, 0x84, 0x11, 0xc0, 0xb8, 0x88, 0x89, 0x25, 0x24, 0x22, 0xc4,
	0x27, 0x89, 0x0d, 0xec, 0x27, 0x63, 0x40, 0x00, 0x00, 0x00, 0xff, 0xff, 0xe4, 0xb4, 0xb5, 0xaf,
	0xe0, 0x00, 0x00, 0x00,
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

// APIServer is the server API for API service.
type APIServer interface {
	Login(*LoginRequest, API_LoginServer) error
}

// UnimplementedAPIServer can be embedded to have forward compatible implementations.
type UnimplementedAPIServer struct {
}

func (*UnimplementedAPIServer) Login(req *LoginRequest, srv API_LoginServer) error {
	return status.Errorf(codes.Unimplemented, "method Login not implemented")
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

var _API_serviceDesc = grpc.ServiceDesc{
	ServiceName: "api.pb.API",
	HandlerType: (*APIServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Login",
			Handler:       _API_Login_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "api.proto",
}
