# YANG-to-wire mapping

**Version:** `1.0.0-draft`

OpenITS publishes its models in YANG and its payloads in two generated
encodings: Protobuf messages and, for notifications, JSON Schema documents
describing the RFC 7951 JSON form. This document is the **contract** for how
the YANG becomes those encodings. The reference implementation is
`tools/yang-proto-gen` in this repo; the test vectors at the end pin it to this
text.

The contract exists because the mapping is ours. OpenConfig's payload encoding
is fixed by outside standards (RFC 7951 JSON, gNMI paths), so no OpenConfig
model's meaning on the wire changes unless the model does. Ours is whatever our
generator emits. In v0.6.0 a generator change gave every optional scalar proto3
explicit presence. The bytes of every event carrying a legitimately zero value
changed, and with them its `ce-id`, but no module revision recorded the
change, because no YANG had changed. From this version on a mapping change is a change to this
specification. It bumps this document's version, and it bumps the revision of
every module whose generated output it alters (see
[Versioning of the mapping](#13-versioning-of-the-mapping)).

The key words MUST, MUST NOT, REQUIRED, SHALL, SHALL NOT, SHOULD, SHOULD NOT,
RECOMMENDED, MAY and OPTIONAL are to be interpreted as described in BCP 14
(RFC 2119, RFC 8174) when, and only when, they appear in all capitals.

This document binds two parties. A **generator** is any implementation of the
mapping; its MUSTs describe what it emits. **Producers and consumers** of
payloads are bound by the statements about what a field carries and what its
absence means. Each rule cites the vector(s) that pin it, in the form
`vector-name`; the index is under [Test vectors](#test-vectors).

## 1. Scope and inputs

### 1.1 Encodings

The mapping defines:

1. **Protobuf.** proto3 source files, and therefore the binary wire encoding
   and the Protobuf JSON mapping (protojson) of the messages they declare.
   protojson derives member names and enum value names from proto names, so
   the names in §4 and §7 are on the wire for protojson consumers, not only in
   source.
2. **JSON Schema.** One draft 2020-12 schema per live notification, describing
   the RFC 7951 JSON encoding of that notification's content.

Protojson and RFC 7951 JSON are different encodings of the same data. They
differ in member names (`occurred_at` or `occurredAt` against `occurred-at`),
in enum values (`LEVEL_LOW` against `low`) and in choice shape (§9). The JSON
Schema describes RFC 7951 JSON only.

### 1.2 Contract surface

The contract is, for Protobuf: each output file's path, `package` and
`go_package`; each message's name and nesting; each field's name, number, type,
label and `oneof` membership; each `oneof` name; each enum's name, value names
and value numbers; and each `reserved` statement. For JSON Schema: each output
file's name and the validation meaning of its content. What is outside the
contract is listed in [§12](#12-what-is-not-part-of-the-contract).

### 1.3 Input module set

The input is every `*.yang` file directly under `yang/` and under `yang/ietf/`,
processed together. Modules under `yang/augments/` and `yang/deviations/` are
not inputs: augments and deviations have no mapping in either encoding. Modules
are processed in module-name order.

## 2. Which statements produce messages

### 2.1 Notifications

Each top-level `notification` of a routed module (§3.1) whose `status` is
neither `deprecated` nor `obsolete` MUST produce one root message in the
module's `events.proto` and one JSON Schema document (§11). A deprecated or
obsolete notification MUST produce neither. Vectors: `module-selection`,
`routing`.

### 2.2 Config/state roots

A module named in the config/state allowlist (§3.3) is a *config/state
module*. Each of its top-level `container` and `list` statements whose `status`
is neither `deprecated` nor `obsolete` MUST produce one root message in the
module's `state.proto`. Top-level leaves, leaf-lists and choices of a
config/state module produce nothing. Config/state roots produce no JSON Schema.
Vector: `config-state`.

A top-level `container` or `list` in a module that is not on the allowlist MUST
produce nothing. Vector: `module-selection`.

### 2.3 Actions

An `action` reached while mapping a message's children MUST produce two
messages, `<Action>Request` from its `input` and `<Action>Response` from its
`output`, and MUST NOT produce a field in the enclosing message. A missing
`input` or `output` produces an empty message. Vector: `config-state`. (An
action with neither `input` nor `output` is mapped differently today; see
[Open issues](#open-issues).)

### 2.4 Statements that produce nothing

- A top-level `rpc` statement. Vectors: `module-selection`, `config-state`.
- A module that matches no route and declares no live notification, such as
  a typedef- or grouping-only module, produces nothing of its own; its
  typedefs and groupings are mapped where other modules use them. Vectors:
  `module-selection`, `shared-grouping`.
- `anydata` and `anyxml`, and a `notification` nested in a data node. These
  produce no field, but see [Open issues](#open-issues) for their effect on
  field numbering.

The `status` of a node below a root is not consulted. A deprecated or obsolete
leaf, container or list inside a mapped root MUST be mapped like any other.

### 2.5 Unrouted notifications are an error

A module that matches no route (§3.1) and declares at least one live
notification MUST fail generation. Silently dropping it is not conformant.
Vector: `unrouted-notification`.

## 3. Routing

### 3.1 Service routes

A module is *routed* when its name starts with one of the prefixes below. The
prefixes are disjoint, so at most one matches.

| Module-name prefix | Proto package | Events file |
|---|---|---|
| `openits-common-` | `openits.common.v1` | `openits/common/v1/events.proto` |
| `openits-signal-control` | `openits.signal_control.v1` | `openits/signal_control/v1/events.proto` |
| `openits-dms` | `openits.dms.v1` | `openits/dms/v1/events.proto` |
| `openits-ess` | `openits.ess.v1` | `openits/ess/v1/events.proto` |
| `openits-rsu` | `openits.rsu.v1` | `openits/rsu/v1/events.proto` |
| `openits-ramp-metering` | `openits.ramp_metering.v1` | `openits/ramp_metering/v1/events.proto` |
| `openits-perception` | `openits.perception.v1` | `openits/perception/v1/events.proto` |
| `openits-traffic-sensor` | `openits.traffic_sensor.v1` | `openits/traffic_sensor/v1/events.proto` |
| `openits-reversible-lane` | `openits.reversible_lane.v1` | `openits/reversible_lane/v1/events.proto` |
| `openits-cctv` | `openits.cctv.v1` | `openits/cctv/v1/events.proto` |
| `openits-zone-occupancy` | `openits.zone_occupancy.v1` | `openits/zone_occupancy/v1/events.proto` |
| `openits-work-zone` | `openits.work_zone.v1` | `openits/work_zone/v1/events.proto` |

The notifications of every module matching one prefix MUST be collected into
that route's single events file. Vector: `routing`. Today a routed module
causes its events file to be written even when no module of the route declares
a notification, leaving the file with an empty body; see
[Open issues](#open-issues).

Adding a route is a change to this table, and therefore to this specification.

### 3.2 Output directory and go_package

A proto package `P` maps to the directory `D(P)`, which is `P` with each `.`
replaced by `/`. Its `go_package` MUST be

```
github.com/Vikasa2M/openits-models/pkg/proto/<D(P)>;<G(P)>
```

where `G(P)` is `P` with the leading `openits.` removed and every `.` and `_`
deleted: `openits.ramp_metering.v1` gives `rampmeteringv1`. Every file of one
proto package shares one `go_package`. Vector: `routing`.

### 3.3 Config/state files

These modules, matched by exact name, are config/state modules. Their roots go
to `D(P)/state.proto`, in the same proto package and `go_package` as the
service's events file. Vector: `config-state`.

| Module | Proto package |
|---|---|
| `openits-cctv` | `openits.cctv.v1` |
| `openits-dms` | `openits.dms.v1` |
| `openits-ess` | `openits.ess.v1` |
| `openits-perception` | `openits.perception.v1` |
| `openits-ramp-metering` | `openits.ramp_metering.v1` |
| `openits-reversible-lane` | `openits.reversible_lane.v1` |
| `openits-rsu` | `openits.rsu.v1` |
| `openits-signal-control` | `openits.signal_control.v1` |
| `openits-traffic-sensor` | `openits.traffic_sensor.v1` |

Each of these names also matches a service route, so a config/state module's
own notifications, if any, go to the events file.

### 3.4 Types files

- **Per-service types.** Every enum used by any file of package `P` MUST be
  declared in `D(P)/types.proto`, in package `P` (§7.1). The file is written
  only when it declares at least one enum. Vectors: `enum-mapping`,
  `enum-registry`.
- **Shared types.** Every shared-grouping message (§10) MUST be declared in
  `openits/types/v1/types.proto`, package `openits.types.v1`, `go_package`
  per §3.2 (`typesv1`). The file is written only when it declares at least one
  message. Vector: `shared-grouping`.

### 3.5 File layout

Each `.proto` file MUST declare `syntax = "proto3";`, its `package`, and its
`go_package` option, which is the only option emitted. It MUST import:

- `google/protobuf/timestamp.proto` when it uses `google.protobuf.Timestamp`;
- its package's `types.proto` when it references an enum declared there;
- `openits/types/v1/types.proto` when it references a shared message.

Vectors: `type-mapping`, `enum-registry`, `shared-grouping`.

### 3.6 JSON Schema files

The schema for notification `N` of module `M` MUST be written to
`D(P)/M.N.schema.json`, beside the events file of `M`'s route. A
`*.schema.json` file in such a directory that the run did not write, because
its notification was deleted, deprecated or obsoleted, MUST be removed.
Vectors: `module-selection`, `routing` (file naming; the vectors start from an
empty directory, so they do not exercise removal).

## 4. Naming

### 4.1 Base conversions

- **Message name** `ProtoName(id)`: split the YANG identifier on `-` and `_`,
  upper-case the first character of each part, leave the rest of each part
  unchanged, and concatenate. `wind_speed-sensor` gives `WindSpeedSensor`;
  `rawADC` gives `RawADC`.
- **Field name** `fieldName(id)`: replace every `-` with `_`. Case is
  preserved: `air_temp-c` gives `air_temp_c`, and `rawADC` stays `rawADC`.

Vector: `naming`.

### 4.2 Root messages

A notification or config/state root is named `ProtoName(id)` and is never
qualified. A root that collides with another message in the same `go_package`
MUST fail generation (§4.7). Vectors: `module-selection`, `duplicate-message`.

### 4.3 Nested container and list messages

A container or list child `c` of a message named `Parent` maps to a message
declared at file level (not nested in `Parent`) and named as follows. A message
name computed here is the *final* name of `Parent`, including any
qualification `Parent` received.

1. If `c` is named `config` or `state`, the name is `Parent` +
   `ProtoName(c)`, always: `RadioConfig`, `RoadsideUnitState`. Vectors:
   `config-state`, `naming`.
2. Otherwise, if `ProtoName(c)` *collides*, the name is `Parent` +
   `ProtoName(c)` at every occurrence; no occurrence keeps the bare name.
   Otherwise the name is `ProtoName(c)`. Vector: `naming`.
3. If the name from step 1 or 2 equals the base name (§7.4) of any enum used
   in the same `go_package`, the name is `Parent` + that name: `GateState`
   becomes `GateGateState`. Vector: `naming`.

A bare name *collides* when it occurs more than once in this count: every
container and list node, at any depth, under every root that maps to the same
`go_package` (events and config/state roots together), excluding nodes named
`config` or `state` (which are counted through, not counted), descending
through `choice` and `case` nodes without counting them, and skipping actions
entirely. Containers that map to shared-grouping messages (§10) are counted
too, although they are not named by this rule. Roots in different
`go_package`s never collide. Vector: `naming`.

### 4.4 Choice-case messages

A `case` with more than one child maps to a message named `ProtoName(case)`,
declared nested inside the enclosing message. Its name is never qualified.
Vector: `choice-case`.

### 4.5 Action messages

An action maps to `ProtoName(action)` + `Request` and `ProtoName(action)` +
`Response`, declared at file level. They are never qualified. Vector:
`config-state`.

### 4.6 Shared-grouping messages

A shared grouping maps to `ProtoName(grouping)`, the grouping's name and not
its container's, declared in `openits.types.v1` and referenced as
`openits.types.v1.<Name>`. Vector: `shared-grouping`.

### 4.7 Name conflicts are errors

Generation MUST fail when:

- one file declares a message name twice, counting messages nested in another
  message (§4.4);
- two files sharing a `go_package` declare the same message name. Vector:
  `duplicate-message`;
- two YANG identifiers of one message map to the same field name, for example
  `rate-vph` and `rate_vph`. Vector: `field-name-collapse`.

A generator MUST NOT resolve any of these by renaming.

## 5. Field numbering

### 5.1 The lock

Field numbers are not derived from the YANG. They are assigned once and
recorded in the field-number lock, `field-numbers.yaml` at the repository root:
a map from *lock key* to a map from proto field name to field number. The lock
key of a message is `<proto package>.<MessageName>`, where `MessageName` is the
message's own name, without the name of any message it is nested in.
`openits.types.v1.Indiana` is the key of the case message `Indiana` nested in
`WireSource`. Vectors: `field-number-lock`, `shared-grouping`.

The lock is part of the source tree. A generator MUST read it before assigning
numbers and MUST write it back after.

### 5.2 Numbered names

The numbered names of a message are, in child order (§5.3), for each child:

- an action: none;
- a choice: for each of its cases in child order, the field name of the case's
  only child if it has one child, otherwise the field name of the case;
- anything else: its field name.

Every numbered name shares the message's one number space, including the
members of every `oneof`. Vector: `choice-case`.

### 5.3 Child order

Children declared directly in a statement are ordered by declaration.
Children with no declaration position in that statement, which are the nodes a
`uses` splices in, follow them, ordered by name. The top-level notifications of
a module are ordered by name. This order decides only which new field takes
which free number; once a number is assigned, the lock is the authority and
order no longer matters. Vectors: `presence`, `shared-grouping`.

### 5.4 Assignment

For each numbered name of a message:

1. If it is a reserved-tag field (§5.5), it MUST receive that tag.
2. Otherwise, if the lock already holds a number for it under this message's
   key, it MUST keep that number.
3. Otherwise it MUST receive the lowest number, counting from 1, that no entry
   under this message's key holds and that is neither 99 nor 100. A hole in
   the lock is therefore filled before the highest number is exceeded.

Vectors: `field-number-lock`, `tag-skips-reserved`.

### 5.5 Reserved tags

- **99** is the tag of a leaf named `kind` whose type is `identityref`, when it
  is a direct child of the message (not inside a choice).
- **100** is the tag of a container named `source` that comes from the grouping
  named `wire-source` when that grouping is shared (§10).

A leaf named `kind` of another type, or a field named `source` of any other
shape, receives an ordinary number. Tags 99 and 100 MUST NOT be assigned under
rule 3 to any field. Vectors: `field-number-lock`, `shared-grouping`,
`tag-skips-reserved`.

### 5.6 Append-only, and tombstones

A generator MUST NOT delete a lock entry. A lock entry whose field the message
no longer has is *retired*. For every retired entry the message MUST declare,
before its fields,

```
reserved <numbers, ascending, comma-separated>;
reserved "<names, in the same order>";
```

so no later field can reuse the number or the name. Renaming a YANG node is a
retirement plus a new field. A lock key whose message is no longer generated
stays in the lock and has no effect. Vectors: `field-number-lock`,
`tag-skips-reserved`.

## 6. Type mapping

### 6.1 Built-in and special-cased types

A typedef maps as the built-in type it resolves to, with the restrictions it
accumulates, except where a row below names the typedef.

| YANG type | Proto type | JSON Schema |
|---|---|---|
| `int8`, `int16`, `int32` | `int32` | `integer`, `minimum`, `maximum` |
| `int64` | `int64` | `string` |
| `uint8`, `uint16`, `uint32` | `uint32` | `integer`, `minimum`, `maximum` |
| `uint64` (and `yang:counter64`) | `uint64` | `string` |
| `decimal64` | `string` | `string` |
| `string` | `string` | `string`, with `pattern`/`allOf`, `minLength`, `maxLength` |
| `boolean` | `bool` | `boolean` |
| `enumeration` | generated enum (§7) | `string` with `enum` |
| `identityref` | `string` | `string` |
| `leafref` | the mapping of the leaf it resolves to (§6.3) | `string` |
| `union` | `string` | `string` |
| `binary` | `string` | `string` |
| `bits` | `string` | `string` |
| `empty` | `string` | `string` |
| `instance-identifier` | `string` | `string` |
| a typedef named `date-and-time` (`yang:date-and-time`) | `google.protobuf.Timestamp` | `string`, `format: date-time` |

A leaf-list maps to `repeated` of its type's proto mapping and to a JSON
`array` whose `items` is its type's schema. Vector: `type-mapping`.

The JSON Schema restrictions are derived as follows. Vector: `type-mapping`.

- **Integers** (8 to 32 bits) carry `minimum` and `maximum`, the lowest bound
  of the type's first range interval and the highest bound of its last. An
  unrestricted type carries its full built-in range.
- **Strings**: one `pattern` becomes `"pattern": "^(<pattern>)$"`; several
  become an `allOf` of one such anchored `pattern` each. The pattern text is
  copied from the YANG unchanged. `minLength` is the lowest length bound and
  `maxLength` the highest, each emitted only when it is greater than zero.
- No other type carries a restriction keyword. In particular `int64`,
  `uint64` and `decimal64` carry no range.

### 6.2 What a string field carries

Where a non-string YANG type maps to proto `string`, the field MUST carry the
value's RFC 7951 string form: `decimal64` as its decimal text; `bits` as the
space-separated names of the set bits; `binary` as base64; `instance-identifier`
per RFC 7951 §6.11. A `union` field MUST carry the lexical form of the member
type the value belongs to. (`empty` is unresolved; see
[Open issues](#open-issues).)

An `identityref` value MUST be the namespace-qualified form
`<defining-module>:<identity>`, in both encodings, even where RFC 7951 §6.8
would permit the bare identity name. Renaming an identity or moving it to
another module therefore changes the value on the wire (see
[Versioning](versioning.md#wire-compatibility)).

A proto integer field wider than its YANG type (`uint8` carried in `uint32`)
does not enforce the YANG range. A producer MUST NOT emit a value outside it.

### 6.3 Leafref

In Protobuf a `leafref` leaf maps exactly as the leaf its `path` resolves to,
including an enum (§7). The path is resolved in the data tree: `..` skips any
enclosing `choice` and `case`, a child step looks through them, `[...]`
predicates are ignored, a prefix on a step is ignored, and absolute paths
resolve from the module root. A chain of leafrefs is followed to its final
non-leafref leaf. A path that does not resolve maps to `string`. Vectors:
`type-mapping`, `presence`, `config-state`.

In JSON Schema every `leafref` maps to `{"type": "string"}`, whatever it
resolves to. See [Open issues](#open-issues).

### 6.4 Timestamps

`google.protobuf.Timestamp` carries an instant. The UTC offset of the
`date-and-time` value is not carried, and a consumer of the Protobuf encoding
MUST NOT expect it to round-trip. A Timestamp field is always a message field,
so it has presence without the `optional` label (§8.1). Vectors:
`type-mapping`, `presence`.

The `format: date-time` keyword is an annotation under the draft 2020-12
default vocabulary; a validator does not have to assert it.

## 7. Enumerations

### 7.1 Where an enum is declared

Every proto enum MUST be declared at file level in the `types.proto` of the
proto package that uses it (§3.4), and every reference to it from that
package's other files MUST use its bare name and import that file. Which file
first needed the enum has no effect on where it is declared. Vectors:
`enum-mapping`, `enum-registry`.

### 7.2 Identity of an enum

Within one proto package, two enumeration-typed leaves use the same enum when
they have the same base name (§7.4) and the same set of (member name, value)
pairs. Every proto package has its own set of enums; the same name in two
packages is two independent enums, and neither is qualified. Vector:
`enum-registry`.

### 7.3 Values

- Every YANG member maps to one enum value whose number is the member's YANG
  `value`, explicit or implicit. The member numbered 0, if there is one, MUST
  be listed first; the rest follow in ascending numeric order, so negative
  values follow the zero member.
- If no member is numbered 0, a sentinel `<PREFIX>_UNSPECIFIED = 0` MUST be
  listed first. The sentinel is not a YANG value. A producer MUST NOT set a
  field to it, and a consumer SHOULD treat it as not a value of the leaf.
- A YANG member that happens to be named `unspecified` and numbered 0 is an
  ordinary member. It maps to `<PREFIX>_UNSPECIFIED` like any other name and
  carries YANG meaning.

Vector: `enum-mapping`.

### 7.4 Names

- The **base name** of an enum is `ProtoName(src)`, where `src` is the
  typedef's name for a leaf typed by a typedef, and the leaf's own name for a
  leaf with an inline `enumeration`.
- An enum that is the first in its package with its base name is named by the
  base name, and its value prefix is `src` upper-cased with `-` replaced by
  `_`. Each value is named `<PREFIX>_<MEMBER>`, where `MEMBER` is the YANG
  member name upper-cased with `-` replaced by `_`: `LEVEL_LOW`,
  `STATE_OUT_OF_SERVICE`.
- An enum whose base name is already used in its package by a different value
  set is *qualified*: `src` is replaced by `<module>-<src>`, where `<module>`
  is the name of the module that instantiates the leaf, for both the enum name
  and the value prefix. `mode` in module `openits-cctv-events` becomes
  `OpenitsCctvEventsMode` with values `OPENITS_CCTV_EVENTS_MODE_*`.
- Which of two such enums is first is decided by emission order: every
  notification root in the package (modules in name order, notifications in
  name order, depth first in child order), then every config/state root
  (modules in name order, roots in declaration order).

An enum's names are never module-qualified except by the rule above. Vectors:
`enum-mapping`, `enum-registry`.

### 7.5 JSON Schema

An enumeration maps to `{"type": "string", "enum": [<member names>]}`. The
member names are the YANG names, not the proto value names. Vector:
`enum-mapping`.

## 8. Presence and cardinality

### 8.1 Proto labels

A leaf outside a choice MUST be emitted **bare** (no label) when it is
mandatory or when its proto type is a message type; otherwise it MUST be
labeled `optional`.

- A leaf is mandatory when its own statement says `mandatory true`, or when a
  `uses` statement of the leaf's immediately enclosing node carries
  `refine <leaf> { mandatory true; }`. A `refine ... { mandatory false; }`
  there makes it not mandatory. A refine wins over the leaf's own statement,
  and of two refines of one leaf the later wins. A refine whose target is
  deeper than a direct child (`refine "a/b"`) is ignored.
- The only message-typed leaf is a `date-and-time` leaf (§6.4).
- For a leafref leaf, mandatory-ness is the leafref leaf's own; whether the
  type is a message is decided by the resolved target (§6.3).
- A `default` statement has no effect on the label. A leaf with a default is
  labeled `optional`, and its absence means the YANG default applies.
- A list key leaf is treated like any other leaf, so it is labeled `optional`
  unless it is mandatory. See [Open issues](#open-issues).

Vector: `presence`.

A leaf-list MUST map to a `repeated` field, a list to a `repeated` field of its
message type, and a container (presence or not) to a singular field of its
message type, which has presence. Vectors: `presence`, `config-state`.

`optional` gives a scalar or enum field explicit presence: a field set to its
zero value is serialized, and a field never set is not. A producer clearing a
field MUST clear it, not set it to zero (see the
[ce-id spec](ce-id-spec.md) for why this matters to event identity).

### 8.2 JSON required and minItems

A property MUST be listed in its object's `required` array exactly when it is
a leaf that is mandatory by the rule in §8.1 and is not inside a choice.
Containers, lists and leaf-lists are never required. A list or leaf-list
carries `minItems` only when a `uses` of the enclosing node refines it with a
positive `min-elements`; a `min-elements` or `max-elements` declared on the list
itself is not mapped. See [Open issues](#open-issues). Vector: `presence`.

## 9. Choice and case

### 9.1 Protobuf

A `choice` MUST map to a `oneof` named `fieldName(choice)`. Its members are,
for each case in child order:

- a case with one child (including the shorthand case formed by a bare leaf in
  a choice): one member with that child's field name and type;
- a case with more than one child: one member named `fieldName(case)`, of the
  message type of §4.4, which is mapped from the case's children like any other
  message, including any choice it contains.

`oneof` members carry no label, and a `mandatory true` on the choice has no
effect. Members share the enclosing message's number space (§5.2). A one-child
case whose child is a leaf-list, container or list is not mapped correctly
today; see [Open issues](#open-issues). Vector: `choice-case`.

### 9.2 JSON Schema

The children of every case MUST be spliced into the enclosing object as if
they were its own properties, as RFC 7951 encodes them. They are never
required, and nothing expresses that members of different cases are mutually
exclusive. Vector: `choice-case`.

## 10. Shared groupings

### 10.1 Which groupings are shared

A grouping is *shared* when both hold over the whole input module set:

1. each `uses` of it splices exactly one node into its parent, and that node
   is a container (it is *container-shaped*);
2. it is used at two or more sites, counting one site per node it is spliced
   into.

Groupings are identified by name alone. A grouping that is flat (it splices
leaves) or used once is not shared, and its nodes map in place like any
others. Vector: `shared-grouping`.

### 10.2 Protobuf

The container of a shared grouping MUST map to one message (§4.6) declared
once in `openits/types/v1/types.proto`, and each use site to a field of type
`openits.types.v1.<Name>` named after the container. A shared grouping whose
container is named `config` or `state` is the exception: each use maps to a
per-parent message by §4.3. Vector: `shared-grouping`.

### 10.3 JSON Schema

Each use of a shared grouping's container MUST map to
`{"$ref": "#/$defs/<Name>"}`, with the container's object schema stored once
under that name in the document's top-level `$defs`. Every document carries
its own `$defs`; nothing is shared between documents. The `config`/`state`
exception of §10.2 does not apply here. Vector: `shared-grouping`.

## 11. JSON Schema mapping

### 11.1 Documents

Each live notification of a routed module MUST have exactly one schema (§3.6).
It describes the notification's content, which is the value of the RFC 7951
member `<module>:<notification>`, not that member itself. The document is an
object schema (§11.2) with `"$schema":
"https://json-schema.org/draft/2020-12/schema"` and, when non-empty, `$defs`
(§10.3). Vectors: `type-mapping`, `presence`.

### 11.2 Objects

The notification, each container, each list entry and each shared definition
MUST map to an object schema with `"type": "object"`, a `properties` member
holding one property per child (§9.2 for choices), `"additionalProperties":
false`, and `required` (§8.2) when at least one property is required. Property
names are the YANG identifiers unchanged. A list maps to
`{"type": "array", "items": <entry object>}`. Containers and lists are
inlined; only shared groupings go to `$defs`. Vectors: `presence`,
`shared-grouping`.

### 11.3 Values

Leaf and leaf-list types map by §6.1, enumerations by §7.5. Vectors:
`type-mapping`, `enum-mapping`.

## 12. What is not part of the contract

A difference in any of these is not a mapping change:

- comments and YANG `description` text, which neither encoding carries;
- whitespace, blank lines, indentation and trailing newlines;
- the order of declarations within a file, the textual order of fields within
  a message (their numbers are the contract), and the order of `import` lines;
- in JSON Schema, the order of object keys, of the `required` array and of the
  `enum` array;
- the formatting of `field-numbers.yaml`;
- the wording of generation errors. The conditions under which generation
  fails are the contract; the vectors pin the current wording only for
  convenience.

## 13. Versioning of the mapping

This specification has a semantic version.

- **MAJOR**: a change that alters the contract surface (§1.2) of output that
  some valid input already produced. The v0.6.0 presence change would have
  been one.
- **MINOR**: a mapping for input that previously failed generation or had no
  mapping, leaving all existing output unchanged.
- **PATCH**: editorial changes and new vectors for existing rules.

A change to the mapping MUST land in one change set with: the specification
text and its changelog entry; regenerated vectors; regenerated artifacts; and a
new `revision` statement in every module whose generated output changes on the
contract surface, with the schema-registry snapshot that follows from it.
[Versioning](versioning.md) describes how module revisions relate to the
release.

## Test vectors

Each vector is a directory under `tools/yang-proto-gen/testdata/wire-mapping/`
holding the YANG it runs on (`input/`), optionally the lock before generation
(`input/field-numbers.yaml`), and either the complete output tree plus the lock
after generation (`expected/`) or the substring the generation error must
contain (`expected-error.txt`).

| Vector | What it pins | Sections |
|---|---|---|
| `module-selection` | live, deprecated and obsolete notifications; rpc; top-level container outside the allowlist; unrouted typedef module | 2.1, 2.2, 2.4, 3.6 |
| `routing` | prefix routing, two modules into one events file, routed module without notifications, `go_package` | 3.1, 3.2, 3.6 |
| `unrouted-notification` | live notification in an unrouted module fails | 2.5 |
| `config-state` | config/state roots, config/state naming, list key leafref, actions, skipped top-level nodes | 2.2, 2.3, 3.3, 4.3, 4.5 |
| `naming` | message and field name conversion, collision qualification across events and state, enum-name avoidance | 4.1, 4.3 |
| `duplicate-message` | a root name used twice in one `go_package` fails | 4.2, 4.7 |
| `field-name-collapse` | two identifiers mapping to one field name fail | 4.7 |
| `field-number-lock` | lock reuse, hole filling, retired tags as `reserved`, tag 99, ordinary `kind`/`source`, stale lock keys | 5.1, 5.4, 5.5, 5.6 |
| `tag-skips-reserved` | 99 and 100 never assigned sequentially | 5.4, 5.5 |
| `type-mapping` | every built-in type and the special-cased derived types, JSON restrictions, leafref resolution | 3.5, 6.1, 6.3, 6.4 |
| `enum-mapping` | value order, synthesized sentinel, typedef reuse, qualification within a file | 7.1, 7.3, 7.4, 7.5 |
| `enum-registry` | one declaration per package in `types.proto`, independent registries per package, events-before-state qualification | 3.4, 7.1, 7.2, 7.4 |
| `presence` | `optional` against bare, `refine`, Timestamps, leafref presence, defaults, list keys, `required` and `minItems` | 5.3, 6.4, 8.1, 8.2 |
| `choice-case` | oneof members, nested case messages, shared number space, JSON splicing | 4.4, 5.2, 9.1, 9.2 |
| `shared-grouping` | shared, flat, single-use and `config` groupings; `openits.types.v1`; tag 100; `$defs` | 3.4, 4.6, 5.5, 10 |

Some vectors pin behavior listed under [Open issues](#open-issues), and say so
in their YANG. They record what the generator does today, so that fixing an
issue shows up as a failing vector and therefore as a change to this
specification.

### Keeping the vectors honest

`TestWireMappingVectors` (`tools/yang-proto-gen/wiremapping_vectors_test.go`)
runs the generator on each vector and compares every output file, and the
lock, byte for byte. When `protoc` is installed it also compiles each vector's
output, so a vector cannot pin a file `protoc` rejects.
`TestWireMappingSpecCitesVectors` fails when this index and the vector
directories disagree, when an indexed vector is cited by no rule, or when this
document's version differs from `testdata/wire-mapping/SPEC_VERSION`. Both run
under `go test ./...`, which CI runs.

The comparison is stricter than the contract: it also catches changes listed
in §12, which then need regenerated vectors but no version bump. To regenerate:

```
go test ./tools/yang-proto-gen -run TestWireMappingVectors -update-wire-vectors
```

then review the diff against §13.

## Open issues

These are behaviors of the reference generator that look wrong, are
inconsistent between the two encodings, or are fragile. The rules above
describe the generator as it is and do not specify around these. Resolving any
of them is a mapping change under §13. References are to files in
`tools/yang-proto-gen/`.

1. **Same-named groupings are conflated.** Groupings are counted and shared by
   bare name (`grouping.go:67`, `grouping.go:93`), and the shared message is
   emitted once per name (`emit.go:748`). Two modules that each declare a
   container-shaped grouping with the same name, each used once, are treated
   as one shared grouping: the second use site's proto field gets the first
   grouping's message, with the wrong fields. JSON Schema, which builds
   `$defs` per document, is correct, so the encodings disagree. Whether such
   a pair counts as container-shaped is also decided by whichever site is
   visited last, in Go map order (`grouping.go:68`), so it is not
   deterministic.
2. **Renaming a message resets its field numbers.** The lock key is the
   derived message name (`emit.go:537`). Collision qualification (§4.3), enum
   avoidance and enum qualification depend on the rest of the package, so an
   additive change elsewhere (a second `sensor` list) can rename an existing
   message. The renamed message starts a new lock bucket: its tombstones are
   lost and, if its tags were not dense and in child order, its fields are
   renumbered.
3. **Moving a field to a reserved tag loses its old tag.** When a field that
   already has a lock entry becomes a reserved-tag field (§5.5), for example a
   `kind` leaf retyped to `identityref`, its entry is overwritten
   (`fieldnum.go:78`). The old number is neither kept nor emitted as
   `reserved`, and a later generation can assign it to a new field. Pinned by
   `field-number-lock` (`ModeChanged`).
4. **Enum names depend on emission order.** The first enum emitted with a base
   name keeps it (`emit.go:1157`). A new notification, or one whose name sorts
   earlier, can take the bare name from an existing enum with a different value
   set, renaming that enum and every value name, which protojson carries.
5. **A third same-named enum in one module duplicates a name.** Qualification
   adds only the module name (`emit.go:1158`), so a third distinct value set
   with the same base name in the same module gets the same qualified name as
   the second, and `protoc` rejects the file.
6. **Enum value names can collide.** Value names are derived by upper-casing
   and replacing `-` (`emit.go:1182`, `emit.go:1190`). Within one enum,
   `foo-bar` and `foo_bar` map to one name, and the second member is silently
   dropped (`emit.go:1233`) while JSON Schema keeps both. Across enums, proto
   enum values share the package namespace, so `a-b`/`c` and `a`/`b-c` both
   produce `A_B_C` and `protoc` rejects the package.
7. **Identifier characters are not sanitized.** `ProtoName` and `fieldName`
   (`emit.go:323`, `emit.go:335`) handle only `-` and `_`. A YANG identifier
   containing `.`, or an enum member name containing a space or other
   character proto does not allow, produces an invalid proto name.
8. **An enum with no zero member and a member named `unspecified` has no zero
   value.** The sentinel is suppressed when it would duplicate that member's
   name (`emit.go:1227`), leaving proto3 without a zero value; `protoc` rejects
   it.
9. **A one-child case holding a leaf-list, container or list is mismapped.**
   The member is typed by `leafFieldType` (`emit.go:597`). A leaf-list becomes
   a singular scalar, losing every element but one; JSON Schema has an array.
   A container or list panics the generator. Pinned for the leaf-list by
   `choice-case` (`track-points`).
10. **An action with neither input nor output is mapped as a container.**
    Actions are recognized by goyang's `RPC` field (`emit.go:521`,
    `emit.go:547`), which goyang leaves nil in that case. The action becomes a
    nested empty message and a field of the enclosing message. Pinned by
    `config-state` (`ping`).
11. **anydata, anyxml and nested notifications consume field numbers.** Every
    child but an action gets a numbered name (`emit.go:535`), but these
    produce no field. The lock records a number for a field that does not
    exist, and the proto does not reserve it.
12. **List keys are optional.** RFC 7950 §7.8.2 makes every key leaf
    mandatory, but presence (`emit.go:806`) and `required`
    (`jsonschema_emit.go:197`) read only the leaf's own `mandatory`. Keys are
    `optional` in proto and not required in JSON. Pinned by `presence` and
    `config-state`.
13. **leafref is resolved in proto but not in JSON Schema.** Proto takes the
    target's type (§6.3); JSON Schema falls through to `string`
    (`jsonschema_type.go:77`). A leafref to `uint32` is a JSON number in RFC
    7951 but the schema requires a string. Pinned by `type-mapping` and
    `presence`.
14. **A typedef of `date-and-time` is not a timestamp.** Detection matches the
    immediate typedef name only (`typemap.go:10`, `jsonschema_type.go:9`). A
    leaf typed by a typedef derived from `yang:date-and-time` maps to proto
    `string` and to JSON `pattern` without `format`, while a direct
    `yang:date-and-time` leaf maps to `Timestamp`. Conversely any typedef
    named `date-and-time`, from any module, maps to `Timestamp`. Pinned by
    `type-mapping` (`at-alias`).
15. **`empty` has no defined value.** It maps to `string` in both encodings
    (`typemap.go:37`, `jsonschema_type.go:77`). RFC 7951 encodes `empty` as
    `[null]`, which the schema rejects, and nothing says what the proto string
    holds.
16. **`union` and `binary` are carried as proto `string`.** A `union` with a
    numeric member is a JSON number in RFC 7951, which the `{"type":
    "string"}` schema rejects (`jsonschema_type.go:77`). `binary` maps to
    proto `string` rather than `bytes` (`typemap.go:37`).
17. **JSON ranges and patterns are approximations.** A multi-interval range
    becomes one `minimum`/`maximum` span (`jsonschema_type.go:23`), so the
    gaps validate; `int64`, `uint64` and `decimal64` carry no range. Patterns
    are copied verbatim (`jsonschema_type.go:56`), but YANG uses the XML
    Schema regex dialect and JSON Schema uses ECMA-262, which differ (for
    example `\p{...}` and character-class subtraction); `invert-match` is not
    handled.
18. **Cardinality is mapped only through `refine`.** `minItems` comes only
    from `refine ... min-elements` (`jsonschema_emit.go:98`); a `min-elements`
    on the list itself, and every `max-elements`, is dropped. Pinned by
    `presence` (`page`). Deep refine paths are ignored in both encodings
    (`emit.go:835`, `jsonschema_emit.go:85`), and a non-presence container
    holding a mandatory leaf is not itself required.
19. **The two encodings disagree on `config`/`state` sharing.** Proto never
    shares a `config` or `state` container (§10.2); JSON Schema does
    (`jsonschema_emit.go:181`). The shapes agree but the `$defs` name
    (`Settings`) has no proto counterpart. Pinned by `shared-grouping`.
20. **Choice-case messages are scoped as if top-level.** Proto scopes a nested
    message to its parent, but uniqueness checks and the lock key treat
    `ProtoName(case)` as package-wide (§4.7, §5.1). Two notifications with a
    multi-child case of the same name fail generation, and a top-level message
    of that name would share its lock bucket.
21. **A routed module with no notifications writes an empty events file.**
    The events file is created on routing, not on use (`main.go:158`). Pinned
    by `routing` (`openits/work_zone/v1/events.proto`).
22. **Obsolete descendants are still mapped.** Status is checked only on
    roots (`main.go:167`, `main.go:194`). Keeping deprecated nodes is correct;
    whether an `obsolete` node should stay on the wire, or be retired into the
    lock's tombstones, is undecided.
23. **The data-model document says the proto comes from ygot.** The
    "Encodings and transports" section of [the data model](data-model.md)
    names the reference encoding as "generated from the YANG via ygot". The
    event and state protos come from `tools/yang-proto-gen`; ygot generates
    only the separate `pkg/yang` Go structs.

## Non-goals

Outside this specification: how `scripts/update-schema-registry.sh` assembles
the schema-registry snapshots from these files; how `bindings/nats/asyncapi.yaml`
embeds the schemas; the Go code `protoc-gen-go` generates from the proto; the
hand-written `api/proto/command.proto` and `api/proto/device.proto`; the ygot
tree under `api/proto/yang` and `pkg/yang`; and `ce-type` and subject
derivation.

## Changelog

- **1.0.0-draft** (2026-11-14): first version. Specifies the mapping as the
  generator implements it at the v0.8.0 release line, and records the open
  issues found while writing it.
