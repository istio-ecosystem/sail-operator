// Copyright 2019 Istio Authors
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.1
// 	protoc        (unknown)
// source: analysis/v1alpha1/message.proto

// $title: Analysis Messages
// $description: Describes the structure of messages generated by Istio analyzers.
// $location: https://istio.io/docs/reference/config/istio.analysis.v1alpha1.html
// $weight: 20

// Describes the structure of messages generated by Istio analyzers.

package v1alpha1

import (
	_struct "github.com/golang/protobuf/ptypes/struct"
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

// The values here are chosen so that more severe messages get sorted higher,
// as well as leaving space in between to add more later
type AnalysisMessageBase_Level int32

const (
	AnalysisMessageBase_UNKNOWN AnalysisMessageBase_Level = 0 // invalid, but included for proto compatibility for 0 values
	AnalysisMessageBase_ERROR   AnalysisMessageBase_Level = 3
	AnalysisMessageBase_WARNING AnalysisMessageBase_Level = 8
	AnalysisMessageBase_INFO    AnalysisMessageBase_Level = 12
)

// Enum value maps for AnalysisMessageBase_Level.
var (
	AnalysisMessageBase_Level_name = map[int32]string{
		0:  "UNKNOWN",
		3:  "ERROR",
		8:  "WARNING",
		12: "INFO",
	}
	AnalysisMessageBase_Level_value = map[string]int32{
		"UNKNOWN": 0,
		"ERROR":   3,
		"WARNING": 8,
		"INFO":    12,
	}
)

func (x AnalysisMessageBase_Level) Enum() *AnalysisMessageBase_Level {
	p := new(AnalysisMessageBase_Level)
	*p = x
	return p
}

func (x AnalysisMessageBase_Level) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (AnalysisMessageBase_Level) Descriptor() protoreflect.EnumDescriptor {
	return file_analysis_v1alpha1_message_proto_enumTypes[0].Descriptor()
}

func (AnalysisMessageBase_Level) Type() protoreflect.EnumType {
	return &file_analysis_v1alpha1_message_proto_enumTypes[0]
}

func (x AnalysisMessageBase_Level) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use AnalysisMessageBase_Level.Descriptor instead.
func (AnalysisMessageBase_Level) EnumDescriptor() ([]byte, []int) {
	return file_analysis_v1alpha1_message_proto_rawDescGZIP(), []int{0, 0}
}

// AnalysisMessageBase describes some common information that is needed for all
// messages. All information should be static with respect to the error code.
type AnalysisMessageBase struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Type *AnalysisMessageBase_Type `protobuf:"bytes,1,opt,name=type,proto3" json:"type,omitempty"`
	// Represents how severe a message is. Required.
	Level AnalysisMessageBase_Level `protobuf:"varint,2,opt,name=level,proto3,enum=istio.analysis.v1alpha1.AnalysisMessageBase_Level" json:"level,omitempty"`
	// A url pointing to the Istio documentation for this specific error type.
	// Should be of the form
	// `^http(s)?://(preliminary\.)?istio.io/docs/reference/config/analysis/`
	// Required.
	DocumentationUrl string `protobuf:"bytes,3,opt,name=documentation_url,json=documentationUrl,proto3" json:"documentation_url,omitempty"`
}

func (x *AnalysisMessageBase) Reset() {
	*x = AnalysisMessageBase{}
	if protoimpl.UnsafeEnabled {
		mi := &file_analysis_v1alpha1_message_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AnalysisMessageBase) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AnalysisMessageBase) ProtoMessage() {}

func (x *AnalysisMessageBase) ProtoReflect() protoreflect.Message {
	mi := &file_analysis_v1alpha1_message_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AnalysisMessageBase.ProtoReflect.Descriptor instead.
func (*AnalysisMessageBase) Descriptor() ([]byte, []int) {
	return file_analysis_v1alpha1_message_proto_rawDescGZIP(), []int{0}
}

func (x *AnalysisMessageBase) GetType() *AnalysisMessageBase_Type {
	if x != nil {
		return x.Type
	}
	return nil
}

func (x *AnalysisMessageBase) GetLevel() AnalysisMessageBase_Level {
	if x != nil {
		return x.Level
	}
	return AnalysisMessageBase_UNKNOWN
}

func (x *AnalysisMessageBase) GetDocumentationUrl() string {
	if x != nil {
		return x.DocumentationUrl
	}
	return ""
}

// AnalysisMessageWeakSchema is the set of information that's needed to define a
// weakly-typed schema. The purpose of this proto is to provide a mechanism for
// validating istio/istio/galley/pkg/config/analysis/msg/messages.yaml to make
// sure that we don't allow committing underspecified types.
type AnalysisMessageWeakSchema struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Required
	MessageBase *AnalysisMessageBase `protobuf:"bytes,1,opt,name=message_base,json=messageBase,proto3" json:"message_base,omitempty"`
	// A human readable description of what the error means. Required.
	Description string `protobuf:"bytes,2,opt,name=description,proto3" json:"description,omitempty"`
	// A go-style template string (https://golang.org/pkg/fmt/#hdr-Printing)
	// defining how to combine the args for a  particular message into a log line.
	// Required.
	Template string `protobuf:"bytes,3,opt,name=template,proto3" json:"template,omitempty"`
	// A description of the arguments for a particular message type
	Args []*AnalysisMessageWeakSchema_ArgType `protobuf:"bytes,4,rep,name=args,proto3" json:"args,omitempty"`
}

func (x *AnalysisMessageWeakSchema) Reset() {
	*x = AnalysisMessageWeakSchema{}
	if protoimpl.UnsafeEnabled {
		mi := &file_analysis_v1alpha1_message_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AnalysisMessageWeakSchema) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AnalysisMessageWeakSchema) ProtoMessage() {}

func (x *AnalysisMessageWeakSchema) ProtoReflect() protoreflect.Message {
	mi := &file_analysis_v1alpha1_message_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AnalysisMessageWeakSchema.ProtoReflect.Descriptor instead.
func (*AnalysisMessageWeakSchema) Descriptor() ([]byte, []int) {
	return file_analysis_v1alpha1_message_proto_rawDescGZIP(), []int{1}
}

func (x *AnalysisMessageWeakSchema) GetMessageBase() *AnalysisMessageBase {
	if x != nil {
		return x.MessageBase
	}
	return nil
}

func (x *AnalysisMessageWeakSchema) GetDescription() string {
	if x != nil {
		return x.Description
	}
	return ""
}

func (x *AnalysisMessageWeakSchema) GetTemplate() string {
	if x != nil {
		return x.Template
	}
	return ""
}

func (x *AnalysisMessageWeakSchema) GetArgs() []*AnalysisMessageWeakSchema_ArgType {
	if x != nil {
		return x.Args
	}
	return nil
}

// GenericAnalysisMessage is an instance of an AnalysisMessage defined by a
// schema, whose metaschema is AnalysisMessageWeakSchema. (Names are hard.) Code
// should be able to perform validation of arguments as needed by using the
// message type information to look at the AnalysisMessageWeakSchema and examine the
// list of args at runtime. Developers can also create stronger-typed versions
// of GenericAnalysisMessage for well-known and stable message types.
type GenericAnalysisMessage struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Required
	MessageBase *AnalysisMessageBase `protobuf:"bytes,1,opt,name=message_base,json=messageBase,proto3" json:"message_base,omitempty"`
	// Any message-type specific arguments that need to get codified. Optional.
	Args *_struct.Struct `protobuf:"bytes,2,opt,name=args,proto3" json:"args,omitempty"`
	// A list of strings specifying the resource identifiers that were the cause
	// of message generation. A "path" here is a (NAMESPACE\/)?RESOURCETYPE/NAME
	// tuple that uniquely identifies a particular resource. There doesn't seem to
	// be a single concept for this, but this is intuitively taken from
	// https://kubernetes.io/docs/reference/using-api/api-concepts/#standard-api-terminology
	// At least one is required.
	ResourcePaths []string `protobuf:"bytes,3,rep,name=resource_paths,json=resourcePaths,proto3" json:"resource_paths,omitempty"`
}

func (x *GenericAnalysisMessage) Reset() {
	*x = GenericAnalysisMessage{}
	if protoimpl.UnsafeEnabled {
		mi := &file_analysis_v1alpha1_message_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GenericAnalysisMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GenericAnalysisMessage) ProtoMessage() {}

func (x *GenericAnalysisMessage) ProtoReflect() protoreflect.Message {
	mi := &file_analysis_v1alpha1_message_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GenericAnalysisMessage.ProtoReflect.Descriptor instead.
func (*GenericAnalysisMessage) Descriptor() ([]byte, []int) {
	return file_analysis_v1alpha1_message_proto_rawDescGZIP(), []int{2}
}

func (x *GenericAnalysisMessage) GetMessageBase() *AnalysisMessageBase {
	if x != nil {
		return x.MessageBase
	}
	return nil
}

func (x *GenericAnalysisMessage) GetArgs() *_struct.Struct {
	if x != nil {
		return x.Args
	}
	return nil
}

func (x *GenericAnalysisMessage) GetResourcePaths() []string {
	if x != nil {
		return x.ResourcePaths
	}
	return nil
}

// InternalErrorAnalysisMessage is a strongly-typed message representing some
// error in Istio code that prevented us from performing analysis at all.
type InternalErrorAnalysisMessage struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Required
	MessageBase *AnalysisMessageBase `protobuf:"bytes,1,opt,name=message_base,json=messageBase,proto3" json:"message_base,omitempty"`
	// Any detail regarding specifics of the error. Should be human-readable.
	Detail string `protobuf:"bytes,2,opt,name=detail,proto3" json:"detail,omitempty"`
}

func (x *InternalErrorAnalysisMessage) Reset() {
	*x = InternalErrorAnalysisMessage{}
	if protoimpl.UnsafeEnabled {
		mi := &file_analysis_v1alpha1_message_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *InternalErrorAnalysisMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*InternalErrorAnalysisMessage) ProtoMessage() {}

func (x *InternalErrorAnalysisMessage) ProtoReflect() protoreflect.Message {
	mi := &file_analysis_v1alpha1_message_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use InternalErrorAnalysisMessage.ProtoReflect.Descriptor instead.
func (*InternalErrorAnalysisMessage) Descriptor() ([]byte, []int) {
	return file_analysis_v1alpha1_message_proto_rawDescGZIP(), []int{3}
}

func (x *InternalErrorAnalysisMessage) GetMessageBase() *AnalysisMessageBase {
	if x != nil {
		return x.MessageBase
	}
	return nil
}

func (x *InternalErrorAnalysisMessage) GetDetail() string {
	if x != nil {
		return x.Detail
	}
	return ""
}

// A unique identifier for the type of message. Name is intended to be
// human-readable, code is intended to be machine readable. There should be a
// one-to-one mapping between name and code. (i.e. do not re-use names or
// codes between message types.)
type AnalysisMessageBase_Type struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// A human-readable name for the message type. e.g. "InternalError",
	// "PodMissingProxy". This should be the same for all messages of the same type.
	// Required.
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	// A 7 character code matching `^IST[0-9]{4}$` intended to uniquely identify
	// the message type. (e.g. "IST0001" is mapped to the "InternalError" message
	// type.) 0000-0100 are reserved. Required.
	Code string `protobuf:"bytes,2,opt,name=code,proto3" json:"code,omitempty"`
}

func (x *AnalysisMessageBase_Type) Reset() {
	*x = AnalysisMessageBase_Type{}
	if protoimpl.UnsafeEnabled {
		mi := &file_analysis_v1alpha1_message_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AnalysisMessageBase_Type) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AnalysisMessageBase_Type) ProtoMessage() {}

func (x *AnalysisMessageBase_Type) ProtoReflect() protoreflect.Message {
	mi := &file_analysis_v1alpha1_message_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AnalysisMessageBase_Type.ProtoReflect.Descriptor instead.
func (*AnalysisMessageBase_Type) Descriptor() ([]byte, []int) {
	return file_analysis_v1alpha1_message_proto_rawDescGZIP(), []int{0, 0}
}

func (x *AnalysisMessageBase_Type) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *AnalysisMessageBase_Type) GetCode() string {
	if x != nil {
		return x.Code
	}
	return ""
}

type AnalysisMessageWeakSchema_ArgType struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Required
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	// Required. Should be a golang type, used in code generation.
	// Ideally this will change to a less language-pinned type before this gets
	// out of alpha, but for compatibility with current istio/istio code it's
	// go_type for now.
	GoType string `protobuf:"bytes,2,opt,name=go_type,json=goType,proto3" json:"go_type,omitempty"`
}

func (x *AnalysisMessageWeakSchema_ArgType) Reset() {
	*x = AnalysisMessageWeakSchema_ArgType{}
	if protoimpl.UnsafeEnabled {
		mi := &file_analysis_v1alpha1_message_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AnalysisMessageWeakSchema_ArgType) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AnalysisMessageWeakSchema_ArgType) ProtoMessage() {}

func (x *AnalysisMessageWeakSchema_ArgType) ProtoReflect() protoreflect.Message {
	mi := &file_analysis_v1alpha1_message_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AnalysisMessageWeakSchema_ArgType.ProtoReflect.Descriptor instead.
func (*AnalysisMessageWeakSchema_ArgType) Descriptor() ([]byte, []int) {
	return file_analysis_v1alpha1_message_proto_rawDescGZIP(), []int{1, 0}
}

func (x *AnalysisMessageWeakSchema_ArgType) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *AnalysisMessageWeakSchema_ArgType) GetGoType() string {
	if x != nil {
		return x.GoType
	}
	return ""
}

var File_analysis_v1alpha1_message_proto protoreflect.FileDescriptor

var file_analysis_v1alpha1_message_proto_rawDesc = []byte{
	0x0a, 0x1f, 0x61, 0x6e, 0x61, 0x6c, 0x79, 0x73, 0x69, 0x73, 0x2f, 0x76, 0x31, 0x61, 0x6c, 0x70,
	0x68, 0x61, 0x31, 0x2f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x17, 0x69, 0x73, 0x74, 0x69, 0x6f, 0x2e, 0x61, 0x6e, 0x61, 0x6c, 0x79, 0x73, 0x69,
	0x73, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x1a, 0x1c, 0x67, 0x6f, 0x6f, 0x67,
	0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x73, 0x74, 0x72, 0x75,
	0x63, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xbb, 0x02, 0x0a, 0x13, 0x41, 0x6e, 0x61,
	0x6c, 0x79, 0x73, 0x69, 0x73, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x42, 0x61, 0x73, 0x65,
	0x12, 0x45, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x31,
	0x2e, 0x69, 0x73, 0x74, 0x69, 0x6f, 0x2e, 0x61, 0x6e, 0x61, 0x6c, 0x79, 0x73, 0x69, 0x73, 0x2e,
	0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x41, 0x6e, 0x61, 0x6c, 0x79, 0x73, 0x69,
	0x73, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x42, 0x61, 0x73, 0x65, 0x2e, 0x54, 0x79, 0x70,
	0x65, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x48, 0x0a, 0x05, 0x6c, 0x65, 0x76, 0x65, 0x6c,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x32, 0x2e, 0x69, 0x73, 0x74, 0x69, 0x6f, 0x2e, 0x61,
	0x6e, 0x61, 0x6c, 0x79, 0x73, 0x69, 0x73, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31,
	0x2e, 0x41, 0x6e, 0x61, 0x6c, 0x79, 0x73, 0x69, 0x73, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65,
	0x42, 0x61, 0x73, 0x65, 0x2e, 0x4c, 0x65, 0x76, 0x65, 0x6c, 0x52, 0x05, 0x6c, 0x65, 0x76, 0x65,
	0x6c, 0x12, 0x2b, 0x0a, 0x11, 0x64, 0x6f, 0x63, 0x75, 0x6d, 0x65, 0x6e, 0x74, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x5f, 0x75, 0x72, 0x6c, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x10, 0x64, 0x6f,
	0x63, 0x75, 0x6d, 0x65, 0x6e, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x55, 0x72, 0x6c, 0x1a, 0x2e,
	0x0a, 0x04, 0x54, 0x79, 0x70, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x63, 0x6f,
	0x64, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x22, 0x36,
	0x0a, 0x05, 0x4c, 0x65, 0x76, 0x65, 0x6c, 0x12, 0x0b, 0x0a, 0x07, 0x55, 0x4e, 0x4b, 0x4e, 0x4f,
	0x57, 0x4e, 0x10, 0x00, 0x12, 0x09, 0x0a, 0x05, 0x45, 0x52, 0x52, 0x4f, 0x52, 0x10, 0x03, 0x12,
	0x0b, 0x0a, 0x07, 0x57, 0x41, 0x52, 0x4e, 0x49, 0x4e, 0x47, 0x10, 0x08, 0x12, 0x08, 0x0a, 0x04,
	0x49, 0x4e, 0x46, 0x4f, 0x10, 0x0c, 0x22, 0xb2, 0x02, 0x0a, 0x19, 0x41, 0x6e, 0x61, 0x6c, 0x79,
	0x73, 0x69, 0x73, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x57, 0x65, 0x61, 0x6b, 0x53, 0x63,
	0x68, 0x65, 0x6d, 0x61, 0x12, 0x4f, 0x0a, 0x0c, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x5f,
	0x62, 0x61, 0x73, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x2c, 0x2e, 0x69, 0x73, 0x74,
	0x69, 0x6f, 0x2e, 0x61, 0x6e, 0x61, 0x6c, 0x79, 0x73, 0x69, 0x73, 0x2e, 0x76, 0x31, 0x61, 0x6c,
	0x70, 0x68, 0x61, 0x31, 0x2e, 0x41, 0x6e, 0x61, 0x6c, 0x79, 0x73, 0x69, 0x73, 0x4d, 0x65, 0x73,
	0x73, 0x61, 0x67, 0x65, 0x42, 0x61, 0x73, 0x65, 0x52, 0x0b, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67,
	0x65, 0x42, 0x61, 0x73, 0x65, 0x12, 0x20, 0x0a, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70,
	0x74, 0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x64, 0x65, 0x73, 0x63,
	0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x1a, 0x0a, 0x08, 0x74, 0x65, 0x6d, 0x70, 0x6c,
	0x61, 0x74, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x74, 0x65, 0x6d, 0x70, 0x6c,
	0x61, 0x74, 0x65, 0x12, 0x4e, 0x0a, 0x04, 0x61, 0x72, 0x67, 0x73, 0x18, 0x04, 0x20, 0x03, 0x28,
	0x0b, 0x32, 0x3a, 0x2e, 0x69, 0x73, 0x74, 0x69, 0x6f, 0x2e, 0x61, 0x6e, 0x61, 0x6c, 0x79, 0x73,
	0x69, 0x73, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x41, 0x6e, 0x61, 0x6c,
	0x79, 0x73, 0x69, 0x73, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x57, 0x65, 0x61, 0x6b, 0x53,
	0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x41, 0x72, 0x67, 0x54, 0x79, 0x70, 0x65, 0x52, 0x04, 0x61,
	0x72, 0x67, 0x73, 0x1a, 0x36, 0x0a, 0x07, 0x41, 0x72, 0x67, 0x54, 0x79, 0x70, 0x65, 0x12, 0x12,
	0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61,
	0x6d, 0x65, 0x12, 0x17, 0x0a, 0x07, 0x67, 0x6f, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x06, 0x67, 0x6f, 0x54, 0x79, 0x70, 0x65, 0x22, 0xbd, 0x01, 0x0a, 0x16,
	0x47, 0x65, 0x6e, 0x65, 0x72, 0x69, 0x63, 0x41, 0x6e, 0x61, 0x6c, 0x79, 0x73, 0x69, 0x73, 0x4d,
	0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x4f, 0x0a, 0x0c, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67,
	0x65, 0x5f, 0x62, 0x61, 0x73, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x2c, 0x2e, 0x69,
	0x73, 0x74, 0x69, 0x6f, 0x2e, 0x61, 0x6e, 0x61, 0x6c, 0x79, 0x73, 0x69, 0x73, 0x2e, 0x76, 0x31,
	0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x41, 0x6e, 0x61, 0x6c, 0x79, 0x73, 0x69, 0x73, 0x4d,
	0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x42, 0x61, 0x73, 0x65, 0x52, 0x0b, 0x6d, 0x65, 0x73, 0x73,
	0x61, 0x67, 0x65, 0x42, 0x61, 0x73, 0x65, 0x12, 0x2b, 0x0a, 0x04, 0x61, 0x72, 0x67, 0x73, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x17, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x53, 0x74, 0x72, 0x75, 0x63, 0x74, 0x52, 0x04,
	0x61, 0x72, 0x67, 0x73, 0x12, 0x25, 0x0a, 0x0e, 0x72, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65,
	0x5f, 0x70, 0x61, 0x74, 0x68, 0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x09, 0x52, 0x0d, 0x72, 0x65,
	0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x50, 0x61, 0x74, 0x68, 0x73, 0x22, 0x87, 0x01, 0x0a, 0x1c,
	0x49, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x41, 0x6e, 0x61,
	0x6c, 0x79, 0x73, 0x69, 0x73, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x4f, 0x0a, 0x0c,
	0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x5f, 0x62, 0x61, 0x73, 0x65, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x2c, 0x2e, 0x69, 0x73, 0x74, 0x69, 0x6f, 0x2e, 0x61, 0x6e, 0x61, 0x6c, 0x79,
	0x73, 0x69, 0x73, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x41, 0x6e, 0x61,
	0x6c, 0x79, 0x73, 0x69, 0x73, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x42, 0x61, 0x73, 0x65,
	0x52, 0x0b, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x42, 0x61, 0x73, 0x65, 0x12, 0x16, 0x0a,
	0x06, 0x64, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x64,
	0x65, 0x74, 0x61, 0x69, 0x6c, 0x42, 0x20, 0x5a, 0x1e, 0x69, 0x73, 0x74, 0x69, 0x6f, 0x2e, 0x69,
	0x6f, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x61, 0x6e, 0x61, 0x6c, 0x79, 0x73, 0x69, 0x73, 0x2f, 0x76,
	0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_analysis_v1alpha1_message_proto_rawDescOnce sync.Once
	file_analysis_v1alpha1_message_proto_rawDescData = file_analysis_v1alpha1_message_proto_rawDesc
)

func file_analysis_v1alpha1_message_proto_rawDescGZIP() []byte {
	file_analysis_v1alpha1_message_proto_rawDescOnce.Do(func() {
		file_analysis_v1alpha1_message_proto_rawDescData = protoimpl.X.CompressGZIP(file_analysis_v1alpha1_message_proto_rawDescData)
	})
	return file_analysis_v1alpha1_message_proto_rawDescData
}

var file_analysis_v1alpha1_message_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_analysis_v1alpha1_message_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_analysis_v1alpha1_message_proto_goTypes = []interface{}{
	(AnalysisMessageBase_Level)(0),            // 0: istio.analysis.v1alpha1.AnalysisMessageBase.Level
	(*AnalysisMessageBase)(nil),               // 1: istio.analysis.v1alpha1.AnalysisMessageBase
	(*AnalysisMessageWeakSchema)(nil),         // 2: istio.analysis.v1alpha1.AnalysisMessageWeakSchema
	(*GenericAnalysisMessage)(nil),            // 3: istio.analysis.v1alpha1.GenericAnalysisMessage
	(*InternalErrorAnalysisMessage)(nil),      // 4: istio.analysis.v1alpha1.InternalErrorAnalysisMessage
	(*AnalysisMessageBase_Type)(nil),          // 5: istio.analysis.v1alpha1.AnalysisMessageBase.Type
	(*AnalysisMessageWeakSchema_ArgType)(nil), // 6: istio.analysis.v1alpha1.AnalysisMessageWeakSchema.ArgType
	(*_struct.Struct)(nil),                    // 7: google.protobuf.Struct
}
var file_analysis_v1alpha1_message_proto_depIdxs = []int32{
	5, // 0: istio.analysis.v1alpha1.AnalysisMessageBase.type:type_name -> istio.analysis.v1alpha1.AnalysisMessageBase.Type
	0, // 1: istio.analysis.v1alpha1.AnalysisMessageBase.level:type_name -> istio.analysis.v1alpha1.AnalysisMessageBase.Level
	1, // 2: istio.analysis.v1alpha1.AnalysisMessageWeakSchema.message_base:type_name -> istio.analysis.v1alpha1.AnalysisMessageBase
	6, // 3: istio.analysis.v1alpha1.AnalysisMessageWeakSchema.args:type_name -> istio.analysis.v1alpha1.AnalysisMessageWeakSchema.ArgType
	1, // 4: istio.analysis.v1alpha1.GenericAnalysisMessage.message_base:type_name -> istio.analysis.v1alpha1.AnalysisMessageBase
	7, // 5: istio.analysis.v1alpha1.GenericAnalysisMessage.args:type_name -> google.protobuf.Struct
	1, // 6: istio.analysis.v1alpha1.InternalErrorAnalysisMessage.message_base:type_name -> istio.analysis.v1alpha1.AnalysisMessageBase
	7, // [7:7] is the sub-list for method output_type
	7, // [7:7] is the sub-list for method input_type
	7, // [7:7] is the sub-list for extension type_name
	7, // [7:7] is the sub-list for extension extendee
	0, // [0:7] is the sub-list for field type_name
}

func init() { file_analysis_v1alpha1_message_proto_init() }
func file_analysis_v1alpha1_message_proto_init() {
	if File_analysis_v1alpha1_message_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_analysis_v1alpha1_message_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AnalysisMessageBase); i {
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
		file_analysis_v1alpha1_message_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AnalysisMessageWeakSchema); i {
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
		file_analysis_v1alpha1_message_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GenericAnalysisMessage); i {
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
		file_analysis_v1alpha1_message_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*InternalErrorAnalysisMessage); i {
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
		file_analysis_v1alpha1_message_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AnalysisMessageBase_Type); i {
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
		file_analysis_v1alpha1_message_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AnalysisMessageWeakSchema_ArgType); i {
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
			RawDescriptor: file_analysis_v1alpha1_message_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_analysis_v1alpha1_message_proto_goTypes,
		DependencyIndexes: file_analysis_v1alpha1_message_proto_depIdxs,
		EnumInfos:         file_analysis_v1alpha1_message_proto_enumTypes,
		MessageInfos:      file_analysis_v1alpha1_message_proto_msgTypes,
	}.Build()
	File_analysis_v1alpha1_message_proto = out.File
	file_analysis_v1alpha1_message_proto_rawDesc = nil
	file_analysis_v1alpha1_message_proto_goTypes = nil
	file_analysis_v1alpha1_message_proto_depIdxs = nil
}
