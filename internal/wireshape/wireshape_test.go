package wireshape

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	_ "google.golang.org/protobuf/types/known/timestamppb"
)

type fieldSpec struct {
	name     string
	number   int32
	typ      descriptorpb.FieldDescriptorProto_Type
	typeName string
	repeated bool
	optional bool
	oneof    *int32
}

func field(f fieldSpec) *descriptorpb.FieldDescriptorProto {
	label := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL
	if f.repeated {
		label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED
	}
	fd := &descriptorpb.FieldDescriptorProto{
		Name:   proto.String(f.name),
		Number: proto.Int32(f.number),
		Label:  label.Enum(),
		Type:   f.typ.Enum(),
	}
	if f.typeName != "" {
		fd.TypeName = proto.String(f.typeName)
	}
	if f.optional {
		fd.Proto3Optional = proto.Bool(true)
	}
	if f.oneof != nil {
		fd.OneofIndex = f.oneof
	}
	return fd
}

const (
	tString  = descriptorpb.FieldDescriptorProto_TYPE_STRING
	tUint32  = descriptorpb.FieldDescriptorProto_TYPE_UINT32
	tMessage = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE
	tEnum    = descriptorpb.FieldDescriptorProto_TYPE_ENUM
)

// corpus builds a two-file corpus: a shared types file and a service events
// file that imports it. shared and event mutate the protos before linking.
func corpus(t *testing.T, shared func(*descriptorpb.FileDescriptorProto), event func(*descriptorpb.FileDescriptorProto)) *protoregistry.Files {
	t.Helper()
	types := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("openits/types/v1/types.proto"),
		Package: proto.String("openits.types.v1"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("WireSource"),
			Field: []*descriptorpb.FieldDescriptorProto{
				field(fieldSpec{name: "decoder", number: 1, typ: tString}),
			},
		}},
		EnumType: []*descriptorpb.EnumDescriptorProto{{
			Name: proto.String("Severity"),
			Value: []*descriptorpb.EnumValueDescriptorProto{
				{Name: proto.String("SEVERITY_UNSPECIFIED"), Number: proto.Int32(0)},
				{Name: proto.String("SEVERITY_MAJOR"), Number: proto.Int32(2)},
				{Name: proto.String("SEVERITY_MINOR"), Number: proto.Int32(1)},
			},
		}},
		Options: &descriptorpb.FileOptions{GoPackage: proto.String("example.com/types/v1;typesv1")},
	}
	zero := int32(0)
	events := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("openits/svc/v1/events.proto"),
		Package:    proto.String("openits.svc.v1"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"openits/types/v1/types.proto", "google/protobuf/timestamp.proto"},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: proto.String("Changed"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field(fieldSpec{name: "kind", number: 99, typ: tString}),
					field(fieldSpec{name: "count", number: 2, typ: tUint32, optional: true, oneof: proto.Int32(1)}),
					field(fieldSpec{name: "source", number: 1, typ: tMessage, typeName: ".openits.types.v1.WireSource"}),
					field(fieldSpec{name: "severity", number: 3, typ: tEnum, typeName: ".openits.types.v1.Severity"}),
					field(fieldSpec{name: "at", number: 4, typ: tMessage, typeName: ".google.protobuf.Timestamp"}),
					field(fieldSpec{name: "item", number: 5, typ: tMessage, typeName: ".openits.svc.v1.Item", repeated: true}),
					field(fieldSpec{name: "a", number: 6, typ: tString, oneof: &zero}),
					field(fieldSpec{name: "b", number: 7, typ: tUint32, oneof: &zero}),
				},
				OneofDecl: []*descriptorpb.OneofDescriptorProto{{Name: proto.String("choice")}, {Name: proto.String("_count")}},
			},
			{
				Name: proto.String("Item"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field(fieldSpec{name: "label", number: 1, typ: tString}),
				},
			},
			{
				Name: proto.String("Sibling"),
				Field: []*descriptorpb.FieldDescriptorProto{
					field(fieldSpec{name: "x", number: 1, typ: tString}),
				},
			},
		},
		Options: &descriptorpb.FileOptions{GoPackage: proto.String("example.com/svc/v1;svcv1")},
	}
	if shared != nil {
		shared(types)
	}
	if event != nil {
		event(events)
	}
	files := new(protoregistry.Files)
	if err := files.RegisterFile(mustFile(t, protoregistry.GlobalFiles, findWKT(t))); err != nil {
		t.Fatal(err)
	}
	for _, fdp := range []*descriptorpb.FileDescriptorProto{types, events} {
		f, err := protodesc.NewFile(fdp, files)
		if err != nil {
			t.Fatalf("link %s: %v", fdp.GetName(), err)
		}
		if err := files.RegisterFile(f); err != nil {
			t.Fatal(err)
		}
	}
	return files
}

func findWKT(t *testing.T) *descriptorpb.FileDescriptorProto {
	t.Helper()
	f, err := protoregistry.GlobalFiles.FindFileByPath("google/protobuf/timestamp.proto")
	if err != nil {
		t.Fatal(err)
	}
	return protodesc.ToFileDescriptorProto(f)
}

func mustFile(t *testing.T, r *protoregistry.Files, fdp *descriptorpb.FileDescriptorProto) protoreflect.FileDescriptor {
	t.Helper()
	f, err := protodesc.NewFile(fdp, r)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func root(t *testing.T, files *protoregistry.Files, name string) protoreflect.MessageDescriptor {
	t.Helper()
	d, err := files.FindDescriptorByName(protoreflect.FullName(name))
	if err != nil {
		t.Fatal(err)
	}
	return d.(protoreflect.MessageDescriptor)
}

func TestRender_CanonicalClosure(t *testing.T) {
	files := corpus(t, nil, nil)
	got := Render(root(t, files, "openits.svc.v1.Changed"))
	want := `message openits.svc.v1.Changed {
  openits.types.v1.WireSource source = 1;
  optional uint32 count = 2;
  openits.types.v1.Severity severity = 3;
  google.protobuf.Timestamp at = 4;
  repeated openits.svc.v1.Item item = 5;
  string kind = 99;
  oneof choice {
    string a = 6;
    uint32 b = 7;
  }
}

message openits.svc.v1.Item {
  string label = 1;
}

enum openits.types.v1.Severity {
  SEVERITY_UNSPECIFIED = 0;
  SEVERITY_MINOR = 1;
  SEVERITY_MAJOR = 2;
}

message openits.types.v1.WireSource {
  string decoder = 1;
}
`
	if got != want {
		t.Fatalf("Render =\n%s\nwant\n%s", got, want)
	}
	if strings.Contains(got, "Sibling") {
		t.Fatal("closure pulled in a message the root does not reach")
	}
}

// The shape must move for every change that moves the wire or its meaning,
// including ones that leave the module's own file untouched, and must not
// move for changes that cannot.
func TestDigest_Sensitivity(t *testing.T) {
	digest := func(shared, event func(*descriptorpb.FileDescriptorProto)) string {
		files := corpus(t, shared, event)
		return Digest(root(t, files, "openits.svc.v1.Changed"))
	}
	base := digest(nil, nil)

	moves := map[string][2]func(*descriptorpb.FileDescriptorProto){
		// The v0.6.0 case: a scalar loses (or gains) explicit presence.
		"presence": {nil, func(f *descriptorpb.FileDescriptorProto) {
			m := f.MessageType[0]
			m.Field[1].Proto3Optional = nil
			m.Field[1].OneofIndex = nil
			m.OneofDecl = m.OneofDecl[:1]
		}},
		"shared message gains a field": {func(f *descriptorpb.FileDescriptorProto) {
			f.MessageType[0].Field = append(f.MessageType[0].Field, field(fieldSpec{name: "extra", number: 2, typ: tString}))
		}, nil},
		"shared enum gains a value": {func(f *descriptorpb.FileDescriptorProto) {
			f.EnumType[0].Value = append(f.EnumType[0].Value, &descriptorpb.EnumValueDescriptorProto{Name: proto.String("SEVERITY_CRITICAL"), Number: proto.Int32(3)})
		}, nil},
		"field renumbered": {nil, func(f *descriptorpb.FileDescriptorProto) {
			f.MessageType[0].Field[0].Number = proto.Int32(98)
		}},
		"field retyped": {nil, func(f *descriptorpb.FileDescriptorProto) {
			f.MessageType[1].Field[0].Type = tUint32.Enum()
		}},
		"field renamed (changes JSON)": {nil, func(f *descriptorpb.FileDescriptorProto) {
			f.MessageType[1].Field[0].Name = proto.String("name")
		}},
	}
	for name, mut := range moves {
		t.Run("moves on "+name, func(t *testing.T) {
			if got := digest(mut[0], mut[1]); got == base {
				t.Fatalf("digest did not change")
			}
		})
	}

	stays := map[string][2]func(*descriptorpb.FileDescriptorProto){
		"fields reordered in source": {nil, func(f *descriptorpb.FileDescriptorProto) {
			fs := f.MessageType[0].Field
			fs[0], fs[2] = fs[2], fs[0]
		}},
		"unreached sibling changes": {nil, func(f *descriptorpb.FileDescriptorProto) {
			f.MessageType[2].Field[0].Type = tUint32.Enum()
		}},
		"reserved range added": {nil, func(f *descriptorpb.FileDescriptorProto) {
			f.MessageType[1].ReservedRange = []*descriptorpb.DescriptorProto_ReservedRange{{Start: proto.Int32(9), End: proto.Int32(10)}}
		}},
	}
	for name, mut := range stays {
		t.Run("holds on "+name, func(t *testing.T) {
			if got := digest(mut[0], mut[1]); got != base {
				t.Fatalf("digest changed")
			}
		})
	}
}

func TestOwnAndRenderFile(t *testing.T) {
	files := corpus(t, nil, nil)
	r := root(t, files, "openits.svc.v1.Changed")
	own := Own(r.ParentFile(), r)
	var names []string
	for _, m := range own {
		names = append(names, string(m.Name()))
	}
	if strings.Join(names, ",") != "Changed,Item" {
		t.Fatalf("Own = %v, want [Changed Item] (Sibling belongs to another module)", names)
	}
	got := RenderFile(r.ParentFile(), own, []string{"header"})
	want := `// header

syntax = "proto3";

package openits.svc.v1;

option go_package = "example.com/svc/v1;svcv1";

import "google/protobuf/timestamp.proto";
import "openits/types/v1/types.proto";

message Changed {
  string kind = 99;
  optional uint32 count = 2;
  .openits.types.v1.WireSource source = 1;
  .openits.types.v1.Severity severity = 3;
  .google.protobuf.Timestamp at = 4;
  repeated .openits.svc.v1.Item item = 5;
  oneof choice {
    string a = 6;
    uint32 b = 7;
  }
}

message Item {
  string label = 1;
}
`
	if got != want {
		t.Fatalf("RenderFile =\n%s\nwant\n%s", got, want)
	}
}

func TestDefaultJSONName(t *testing.T) {
	for in, want := range map[string]string{"occupancy_count": "occupancyCount", "kind": "kind", "a_b_c": "aBC", "x_1": "x1"} {
		if got := defaultJSONName(in); got != want {
			t.Errorf("defaultJSONName(%q) = %q, want %q", in, got, want)
		}
	}
}
