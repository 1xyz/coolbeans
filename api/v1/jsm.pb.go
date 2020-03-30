// Code generated by protoc-gen-go. DO NOT EDIT.
// source: jsm.proto

package coolbeans_api_v1

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

type PutRequest struct {
	// Refer Job.priority
	Priority uint32 `protobuf:"varint,1,opt,name=priority,proto3" json:"priority,omitempty"`
	// Refer Job.delay
	Delay int64 `protobuf:"varint,2,opt,name=delay,proto3" json:"delay,omitempty"`
	// Refer Job.ttr
	Ttr int32 `protobuf:"varint,3,opt,name=ttr,proto3" json:"ttr,omitempty"`
	// Refer Job.tube_na,e
	TubeName string `protobuf:"bytes,4,opt,name=tube_name,json=tubeName,proto3" json:"tube_name,omitempty"`
	// Refer Job.body_size
	BodySize int32 `protobuf:"varint,5,opt,name=body_size,json=bodySize,proto3" json:"body_size,omitempty"`
	// Refer Job.body
	Body                 []byte   `protobuf:"bytes,6,opt,name=body,proto3" json:"body,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *PutRequest) Reset()         { *m = PutRequest{} }
func (m *PutRequest) String() string { return proto.CompactTextString(m) }
func (*PutRequest) ProtoMessage()    {}
func (*PutRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_cf6c16014e2432ed, []int{0}
}

func (m *PutRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PutRequest.Unmarshal(m, b)
}
func (m *PutRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PutRequest.Marshal(b, m, deterministic)
}
func (m *PutRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PutRequest.Merge(m, src)
}
func (m *PutRequest) XXX_Size() int {
	return xxx_messageInfo_PutRequest.Size(m)
}
func (m *PutRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_PutRequest.DiscardUnknown(m)
}

var xxx_messageInfo_PutRequest proto.InternalMessageInfo

func (m *PutRequest) GetPriority() uint32 {
	if m != nil {
		return m.Priority
	}
	return 0
}

func (m *PutRequest) GetDelay() int64 {
	if m != nil {
		return m.Delay
	}
	return 0
}

func (m *PutRequest) GetTtr() int32 {
	if m != nil {
		return m.Ttr
	}
	return 0
}

func (m *PutRequest) GetTubeName() string {
	if m != nil {
		return m.TubeName
	}
	return ""
}

func (m *PutRequest) GetBodySize() int32 {
	if m != nil {
		return m.BodySize
	}
	return 0
}

func (m *PutRequest) GetBody() []byte {
	if m != nil {
		return m.Body
	}
	return nil
}

type PutResponse struct {
	// The job identifier of the new job created
	JobId                int64    `protobuf:"varint,1,opt,name=job_id,json=jobId,proto3" json:"job_id,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *PutResponse) Reset()         { *m = PutResponse{} }
func (m *PutResponse) String() string { return proto.CompactTextString(m) }
func (*PutResponse) ProtoMessage()    {}
func (*PutResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_cf6c16014e2432ed, []int{1}
}

func (m *PutResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PutResponse.Unmarshal(m, b)
}
func (m *PutResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PutResponse.Marshal(b, m, deterministic)
}
func (m *PutResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PutResponse.Merge(m, src)
}
func (m *PutResponse) XXX_Size() int {
	return xxx_messageInfo_PutResponse.Size(m)
}
func (m *PutResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_PutResponse.DiscardUnknown(m)
}

var xxx_messageInfo_PutResponse proto.InternalMessageInfo

func (m *PutResponse) GetJobId() int64 {
	if m != nil {
		return m.JobId
	}
	return 0
}

type DeleteRequest struct {
	// The job identifier of the job to be deleted
	JobId                int64    `protobuf:"varint,1,opt,name=job_id,json=jobId,proto3" json:"job_id,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *DeleteRequest) Reset()         { *m = DeleteRequest{} }
func (m *DeleteRequest) String() string { return proto.CompactTextString(m) }
func (*DeleteRequest) ProtoMessage()    {}
func (*DeleteRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_cf6c16014e2432ed, []int{2}
}

func (m *DeleteRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_DeleteRequest.Unmarshal(m, b)
}
func (m *DeleteRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_DeleteRequest.Marshal(b, m, deterministic)
}
func (m *DeleteRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DeleteRequest.Merge(m, src)
}
func (m *DeleteRequest) XXX_Size() int {
	return xxx_messageInfo_DeleteRequest.Size(m)
}
func (m *DeleteRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_DeleteRequest.DiscardUnknown(m)
}

var xxx_messageInfo_DeleteRequest proto.InternalMessageInfo

func (m *DeleteRequest) GetJobId() int64 {
	if m != nil {
		return m.JobId
	}
	return 0
}

type ReserveRequest struct {
	// client id of the reservation
	ClientId string `protobuf:"bytes,1,opt,name=client_id,json=clientId,proto3" json:"client_id,omitempty"`
	// reservation timeout in seconds
	TimeoutSecs int32 `protobuf:"varint,2,opt,name=timeout_secs,json=timeoutSecs,proto3" json:"timeout_secs,omitempty"`
	// array of tubes to watch
	Tubes                []string `protobuf:"bytes,3,rep,name=tubes,proto3" json:"tubes,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ReserveRequest) Reset()         { *m = ReserveRequest{} }
func (m *ReserveRequest) String() string { return proto.CompactTextString(m) }
func (*ReserveRequest) ProtoMessage()    {}
func (*ReserveRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_cf6c16014e2432ed, []int{3}
}

func (m *ReserveRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ReserveRequest.Unmarshal(m, b)
}
func (m *ReserveRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ReserveRequest.Marshal(b, m, deterministic)
}
func (m *ReserveRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ReserveRequest.Merge(m, src)
}
func (m *ReserveRequest) XXX_Size() int {
	return xxx_messageInfo_ReserveRequest.Size(m)
}
func (m *ReserveRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_ReserveRequest.DiscardUnknown(m)
}

var xxx_messageInfo_ReserveRequest proto.InternalMessageInfo

func (m *ReserveRequest) GetClientId() string {
	if m != nil {
		return m.ClientId
	}
	return ""
}

func (m *ReserveRequest) GetTimeoutSecs() int32 {
	if m != nil {
		return m.TimeoutSecs
	}
	return 0
}

func (m *ReserveRequest) GetTubes() []string {
	if m != nil {
		return m.Tubes
	}
	return nil
}

type ReserveResponse struct {
	// id of the client assigned to the reservation
	ClientId string `protobuf:"bytes,1,opt,name=client_id,json=clientId,proto3" json:"client_id,omitempty"`
	// if of the response
	ReservedJob          *JobProto `protobuf:"bytes,2,opt,name=reserved_job,json=reservedJob,proto3" json:"reserved_job,omitempty"`
	XXX_NoUnkeyedLiteral struct{}  `json:"-"`
	XXX_unrecognized     []byte    `json:"-"`
	XXX_sizecache        int32     `json:"-"`
}

func (m *ReserveResponse) Reset()         { *m = ReserveResponse{} }
func (m *ReserveResponse) String() string { return proto.CompactTextString(m) }
func (*ReserveResponse) ProtoMessage()    {}
func (*ReserveResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_cf6c16014e2432ed, []int{4}
}

func (m *ReserveResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ReserveResponse.Unmarshal(m, b)
}
func (m *ReserveResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ReserveResponse.Marshal(b, m, deterministic)
}
func (m *ReserveResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ReserveResponse.Merge(m, src)
}
func (m *ReserveResponse) XXX_Size() int {
	return xxx_messageInfo_ReserveResponse.Size(m)
}
func (m *ReserveResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_ReserveResponse.DiscardUnknown(m)
}

var xxx_messageInfo_ReserveResponse proto.InternalMessageInfo

func (m *ReserveResponse) GetClientId() string {
	if m != nil {
		return m.ClientId
	}
	return ""
}

func (m *ReserveResponse) GetReservedJob() *JobProto {
	if m != nil {
		return m.ReservedJob
	}
	return nil
}

type ReleaseRequest struct {
	// The job identifier of the job to be released
	JobId int64 `protobuf:"varint,1,opt,name=job_id,json=jobId,proto3" json:"job_id,omitempty"`
	// The identifier of the client asking for the release
	ClientId int64 `protobuf:"varint,2,opt,name=client_id,json=clientId,proto3" json:"client_id,omitempty"`
	// A delay if set to a value > 0 marks the job as delayed
	Delay                int32    `protobuf:"varint,3,opt,name=delay,proto3" json:"delay,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ReleaseRequest) Reset()         { *m = ReleaseRequest{} }
func (m *ReleaseRequest) String() string { return proto.CompactTextString(m) }
func (*ReleaseRequest) ProtoMessage()    {}
func (*ReleaseRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_cf6c16014e2432ed, []int{5}
}

func (m *ReleaseRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ReleaseRequest.Unmarshal(m, b)
}
func (m *ReleaseRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ReleaseRequest.Marshal(b, m, deterministic)
}
func (m *ReleaseRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ReleaseRequest.Merge(m, src)
}
func (m *ReleaseRequest) XXX_Size() int {
	return xxx_messageInfo_ReleaseRequest.Size(m)
}
func (m *ReleaseRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_ReleaseRequest.DiscardUnknown(m)
}

var xxx_messageInfo_ReleaseRequest proto.InternalMessageInfo

func (m *ReleaseRequest) GetJobId() int64 {
	if m != nil {
		return m.JobId
	}
	return 0
}

func (m *ReleaseRequest) GetClientId() int64 {
	if m != nil {
		return m.ClientId
	}
	return 0
}

func (m *ReleaseRequest) GetDelay() int32 {
	if m != nil {
		return m.Delay
	}
	return 0
}

type TouchRequest struct {
	// The job identifier of the job to be touched
	JobId                int64    `protobuf:"varint,1,opt,name=job_id,json=jobId,proto3" json:"job_id,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *TouchRequest) Reset()         { *m = TouchRequest{} }
func (m *TouchRequest) String() string { return proto.CompactTextString(m) }
func (*TouchRequest) ProtoMessage()    {}
func (*TouchRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_cf6c16014e2432ed, []int{6}
}

func (m *TouchRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_TouchRequest.Unmarshal(m, b)
}
func (m *TouchRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_TouchRequest.Marshal(b, m, deterministic)
}
func (m *TouchRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_TouchRequest.Merge(m, src)
}
func (m *TouchRequest) XXX_Size() int {
	return xxx_messageInfo_TouchRequest.Size(m)
}
func (m *TouchRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_TouchRequest.DiscardUnknown(m)
}

var xxx_messageInfo_TouchRequest proto.InternalMessageInfo

func (m *TouchRequest) GetJobId() int64 {
	if m != nil {
		return m.JobId
	}
	return 0
}

type BuryRequest struct {
	// The job identifier of the job to be buried
	JobId                int64    `protobuf:"varint,1,opt,name=job_id,json=jobId,proto3" json:"job_id,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *BuryRequest) Reset()         { *m = BuryRequest{} }
func (m *BuryRequest) String() string { return proto.CompactTextString(m) }
func (*BuryRequest) ProtoMessage()    {}
func (*BuryRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_cf6c16014e2432ed, []int{7}
}

func (m *BuryRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_BuryRequest.Unmarshal(m, b)
}
func (m *BuryRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_BuryRequest.Marshal(b, m, deterministic)
}
func (m *BuryRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BuryRequest.Merge(m, src)
}
func (m *BuryRequest) XXX_Size() int {
	return xxx_messageInfo_BuryRequest.Size(m)
}
func (m *BuryRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_BuryRequest.DiscardUnknown(m)
}

var xxx_messageInfo_BuryRequest proto.InternalMessageInfo

func (m *BuryRequest) GetJobId() int64 {
	if m != nil {
		return m.JobId
	}
	return 0
}

type KickRequest struct {
	// The job identifier of the job to be Kicked
	JobId                int64    `protobuf:"varint,1,opt,name=job_id,json=jobId,proto3" json:"job_id,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *KickRequest) Reset()         { *m = KickRequest{} }
func (m *KickRequest) String() string { return proto.CompactTextString(m) }
func (*KickRequest) ProtoMessage()    {}
func (*KickRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_cf6c16014e2432ed, []int{8}
}

func (m *KickRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_KickRequest.Unmarshal(m, b)
}
func (m *KickRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_KickRequest.Marshal(b, m, deterministic)
}
func (m *KickRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_KickRequest.Merge(m, src)
}
func (m *KickRequest) XXX_Size() int {
	return xxx_messageInfo_KickRequest.Size(m)
}
func (m *KickRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_KickRequest.DiscardUnknown(m)
}

var xxx_messageInfo_KickRequest proto.InternalMessageInfo

func (m *KickRequest) GetJobId() int64 {
	if m != nil {
		return m.JobId
	}
	return 0
}

// Encapsulates a snap of the entire system
type SnapshotProto struct {
	// Array of all jobs currently in the system
	Jobs []*JobProto `protobuf:"bytes,1,rep,name=jobs,proto3" json:"jobs,omitempty"`
	// Arrayy of all client reservation entries in the system
	Reservations         []*ClientResvEntryProto `protobuf:"bytes,2,rep,name=reservations,proto3" json:"reservations,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                `json:"-"`
	XXX_unrecognized     []byte                  `json:"-"`
	XXX_sizecache        int32                   `json:"-"`
}

func (m *SnapshotProto) Reset()         { *m = SnapshotProto{} }
func (m *SnapshotProto) String() string { return proto.CompactTextString(m) }
func (*SnapshotProto) ProtoMessage()    {}
func (*SnapshotProto) Descriptor() ([]byte, []int) {
	return fileDescriptor_cf6c16014e2432ed, []int{9}
}

func (m *SnapshotProto) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_SnapshotProto.Unmarshal(m, b)
}
func (m *SnapshotProto) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_SnapshotProto.Marshal(b, m, deterministic)
}
func (m *SnapshotProto) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SnapshotProto.Merge(m, src)
}
func (m *SnapshotProto) XXX_Size() int {
	return xxx_messageInfo_SnapshotProto.Size(m)
}
func (m *SnapshotProto) XXX_DiscardUnknown() {
	xxx_messageInfo_SnapshotProto.DiscardUnknown(m)
}

var xxx_messageInfo_SnapshotProto proto.InternalMessageInfo

func (m *SnapshotProto) GetJobs() []*JobProto {
	if m != nil {
		return m.Jobs
	}
	return nil
}

func (m *SnapshotProto) GetReservations() []*ClientResvEntryProto {
	if m != nil {
		return m.Reservations
	}
	return nil
}

func init() {
	proto.RegisterType((*PutRequest)(nil), "coolbeans.api.v1.PutRequest")
	proto.RegisterType((*PutResponse)(nil), "coolbeans.api.v1.PutResponse")
	proto.RegisterType((*DeleteRequest)(nil), "coolbeans.api.v1.DeleteRequest")
	proto.RegisterType((*ReserveRequest)(nil), "coolbeans.api.v1.ReserveRequest")
	proto.RegisterType((*ReserveResponse)(nil), "coolbeans.api.v1.ReserveResponse")
	proto.RegisterType((*ReleaseRequest)(nil), "coolbeans.api.v1.ReleaseRequest")
	proto.RegisterType((*TouchRequest)(nil), "coolbeans.api.v1.TouchRequest")
	proto.RegisterType((*BuryRequest)(nil), "coolbeans.api.v1.BuryRequest")
	proto.RegisterType((*KickRequest)(nil), "coolbeans.api.v1.KickRequest")
	proto.RegisterType((*SnapshotProto)(nil), "coolbeans.api.v1.SnapshotProto")
}

func init() {
	proto.RegisterFile("jsm.proto", fileDescriptor_cf6c16014e2432ed)
}

var fileDescriptor_cf6c16014e2432ed = []byte{
	// 574 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x84, 0x54, 0x4d, 0x6f, 0xd3, 0x40,
	0x10, 0xad, 0x71, 0x12, 0xe2, 0x49, 0x4a, 0xab, 0x15, 0x08, 0xcb, 0xe5, 0xc3, 0xb5, 0x4a, 0xe5,
	0x53, 0x04, 0xe5, 0x8c, 0x84, 0x4a, 0x8b, 0xd4, 0x20, 0x50, 0xb5, 0xe9, 0x89, 0x4b, 0xe4, 0x75,
	0x06, 0xd5, 0x26, 0xf6, 0x1a, 0xef, 0xba, 0x92, 0x7b, 0xe5, 0xc8, 0x7f, 0xe0, 0xb7, 0xa2, 0xdd,
	0x4d, 0xd2, 0x04, 0xb7, 0xf1, 0x6d, 0x67, 0xf6, 0xcd, 0xdb, 0x79, 0x33, 0xcf, 0x06, 0x27, 0x15,
	0xd9, 0xa8, 0x28, 0xb9, 0xe4, 0x64, 0x3f, 0xe6, 0x7c, 0xce, 0x30, 0xca, 0xc5, 0x28, 0x2a, 0x92,
	0xd1, 0xcd, 0x3b, 0x6f, 0x18, 0xcf, 0x13, 0xcc, 0xa5, 0xb9, 0xf7, 0x9c, 0x94, 0xb3, 0xc5, 0x71,
	0x80, 0x59, 0x21, 0x6b, 0x13, 0x04, 0x7f, 0x2d, 0x80, 0xcb, 0x4a, 0x52, 0xfc, 0x55, 0xa1, 0x90,
	0xc4, 0x83, 0x7e, 0x51, 0x26, 0xbc, 0x4c, 0x64, 0xed, 0x5a, 0xbe, 0x15, 0xee, 0xd2, 0x55, 0x4c,
	0x9e, 0x42, 0x77, 0x86, 0xf3, 0xa8, 0x76, 0x1f, 0xf9, 0x56, 0x68, 0x53, 0x13, 0x90, 0x7d, 0xb0,
	0xa5, 0x2c, 0x5d, 0xdb, 0xb7, 0xc2, 0x2e, 0x55, 0x47, 0x72, 0x00, 0x8e, 0xac, 0x18, 0x4e, 0xf3,
	0x28, 0x43, 0xb7, 0xe3, 0x5b, 0xa1, 0x43, 0xfb, 0x2a, 0xf1, 0x2d, 0xca, 0x50, 0x5d, 0x32, 0x3e,
	0xab, 0xa7, 0x22, 0xb9, 0x45, 0xb7, 0xab, 0x8b, 0xfa, 0x2a, 0x31, 0x49, 0x6e, 0x91, 0x10, 0xe8,
	0xa8, 0xb3, 0xdb, 0xf3, 0xad, 0x70, 0x48, 0xf5, 0x39, 0x38, 0x82, 0x81, 0xee, 0x4f, 0x14, 0x3c,
	0x17, 0x48, 0x9e, 0x41, 0x2f, 0xe5, 0x6c, 0x9a, 0xcc, 0x74, 0x7b, 0x36, 0xed, 0xa6, 0x9c, 0x5d,
	0xcc, 0x82, 0x63, 0xd8, 0x3d, 0xc3, 0x39, 0x4a, 0x5c, 0x0a, 0x79, 0x00, 0xf7, 0x03, 0x9e, 0x50,
	0x14, 0x58, 0xde, 0xac, 0x80, 0x07, 0xe0, 0x98, 0x41, 0x2d, 0xb1, 0x0e, 0xed, 0x9b, 0xc4, 0xc5,
	0x8c, 0x1c, 0xc2, 0x50, 0x26, 0x19, 0xf2, 0x4a, 0x4e, 0x05, 0xc6, 0x42, 0x2b, 0xef, 0xd2, 0xc1,
	0x22, 0x37, 0xc1, 0x58, 0xa8, 0xa9, 0x28, 0x71, 0xc2, 0xb5, 0x7d, 0x3b, 0x74, 0xa8, 0x09, 0x82,
	0x0c, 0xf6, 0x56, 0xef, 0x2c, 0x3a, 0xdf, 0xfa, 0xd0, 0x07, 0x18, 0x96, 0x06, 0x3f, 0x9b, 0xa6,
	0x9c, 0xe9, 0x87, 0x06, 0x27, 0xde, 0xe8, 0xff, 0xad, 0x8e, 0xc6, 0x9c, 0x5d, 0xaa, 0xc5, 0xd1,
	0xc1, 0x12, 0x3f, 0xe6, 0x2c, 0xf8, 0xae, 0x64, 0xcd, 0x31, 0x12, 0x2d, 0xfa, 0x37, 0x9b, 0x30,
	0x7b, 0xbc, 0x6b, 0x62, 0xb5, 0x60, 0xb3, 0x4c, 0x13, 0x04, 0x6f, 0x60, 0x78, 0xc5, 0xab, 0xf8,
	0xba, 0x65, 0xb2, 0x47, 0x30, 0x38, 0xad, 0xca, 0xba, 0x1d, 0xf5, 0x25, 0x89, 0x7f, 0xb6, 0xa0,
	0xfe, 0x58, 0xb0, 0x3b, 0xc9, 0xa3, 0x42, 0x5c, 0x73, 0xa9, 0xd5, 0x92, 0x11, 0x74, 0x52, 0xce,
	0x84, 0x6b, 0xf9, 0x76, 0xcb, 0x5c, 0x34, 0x8e, 0x8c, 0x97, 0xf3, 0x8c, 0x64, 0xc2, 0x73, 0xb5,
	0x38, 0x55, 0x77, 0xdc, 0xac, 0xfb, 0xa4, 0xc5, 0x53, 0x14, 0x37, 0xe7, 0xb9, 0x2c, 0x6b, 0xc3,
	0xb1, 0x51, 0x7b, 0xf2, 0xbb, 0x03, 0x7b, 0x63, 0xce, 0x26, 0x32, 0x92, 0xf8, 0x35, 0x8a, 0xaf,
	0x93, 0x1c, 0xc9, 0x19, 0xd8, 0x97, 0x95, 0x24, 0x2f, 0x9a, 0x84, 0x77, 0x1f, 0x93, 0xf7, 0xf2,
	0x81, 0x5b, 0x63, 0x88, 0x60, 0x87, 0x9c, 0x41, 0xcf, 0xb8, 0x96, 0xbc, 0x6e, 0x42, 0x37, 0xfc,
	0xec, 0x3d, 0x6f, 0x02, 0xce, 0xd5, 0x67, 0x1c, 0xec, 0x90, 0x2b, 0x78, 0xbc, 0xf0, 0x1a, 0xf1,
	0x9b, 0xa8, 0x4d, 0xbb, 0x7b, 0x87, 0x5b, 0x10, 0xcb, 0xbe, 0x42, 0xeb, 0xad, 0x45, 0x3e, 0x2b,
	0x56, 0x6d, 0xa9, 0xfb, 0x59, 0xd7, 0xdd, 0xb6, 0xad, 0xbb, 0x53, 0xe8, 0x6a, 0xfb, 0x90, 0x57,
	0x4d, 0xcc, 0xba, 0xaf, 0xb6, 0x71, 0x7c, 0x84, 0x8e, 0xf2, 0x16, 0xb9, 0x67, 0xa0, 0x6b, 0x9e,
	0x6b, 0x61, 0x50, 0xbe, 0xbb, 0x8f, 0x61, 0xcd, 0x8f, 0x5b, 0x18, 0x58, 0x4f, 0xff, 0x2f, 0xdf,
	0xff, 0x0b, 0x00, 0x00, 0xff, 0xff, 0xd6, 0x62, 0xca, 0xed, 0x74, 0x05, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConnInterface

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion6

// JobStateMachineClient is the client API for JobStateMachine service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type JobStateMachineClient interface {
	// Put creates a new job with the provided job parameters
	//
	// The response contains the identifier of the job created
	Put(ctx context.Context, in *PutRequest, opts ...grpc.CallOption) (*PutResponse, error)
	// Delete a job with the provided job id
	Delete(ctx context.Context, in *DeleteRequest, opts ...grpc.CallOption) (*Empty, error)
	// Reserve is a bi-directional streaming RPC
	//
	// Accepts a stream of reservation requests. Reservation are streamed
	// as they are available.
	Reserve(ctx context.Context, opts ...grpc.CallOption) (JobStateMachine_ReserveClient, error)
	// Release a reserved job back to either a Ready or a Delayed state
	Release(ctx context.Context, in *ReleaseRequest, opts ...grpc.CallOption) (*Empty, error)
	// Extend a reserved job's reservation TTL by its TTR (time-to-run)
	Touch(ctx context.Context, in *TouchRequest, opts ...grpc.CallOption) (*Empty, error)
	// Bury this job, if this job is in the reserved state
	Bury(ctx context.Context, in *BuryRequest, opts ...grpc.CallOption) (*Empty, error)
	// Kick this job, if this job is in a buried stated to ready state
	Kick(ctx context.Context, in *KickRequest, opts ...grpc.CallOption) (*Empty, error)
}

type jobStateMachineClient struct {
	cc grpc.ClientConnInterface
}

func NewJobStateMachineClient(cc grpc.ClientConnInterface) JobStateMachineClient {
	return &jobStateMachineClient{cc}
}

func (c *jobStateMachineClient) Put(ctx context.Context, in *PutRequest, opts ...grpc.CallOption) (*PutResponse, error) {
	out := new(PutResponse)
	err := c.cc.Invoke(ctx, "/coolbeans.api.v1.JobStateMachine/Put", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *jobStateMachineClient) Delete(ctx context.Context, in *DeleteRequest, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := c.cc.Invoke(ctx, "/coolbeans.api.v1.JobStateMachine/Delete", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *jobStateMachineClient) Reserve(ctx context.Context, opts ...grpc.CallOption) (JobStateMachine_ReserveClient, error) {
	stream, err := c.cc.NewStream(ctx, &_JobStateMachine_serviceDesc.Streams[0], "/coolbeans.api.v1.JobStateMachine/Reserve", opts...)
	if err != nil {
		return nil, err
	}
	x := &jobStateMachineReserveClient{stream}
	return x, nil
}

type JobStateMachine_ReserveClient interface {
	Send(*ReserveRequest) error
	Recv() (*ReserveResponse, error)
	grpc.ClientStream
}

type jobStateMachineReserveClient struct {
	grpc.ClientStream
}

func (x *jobStateMachineReserveClient) Send(m *ReserveRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *jobStateMachineReserveClient) Recv() (*ReserveResponse, error) {
	m := new(ReserveResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *jobStateMachineClient) Release(ctx context.Context, in *ReleaseRequest, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := c.cc.Invoke(ctx, "/coolbeans.api.v1.JobStateMachine/Release", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *jobStateMachineClient) Touch(ctx context.Context, in *TouchRequest, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := c.cc.Invoke(ctx, "/coolbeans.api.v1.JobStateMachine/Touch", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *jobStateMachineClient) Bury(ctx context.Context, in *BuryRequest, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := c.cc.Invoke(ctx, "/coolbeans.api.v1.JobStateMachine/Bury", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *jobStateMachineClient) Kick(ctx context.Context, in *KickRequest, opts ...grpc.CallOption) (*Empty, error) {
	out := new(Empty)
	err := c.cc.Invoke(ctx, "/coolbeans.api.v1.JobStateMachine/Kick", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// JobStateMachineServer is the server API for JobStateMachine service.
type JobStateMachineServer interface {
	// Put creates a new job with the provided job parameters
	//
	// The response contains the identifier of the job created
	Put(context.Context, *PutRequest) (*PutResponse, error)
	// Delete a job with the provided job id
	Delete(context.Context, *DeleteRequest) (*Empty, error)
	// Reserve is a bi-directional streaming RPC
	//
	// Accepts a stream of reservation requests. Reservation are streamed
	// as they are available.
	Reserve(JobStateMachine_ReserveServer) error
	// Release a reserved job back to either a Ready or a Delayed state
	Release(context.Context, *ReleaseRequest) (*Empty, error)
	// Extend a reserved job's reservation TTL by its TTR (time-to-run)
	Touch(context.Context, *TouchRequest) (*Empty, error)
	// Bury this job, if this job is in the reserved state
	Bury(context.Context, *BuryRequest) (*Empty, error)
	// Kick this job, if this job is in a buried stated to ready state
	Kick(context.Context, *KickRequest) (*Empty, error)
}

// UnimplementedJobStateMachineServer can be embedded to have forward compatible implementations.
type UnimplementedJobStateMachineServer struct {
}

func (*UnimplementedJobStateMachineServer) Put(ctx context.Context, req *PutRequest) (*PutResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Put not implemented")
}
func (*UnimplementedJobStateMachineServer) Delete(ctx context.Context, req *DeleteRequest) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Delete not implemented")
}
func (*UnimplementedJobStateMachineServer) Reserve(srv JobStateMachine_ReserveServer) error {
	return status.Errorf(codes.Unimplemented, "method Reserve not implemented")
}
func (*UnimplementedJobStateMachineServer) Release(ctx context.Context, req *ReleaseRequest) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Release not implemented")
}
func (*UnimplementedJobStateMachineServer) Touch(ctx context.Context, req *TouchRequest) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Touch not implemented")
}
func (*UnimplementedJobStateMachineServer) Bury(ctx context.Context, req *BuryRequest) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Bury not implemented")
}
func (*UnimplementedJobStateMachineServer) Kick(ctx context.Context, req *KickRequest) (*Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Kick not implemented")
}

func RegisterJobStateMachineServer(s *grpc.Server, srv JobStateMachineServer) {
	s.RegisterService(&_JobStateMachine_serviceDesc, srv)
}

func _JobStateMachine_Put_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PutRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(JobStateMachineServer).Put(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/coolbeans.api.v1.JobStateMachine/Put",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(JobStateMachineServer).Put(ctx, req.(*PutRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _JobStateMachine_Delete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(JobStateMachineServer).Delete(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/coolbeans.api.v1.JobStateMachine/Delete",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(JobStateMachineServer).Delete(ctx, req.(*DeleteRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _JobStateMachine_Reserve_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(JobStateMachineServer).Reserve(&jobStateMachineReserveServer{stream})
}

type JobStateMachine_ReserveServer interface {
	Send(*ReserveResponse) error
	Recv() (*ReserveRequest, error)
	grpc.ServerStream
}

type jobStateMachineReserveServer struct {
	grpc.ServerStream
}

func (x *jobStateMachineReserveServer) Send(m *ReserveResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *jobStateMachineReserveServer) Recv() (*ReserveRequest, error) {
	m := new(ReserveRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _JobStateMachine_Release_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReleaseRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(JobStateMachineServer).Release(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/coolbeans.api.v1.JobStateMachine/Release",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(JobStateMachineServer).Release(ctx, req.(*ReleaseRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _JobStateMachine_Touch_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TouchRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(JobStateMachineServer).Touch(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/coolbeans.api.v1.JobStateMachine/Touch",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(JobStateMachineServer).Touch(ctx, req.(*TouchRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _JobStateMachine_Bury_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(BuryRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(JobStateMachineServer).Bury(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/coolbeans.api.v1.JobStateMachine/Bury",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(JobStateMachineServer).Bury(ctx, req.(*BuryRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _JobStateMachine_Kick_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(KickRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(JobStateMachineServer).Kick(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/coolbeans.api.v1.JobStateMachine/Kick",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(JobStateMachineServer).Kick(ctx, req.(*KickRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _JobStateMachine_serviceDesc = grpc.ServiceDesc{
	ServiceName: "coolbeans.api.v1.JobStateMachine",
	HandlerType: (*JobStateMachineServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Put",
			Handler:    _JobStateMachine_Put_Handler,
		},
		{
			MethodName: "Delete",
			Handler:    _JobStateMachine_Delete_Handler,
		},
		{
			MethodName: "Release",
			Handler:    _JobStateMachine_Release_Handler,
		},
		{
			MethodName: "Touch",
			Handler:    _JobStateMachine_Touch_Handler,
		},
		{
			MethodName: "Bury",
			Handler:    _JobStateMachine_Bury_Handler,
		},
		{
			MethodName: "Kick",
			Handler:    _JobStateMachine_Kick_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Reserve",
			Handler:       _JobStateMachine_Reserve_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "jsm.proto",
}
