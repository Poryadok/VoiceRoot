package r23contracttest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type manifest struct {
	Version               int                    `json:"version"`
	Metadata              metadata               `json:"metadata"`
	Files                 []fileSpec             `json:"files"`
	ForbiddenRPCExposures []forbiddenRPCExposure `json:"forbidden_rpc_exposure"`
	DraftContracts        []draftContract        `json:"draft_contracts,omitempty"`
}

type metadata struct {
	BaseSHA                string `json:"base_sha"`
	DocsMergeSHA           string `json:"docs_merge_sha"`
	ProposalSHA256         string `json:"proposal_sha256"`
	BallotSHA256           string `json:"ballot_sha256"`
	PlanSHA256             string `json:"plan_sha256"`
	DispatchSHA256         string `json:"dispatch_sha256"`
	HashRule               string `json:"hash_rule"`
	ProofDigestRule        string `json:"proof_digest_rule"`
	P2ParityGate           string `json:"p2_generated_parity_gate"`
	GeneratedTargetsSHA256 string `json:"generated_targets_sha256"`
	VectorsSHA256          string `json:"deterministic_vectors_sha256"`
	CompatibilityDoc       string `json:"compatibility_rule"`
}

type fileSpec struct {
	Path                     string        `json:"path"`
	Package                  string        `json:"package"`
	Syntax                   string        `json:"syntax"`
	Dependencies             []string      `json:"dependencies"`
	Options                  fileOptions   `json:"options"`
	ContractProjectionSHA256 string        `json:"contract_projection_sha256"`
	Enums                    []enumSpec    `json:"enums,omitempty"`
	Messages                 []messageSpec `json:"messages,omitempty"`
	Services                 []serviceSpec `json:"services,omitempty"`
}

type fileOptions struct {
	GoPackage         string `json:"go_package,omitempty"`
	JavaPackage       string `json:"java_package,omitempty"`
	JavaMultipleFiles bool   `json:"java_multiple_files,omitempty"`
}

type reservedRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type enumSpec struct {
	Name           string          `json:"name"`
	Exact          bool            `json:"exact"`
	Values         map[string]int  `json:"values"`
	ReservedNames  []string        `json:"reserved_names"`
	ReservedRanges []reservedRange `json:"reserved_ranges"`
}

type messageSpec struct {
	Name            string          `json:"name"`
	Exact           bool            `json:"exact"`
	UnknownFields   string          `json:"unknown_fields,omitempty"`
	Hashing         string          `json:"hashing,omitempty"`
	ForbiddenFields []string        `json:"forbidden_fields,omitempty"`
	ReservedNames   []string        `json:"reserved_names"`
	ReservedRanges  []reservedRange `json:"reserved_ranges"`
	Fields          []fieldSpec     `json:"fields"`
}

type forbiddenRPCExposure struct {
	Package           string   `json:"package"`
	AllowedR23Methods []string `json:"allowed_r23_methods"`
}

type draftContract struct {
	ID         string        `json:"id"`
	Status     string        `json:"status"`
	Package    string        `json:"package"`
	Unresolved []string      `json:"unresolved"`
	Enums      []enumSpec    `json:"enums,omitempty"`
	Messages   []messageSpec `json:"messages,omitempty"`
	Services   []serviceSpec `json:"services,omitempty"`
}

type fieldSpec struct {
	Name           string `json:"name"`
	Number         int    `json:"number"`
	Label          string `json:"label"`
	Type           string `json:"type"`
	TypeName       string `json:"type_name,omitempty"`
	Oneof          string `json:"oneof,omitempty"`
	Proto3Optional bool   `json:"proto3_optional,omitempty"`
	Deprecated     bool   `json:"deprecated,omitempty"`
}

type serviceSpec struct {
	Name    string       `json:"name"`
	Methods []methodSpec `json:"methods"`
}

type methodSpec struct {
	Name        string   `json:"name"`
	Input       string   `json:"input"`
	Output      string   `json:"output"`
	Exposure    string   `json:"exposure"`
	Callers     []string `json:"callers"`
	NoStreaming bool     `json:"no_streaming"`
	Annotations []string `json:"annotations,omitempty"`
}

type generatedTargetsManifest struct {
	Version   int               `json:"version"`
	Authority string            `json:"authority"`
	Gate      string            `json:"gate"`
	Targets   []generatedTarget `json:"targets"`
}

type generatedTarget struct {
	Language        string   `json:"language"`
	Path            string   `json:"path"`
	SourceProto     string   `json:"source_proto"`
	Committed       bool     `json:"committed"`
	RequiredSymbols []string `json:"required_symbols"`
}

type deterministicVectors struct {
	Version            int                   `json:"version"`
	HashRule           string                `json:"hash_rule"`
	Schemas            map[string]wireSchema `json:"schemas"`
	Vectors            []deterministicCase   `json:"vectors"`
	Relations          vectorRelations       `json:"relations"`
	RuntimeConsumers   runtimeConsumers      `json:"runtime_consumers"`
	ProofDigestVectors []proofDigestCase     `json:"proof_digest_vectors"`
}

type wireSchema struct {
	UnknownPolicy string            `json:"unknown_policy"`
	Fields        []wireFieldSchema `json:"fields"`
}

type wireFieldSchema struct {
	Number   int    `json:"number"`
	Name     string `json:"name"`
	WireType int    `json:"wire_type"`
	Repeated bool   `json:"repeated"`
	TypeName string `json:"type_name,omitempty"`
}

type deterministicCase struct {
	ID               string `json:"id"`
	Message          string `json:"message"`
	InputWireHex     string `json:"input_wire_hex"`
	CanonicalWireHex string `json:"canonical_wire_hex"`
	DomainSHA256     string `json:"domain_sha256"`
}

type vectorRelations struct {
	ChangedKnownFieldHashDiffers []string `json:"changed_known_field_hash_differs"`
	UnknownValueOrderHashDiffers []string `json:"unknown_value_order_hash_differs"`
	RepeatedOrderHashDiffers     []string `json:"repeated_order_hash_differs"`
	AuthRepeatedOrderHashDiffers []string `json:"auth_repeated_order_hash_differs"`
	FQNPrefixHashDiffers         []string `json:"fqn_prefix_hash_differs"`
	WrapperVectors               []string `json:"wrapper_vectors"`
}

type runtimeConsumers struct {
	Go   []string `json:"go"`
	Java []string `json:"java"`
}

type proofDigestCase struct {
	ExactUTF8 string `json:"exact_utf8"`
	SHA256    string `json:"sha256"`
}

type descriptorSet struct {
	Files []fileDescriptor `json:"file"`
}

type fileDescriptor struct {
	Name           string                `json:"name"`
	Package        string                `json:"package"`
	Syntax         string                `json:"syntax"`
	Dependencies   []string              `json:"dependency"`
	Options        descriptorFileOptions `json:"options"`
	Enums          []enumDescriptor      `json:"enumType"`
	Messages       []msgDescriptor       `json:"messageType"`
	Services       []svcDescriptor       `json:"service"`
	SourceCodeInfo sourceCodeInfo        `json:"sourceCodeInfo"`
}

type enumDescriptor struct {
	Name           string                `json:"name"`
	Values         []enumValueDescriptor `json:"value"`
	ReservedNames  []string              `json:"reservedName"`
	ReservedRanges []reservedRange       `json:"reservedRange"`
}

type descriptorFileOptions struct {
	GoPackage         string `json:"goPackage"`
	JavaPackage       string `json:"javaPackage"`
	JavaMultipleFiles bool   `json:"javaMultipleFiles"`
}

type enumValueDescriptor struct {
	Name   string `json:"name"`
	Number int    `json:"number"`
}

type msgDescriptor struct {
	Name           string            `json:"name"`
	Fields         []fieldDescriptor `json:"field"`
	Oneofs         []oneofDescriptor `json:"oneofDecl"`
	ReservedNames  []string          `json:"reservedName"`
	ReservedRanges []reservedRange   `json:"reservedRange"`
}

type oneofDescriptor struct {
	Name string `json:"name"`
}

type fieldDescriptor struct {
	Name           string       `json:"name"`
	Number         int          `json:"number"`
	Label          string       `json:"label"`
	Type           string       `json:"type"`
	TypeName       string       `json:"typeName"`
	OneofIndex     optionalInt  `json:"oneofIndex"`
	Proto3Optional bool         `json:"proto3Optional"`
	Options        fieldOptions `json:"options"`
}

type optionalInt struct {
	Set   bool
	Value int
}

func (o *optionalInt) UnmarshalJSON(data []byte) error {
	o.Set = true
	return json.Unmarshal(data, &o.Value)
}

type fieldOptions struct {
	Deprecated bool `json:"deprecated"`
}

type svcDescriptor struct {
	Name    string             `json:"name"`
	Methods []methodDescriptor `json:"method"`
}

type methodDescriptor struct {
	Name            string `json:"name"`
	Input           string `json:"inputType"`
	Output          string `json:"outputType"`
	ClientStreaming bool   `json:"clientStreaming"`
	ServerStreaming bool   `json:"serverStreaming"`
}

type sourceCodeInfo struct {
	Locations []sourceLocation `json:"location"`
}

type sourceLocation struct {
	Path                    []int    `json:"path"`
	LeadingComments         string   `json:"leadingComments"`
	TrailingComments        string   `json:"trailingComments"`
	LeadingDetachedComments []string `json:"leadingDetachedComments"`
}

func TestManifestSelfConsistency(t *testing.T) {
	m := loadManifest(t)
	if m.Version != 2 {
		t.Fatalf("manifest version = %d, want 2", m.Version)
	}
	wantMetadata := metadata{
		BaseSHA:                "edc52d46406d97283f81dfbdfc916660f50dcc69",
		DocsMergeSHA:           "2bf7e0844bda087258886b08cb77f25606ff1387",
		ProposalSHA256:         "3877EBCDB9CA4E7308DF8CD3122112DC2A7FC4A7D3D8BD020ACD8AB12EEFE4F4",
		BallotSHA256:           "563A79303638060B289EC438B029518E807EEBFDB63DF6E631033EE4B2B3BEB7",
		PlanSHA256:             "B6E5A242AF8420C3958B4C68764A2C3FCD98936B6161F5C194C87EDE446D1F9A",
		DispatchSHA256:         "1855B1DC665CA9EACDE5BA4732DC12661BA8736F0D41EE712A3075723BCC60F6",
		HashRule:               "sha256(utf8_fully_qualified_message_name || 0x00 || deterministic_protobuf_bytes)",
		ProofDigestRule:        "sha256(exact_utf8_proof_bytes)",
		P2ParityGate:           "make r23-p2-generated-parity",
		GeneratedTargetsSHA256: "0e250cdf2d5a9d5a2d0b48ff32dd95affabb95e9a31e683506449c0752d62da4",
		VectorsSHA256:          "af12194346ecd704ac79ca67ac04c993fa9bca292608f01a6dbd759f18ee6656",
		CompatibilityDoc:       "additive; preserve existing numbers; removed names and numbers are reserved",
	}
	if !reflect.DeepEqual(m.Metadata, wantMetadata) {
		t.Fatalf("authority metadata changed\n got: %#v\nwant: %#v", m.Metadata, wantMetadata)
	}
	assertFileSHA256(t, filepath.Join(repoRoot(t), "protos", "voice", "r23_contract_test", "generated_targets.json"), m.Metadata.GeneratedTargetsSHA256)
	assertFileSHA256(t, filepath.Join(repoRoot(t), "protos", "voice", "r23_contract_test", "testdata", "deterministic_contract_vectors.json"), m.Metadata.VectorsSHA256)

	wantFiles := []string{
		"voice/auth/v1/auth.proto",
		"voice/bot/v1/bot.proto",
		"voice/calls/v1/calls.proto",
		"voice/chat/v1/chat.proto",
		"voice/common/v1/space_lifecycle.proto",
		"voice/events/v1/jetstream_events.proto",
		"voice/file/v1/file.proto",
		"voice/matchmaking/v1/matchmaking.proto",
		"voice/messaging/v1/messaging.proto",
		"voice/notification/v1/notification.proto",
		"voice/role/v1/role.proto",
		"voice/search/v1/search.proto",
		"voice/space/v1/space.proto",
		"voice/subscription/v1/subscription.proto",
	}
	seenFiles := map[string]bool{}
	for _, file := range m.Files {
		if seenFiles[file.Path] {
			t.Errorf("duplicate file %q", file.Path)
		}
		seenFiles[file.Path] = true
		if file.Path == "voice/common/v1/common.proto" {
			t.Errorf("R23 lifecycle types must use dedicated space_lifecycle.proto, not common.proto")
		}
		if file.Path == "voice/common/v1/space_lifecycle.proto" {
			wantOptions := fileOptions{GoPackage: "voice.app/voice/common/v1;commonv1", JavaPackage: "app.voice.common.v1", JavaMultipleFiles: true}
			if !reflect.DeepEqual(file.Options, wantOptions) {
				t.Errorf("shared lifecycle codegen options = %#v, want %#v", file.Options, wantOptions)
			}
		}
		if file.Syntax != "proto3" {
			t.Errorf("%s syntax = %q, want proto3", file.Path, file.Syntax)
		}
		if !sort.StringsAreSorted(file.Dependencies) {
			t.Errorf("%s dependencies must be sorted", file.Path)
		}
		if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(file.ContractProjectionSHA256) {
			t.Errorf("%s contract_projection_sha256 must be 64 lowercase hex", file.Path)
		}
		validateFileSpec(t, file)
		got := fingerprint(file)
		if got != file.ContractProjectionSHA256 {
			t.Errorf("%s manifest projection fingerprint = %s, want %s", file.Path, got, file.ContractProjectionSHA256)
		}
	}
	gotFiles := make([]string, 0, len(seenFiles))
	for path := range seenFiles {
		gotFiles = append(gotFiles, path)
	}
	sort.Strings(gotFiles)
	if !reflect.DeepEqual(gotFiles, wantFiles) {
		t.Errorf("contract file set changed\n got: %v\nwant: %v", gotFiles, wantFiles)
	}
	requireParticipantCoverage(t, m)
	validateForbiddenRPCExposure(t, m.ForbiddenRPCExposures)
	if len(m.DraftContracts) != 0 {
		t.Errorf("independently frozen manifest must not retain review-required drafts: %d", len(m.DraftContracts))
	}
}

func requireParticipantCoverage(t *testing.T, m manifest) {
	t.Helper()
	want := map[string]string{
		"voice.role.v1.RoleService":                 "RetireSpace",
		"voice.chat.v1.ChatService":                 "PurgeSpace",
		"voice.messaging.v1.MessagingService":       "PurgeSpace",
		"voice.file.v1.FileService":                 "PurgeSpace",
		"voice.calls.v1.VoiceService":               "PurgeSpace",
		"voice.matchmaking.v1.MatchmakingService":   "PurgeSpace",
		"voice.search.v1.SearchService":             "PurgeSpace",
		"voice.subscription.v1.SubscriptionService": "PurgeSpace",
		"voice.bot.v1.BotService":                   "PurgeSpace",
		"voice.notification.v1.NotificationService": "PurgeSpace",
	}
	covered := map[string]bool{}
	for _, file := range m.Files {
		for _, service := range file.Services {
			fqService := file.Package + "." + service.Name
			terminal, participant := want[fqService]
			if !participant {
				continue
			}
			methods := map[string]bool{}
			for _, method := range service.Methods {
				methods[method.Name] = true
			}
			if !methods["ApplySpaceLifecycleFence"] || !methods[terminal] {
				t.Errorf("%s must expose ApplySpaceLifecycleFence and %s", fqService, terminal)
			}
			covered[fqService] = true
		}
	}
	if len(covered) != len(want) {
		t.Errorf("participant coverage = %d, want %d", len(covered), len(want))
	}
}

func validateForbiddenRPCExposure(t *testing.T, specs []forbiddenRPCExposure) {
	t.Helper()
	wantPackages := []string{"voice.analytics.v1", "voice.auth.v1", "voice.moderation.v1", "voice.realtime.v1", "voice.story.v1", "voice.user.v1"}
	gotPackages := make([]string, 0, len(specs))
	for _, spec := range specs {
		gotPackages = append(gotPackages, spec.Package)
		if spec.Package == "voice.auth.v1" {
			want := []string{"AcknowledgeSpaceDeletionProofReceipt", "ConsumeSpaceDeletionProof", "GetSpaceDeletionProofReceipt", "IssueSpaceDeletionProof"}
			got := append([]string(nil), spec.AllowedR23Methods...)
			sort.Strings(got)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Auth allowed R23 RPCs = %v, want %v", got, want)
			}
		} else if len(spec.AllowedR23Methods) != 0 {
			t.Errorf("%s must expose no R23 RPC", spec.Package)
		}
	}
	sort.Strings(gotPackages)
	if !reflect.DeepEqual(gotPackages, wantPackages) {
		t.Errorf("negative RPC exposure packages = %v, want %v", gotPackages, wantPackages)
	}
}

func TestHarnessIndexesSyntheticDescriptor(t *testing.T) {
	data := []byte(`{"file":[{"name":"voice/example/v1/example.proto","package":"voice.example.v1","messageType":[{"name":"Request","field":[{"name":"id","number":1,"label":"LABEL_OPTIONAL","type":"TYPE_STRING","jsonName":"id"}]}],"service":[{"name":"ExampleService","method":[{"name":"Get","inputType":".voice.example.v1.Request","outputType":".voice.example.v1.Request"}]}]}]}`)
	var set descriptorSet
	if err := json.Unmarshal(data, &set); err != nil {
		t.Fatal(err)
	}
	if len(set.Files) != 1 || set.Files[0].Messages[0].Fields[0].Number != 1 {
		t.Fatalf("descriptor fixture decoded incorrectly: %#v", set)
	}
}

func TestDeterministicContractVectors(t *testing.T) {
	var vectors deterministicVectors
	loadStrictJSON(t, filepath.Join(repoRoot(t), "protos", "voice", "r23_contract_test", "testdata", "deterministic_contract_vectors.json"), &vectors)
	if vectors.Version != 3 || vectors.HashRule != "sha256(utf8_fully_qualified_message_name || 0x00 || deterministic_protobuf_bytes)" {
		t.Fatalf("unexpected deterministic vector authority: %#v", vectors)
	}
	if len(vectors.Schemas) != 7 || len(vectors.Vectors) != 12 || len(vectors.ProofDigestVectors) != 1 {
		t.Fatalf("insufficient deterministic fixtures")
	}
	validateVectorSchemasAgainstManifest(t, vectors.Schemas, loadManifest(t))
	byID := map[string]deterministicCase{}
	for _, vector := range vectors.Vectors {
		if byID[vector.ID].ID != "" {
			t.Fatalf("duplicate vector id %s", vector.ID)
		}
		byID[vector.ID] = vector
		schema, ok := vectors.Schemas[vector.Message]
		if !ok {
			t.Errorf("%s lacks schema for %s", vector.ID, vector.Message)
			continue
		}
		input, err := hex.DecodeString(vector.InputWireHex)
		if err != nil {
			t.Errorf("%s invalid input wire hex: %v", vector.ID, err)
			continue
		}
		canonical, err := canonicalizeWire(input, schema)
		if err != nil {
			t.Errorf("%s schema decode: %v", vector.ID, err)
			continue
		}
		if got := hex.EncodeToString(canonical); got != vector.CanonicalWireHex {
			t.Errorf("%s canonical wire = %s, want %s", vector.ID, got, vector.CanonicalWireHex)
		}
		reencoded, err := canonicalizeWire(canonical, schema)
		if err != nil || !bytes.Equal(reencoded, canonical) {
			t.Errorf("%s decode/re-encode is not stable: %x %v", vector.ID, reencoded, err)
		}
		hashInput := append(append([]byte(vector.Message), 0), canonical...)
		sum := sha256.Sum256(hashInput)
		if got := hex.EncodeToString(sum[:]); got != vector.DomainSHA256 {
			t.Errorf("%s domain hash = %s, want %s", vector.ID, got, vector.DomainSHA256)
		}
	}
	assertHashDiffRelation(t, byID, "changed known field", vectors.Relations.ChangedKnownFieldHashDiffers, false)
	assertHashDiffRelation(t, byID, "unknown value/order", vectors.Relations.UnknownValueOrderHashDiffers, false)
	assertHashDiffRelation(t, byID, "repeated order", vectors.Relations.RepeatedOrderHashDiffers, false)
	assertHashDiffRelation(t, byID, "Auth repeated order", vectors.Relations.AuthRepeatedOrderHashDiffers, false)
	assertHashDiffRelation(t, byID, "FQN prefix", vectors.Relations.FQNPrefixHashDiffers, true)
	for _, id := range vectors.Relations.WrapperVectors {
		vector := byID[id]
		wire, _ := hex.DecodeString(vector.CanonicalWireHex)
		fields, err := parseWire(wire)
		if err != nil || len(fields) != 1 || fields[0].Number != 1 || fields[0].WireType != 2 {
			t.Errorf("%s is not a valid one-field wrapper: %#v %v", id, fields, err)
			continue
		}
		schema := vectors.Schemas[vector.Message]
		nestedSchema, hasNestedSchema := vectors.Schemas[strings.TrimPrefix(schema.Fields[0].TypeName, ".")]
		if hasNestedSchema {
			if _, err := canonicalizeWire(fields[0].Payload, nestedSchema); err != nil {
				t.Errorf("%s nested message does not decode: %v", id, err)
			}
		}
	}
	if !reflect.DeepEqual(vectors.RuntimeConsumers.Go, []string{"manifest_base", "fence_request", "chat_wrapper", "file_wrapper_same_wire", "page_repeated_ab", "page_repeated_ba"}) ||
		!reflect.DeepEqual(vectors.RuntimeConsumers.Java, []string{"auth_binding_factors_12", "auth_binding_factors_21", "auth_receipt_wrapper"}) {
		t.Errorf("runtime consumer vector assignment changed: %#v", vectors.RuntimeConsumers)
	}
	for _, vector := range vectors.ProofDigestVectors {
		sum := sha256.Sum256([]byte(vector.ExactUTF8))
		if got := hex.EncodeToString(sum[:]); got != vector.SHA256 {
			t.Errorf("proof digest = %s, want %s", got, vector.SHA256)
		}
	}
}

func validateVectorSchemasAgainstManifest(t *testing.T, schemas map[string]wireSchema, manifest manifest) {
	t.Helper()
	messages := map[string]messageSpec{}
	for _, file := range manifest.Files {
		for _, message := range file.Messages {
			messages[file.Package+"."+message.Name] = message
		}
	}
	for fqn, schema := range schemas {
		contract, ok := messages[fqn]
		if !ok {
			t.Errorf("vector schema %s has no matching contract manifest message", fqn)
			continue
		}
		if schema.UnknownPolicy != contract.UnknownFields {
			t.Errorf("%s vector unknown policy = %q, manifest = %q", fqn, schema.UnknownPolicy, contract.UnknownFields)
		}
		if len(schema.Fields) != len(contract.Fields) {
			t.Errorf("%s vector schema fields = %d, manifest = %d", fqn, len(schema.Fields), len(contract.Fields))
		}
		byNumber := map[int]fieldSpec{}
		for _, field := range contract.Fields {
			byNumber[field.Number] = field
		}
		for _, field := range schema.Fields {
			expected, ok := byNumber[field.Number]
			if !ok {
				t.Errorf("%s vector field %s=%d absent from manifest", fqn, field.Name, field.Number)
				continue
			}
			wantWire, err := protobufWireType(expected.Type)
			if err != nil {
				t.Errorf("%s.%s: %v", fqn, field.Name, err)
				continue
			}
			wantRepeated := expected.Label == "LABEL_REPEATED"
			if field.Name != expected.Name || field.WireType != wantWire || field.Repeated != wantRepeated || field.TypeName != expected.TypeName {
				t.Errorf("%s field %d vector=%#v manifest=%#v", fqn, field.Number, field, expected)
			}
		}
	}
}

func protobufWireType(fieldType string) (int, error) {
	switch fieldType {
	case "TYPE_INT32", "TYPE_INT64", "TYPE_UINT32", "TYPE_UINT64", "TYPE_SINT32", "TYPE_SINT64", "TYPE_BOOL", "TYPE_ENUM":
		return 0, nil
	case "TYPE_FIXED64", "TYPE_SFIXED64", "TYPE_DOUBLE":
		return 1, nil
	case "TYPE_STRING", "TYPE_BYTES", "TYPE_MESSAGE":
		return 2, nil
	case "TYPE_FIXED32", "TYPE_SFIXED32", "TYPE_FLOAT":
		return 5, nil
	default:
		return 0, fmt.Errorf("unsupported protobuf type %s", fieldType)
	}
}

type wireOccurrence struct {
	Number   int
	WireType int
	Raw      []byte
	Payload  []byte
}

func parseWire(data []byte) ([]wireOccurrence, error) {
	var fields []wireOccurrence
	for offset := 0; offset < len(data); {
		start := offset
		key, n, err := readVarint(data[offset:])
		if err != nil {
			return nil, err
		}
		offset += n
		number, wireType := int(key>>3), int(key&7)
		if number <= 0 {
			return nil, fmt.Errorf("invalid field number %d", number)
		}
		var payload []byte
		switch wireType {
		case 0:
			_, n, err = readVarint(data[offset:])
			if err != nil {
				return nil, err
			}
			payload = append([]byte(nil), data[offset:offset+n]...)
			offset += n
		case 2:
			length, lengthBytes, err := readVarint(data[offset:])
			if err != nil {
				return nil, err
			}
			offset += lengthBytes
			if length > uint64(len(data)-offset) {
				return nil, fmt.Errorf("field %d length %d exceeds remaining bytes", number, length)
			}
			payload = append([]byte(nil), data[offset:offset+int(length)]...)
			offset += int(length)
		default:
			return nil, fmt.Errorf("fixture field %d uses unsupported wire type %d", number, wireType)
		}
		fields = append(fields, wireOccurrence{Number: number, WireType: wireType, Raw: append([]byte(nil), data[start:offset]...), Payload: payload})
	}
	return fields, nil
}

func readVarint(data []byte) (uint64, int, error) {
	var value uint64
	for i, b := range data {
		if i >= 10 || i == 9 && b > 1 {
			return 0, 0, fmt.Errorf("varint overflow")
		}
		value |= uint64(b&0x7f) << (7 * i)
		if b < 0x80 {
			return value, i + 1, nil
		}
	}
	return 0, 0, fmt.Errorf("truncated varint")
}

func canonicalizeWire(input []byte, schema wireSchema) ([]byte, error) {
	fields, err := parseWire(input)
	if err != nil {
		return nil, err
	}
	knownSchema := map[int]wireFieldSchema{}
	for _, field := range schema.Fields {
		if field.Number <= 0 || field.Name == "" || knownSchema[field.Number].Number != 0 {
			return nil, fmt.Errorf("invalid schema field %#v", field)
		}
		knownSchema[field.Number] = field
	}
	var known, unknown []wireOccurrence
	seen := map[int]bool{}
	for _, field := range fields {
		expected, ok := knownSchema[field.Number]
		if !ok {
			if schema.UnknownPolicy == "reject" {
				return nil, fmt.Errorf("unknown field %d rejected", field.Number)
			}
			unknown = append(unknown, field)
			continue
		}
		packedNumeric := expected.Repeated && expected.WireType == 0 && field.WireType == 2
		if expected.WireType != field.WireType && !packedNumeric {
			return nil, fmt.Errorf("field %s wire type %d, want %d", expected.Name, field.WireType, expected.WireType)
		}
		if seen[field.Number] && !expected.Repeated {
			return nil, fmt.Errorf("duplicate non-repeated field %s", expected.Name)
		}
		seen[field.Number] = true
		known = append(known, field)
	}
	sort.SliceStable(known, func(i, j int) bool { return known[i].Number < known[j].Number })
	var result []byte
	for _, field := range append(known, unknown...) {
		result = append(result, field.Raw...)
	}
	return result, nil
}

func assertHashDiffRelation(t *testing.T, vectors map[string]deterministicCase, name string, ids []string, requireSameWire bool) {
	t.Helper()
	if len(ids) != 2 {
		t.Errorf("%s relation must name exactly two vectors", name)
		return
	}
	left, leftOK := vectors[ids[0]]
	right, rightOK := vectors[ids[1]]
	if !leftOK || !rightOK {
		t.Errorf("%s relation references missing vector %v", name, ids)
		return
	}
	if left.DomainSHA256 == right.DomainSHA256 {
		t.Errorf("%s relation hashes unexpectedly match", name)
	}
	if requireSameWire && (left.CanonicalWireHex != right.CanonicalWireHex || left.Message == right.Message) {
		t.Errorf("%s relation must use identical wire and distinct FQNs", name)
	}
}

func TestGeneratedTargetsManifest(t *testing.T) {
	manifest := loadManifest(t)
	generated := loadGeneratedTargets(t)
	if generated.Version != 1 || generated.Gate != "make r23-p2-generated-parity" || len(generated.Targets) != 143 {
		t.Fatalf("generated target authority changed: version=%d gate=%q targets=%d", generated.Version, generated.Gate, len(generated.Targets))
	}
	sources := map[string]bool{}
	for _, file := range manifest.Files {
		sources[file.Path] = true
	}
	seen := map[string]bool{}
	for _, target := range generated.Targets {
		key := target.Language + ":" + target.Path
		if seen[key] {
			t.Errorf("duplicate generated target %s", key)
		}
		seen[key] = true
		if !sources[target.SourceProto] {
			t.Errorf("%s has unknown source proto %s", key, target.SourceProto)
		}
		if strings.Contains(target.Path, "\\") || filepath.IsAbs(target.Path) {
			t.Errorf("%s path must be repository-relative slash form", key)
		}
		if target.Language == "java" && target.Committed {
			t.Errorf("Java target must remain uncommitted: %s", target.Path)
		}
	}
}

func TestR23GeneratedTargets(t *testing.T) {
	generated := loadGeneratedTargets(t)
	root := repoRoot(t)
	checkJava := os.Getenv("R23_CHECK_UNCOMMITTED_JAVA") == "1"
	for _, target := range generated.Targets {
		if !target.Committed && !(checkJava && target.Language == "java") {
			continue
		}
		target := target
		t.Run(strings.ReplaceAll(target.Language+"_"+target.Path, "/", "_"), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(target.Path)))
			if err != nil {
				t.Fatalf("missing generated target for %s: %v", target.SourceProto, err)
			}
			for _, symbol := range target.RequiredSymbols {
				if !bytes.Contains(data, []byte(symbol)) {
					t.Errorf("generated target lacks R23 symbol %s", symbol)
				}
			}
		})
	}
}

// TestR23GeneratedRuntimeConsumers is the P2 executable parity proof. It writes
// consumers to a temporary directory so RED owns no generated or product source.
func TestR23GeneratedRuntimeConsumers(t *testing.T) {
	var vectors deterministicVectors
	loadStrictJSON(t, filepath.Join(repoRoot(t), "protos", "voice", "r23_contract_test", "testdata", "deterministic_contract_vectors.json"), &vectors)
	byID := map[string]deterministicCase{}
	for _, vector := range vectors.Vectors {
		byID[vector.ID] = vector
	}
	t.Run("Go", func(t *testing.T) { runGoRuntimeConsumer(t, byID) })
	t.Run("Java", func(t *testing.T) {
		if os.Getenv("R23_CHECK_UNCOMMITTED_JAVA") != "1" {
			t.Skip("Java generated output is uncommitted; P2 enables this consumer after Maven generation")
		}
		runJavaRuntimeConsumer(t, byID)
	})
}

func runGoRuntimeConsumer(t *testing.T, byID map[string]deterministicCase) {
	t.Helper()
	ids := []string{"manifest_base", "fence_request", "chat_wrapper", "file_wrapper_same_wire", "page_repeated_ab", "page_repeated_ba"}
	args := make([]any, 0, len(ids)*3)
	for _, id := range ids {
		v := byID[id]
		args = append(args, v.CanonicalWireHex, v.Message, v.DomainSHA256)
	}
	source := fmt.Sprintf(`package main
import (
 "bytes"; "crypto/sha256"; "encoding/hex"; "fmt"
 chatv1 "voice.app/voice/chat/v1"
 commonv1 "voice.app/voice/common/v1"
 filev1 "voice.app/voice/file/v1"
 "google.golang.org/protobuf/proto"
 "google.golang.org/protobuf/reflect/protoreflect"
)
type tc struct { wire, fqn, hash string; msg proto.Message }
func must(ok bool, f string, a ...any) { if !ok { panic(fmt.Sprintf(f,a...)) } }
func val(m protoreflect.Message, name string) protoreflect.Value { f:=m.Descriptor().Fields().ByName(protoreflect.Name(name)); must(f!=nil,"missing named field %%s.%%s",m.Descriptor().FullName(),name); return m.Get(f) }
func check(c tc) { b,e:=hex.DecodeString(c.wire); must(e==nil,"hex: %%v",e); must(string(c.msg.ProtoReflect().Descriptor().FullName())==c.fqn,"FQN mismatch"); must(proto.Unmarshal(b,c.msg)==nil,"decode %%s",c.fqn); out,e:=proto.MarshalOptions{Deterministic:true}.Marshal(c.msg); must(e==nil && bytes.Equal(out,b),"deterministic re-encode %%s: %%x != %%x",c.fqn,out,b); h:=sha256.Sum256(append(append([]byte(c.fqn),0),out...)); must(hex.EncodeToString(h[:])==c.hash,"domain hash %%s",c.fqn) }
func main() {
 cases:=[]tc{
  {%q,%q,%q,&commonv1.ManifestBinding{}}, {%q,%q,%q,&commonv1.SpaceLifecycleFenceRequest{}},
  {%q,%q,%q,&chatv1.ApplySpaceLifecycleFenceRequest{}}, {%q,%q,%q,&filev1.ApplySpaceLifecycleFenceRequest{}},
  {%q,%q,%q,&chatv1.SpacePurgeManifestPage{}}, {%q,%q,%q,&chatv1.SpacePurgeManifestPage{}},
 }
 for _,c:=range cases { check(c) }
 manifest:=cases[0].msg.ProtoReflect(); must(val(manifest,"manifest_id").String()=="m" && val(manifest,"item_count").Uint()==3,"ManifestBinding values")
 fence:=cases[1].msg.ProtoReflect(); must(val(fence,"space_id").String()=="s" && val(fence,"desired_state").Enum()==1,"fence named values"); must(val(val(fence,"manifest").Message(),"manifest_id").String()=="m","fence nested manifest")
 for _,i:=range []int{2,3} { wrapper:=cases[i].msg.ProtoReflect(); must(val(val(wrapper,"fence").Message(),"deletion_operation_id").String()=="o","wrapper nested fence") }
 a,b:=cases[4].msg.ProtoReflect(),cases[5].msg.ProtoReflect(); al,bl:=val(a,"item_ids").List(),val(b,"item_ids").List(); must(al.Len()==2 && al.Get(0).String()=="a" && al.Get(1).String()=="b","repeated AB"); must(bl.Len()==2 && bl.Get(0).String()=="b" && bl.Get(1).String()=="a","repeated BA"); must(cases[2].wire==cases[3].wire && cases[2].hash!=cases[3].hash,"FQN-domain relation"); must(cases[4].wire!=cases[5].wire && cases[4].hash!=cases[5].hash,"repeated-order relation")
}
`, args...)
	dir := t.TempDir()
	path := filepath.Join(dir, "r23_runtime_fixture.go")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", path)
	cmd.Dir = filepath.Join(repoRoot(t), "src", "backend", "file")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated Go runtime fixture failed: %v\n%s", err, out)
	}
}

func runJavaRuntimeConsumer(t *testing.T, byID map[string]deterministicCase) {
	t.Helper()
	a, b, receipt := byID["auth_binding_factors_12"], byID["auth_binding_factors_21"], byID["auth_receipt_wrapper"]
	source := fmt.Sprintf(`import app.voice.auth.v1.SpaceDeletionProofBinding;
import app.voice.auth.v1.ConsumeSpaceDeletionProofResponse;
import com.google.protobuf.*;
import java.security.MessageDigest;
import java.util.*;
public final class R23JavaFixture {
 static void must(boolean v,String m){if(!v)throw new AssertionError(m);}
 static byte[] hex(String s){return HexFormat.of().parseHex(s);}
 static byte[] deterministic(Message m)throws Exception{byte[] b=new byte[m.getSerializedSize()];CodedOutputStream o=CodedOutputStream.newInstance(b);o.useDeterministicSerialization();m.writeTo(o);o.checkNoSpaceLeft();return b;}
 static void check(Message m,String fqn,String wire,String hash)throws Exception{byte[] out=deterministic(m);must(Arrays.equals(out,hex(wire)),"deterministic re-encode "+fqn);must(m.getDescriptorForType().getFullName().equals(fqn),"FQN "+fqn);MessageDigest d=MessageDigest.getInstance("SHA-256");d.update(fqn.getBytes(java.nio.charset.StandardCharsets.UTF_8));d.update((byte)0);d.update(out);must(HexFormat.of().formatHex(d.digest()).equals(hash),"domain hash "+fqn);}
 static Object field(Message m,String n){Descriptors.FieldDescriptor f=m.getDescriptorForType().findFieldByName(n);must(f!=null,"missing named field "+n);return m.getField(f);}
 public static void main(String[] z)throws Exception{
  SpaceDeletionProofBinding a=SpaceDeletionProofBinding.parseFrom(hex(%q)); SpaceDeletionProofBinding b=SpaceDeletionProofBinding.parseFrom(hex(%q));
  check(a,%q,%q,%q); check(b,%q,%q,%q);
  must(field(a,"account_id").equals("a") && field(a,"profile_id").equals("p") && field(a,"space_id").equals("s") && field(a,"operation_id").equals("o"),"binding named values");
	  List<?> af=(List<?>)field(a,"verified_factors"), bf=(List<?>)field(b,"verified_factors"); must(af.get(0).toString().equals("VERIFIED_FACTOR_PASSWORD") && af.get(1).toString().equals("VERIFIED_FACTOR_TOTP"),"factor order 1,2"); must(bf.get(0).toString().equals("VERIFIED_FACTOR_TOTP") && bf.get(1).toString().equals("VERIFIED_FACTOR_PASSWORD"),"factor order 2,1"); must(!Arrays.equals(deterministic(a),deterministic(b)) && !%q.equals(%q),"repeated order/hash relation");
	  ConsumeSpaceDeletionProofResponse r=ConsumeSpaceDeletionProofResponse.parseFrom(hex(%q)); check(r,%q,%q,%q); Message nested=(Message)field(r,"receipt"); must(field(nested,"receipt_id").equals("r") && field(nested,"operation_id").equals("o"),"nested receipt values"); must(((List<?>)field(nested,"verified_factors")).size()==2,"nested repeated factors"); must(((Message)field(nested,"consumed_at")).getSerializedSize()>0,"nested timestamp");
 }
}`, a.CanonicalWireHex, b.CanonicalWireHex, a.Message, a.CanonicalWireHex, a.DomainSHA256, b.Message, b.CanonicalWireHex, b.DomainSHA256, a.DomainSHA256, b.DomainSHA256, receipt.CanonicalWireHex, receipt.Message, receipt.CanonicalWireHex, receipt.DomainSHA256)
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "R23JavaFixture.java")
	classes := filepath.Join(dir, "classes")
	if err := os.Mkdir(classes, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	authDir := filepath.Join(repoRoot(t), "src", "backend", "auth")
	cpFile := filepath.Join(dir, "classpath.txt")
	cmd := exec.Command("mvn", "-q", "-DincludeScope=compile", "dependency:build-classpath", "-Dmdep.outputFile="+cpFile)
	cmd.Dir = authDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build Java fixture classpath: %v\n%s", err, out)
	}
	cp, err := os.ReadFile(cpFile)
	if err != nil {
		t.Fatal(err)
	}
	classpath := filepath.Join(authDir, "target", "classes") + string(os.PathListSeparator) + strings.TrimSpace(string(cp))
	cmd = exec.Command("javac", "-cp", classpath, "-d", classes, sourcePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile generated Java runtime fixture: %v\n%s", err, out)
	}
	cmd = exec.Command("java", "-cp", classes+string(os.PathListSeparator)+classpath, "R23JavaFixture")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated Java runtime fixture failed: %v\n%s", err, out)
	}
}

func TestR23ContractDescriptors(t *testing.T) {
	m := loadManifest(t)
	root := repoRoot(t)
	cmd := exec.Command("buf", "build", "protos", "--as-file-descriptor-set", "-o", "-#format=json")
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("buf build: %v\n%s", err, stderr.String())
	}
	var set descriptorSet
	if err := json.Unmarshal(out, &set); err != nil {
		t.Fatalf("decode FileDescriptorSet JSON: %v", err)
	}
	actualFiles := map[string]fileDescriptor{}
	for _, file := range set.Files {
		actualFiles[file.Name] = file
	}
	for _, expected := range m.Files {
		expected := expected
		t.Run(strings.ReplaceAll(expected.Path, "/", "_"), func(t *testing.T) {
			actual, ok := actualFiles[expected.Path]
			if !ok {
				t.Fatalf("missing R23 proto file %s", expected.Path)
			}
			checkFile(t, root, expected, actual)
		})
	}
	checkForbiddenRPCExposure(t, m.ForbiddenRPCExposures, set.Files)
}

func checkForbiddenRPCExposure(t *testing.T, specs []forbiddenRPCExposure, files []fileDescriptor) {
	t.Helper()
	allowed := map[string]map[string]bool{}
	for _, spec := range specs {
		allowed[spec.Package] = map[string]bool{}
		for _, method := range spec.AllowedR23Methods {
			allowed[spec.Package][method] = true
		}
	}
	for _, file := range files {
		packageAllowed, selected := allowed[file.Package]
		if !selected {
			continue
		}
		for _, service := range file.Services {
			for _, method := range service.Methods {
				if isR23Method(method.Name) && !packageAllowed[method.Name] {
					t.Errorf("forbidden R23 RPC exposure %s.%s/%s", file.Package, service.Name, method.Name)
				}
			}
		}
	}
}

func isR23Method(name string) bool {
	return strings.Contains(name, "SpaceDeletion") || strings.Contains(name, "SpacePurge") ||
		strings.Contains(name, "SpaceLifecycle") || name == "PurgeSpace" || name == "RetireSpace"
}

func validateFileSpec(t *testing.T, file fileSpec) {
	t.Helper()
	seenTypes := map[string]bool{}
	for _, enum := range file.Enums {
		if seenTypes[enum.Name] {
			t.Errorf("%s duplicate type %s", file.Path, enum.Name)
		}
		seenTypes[enum.Name] = true
		seenNumbers := map[int]bool{}
		for name, number := range enum.Values {
			if name == "" || number < 0 || seenNumbers[number] {
				t.Errorf("%s.%s invalid enum value %q=%d", file.Package, enum.Name, name, number)
			}
			seenNumbers[number] = true
		}
		validateReservations(t, file.Package+"."+enum.Name, enum.ReservedNames, enum.ReservedRanges)
	}
	for _, message := range file.Messages {
		if seenTypes[message.Name] {
			t.Errorf("%s duplicate type %s", file.Path, message.Name)
		}
		seenTypes[message.Name] = true
		seenNames, seenNumbers := map[string]bool{}, map[int]bool{}
		for _, field := range message.Fields {
			if field.Name == "" || field.Number <= 0 || seenNames[field.Name] || seenNumbers[field.Number] {
				t.Errorf("%s.%s invalid/duplicate field %q=%d", file.Package, message.Name, field.Name, field.Number)
			}
			seenNames[field.Name], seenNumbers[field.Number] = true, true
			if (field.Type == "TYPE_MESSAGE" || field.Type == "TYPE_ENUM") != strings.HasPrefix(field.TypeName, ".") {
				t.Errorf("%s.%s.%s type_name mismatch", file.Package, message.Name, field.Name)
			}
		}
		validateReservations(t, file.Package+"."+message.Name, message.ReservedNames, message.ReservedRanges)
	}
	for _, service := range file.Services {
		for _, method := range service.Methods {
			if method.Name == "" || !strings.HasPrefix(method.Input, ".") || !strings.HasPrefix(method.Output, ".") {
				t.Errorf("%s.%s invalid method %#v", file.Package, service.Name, method)
			}
			if method.Exposure != "public_gateway" && method.Exposure != "protected" {
				t.Errorf("%s.%s invalid exposure %q", service.Name, method.Name, method.Exposure)
			}
			if len(method.Callers) == 0 {
				t.Errorf("%s.%s has empty caller allow-list", service.Name, method.Name)
			}
			if method.Exposure == "protected" {
				for _, caller := range method.Callers {
					if caller == "service:gateway" {
						t.Errorf("%s.%s protected RPC must be absent from public Gateway", service.Name, method.Name)
					}
				}
			}
		}
	}
}

func validateReservations(t *testing.T, owner string, names []string, ranges []reservedRange) {
	t.Helper()
	seenNames := map[string]bool{}
	for _, name := range names {
		if name == "" || seenNames[name] {
			t.Errorf("%s invalid/duplicate reserved name %q", owner, name)
		}
		seenNames[name] = true
	}
	for _, rr := range ranges {
		if rr.Start <= 0 || rr.End <= rr.Start {
			t.Errorf("%s invalid reserved range [%d,%d)", owner, rr.Start, rr.End)
		}
	}
}

func checkFile(t *testing.T, root string, expected fileSpec, actual fileDescriptor) {
	t.Helper()
	if actual.Package != expected.Package {
		t.Errorf("package = %q, want %q", actual.Package, expected.Package)
	}
	if actual.Syntax != expected.Syntax {
		t.Errorf("syntax = %q, want %q", actual.Syntax, expected.Syntax)
	}
	actualDependencies := append([]string(nil), actual.Dependencies...)
	sort.Strings(actualDependencies)
	if !reflect.DeepEqual(actualDependencies, expected.Dependencies) {
		t.Errorf("dependencies = %v, want %v", actualDependencies, expected.Dependencies)
	}
	actualOptions := fileOptions{GoPackage: actual.Options.GoPackage, JavaPackage: actual.Options.JavaPackage, JavaMultipleFiles: actual.Options.JavaMultipleFiles}
	if !reflect.DeepEqual(actualOptions, expected.Options) {
		t.Errorf("file options = %#v, want %#v", actualOptions, expected.Options)
	}
	actualEnums := map[string]enumDescriptor{}
	for _, enum := range actual.Enums {
		actualEnums[enum.Name] = enum
	}
	for _, want := range expected.Enums {
		got, ok := actualEnums[want.Name]
		if !ok {
			t.Errorf("missing enum %s.%s", expected.Package, want.Name)
			continue
		}
		values := map[string]int{}
		for _, value := range got.Values {
			values[value.Name] = value.Number
		}
		if !reflect.DeepEqual(sortedStrings(got.ReservedNames), sortedStrings(want.ReservedNames)) || !reflect.DeepEqual(sortedRanges(got.ReservedRanges), sortedRanges(want.ReservedRanges)) {
			t.Errorf("%s.%s reservations changed", expected.Package, want.Name)
		}
		if want.Exact && len(values) != len(want.Values) {
			t.Errorf("%s.%s enum value count = %d, want %d", expected.Package, want.Name, len(values), len(want.Values))
		}
		for name, number := range want.Values {
			if values[name] != number {
				t.Errorf("%s.%s.%s = %d, want %d", expected.Package, want.Name, name, values[name], number)
			}
		}
	}

	actualMessages := map[string]msgDescriptor{}
	messageIndexes := map[string]int{}
	for i, message := range actual.Messages {
		actualMessages[message.Name] = message
		messageIndexes[message.Name] = i
	}
	for _, want := range expected.Messages {
		got, ok := actualMessages[want.Name]
		if !ok {
			t.Errorf("missing message %s.%s", expected.Package, want.Name)
			continue
		}
		checkMessage(t, expected.Package, want, got)
		comments := commentsAt(actual, []int{4, messageIndexes[want.Name]})
		if want.UnknownFields != "" {
			requireComment(t, expected.Package+"."+want.Name, comments, "@voice.unknown_fields="+want.UnknownFields)
		}
		if want.Hashing != "" {
			requireComment(t, expected.Package+"."+want.Name, comments, "@voice.hash="+want.Hashing)
		}
	}

	actualServices := map[string]svcDescriptor{}
	serviceIndexes := map[string]int{}
	for i, service := range actual.Services {
		actualServices[service.Name] = service
		serviceIndexes[service.Name] = i
	}
	for _, wantService := range expected.Services {
		got, ok := actualServices[wantService.Name]
		if !ok {
			t.Errorf("missing service %s.%s", expected.Package, wantService.Name)
			continue
		}
		methods := map[string]methodDescriptor{}
		methodIndexes := map[string]int{}
		for i, method := range got.Methods {
			methods[method.Name] = method
			methodIndexes[method.Name] = i
		}
		for _, want := range wantService.Methods {
			method, ok := methods[want.Name]
			if !ok {
				t.Errorf("missing RPC %s.%s/%s", expected.Package, wantService.Name, want.Name)
				continue
			}
			if method.Input != want.Input || method.Output != want.Output {
				t.Errorf("%s.%s/%s types = %s -> %s, want %s -> %s", expected.Package, wantService.Name, want.Name, method.Input, method.Output, want.Input, want.Output)
			}
			if want.NoStreaming && (method.ClientStreaming || method.ServerStreaming) {
				t.Errorf("%s.%s/%s must be unary", expected.Package, wantService.Name, want.Name)
			}
			comments := commentsAt(actual, []int{6, serviceIndexes[wantService.Name], 2, methodIndexes[want.Name]})
			annotation := fmt.Sprintf("@voice.security=%s;callers=%s", want.Exposure, strings.Join(want.Callers, ","))
			requireComment(t, expected.Package+"."+wantService.Name+"/"+want.Name, comments, annotation)
			for _, annotation := range want.Annotations {
				requireComment(t, expected.Package+"."+wantService.Name+"/"+want.Name, comments, annotation)
			}
		}
	}

	if got := fingerprint(projectActual(expected, actual)); got != expected.ContractProjectionSHA256 {
		t.Errorf("canonical contract projection sha256 = %s, want %s", got, expected.ContractProjectionSHA256)
	}
	_ = root
}

func checkMessage(t *testing.T, pkg string, want messageSpec, got msgDescriptor) {
	t.Helper()
	if !reflect.DeepEqual(sortedStrings(got.ReservedNames), sortedStrings(want.ReservedNames)) || !reflect.DeepEqual(sortedRanges(got.ReservedRanges), sortedRanges(want.ReservedRanges)) {
		t.Errorf("%s.%s reservations changed", pkg, want.Name)
	}
	fields := map[string]fieldSpec{}
	for _, field := range got.Fields {
		oneof := ""
		if field.OneofIndex.Set && field.OneofIndex.Value < len(got.Oneofs) {
			oneof = got.Oneofs[field.OneofIndex.Value].Name
		}
		fields[field.Name] = fieldSpec{Name: field.Name, Number: field.Number, Label: field.Label, Type: field.Type, TypeName: field.TypeName, Oneof: oneof, Proto3Optional: field.Proto3Optional, Deprecated: field.Options.Deprecated}
	}
	if want.Exact && len(fields) != len(want.Fields) {
		t.Errorf("%s.%s field count = %d, want %d", pkg, want.Name, len(fields), len(want.Fields))
	}
	for _, expected := range want.Fields {
		actual, ok := fields[expected.Name]
		if !ok {
			t.Errorf("missing field %s.%s.%s", pkg, want.Name, expected.Name)
			continue
		}
		if !reflect.DeepEqual(actual, expected) {
			t.Errorf("%s.%s.%s = %#v, want %#v", pkg, want.Name, expected.Name, actual, expected)
		}
	}
	for _, forbidden := range want.ForbiddenFields {
		for name := range fields {
			if strings.Contains(strings.ToLower(name), strings.ToLower(forbidden)) {
				t.Errorf("%s.%s exposes forbidden field %q matching %q", pkg, want.Name, name, forbidden)
			}
		}
	}
}

func sortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

func sortedRanges(values []reservedRange) []reservedRange {
	result := append([]reservedRange(nil), values...)
	sort.Slice(result, func(i, j int) bool {
		if result[i].Start == result[j].Start {
			return result[i].End < result[j].End
		}
		return result[i].Start < result[j].Start
	})
	return result
}

func commentsAt(file fileDescriptor, path []int) string {
	for _, location := range file.SourceCodeInfo.Locations {
		if reflect.DeepEqual(location.Path, path) {
			return strings.Join(append(append([]string{}, location.LeadingDetachedComments...), location.LeadingComments, location.TrailingComments), "\n")
		}
	}
	return ""
}

func requireComment(t *testing.T, owner, comments, want string) {
	t.Helper()
	if !strings.Contains(comments, want) {
		t.Errorf("%s descriptor comments must contain %q", owner, want)
	}
}

func projectActual(expected fileSpec, actual fileDescriptor) fileSpec {
	projected := expected
	projected.ContractProjectionSHA256 = ""
	projected.Syntax = actual.Syntax
	projected.Dependencies = append([]string(nil), actual.Dependencies...)
	sort.Strings(projected.Dependencies)
	projected.Options = fileOptions{GoPackage: actual.Options.GoPackage, JavaPackage: actual.Options.JavaPackage, JavaMultipleFiles: actual.Options.JavaMultipleFiles}
	actualEnums := map[string]enumDescriptor{}
	for _, enum := range actual.Enums {
		actualEnums[enum.Name] = enum
	}
	for i, enum := range projected.Enums {
		if got, ok := actualEnums[enum.Name]; ok {
			projected.Enums[i].ReservedNames = sortedStrings(got.ReservedNames)
			projected.Enums[i].ReservedRanges = sortedRanges(got.ReservedRanges)
			projected.Enums[i].Values = map[string]int{}
			for _, value := range got.Values {
				if _, selected := enum.Values[value.Name]; selected || enum.Exact {
					projected.Enums[i].Values[value.Name] = value.Number
				}
			}
		}
	}
	actualMessages := map[string]msgDescriptor{}
	for _, message := range actual.Messages {
		actualMessages[message.Name] = message
	}
	for i, message := range projected.Messages {
		if got, ok := actualMessages[message.Name]; ok {
			projected.Messages[i].ReservedNames = sortedStrings(got.ReservedNames)
			projected.Messages[i].ReservedRanges = sortedRanges(got.ReservedRanges)
			selected := map[string]bool{}
			for _, field := range message.Fields {
				selected[field.Name] = true
			}
			// Keep exact zero-field messages as [] rather than null so their
			// canonical projection matches the manifest's JSON representation.
			projected.Messages[i].Fields = make([]fieldSpec, 0)
			for _, field := range got.Fields {
				if !message.Exact && !selected[field.Name] {
					continue
				}
				oneof := ""
				if field.OneofIndex.Set && field.OneofIndex.Value < len(got.Oneofs) {
					oneof = got.Oneofs[field.OneofIndex.Value].Name
				}
				projected.Messages[i].Fields = append(projected.Messages[i].Fields, fieldSpec{Name: field.Name, Number: field.Number, Label: field.Label, Type: field.Type, TypeName: field.TypeName, Oneof: oneof, Proto3Optional: field.Proto3Optional, Deprecated: field.Options.Deprecated})
			}
		}
	}
	actualServices := map[string]svcDescriptor{}
	for _, service := range actual.Services {
		actualServices[service.Name] = service
	}
	for i, service := range projected.Services {
		if got, ok := actualServices[service.Name]; ok {
			actualMethods := map[string]methodDescriptor{}
			for _, method := range got.Methods {
				actualMethods[method.Name] = method
			}
			for j, method := range service.Methods {
				if actual, ok := actualMethods[method.Name]; ok {
					projected.Services[i].Methods[j].Input = actual.Input
					projected.Services[i].Methods[j].Output = actual.Output
					projected.Services[i].Methods[j].NoStreaming = !actual.ClientStreaming && !actual.ServerStreaming
				}
			}
		}
	}
	return projected
}

func fingerprint(file fileSpec) string {
	file.ContractProjectionSHA256 = ""
	sort.Strings(file.Dependencies)
	sort.Slice(file.Enums, func(i, j int) bool { return file.Enums[i].Name < file.Enums[j].Name })
	for i := range file.Enums {
		file.Enums[i].ReservedNames = sortedStrings(file.Enums[i].ReservedNames)
		file.Enums[i].ReservedRanges = sortedRanges(file.Enums[i].ReservedRanges)
	}
	sort.Slice(file.Messages, func(i, j int) bool { return file.Messages[i].Name < file.Messages[j].Name })
	for i := range file.Messages {
		sort.Slice(file.Messages[i].Fields, func(a, b int) bool { return file.Messages[i].Fields[a].Number < file.Messages[i].Fields[b].Number })
		sort.Strings(file.Messages[i].ForbiddenFields)
		file.Messages[i].ReservedNames = sortedStrings(file.Messages[i].ReservedNames)
		file.Messages[i].ReservedRanges = sortedRanges(file.Messages[i].ReservedRanges)
	}
	sort.Slice(file.Services, func(i, j int) bool { return file.Services[i].Name < file.Services[j].Name })
	for i := range file.Services {
		sort.Slice(file.Services[i].Methods, func(a, b int) bool { return file.Services[i].Methods[a].Name < file.Services[i].Methods[b].Name })
	}
	data, err := json.Marshal(file)
	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func loadManifest(t *testing.T) manifest {
	t.Helper()
	var m manifest
	loadStrictJSON(t, filepath.Join(repoRoot(t), "protos", "voice", "r23_contract_manifest.json"), &m)
	return m
}

func loadGeneratedTargets(t *testing.T) generatedTargetsManifest {
	t.Helper()
	var targets generatedTargetsManifest
	loadStrictJSON(t, filepath.Join(repoRoot(t), "protos", "voice", "r23_contract_test", "generated_targets.json"), &targets)
	return targets
}

func loadStrictJSON(t *testing.T, path string, destination any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func assertFileSHA256(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("%s sha256 = %s, want %s", path, got, want)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "protos", "buf.yaml")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root with protos/buf.yaml not found")
		}
		dir = parent
	}
}
