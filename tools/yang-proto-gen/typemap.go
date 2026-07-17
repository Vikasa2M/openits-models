package main

import "github.com/openconfig/goyang/pkg/yang"

// ProtoScalar maps a leaf's YangType to a proto scalar type. The bool return
// reports the google.protobuf.Timestamp special case (yang:date-and-time), so
// the caller can add the import.
func ProtoScalar(t *yang.YangType) (string, bool) {
	// date-and-time is modeled as a string typedef; detect by typedef name.
	if t.Name == "date-and-time" {
		return "google.protobuf.Timestamp", true
	}
	switch t.Kind {
	case yang.Ystring:
		return "string", false
	case yang.Ybool:
		return "bool", false
	case yang.Yint8, yang.Yint16, yang.Yint32:
		return "int32", false
	case yang.Yint64:
		return "int64", false
	case yang.Yuint8, yang.Yuint16, yang.Yuint32:
		return "uint32", false
	case yang.Yuint64:
		return "uint64", false
	case yang.Ydecimal64:
		return "string", false // RFC 7951: decimal64 is a JSON string
	case yang.Yidentityref:
		return "string", false // RFC 7951: "module:identity"
	case yang.Yenum:
		// EmitMessage intercepts enum-kind leaves before calling ProtoScalar
		// (see leafFieldType) and emits a proto enum instead; this case is a
		// defensive fallback for any caller that uses ProtoScalar directly.
		return "string", false
	case yang.Yleafref, yang.Yunion:
		return "string", false // resolved/handled in a later task; string fallback
	default:
		return "string", false
	}
}
