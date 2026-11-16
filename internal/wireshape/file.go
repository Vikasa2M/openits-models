package wireshape

import (
	"fmt"
	"sort"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// Own returns the top-level messages of file that roots reach, in
// declaration order: the messages a module itself contributes to a file it
// may share with sibling modules. A message is owned when it, or any message
// nested in it, is in the closure of roots.
func Own(file protoreflect.FileDescriptor, roots ...protoreflect.MessageDescriptor) []protoreflect.MessageDescriptor {
	reached := map[protoreflect.FullName]bool{}
	for _, d := range Closure(roots...) {
		reached[d.FullName()] = true
	}
	var out []protoreflect.MessageDescriptor
	msgs := file.Messages()
	for i := 0; i < msgs.Len(); i++ {
		md := msgs.Get(i)
		if reachedWithin(md, reached) {
			out = append(out, md)
		}
	}
	return out
}

func reachedWithin(md protoreflect.MessageDescriptor, reached map[protoreflect.FullName]bool) bool {
	if reached[md.FullName()] {
		return true
	}
	nested := md.Messages()
	for i := 0; i < nested.Len(); i++ {
		if reachedWithin(nested.Get(i), reached) {
			return true
		}
	}
	return false
}

// RenderFile prints msgs, which must all be top-level messages of file, as a
// compilable proto3 file in file's package with file's go_package. Fields
// keep declaration order; every type reference is fully qualified; imports
// are exactly the files the printed messages reference outside themselves.
// header is written first as // comment lines.
func RenderFile(file protoreflect.FileDescriptor, msgs []protoreflect.MessageDescriptor, header []string) string {
	printed := map[protoreflect.FullName]bool{}
	var mark func(md protoreflect.MessageDescriptor)
	mark = func(md protoreflect.MessageDescriptor) {
		printed[md.FullName()] = true
		nested := md.Messages()
		for i := 0; i < nested.Len(); i++ {
			mark(nested.Get(i))
		}
	}
	for _, md := range msgs {
		mark(md)
	}
	imports := map[string]bool{}
	var collect func(md protoreflect.MessageDescriptor)
	collect = func(md protoreflect.MessageDescriptor) {
		fields := md.Fields()
		for i := 0; i < fields.Len(); i++ {
			fd := fields.Get(i)
			for _, t := range fieldTypes(fd) {
				if !printed[t.FullName()] {
					imports[t.ParentFile().Path()] = true
				}
			}
		}
		nested := md.Messages()
		for i := 0; i < nested.Len(); i++ {
			if !nested.Get(i).IsMapEntry() {
				collect(nested.Get(i))
			}
		}
	}
	for _, md := range msgs {
		collect(md)
	}

	var b strings.Builder
	for _, h := range header {
		b.WriteString(strings.TrimRight("// "+h, " ") + "\n")
	}
	if len(header) > 0 {
		b.WriteByte('\n')
	}
	b.WriteString("syntax = \"proto3\";\n\n")
	fmt.Fprintf(&b, "package %s;\n\n", file.Package())
	if gp := goPackage(file); gp != "" {
		fmt.Fprintf(&b, "option go_package = %q;\n\n", gp)
	}
	if len(imports) > 0 {
		paths := make([]string, 0, len(imports))
		for p := range imports {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			fmt.Fprintf(&b, "import %q;\n", p)
		}
		b.WriteByte('\n')
	}
	for i, md := range msgs {
		if i > 0 {
			b.WriteByte('\n')
		}
		printMessage(&b, md, "")
	}
	return b.String()
}

// fieldTypes is the message and enum types a field refers to, looking
// through map entries to their key and value.
func fieldTypes(fd protoreflect.FieldDescriptor) []protoreflect.Descriptor {
	if fd.IsMap() {
		return append(fieldTypes(fd.MapKey()), fieldTypes(fd.MapValue())...)
	}
	switch {
	case fd.Message() != nil:
		return []protoreflect.Descriptor{fd.Message()}
	case fd.Enum() != nil:
		return []protoreflect.Descriptor{fd.Enum()}
	}
	return nil
}

func goPackage(file protoreflect.FileDescriptor) string {
	type goPackager interface{ GetGoPackage() string }
	if o, ok := file.Options().(goPackager); ok {
		return o.GetGoPackage()
	}
	return ""
}

func printMessage(b *strings.Builder, md protoreflect.MessageDescriptor, indent string) {
	fmt.Fprintf(b, "%smessage %s {\n", indent, md.Name())
	inner := indent + "  "
	fields := md.Fields()
	done := map[protoreflect.FullName]bool{}
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		od := fd.ContainingOneof()
		if od == nil || od.IsSynthetic() {
			b.WriteString(inner)
			b.WriteString(sourceFieldLine(fd))
			b.WriteByte('\n')
			continue
		}
		if done[od.FullName()] {
			continue
		}
		done[od.FullName()] = true
		fmt.Fprintf(b, "%soneof %s {\n", inner, od.Name())
		ofs := od.Fields()
		for j := 0; j < ofs.Len(); j++ {
			b.WriteString(inner + "  ")
			b.WriteString(sourceFieldLine(ofs.Get(j)))
			b.WriteByte('\n')
		}
		fmt.Fprintf(b, "%s}\n", inner)
	}
	nested := md.Messages()
	for i := 0; i < nested.Len(); i++ {
		if nested.Get(i).IsMapEntry() {
			continue
		}
		b.WriteByte('\n')
		printMessage(b, nested.Get(i), inner)
	}
	enums := md.Enums()
	for i := 0; i < enums.Len(); i++ {
		b.WriteByte('\n')
		printEnum(b, enums.Get(i), inner)
	}
	fmt.Fprintf(b, "%s}\n", indent)
}

func printEnum(b *strings.Builder, ed protoreflect.EnumDescriptor, indent string) {
	fmt.Fprintf(b, "%senum %s {\n", indent, ed.Name())
	vals := ed.Values()
	for i := 0; i < vals.Len(); i++ {
		fmt.Fprintf(b, "%s  %s = %d;\n", indent, vals.Get(i).Name(), vals.Get(i).Number())
	}
	fmt.Fprintf(b, "%s}\n", indent)
}

// sourceFieldLine is fieldLine with proto source syntax for type references
// (a leading dot makes every name fully qualified).
func sourceFieldLine(fd protoreflect.FieldDescriptor) string {
	line := fieldLine(fd)
	for _, t := range fieldTypes(fd) {
		name := string(t.FullName())
		line = strings.Replace(line, name+" ", "."+name+" ", 1)
		line = strings.Replace(line, name+">", "."+name+">", 1)
		line = strings.Replace(line, name+",", "."+name+",", 1)
	}
	return line
}
