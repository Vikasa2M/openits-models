package main

import (
	"testing"

	"github.com/openconfig/goyang/pkg/yang"
)

func TestProtoScalar(t *testing.T) {
	cases := []struct {
		name   string
		typ    *yang.YangType
		want   string
		wantTS bool
	}{
		{"string", &yang.YangType{Kind: yang.Ystring}, "string", false},
		{"bool", &yang.YangType{Kind: yang.Ybool}, "bool", false},
		{"int8", &yang.YangType{Kind: yang.Yint8}, "int32", false},
		{"int64", &yang.YangType{Kind: yang.Yint64}, "int64", false},
		{"uint16", &yang.YangType{Kind: yang.Yuint16}, "uint32", false},
		{"uint64", &yang.YangType{Kind: yang.Yuint64}, "uint64", false},
		{"decimal64", &yang.YangType{Kind: yang.Ydecimal64}, "string", false},
		{"identityref", &yang.YangType{Kind: yang.Yidentityref}, "string", false},
		// date-and-time is a typedef whose Name is "date-and-time".
		{"date-and-time", &yang.YangType{Kind: yang.Ystring, Name: "date-and-time"}, "google.protobuf.Timestamp", true},
	}
	for _, c := range cases {
		got, ts := ProtoScalar(c.typ)
		if got != c.want || ts != c.wantTS {
			t.Errorf("%s: got (%q,%v) want (%q,%v)", c.name, got, ts, c.want, c.wantTS)
		}
	}
}
