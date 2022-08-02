// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.26.0
// 	protoc        v3.6.1
// source: offersrpc.proto

package offersrpc

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type SendOnionMessageRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The pubkey of the peer to send the message to. As currently implemented
	// the server will make an ad-hoc connection to the peer (if not already
	// connected) to deliver the message.
	//
	// NB: This will reveal your IP address if you are not running tor. In
	// future iterations this API will find a route to the peer and send the
	// message along a blinded path, protecting sender identity.
	Pubkey []byte `protobuf:"bytes,1,opt,name=pubkey,proto3" json:"pubkey,omitempty"`
	// A map of TLV type to encoded value to deliver to the target node. Note
	// that the bytes for each payload will be written directly, so should
	// already be encoded as the recipient is expecting. TLVs >= 64 are
	// reserved for the final hop, so all values in the map must be in this
	// in this range.
	FinalPayloads map[uint64][]byte `protobuf:"bytes,2,rep,name=final_payloads,json=finalPayloads,proto3" json:"final_payloads,omitempty" protobuf_key:"varint,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (x *SendOnionMessageRequest) Reset() {
	*x = SendOnionMessageRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_offersrpc_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SendOnionMessageRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SendOnionMessageRequest) ProtoMessage() {}

func (x *SendOnionMessageRequest) ProtoReflect() protoreflect.Message {
	mi := &file_offersrpc_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SendOnionMessageRequest.ProtoReflect.Descriptor instead.
func (*SendOnionMessageRequest) Descriptor() ([]byte, []int) {
	return file_offersrpc_proto_rawDescGZIP(), []int{0}
}

func (x *SendOnionMessageRequest) GetPubkey() []byte {
	if x != nil {
		return x.Pubkey
	}
	return nil
}

func (x *SendOnionMessageRequest) GetFinalPayloads() map[uint64][]byte {
	if x != nil {
		return x.FinalPayloads
	}
	return nil
}

type SendOnionMessageResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *SendOnionMessageResponse) Reset() {
	*x = SendOnionMessageResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_offersrpc_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SendOnionMessageResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SendOnionMessageResponse) ProtoMessage() {}

func (x *SendOnionMessageResponse) ProtoReflect() protoreflect.Message {
	mi := &file_offersrpc_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SendOnionMessageResponse.ProtoReflect.Descriptor instead.
func (*SendOnionMessageResponse) Descriptor() ([]byte, []int) {
	return file_offersrpc_proto_rawDescGZIP(), []int{1}
}

type DecodeOfferRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The encoded offer string to be decoded.
	Offer string `protobuf:"bytes,1,opt,name=offer,proto3" json:"offer,omitempty"`
}

func (x *DecodeOfferRequest) Reset() {
	*x = DecodeOfferRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_offersrpc_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DecodeOfferRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DecodeOfferRequest) ProtoMessage() {}

func (x *DecodeOfferRequest) ProtoReflect() protoreflect.Message {
	mi := &file_offersrpc_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DecodeOfferRequest.ProtoReflect.Descriptor instead.
func (*DecodeOfferRequest) Descriptor() ([]byte, []int) {
	return file_offersrpc_proto_rawDescGZIP(), []int{2}
}

func (x *DecodeOfferRequest) GetOffer() string {
	if x != nil {
		return x.Offer
	}
	return ""
}

type DecodeOfferResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The decoded offer.
	Offer *Offer `protobuf:"bytes,1,opt,name=offer,proto3" json:"offer,omitempty"`
}

func (x *DecodeOfferResponse) Reset() {
	*x = DecodeOfferResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_offersrpc_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DecodeOfferResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DecodeOfferResponse) ProtoMessage() {}

func (x *DecodeOfferResponse) ProtoReflect() protoreflect.Message {
	mi := &file_offersrpc_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DecodeOfferResponse.ProtoReflect.Descriptor instead.
func (*DecodeOfferResponse) Descriptor() ([]byte, []int) {
	return file_offersrpc_proto_rawDescGZIP(), []int{3}
}

func (x *DecodeOfferResponse) GetOffer() *Offer {
	if x != nil {
		return x.Offer
	}
	return nil
}

type Offer struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Minimum amount is the minimum payment amount that the offer is for,
	// expressed in millisatoshis.
	MinAmountMsat uint64 `protobuf:"varint,1,opt,name=min_amount_msat,json=minAmountMsat,proto3" json:"min_amount_msat,omitempty"`
	// The description of what the offer is for.
	Description string `protobuf:"bytes,2,opt,name=description,proto3" json:"description,omitempty"`
	// The BOLT feature vector for the offers, encoded as a bit vector.
	Features []byte `protobuf:"bytes,3,opt,name=features,proto3" json:"features,omitempty"`
	// The expiry time for the offer, expressed as seconds from the unix epoch.
	ExpiryUnixSeconds uint64 `protobuf:"varint,4,opt,name=expiry_unix_seconds,json=expiryUnixSeconds,proto3" json:"expiry_unix_seconds,omitempty"`
	// The issuer of the offer.
	Issuer string `protobuf:"bytes,5,opt,name=issuer,proto3" json:"issuer,omitempty"`
	// The minimum number of items for the offer.
	MinQuantity uint64 `protobuf:"varint,6,opt,name=min_quantity,json=minQuantity,proto3" json:"min_quantity,omitempty"`
	// The maximum number of items for the offer.
	MaxQuantity uint64 `protobuf:"varint,7,opt,name=max_quantity,json=maxQuantity,proto3" json:"max_quantity,omitempty"`
	// The hex-encoded node ID of the party making the offer, expressed in
	// x-only format.
	NodeId string `protobuf:"bytes,8,opt,name=node_id,json=nodeId,proto3" json:"node_id,omitempty"`
	// The 64 byte bip340 hex-encoded signature for the offer, generated using
	// node_id's corresponding private key.
	Signature string `protobuf:"bytes,9,opt,name=signature,proto3" json:"signature,omitempty"`
}

func (x *Offer) Reset() {
	*x = Offer{}
	if protoimpl.UnsafeEnabled {
		mi := &file_offersrpc_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Offer) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Offer) ProtoMessage() {}

func (x *Offer) ProtoReflect() protoreflect.Message {
	mi := &file_offersrpc_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Offer.ProtoReflect.Descriptor instead.
func (*Offer) Descriptor() ([]byte, []int) {
	return file_offersrpc_proto_rawDescGZIP(), []int{4}
}

func (x *Offer) GetMinAmountMsat() uint64 {
	if x != nil {
		return x.MinAmountMsat
	}
	return 0
}

func (x *Offer) GetDescription() string {
	if x != nil {
		return x.Description
	}
	return ""
}

func (x *Offer) GetFeatures() []byte {
	if x != nil {
		return x.Features
	}
	return nil
}

func (x *Offer) GetExpiryUnixSeconds() uint64 {
	if x != nil {
		return x.ExpiryUnixSeconds
	}
	return 0
}

func (x *Offer) GetIssuer() string {
	if x != nil {
		return x.Issuer
	}
	return ""
}

func (x *Offer) GetMinQuantity() uint64 {
	if x != nil {
		return x.MinQuantity
	}
	return 0
}

func (x *Offer) GetMaxQuantity() uint64 {
	if x != nil {
		return x.MaxQuantity
	}
	return 0
}

func (x *Offer) GetNodeId() string {
	if x != nil {
		return x.NodeId
	}
	return ""
}

func (x *Offer) GetSignature() string {
	if x != nil {
		return x.Signature
	}
	return ""
}

type SubscribeOnionPayloadRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Onion messages reserve tlv values 64 and above for message intended for
	// the final node. These tlv records are considered "sub-namespaces", which
	// themselves carry a tlv stream for the end application. The tlv type
	// provided here will register a subscription for any onion messages that
	// are delivered to our node that populate the tlv type specified.
	TlvType uint64 `protobuf:"varint,1,opt,name=tlv_type,json=tlvType,proto3" json:"tlv_type,omitempty"`
}

func (x *SubscribeOnionPayloadRequest) Reset() {
	*x = SubscribeOnionPayloadRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_offersrpc_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SubscribeOnionPayloadRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SubscribeOnionPayloadRequest) ProtoMessage() {}

func (x *SubscribeOnionPayloadRequest) ProtoReflect() protoreflect.Message {
	mi := &file_offersrpc_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SubscribeOnionPayloadRequest.ProtoReflect.Descriptor instead.
func (*SubscribeOnionPayloadRequest) Descriptor() ([]byte, []int) {
	return file_offersrpc_proto_rawDescGZIP(), []int{5}
}

func (x *SubscribeOnionPayloadRequest) GetTlvType() uint64 {
	if x != nil {
		return x.TlvType
	}
	return 0
}

type SubscribeOnionPayloadResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Value is the raw bytes extracted from the tlv type that the subscription
	// is for.
	Value []byte `protobuf:"bytes,1,opt,name=value,proto3" json:"value,omitempty"`
}

func (x *SubscribeOnionPayloadResponse) Reset() {
	*x = SubscribeOnionPayloadResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_offersrpc_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SubscribeOnionPayloadResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SubscribeOnionPayloadResponse) ProtoMessage() {}

func (x *SubscribeOnionPayloadResponse) ProtoReflect() protoreflect.Message {
	mi := &file_offersrpc_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SubscribeOnionPayloadResponse.ProtoReflect.Descriptor instead.
func (*SubscribeOnionPayloadResponse) Descriptor() ([]byte, []int) {
	return file_offersrpc_proto_rawDescGZIP(), []int{6}
}

func (x *SubscribeOnionPayloadResponse) GetValue() []byte {
	if x != nil {
		return x.Value
	}
	return nil
}

type PayOfferRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The encoded offer to be paid.
	Offer string `protobuf:"bytes,1,opt,name=offer,proto3" json:"offer,omitempty"`
	// The amount to pay for this offer, expressed in millisatoshis. This value
	// must be at least the minimum amount specified in the offer.
	AmountMsat uint64 `protobuf:"varint,2,opt,name=amount_msat,json=amountMsat,proto3" json:"amount_msat,omitempty"`
	// An optional note to include with the payment.
	PayerNote string `protobuf:"bytes,3,opt,name=payer_note,json=payerNote,proto3" json:"payer_note,omitempty"`
}

func (x *PayOfferRequest) Reset() {
	*x = PayOfferRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_offersrpc_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PayOfferRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PayOfferRequest) ProtoMessage() {}

func (x *PayOfferRequest) ProtoReflect() protoreflect.Message {
	mi := &file_offersrpc_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PayOfferRequest.ProtoReflect.Descriptor instead.
func (*PayOfferRequest) Descriptor() ([]byte, []int) {
	return file_offersrpc_proto_rawDescGZIP(), []int{7}
}

func (x *PayOfferRequest) GetOffer() string {
	if x != nil {
		return x.Offer
	}
	return ""
}

func (x *PayOfferRequest) GetAmountMsat() uint64 {
	if x != nil {
		return x.AmountMsat
	}
	return 0
}

func (x *PayOfferRequest) GetPayerNote() string {
	if x != nil {
		return x.PayerNote
	}
	return ""
}

type PayOfferResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *PayOfferResponse) Reset() {
	*x = PayOfferResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_offersrpc_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PayOfferResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PayOfferResponse) ProtoMessage() {}

func (x *PayOfferResponse) ProtoReflect() protoreflect.Message {
	mi := &file_offersrpc_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PayOfferResponse.ProtoReflect.Descriptor instead.
func (*PayOfferResponse) Descriptor() ([]byte, []int) {
	return file_offersrpc_proto_rawDescGZIP(), []int{8}
}

var File_offersrpc_proto protoreflect.FileDescriptor

var file_offersrpc_proto_rawDesc = []byte{
	0x0a, 0x0f, 0x6f, 0x66, 0x66, 0x65, 0x72, 0x73, 0x72, 0x70, 0x63, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x09, 0x6f, 0x66, 0x66, 0x65, 0x72, 0x73, 0x72, 0x70, 0x63, 0x22, 0xd1, 0x01, 0x0a,
	0x17, 0x53, 0x65, 0x6e, 0x64, 0x4f, 0x6e, 0x69, 0x6f, 0x6e, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67,
	0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x16, 0x0a, 0x06, 0x70, 0x75, 0x62, 0x6b,
	0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x06, 0x70, 0x75, 0x62, 0x6b, 0x65, 0x79,
	0x12, 0x5c, 0x0a, 0x0e, 0x66, 0x69, 0x6e, 0x61, 0x6c, 0x5f, 0x70, 0x61, 0x79, 0x6c, 0x6f, 0x61,
	0x64, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x35, 0x2e, 0x6f, 0x66, 0x66, 0x65, 0x72,
	0x73, 0x72, 0x70, 0x63, 0x2e, 0x53, 0x65, 0x6e, 0x64, 0x4f, 0x6e, 0x69, 0x6f, 0x6e, 0x4d, 0x65,
	0x73, 0x73, 0x61, 0x67, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x2e, 0x46, 0x69, 0x6e,
	0x61, 0x6c, 0x50, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52,
	0x0d, 0x66, 0x69, 0x6e, 0x61, 0x6c, 0x50, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x73, 0x1a, 0x40,
	0x0a, 0x12, 0x46, 0x69, 0x6e, 0x61, 0x6c, 0x50, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x73, 0x45,
	0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x04, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01,
	0x22, 0x1a, 0x0a, 0x18, 0x53, 0x65, 0x6e, 0x64, 0x4f, 0x6e, 0x69, 0x6f, 0x6e, 0x4d, 0x65, 0x73,
	0x73, 0x61, 0x67, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x2a, 0x0a, 0x12,
	0x44, 0x65, 0x63, 0x6f, 0x64, 0x65, 0x4f, 0x66, 0x66, 0x65, 0x72, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x12, 0x14, 0x0a, 0x05, 0x6f, 0x66, 0x66, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x05, 0x6f, 0x66, 0x66, 0x65, 0x72, 0x22, 0x3d, 0x0a, 0x13, 0x44, 0x65, 0x63, 0x6f,
	0x64, 0x65, 0x4f, 0x66, 0x66, 0x65, 0x72, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12,
	0x26, 0x0a, 0x05, 0x6f, 0x66, 0x66, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x10,
	0x2e, 0x6f, 0x66, 0x66, 0x65, 0x72, 0x73, 0x72, 0x70, 0x63, 0x2e, 0x4f, 0x66, 0x66, 0x65, 0x72,
	0x52, 0x05, 0x6f, 0x66, 0x66, 0x65, 0x72, 0x22, 0xb2, 0x02, 0x0a, 0x05, 0x4f, 0x66, 0x66, 0x65,
	0x72, 0x12, 0x26, 0x0a, 0x0f, 0x6d, 0x69, 0x6e, 0x5f, 0x61, 0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x5f,
	0x6d, 0x73, 0x61, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0d, 0x6d, 0x69, 0x6e, 0x41,
	0x6d, 0x6f, 0x75, 0x6e, 0x74, 0x4d, 0x73, 0x61, 0x74, 0x12, 0x20, 0x0a, 0x0b, 0x64, 0x65, 0x73,
	0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b,
	0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x1a, 0x0a, 0x08, 0x66,
	0x65, 0x61, 0x74, 0x75, 0x72, 0x65, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x08, 0x66,
	0x65, 0x61, 0x74, 0x75, 0x72, 0x65, 0x73, 0x12, 0x2e, 0x0a, 0x13, 0x65, 0x78, 0x70, 0x69, 0x72,
	0x79, 0x5f, 0x75, 0x6e, 0x69, 0x78, 0x5f, 0x73, 0x65, 0x63, 0x6f, 0x6e, 0x64, 0x73, 0x18, 0x04,
	0x20, 0x01, 0x28, 0x04, 0x52, 0x11, 0x65, 0x78, 0x70, 0x69, 0x72, 0x79, 0x55, 0x6e, 0x69, 0x78,
	0x53, 0x65, 0x63, 0x6f, 0x6e, 0x64, 0x73, 0x12, 0x16, 0x0a, 0x06, 0x69, 0x73, 0x73, 0x75, 0x65,
	0x72, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x69, 0x73, 0x73, 0x75, 0x65, 0x72, 0x12,
	0x21, 0x0a, 0x0c, 0x6d, 0x69, 0x6e, 0x5f, 0x71, 0x75, 0x61, 0x6e, 0x74, 0x69, 0x74, 0x79, 0x18,
	0x06, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0b, 0x6d, 0x69, 0x6e, 0x51, 0x75, 0x61, 0x6e, 0x74, 0x69,
	0x74, 0x79, 0x12, 0x21, 0x0a, 0x0c, 0x6d, 0x61, 0x78, 0x5f, 0x71, 0x75, 0x61, 0x6e, 0x74, 0x69,
	0x74, 0x79, 0x18, 0x07, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0b, 0x6d, 0x61, 0x78, 0x51, 0x75, 0x61,
	0x6e, 0x74, 0x69, 0x74, 0x79, 0x12, 0x17, 0x0a, 0x07, 0x6e, 0x6f, 0x64, 0x65, 0x5f, 0x69, 0x64,
	0x18, 0x08, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x6e, 0x6f, 0x64, 0x65, 0x49, 0x64, 0x12, 0x1c,
	0x0a, 0x09, 0x73, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x18, 0x09, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x09, 0x73, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x22, 0x39, 0x0a, 0x1c,
	0x53, 0x75, 0x62, 0x73, 0x63, 0x72, 0x69, 0x62, 0x65, 0x4f, 0x6e, 0x69, 0x6f, 0x6e, 0x50, 0x61,
	0x79, 0x6c, 0x6f, 0x61, 0x64, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x19, 0x0a, 0x08,
	0x74, 0x6c, 0x76, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x07,
	0x74, 0x6c, 0x76, 0x54, 0x79, 0x70, 0x65, 0x22, 0x35, 0x0a, 0x1d, 0x53, 0x75, 0x62, 0x73, 0x63,
	0x72, 0x69, 0x62, 0x65, 0x4f, 0x6e, 0x69, 0x6f, 0x6e, 0x50, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75,
	0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x22, 0x67,
	0x0a, 0x0f, 0x50, 0x61, 0x79, 0x4f, 0x66, 0x66, 0x65, 0x72, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x12, 0x14, 0x0a, 0x05, 0x6f, 0x66, 0x66, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x05, 0x6f, 0x66, 0x66, 0x65, 0x72, 0x12, 0x1f, 0x0a, 0x0b, 0x61, 0x6d, 0x6f, 0x75, 0x6e,
	0x74, 0x5f, 0x6d, 0x73, 0x61, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0a, 0x61, 0x6d,
	0x6f, 0x75, 0x6e, 0x74, 0x4d, 0x73, 0x61, 0x74, 0x12, 0x1d, 0x0a, 0x0a, 0x70, 0x61, 0x79, 0x65,
	0x72, 0x5f, 0x6e, 0x6f, 0x74, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x70, 0x61,
	0x79, 0x65, 0x72, 0x4e, 0x6f, 0x74, 0x65, 0x22, 0x12, 0x0a, 0x10, 0x50, 0x61, 0x79, 0x4f, 0x66,
	0x66, 0x65, 0x72, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x32, 0xe6, 0x02, 0x0a, 0x06,
	0x4f, 0x66, 0x66, 0x65, 0x72, 0x73, 0x12, 0x5b, 0x0a, 0x10, 0x53, 0x65, 0x6e, 0x64, 0x4f, 0x6e,
	0x69, 0x6f, 0x6e, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x22, 0x2e, 0x6f, 0x66, 0x66,
	0x65, 0x72, 0x73, 0x72, 0x70, 0x63, 0x2e, 0x53, 0x65, 0x6e, 0x64, 0x4f, 0x6e, 0x69, 0x6f, 0x6e,
	0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x23,
	0x2e, 0x6f, 0x66, 0x66, 0x65, 0x72, 0x73, 0x72, 0x70, 0x63, 0x2e, 0x53, 0x65, 0x6e, 0x64, 0x4f,
	0x6e, 0x69, 0x6f, 0x6e, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x12, 0x4c, 0x0a, 0x0b, 0x44, 0x65, 0x63, 0x6f, 0x64, 0x65, 0x4f, 0x66, 0x66,
	0x65, 0x72, 0x12, 0x1d, 0x2e, 0x6f, 0x66, 0x66, 0x65, 0x72, 0x73, 0x72, 0x70, 0x63, 0x2e, 0x44,
	0x65, 0x63, 0x6f, 0x64, 0x65, 0x4f, 0x66, 0x66, 0x65, 0x72, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x1a, 0x1e, 0x2e, 0x6f, 0x66, 0x66, 0x65, 0x72, 0x73, 0x72, 0x70, 0x63, 0x2e, 0x44, 0x65,
	0x63, 0x6f, 0x64, 0x65, 0x4f, 0x66, 0x66, 0x65, 0x72, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x6c, 0x0a, 0x15, 0x53, 0x75, 0x62, 0x73, 0x63, 0x72, 0x69, 0x62, 0x65, 0x4f, 0x6e,
	0x69, 0x6f, 0x6e, 0x50, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x12, 0x27, 0x2e, 0x6f, 0x66, 0x66,
	0x65, 0x72, 0x73, 0x72, 0x70, 0x63, 0x2e, 0x53, 0x75, 0x62, 0x73, 0x63, 0x72, 0x69, 0x62, 0x65,
	0x4f, 0x6e, 0x69, 0x6f, 0x6e, 0x50, 0x61, 0x79, 0x6c, 0x6f, 0x61, 0x64, 0x52, 0x65, 0x71, 0x75,
	0x65, 0x73, 0x74, 0x1a, 0x28, 0x2e, 0x6f, 0x66, 0x66, 0x65, 0x72, 0x73, 0x72, 0x70, 0x63, 0x2e,
	0x53, 0x75, 0x62, 0x73, 0x63, 0x72, 0x69, 0x62, 0x65, 0x4f, 0x6e, 0x69, 0x6f, 0x6e, 0x50, 0x61,
	0x79, 0x6c, 0x6f, 0x61, 0x64, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x30, 0x01, 0x12,
	0x43, 0x0a, 0x08, 0x50, 0x61, 0x79, 0x4f, 0x66, 0x66, 0x65, 0x72, 0x12, 0x1a, 0x2e, 0x6f, 0x66,
	0x66, 0x65, 0x72, 0x73, 0x72, 0x70, 0x63, 0x2e, 0x50, 0x61, 0x79, 0x4f, 0x66, 0x66, 0x65, 0x72,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1b, 0x2e, 0x6f, 0x66, 0x66, 0x65, 0x72, 0x73,
	0x72, 0x70, 0x63, 0x2e, 0x50, 0x61, 0x79, 0x4f, 0x66, 0x66, 0x65, 0x72, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x42, 0x25, 0x5a, 0x23, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63,
	0x6f, 0x6d, 0x2f, 0x63, 0x61, 0x72, 0x6c, 0x61, 0x6b, 0x63, 0x2f, 0x62, 0x6f, 0x6c, 0x74, 0x6e,
	0x64, 0x2f, 0x6f, 0x66, 0x66, 0x65, 0x72, 0x73, 0x72, 0x70, 0x63, 0x62, 0x06, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x33,
}

var (
	file_offersrpc_proto_rawDescOnce sync.Once
	file_offersrpc_proto_rawDescData = file_offersrpc_proto_rawDesc
)

func file_offersrpc_proto_rawDescGZIP() []byte {
	file_offersrpc_proto_rawDescOnce.Do(func() {
		file_offersrpc_proto_rawDescData = protoimpl.X.CompressGZIP(file_offersrpc_proto_rawDescData)
	})
	return file_offersrpc_proto_rawDescData
}

var file_offersrpc_proto_msgTypes = make([]protoimpl.MessageInfo, 10)
var file_offersrpc_proto_goTypes = []interface{}{
	(*SendOnionMessageRequest)(nil),       // 0: offersrpc.SendOnionMessageRequest
	(*SendOnionMessageResponse)(nil),      // 1: offersrpc.SendOnionMessageResponse
	(*DecodeOfferRequest)(nil),            // 2: offersrpc.DecodeOfferRequest
	(*DecodeOfferResponse)(nil),           // 3: offersrpc.DecodeOfferResponse
	(*Offer)(nil),                         // 4: offersrpc.Offer
	(*SubscribeOnionPayloadRequest)(nil),  // 5: offersrpc.SubscribeOnionPayloadRequest
	(*SubscribeOnionPayloadResponse)(nil), // 6: offersrpc.SubscribeOnionPayloadResponse
	(*PayOfferRequest)(nil),               // 7: offersrpc.PayOfferRequest
	(*PayOfferResponse)(nil),              // 8: offersrpc.PayOfferResponse
	nil,                                   // 9: offersrpc.SendOnionMessageRequest.FinalPayloadsEntry
}
var file_offersrpc_proto_depIdxs = []int32{
	9, // 0: offersrpc.SendOnionMessageRequest.final_payloads:type_name -> offersrpc.SendOnionMessageRequest.FinalPayloadsEntry
	4, // 1: offersrpc.DecodeOfferResponse.offer:type_name -> offersrpc.Offer
	0, // 2: offersrpc.Offers.SendOnionMessage:input_type -> offersrpc.SendOnionMessageRequest
	2, // 3: offersrpc.Offers.DecodeOffer:input_type -> offersrpc.DecodeOfferRequest
	5, // 4: offersrpc.Offers.SubscribeOnionPayload:input_type -> offersrpc.SubscribeOnionPayloadRequest
	7, // 5: offersrpc.Offers.PayOffer:input_type -> offersrpc.PayOfferRequest
	1, // 6: offersrpc.Offers.SendOnionMessage:output_type -> offersrpc.SendOnionMessageResponse
	3, // 7: offersrpc.Offers.DecodeOffer:output_type -> offersrpc.DecodeOfferResponse
	6, // 8: offersrpc.Offers.SubscribeOnionPayload:output_type -> offersrpc.SubscribeOnionPayloadResponse
	8, // 9: offersrpc.Offers.PayOffer:output_type -> offersrpc.PayOfferResponse
	6, // [6:10] is the sub-list for method output_type
	2, // [2:6] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_offersrpc_proto_init() }
func file_offersrpc_proto_init() {
	if File_offersrpc_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_offersrpc_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SendOnionMessageRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_offersrpc_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SendOnionMessageResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_offersrpc_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DecodeOfferRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_offersrpc_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DecodeOfferResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_offersrpc_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Offer); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_offersrpc_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SubscribeOnionPayloadRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_offersrpc_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SubscribeOnionPayloadResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_offersrpc_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PayOfferRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_offersrpc_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PayOfferResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_offersrpc_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   10,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_offersrpc_proto_goTypes,
		DependencyIndexes: file_offersrpc_proto_depIdxs,
		MessageInfos:      file_offersrpc_proto_msgTypes,
	}.Build()
	File_offersrpc_proto = out.File
	file_offersrpc_proto_rawDesc = nil
	file_offersrpc_proto_goTypes = nil
	file_offersrpc_proto_depIdxs = nil
}
