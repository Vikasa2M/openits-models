package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

const (
	reservedKind   = 99
	reservedSource = 100
)

// FieldLock is the checked-in wire map: message -> field -> tag. It guarantees
// a field's tag never changes once assigned.
type FieldLock struct {
	// Messages[msg][field] = tag
	Messages map[string]map[string]int `yaml:"messages"`
}

func LoadFieldLock(path string) (*FieldLock, error) {
	l := &FieldLock{Messages: map[string]map[string]int{}}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return l, nil
	}
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(b, l); err != nil {
		return nil, err
	}
	if l.Messages == nil {
		l.Messages = map[string]map[string]int{}
	}
	return l, nil
}

// Assign returns the tag for every field, in a stable way. reserved maps a
// subset of fields (by name) to the exact tag they must receive — normally
// at most "kind" -> reservedKind and/or "source" -> reservedSource. Assign
// itself has no notion of which YANG *type* a "kind"/"source"-named field
// has: the wire-format invariant that tag 99 always means "identityref
// kind" and tag 100 always means "the WireSource shared-message reference"
// — never merely "a field spelled kind/source" — can only be enforced by
// the caller (EmitMessage), which is the only place with the field's YANG
// type/grouping info to decide. Fields absent from reserved are treated
// exactly like any other field, including ones literally named "kind" or
// "source" (e.g. a plain `leaf source { type string; }`): they get a normal
// sequential tag. Regardless of reserved, tags 99/100 are never handed out
// as a sequential tag to any field, reserved or not — so a reserved field
// added to the message later can never collide with a sequential tag
// assigned earlier. Previously-assigned data fields keep their tag; new
// fields take the next free tag >=1 (skipping 99/100 and any tag already
// used).
func (l *FieldLock) Assign(message string, fields []string, reserved map[string]int) map[string]int {
	m := l.Messages[message]
	if m == nil {
		m = map[string]int{}
		l.Messages[message] = m
	}
	used := map[int]bool{}
	for _, tag := range m {
		used[tag] = true
	}
	out := map[string]int{}
	next := 1
	nextFree := func() int {
		for used[next] || next == reservedKind || next == reservedSource {
			next++
		}
		used[next] = true
		return next
	}
	for _, f := range fields {
		if tag, ok := reserved[f]; ok {
			m[f] = tag
		} else if _, ok := m[f]; !ok {
			m[f] = nextFree()
		}
		out[f] = m[f]
	}
	return out
}

func (l *FieldLock) Save(path string) error {
	// Deterministic YAML: yaml.v3 sorts map keys, so output is stable.
	b, err := yaml.Marshal(l)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
