// Package wireshape renders the wire shape of protobuf messages: a canonical,
// diffable text of every message and enum a set of root messages can put on
// the wire, as compiled.
//
// A schema-registry snapshot pins one module revision's contract. Its YANG
// says what the model means; its shape says what a producer compiled from
// that revision actually sends. The shape is computed from compiled
// descriptors rather than from YANG, so it changes whenever the bytes or
// their meaning change, whatever caused it: a YANG edit, a generator change
// (the YANG-to-proto mapping), or a change to a shared type the module
// reaches through an import. It ignores anything that cannot change the
// wire: comments, declaration order, reserved ranges and file layout.
//
// The same renderer runs at generation time over a protoc descriptor set and
// at conformance time over the descriptors a producer has linked in, so a
// producer can be checked against the snapshot its ce-dataschema names.
package wireshape

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// External reports whether a declaration belongs to a package whose shape is
// fixed outside this repository (the protobuf well-known types). Closures
// name such types but do not expand them.
func External(d protoreflect.Descriptor) bool {
	return strings.HasPrefix(string(d.FullName()), "google.protobuf.")
}

// Closure returns every message and enum reachable from roots through field
// types, roots included, sorted by full name. Map entry messages are not
// listed (Render shows a map field as map<K, V>), and external types are
// not expanded.
func Closure(roots ...protoreflect.MessageDescriptor) []protoreflect.Descriptor {
	seen := map[protoreflect.FullName]protoreflect.Descriptor{}
	var visit func(md protoreflect.MessageDescriptor)
	visit = func(md protoreflect.MessageDescriptor) {
		if External(md) {
			return
		}
		if !md.IsMapEntry() {
			if _, ok := seen[md.FullName()]; ok {
				return
			}
			seen[md.FullName()] = md
		}
		fields := md.Fields()
		for i := 0; i < fields.Len(); i++ {
			fd := fields.Get(i)
			switch {
			case fd.Message() != nil:
				visit(fd.Message())
			case fd.Enum() != nil && !External(fd.Enum()):
				seen[fd.Enum().FullName()] = fd.Enum()
			}
		}
	}
	for _, r := range roots {
		visit(r)
	}
	out := make([]protoreflect.Descriptor, 0, len(seen))
	for _, d := range seen {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FullName() < out[j].FullName() })
	return out
}

// Render returns the canonical shape text of the closure of roots: each
// message and enum once, in full-name order; a message's fields in field
// number order, real oneofs after the plain fields in name order; an enum's
// values in number order. Types are written by full name.
func Render(roots ...protoreflect.MessageDescriptor) string {
	var b strings.Builder
	for i, d := range Closure(roots...) {
		if i > 0 {
			b.WriteByte('\n')
		}
		switch d := d.(type) {
		case protoreflect.MessageDescriptor:
			renderMessage(&b, d)
		case protoreflect.EnumDescriptor:
			renderEnum(&b, d)
		}
	}
	return b.String()
}

// Digest is the sha256 of Render(roots), written "sha256:<hex>".
func Digest(roots ...protoreflect.MessageDescriptor) string {
	sum := sha256.Sum256([]byte(Render(roots...)))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func renderMessage(b *strings.Builder, md protoreflect.MessageDescriptor) {
	fmt.Fprintf(b, "message %s {\n", md.FullName())
	var plain []protoreflect.FieldDescriptor
	oneofs := map[string][]protoreflect.FieldDescriptor{}
	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		if od := fd.ContainingOneof(); od != nil && !od.IsSynthetic() {
			oneofs[string(od.Name())] = append(oneofs[string(od.Name())], fd)
			continue
		}
		plain = append(plain, fd)
	}
	byNumber := func(fs []protoreflect.FieldDescriptor) {
		sort.Slice(fs, func(i, j int) bool { return fs[i].Number() < fs[j].Number() })
	}
	byNumber(plain)
	for _, fd := range plain {
		b.WriteString("  " + fieldLine(fd) + "\n")
	}
	names := make([]string, 0, len(oneofs))
	for n := range oneofs {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fs := oneofs[n]
		byNumber(fs)
		fmt.Fprintf(b, "  oneof %s {\n", n)
		for _, fd := range fs {
			b.WriteString("    " + fieldLine(fd) + "\n")
		}
		b.WriteString("  }\n")
	}
	b.WriteString("}\n")
}

func renderEnum(b *strings.Builder, ed protoreflect.EnumDescriptor) {
	fmt.Fprintf(b, "enum %s {\n", ed.FullName())
	vals := ed.Values()
	vs := make([]protoreflect.EnumValueDescriptor, 0, vals.Len())
	for i := 0; i < vals.Len(); i++ {
		vs = append(vs, vals.Get(i))
	}
	sort.Slice(vs, func(i, j int) bool {
		if vs[i].Number() != vs[j].Number() {
			return vs[i].Number() < vs[j].Number()
		}
		return vs[i].Name() < vs[j].Name()
	})
	for _, v := range vs {
		fmt.Fprintf(b, "  %s = %d;\n", v.Name(), v.Number())
	}
	b.WriteString("}\n")
}

// fieldLine renders one field as `[label] type name = number [opts];`. Only
// options that change the encoding are shown: a json_name that differs from
// the default, and packed = false on a packable repeated field.
func fieldLine(fd protoreflect.FieldDescriptor) string {
	var parts []string
	switch {
	case fd.IsMap():
		parts = append(parts, fmt.Sprintf("map<%s, %s>", typeName(fd.MapKey()), typeName(fd.MapValue())))
	case fd.IsList():
		parts = append(parts, "repeated", typeName(fd))
	case fd.HasOptionalKeyword():
		parts = append(parts, "optional", typeName(fd))
	default:
		parts = append(parts, typeName(fd))
	}
	line := fmt.Sprintf("%s %s = %d", strings.Join(parts, " "), fd.Name(), fd.Number())
	var opts []string
	if fd.JSONName() != defaultJSONName(string(fd.Name())) {
		opts = append(opts, fmt.Sprintf("json_name = %q", fd.JSONName()))
	}
	if fd.IsList() && packable(fd.Kind()) && !fd.IsPacked() {
		opts = append(opts, "packed = false")
	}
	if len(opts) > 0 {
		line += " [" + strings.Join(opts, ", ") + "]"
	}
	return line + ";"
}

func typeName(fd protoreflect.FieldDescriptor) string {
	switch fd.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return string(fd.Message().FullName())
	case protoreflect.EnumKind:
		return string(fd.Enum().FullName())
	default:
		return fd.Kind().String()
	}
}

func packable(k protoreflect.Kind) bool {
	switch k {
	case protoreflect.StringKind, protoreflect.BytesKind, protoreflect.MessageKind, protoreflect.GroupKind:
		return false
	}
	return true
}

// defaultJSONName is protoc's default json_name: underscores dropped, the
// letter after each one upper-cased.
func defaultJSONName(name string) string {
	var b strings.Builder
	upper := false
	for _, r := range name {
		if r == '_' {
			upper = true
			continue
		}
		if upper && 'a' <= r && r <= 'z' {
			r -= 'a' - 'A'
		}
		upper = false
		b.WriteRune(r)
	}
	return b.String()
}
