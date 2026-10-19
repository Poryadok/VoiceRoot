package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type LegacyProjectionStepRef struct {
	ID   uuid.UUID
	Hash [32]byte
}
type LegacyKeyImage struct {
	Kind, Value        string
	Members            []string
	ExpiresAtUnixMilli int64
}
type LegacyKeyTransition struct {
	Key                 string
	Before, After       LegacyKeyImage
	AllowDeadlineExpiry bool
}

// The complete binding bytes resolve BOTH owner works, C/P and resource proof.
// Only the protected planner/journal resolver produces this physical intent.
// Callers of the executor supply references, never keys or before/after JSON.
type LegacyProjectionStep struct {
	ID, CaptureID               uuid.UUID
	Ordinal                     uint64
	ScopePrefix, EvidencePrefix string
	ExpectedRunID               string
	BindingBytes                []byte
	PreviousReceipt             []byte
	Transitions                 []LegacyKeyTransition
}

type LegacyProjectionStepResolver interface {
	// Resolve revalidates accepted owner footprints, proof and current physical
	// storage lineage/custody; a role/name/run_id/config hash alone is insufficient.
	ResolveProjectionStep(context.Context, LegacyProjectionStepRef) (LegacyProjectionStep, error)
}

// LegacyProjectionExecutor is separate from serving CallStore. Its client has
// scoped maintenance/evidence-namespace credentials; ordinary principals must
// lack those rights. It is instantiated only by the non-serving composition.
type LegacyProjectionExecutor struct {
	client   *redis.Client
	resolver LegacyProjectionStepResolver
}

func NewLegacyProjectionExecutor(client *redis.Client, resolver LegacyProjectionStepResolver) *LegacyProjectionExecutor {
	return &LegacyProjectionExecutor{client, resolver}
}

func LegacyProjectionStepHash(step LegacyProjectionStep) ([32]byte, error) {
	b, e := json.Marshal(step)
	if e != nil {
		return [32]byte{}, e
	}
	return sha256.Sum256(append([]byte("voice.legacy.projection-step.v1\x00"), b...)), nil
}

func (e *LegacyProjectionExecutor) Apply(ctx context.Context, ref LegacyProjectionStepRef) ([]byte, error) {
	if e == nil || e.client == nil || e.resolver == nil {
		return nil, ErrLegacyPartitionRequired
	}
	step, err := e.resolver.ResolveProjectionStep(ctx, ref)
	if err != nil {
		return nil, err
	}
	hash, err := LegacyProjectionStepHash(step)
	if err != nil || step.ID != ref.ID || hash != ref.Hash {
		return nil, ErrOperationConflict
	}
	if err = validateLegacyStep(step); err != nil {
		return nil, err
	}
	before, _ := json.Marshal(step.Transitions)
	beforeHash := sha256.Sum256(before)
	bindingHash := sha256.Sum256(step.BindingBytes)
	header := "voice.legacy.projection-receipt.v1\n" + ref.ID.String() + "\n" + hex.EncodeToString(ref.Hash[:]) + "\n" + hex.EncodeToString(bindingHash[:]) + "\n" + hex.EncodeToString(beforeHash[:]) + "\n" + step.ExpectedRunID + "\n"
	control, err := json.Marshal(struct {
		Hash, Header, Previous, RunID string
		Ordinal                       uint64
		Transitions                   []LegacyKeyTransition
	}{hex.EncodeToString(ref.Hash[:]), header, string(step.PreviousReceipt), step.ExpectedRunID, step.Ordinal, step.Transitions})
	if err != nil {
		return nil, err
	}
	if len(control) > 1024*1024 {
		return nil, ErrInvalidState
	}
	base := step.EvidencePrefix + step.CaptureID.String() + ":"
	keys := []string{base + "cursor", base + "pending", base + "step:" + ref.ID.String()}
	for _, t := range step.Transitions {
		keys = append(keys, t.Key)
	}
	raw, err := e.client.Eval(ctx, legacyProjectionScript, keys, string(control)).Text()
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(raw, header) {
		return nil, ErrOperationConflict
	}
	remainder := strings.TrimPrefix(raw, header)
	fields := strings.Split(remainder, "\n")
	if len(fields) != 2 {
		return nil, ErrOperationConflict
	}
	when, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil || when <= 0 {
		return nil, ErrOperationConflict
	}
	dispositions := strings.Split(fields[1], ",")
	if len(dispositions) != len(step.Transitions) {
		return nil, ErrOperationConflict
	}
	for _, v := range dispositions {
		if v != "APPLIED" && v != "EXPIRED" {
			return nil, ErrOperationConflict
		}
	}
	return []byte(raw), nil
}

func validateLegacyStep(s LegacyProjectionStep) error {
	if s.ID == uuid.Nil || s.CaptureID == uuid.Nil || s.Ordinal == 0 || s.Ordinal > 1<<53 || len(s.Transitions) == 0 || len(s.Transitions) > 64 || len(s.ExpectedRunID) != 40 || !json.Valid(s.BindingBytes) || len(s.PreviousReceipt) > 32768 {
		return ErrInvalidState
	}
	if s.Ordinal == 1 && len(s.PreviousReceipt) != 0 || s.Ordinal > 1 && len(s.PreviousReceipt) == 0 {
		return ErrOperationConflict
	}
	for _, p := range []string{s.ScopePrefix, s.EvidencePrefix} {
		if p == "" || !utf8.ValidString(p) || strings.ContainsAny(p, "*?[]\\") {
			return ErrInvalidState
		}
	}
	if strings.HasPrefix(s.ScopePrefix, s.EvidencePrefix) || strings.HasPrefix(s.EvidencePrefix, s.ScopePrefix) {
		return ErrInvalidState
	}
	seen := map[string]bool{}
	for _, t := range s.Transitions {
		if !strings.HasPrefix(t.Key, s.ScopePrefix) || !utf8.ValidString(t.Key) || seen[t.Key] || retainedLegacyKey(strings.TrimPrefix(t.Key, s.ScopePrefix)) {
			return ErrInvalidState
		}
		seen[t.Key] = true
		for _, image := range []LegacyKeyImage{t.Before, t.After} {
			if image.Kind != "none" && image.Kind != "string" && image.Kind != "set" || image.ExpiresAtUnixMilli < 0 || !utf8.ValidString(image.Value) {
				return ErrInvalidState
			}
			if image.Kind == "none" && (image.Value != "" || len(image.Members) != 0 || image.ExpiresAtUnixMilli != 0) || image.Kind == "string" && len(image.Members) != 0 || image.Kind == "set" && (image.Value != "" || len(image.Members) == 0 || !sort.StringsAreSorted(image.Members)) {
				return ErrInvalidState
			}
			for i, v := range image.Members {
				if !utf8.ValidString(v) || i > 0 && v == image.Members[i-1] {
					return ErrInvalidState
				}
			}
		}
		if t.Before.Kind == "none" && t.After.Kind != "none" || t.After.Kind != "none" && t.After.Kind != t.Before.Kind {
			return ErrInvalidState
		}
		if t.Before.Kind == "set" && t.After.Kind == "set" {
			if t.Before.ExpiresAtUnixMilli != t.After.ExpiresAtUnixMilli {
				return ErrInvalidState
			}
			for _, m := range t.After.Members {
				at := sort.SearchStrings(t.Before.Members, m)
				if at == len(t.Before.Members) || t.Before.Members[at] != m {
					return ErrInvalidState
				}
			}
		}
		if t.AllowDeadlineExpiry && (t.Before.ExpiresAtUnixMilli <= 0 || t.After.Kind != "none") {
			return ErrInvalidState
		}
	}
	return nil
}

// The script validates all commands/types and every preimage before mutation.
// An immutable pending identity permits only declared per-key PRE/POST recovery.
// No success receipt is written after a failed command. Set transitions remove
// exact members in one SREM and never DEL/rebuild a shared set or add members.
const legacyProjectionScript = `
local ok,s=pcall(cjson.decode,ARGV[1]);if not ok then return redis.error_reply('INVALID_INTENT') end
local info=redis.call('INFO','server');local run=string.match(info,'run_id:([a-f0-9]+)')
if run~=s.RunID then return redis.error_reply('RESOURCE_INCARNATION_CONFLICT') end
local previous=redis.call('GET',KEYS[1]) or '';local pending=redis.call('GET',KEYS[2]) or '';local saved=redis.call('GET',KEYS[3])
local function matches(key,image)
 local kind=redis.call('TYPE',key)['ok'];if kind~=image.Kind then return false end
 if kind=='none' then return true end
 if redis.call('PEXPIRETIME',key)~=(image.ExpiresAtUnixMilli==0 and -1 or image.ExpiresAtUnixMilli) then return false end
 if kind=='string' then return redis.call('GET',key)==image.Value end
 local members=redis.call('SMEMBERS',key);table.sort(members);if #members~=#image.Members then return false end
 for i,v in ipairs(members) do if v~=image.Members[i] then return false end end;return true
end
if saved then
 if string.sub(saved,1,#s.Header)~=s.Header then return redis.error_reply('RECEIPT_CONFLICT') end
 local suffix=string.sub(saved,#s.Header+1);local timestamp,states=string.match(suffix,'^(%d+)\n([A-Z_,]+)$')
 if not timestamp or tonumber(timestamp)<=0 or string.format('%.0f',tonumber(timestamp))~=timestamp then return redis.error_reply('RECEIPT_CONFLICT') end
 local values={};for state in string.gmatch(states,'[A-Z_]+') do table.insert(values,state) end
 if #values~=#s.Transitions or table.concat(values,',')~=states then return redis.error_reply('RECEIPT_CONFLICT') end
 for i,state in ipairs(values) do if state~='APPLIED' and (state~='EXPIRED' or not s.Transitions[i].AllowDeadlineExpiry) then return redis.error_reply('RECEIPT_CONFLICT') end end
 if previous==s.Previous then
  for i,t in ipairs(s.Transitions) do if not matches(KEYS[i+3],t.After) then return redis.error_reply('RECEIPT_POSTIMAGE_CONFLICT') end end
  redis.call('SET',KEYS[1],saved)
 end
 if pending==s.Hash then redis.call('DEL',KEYS[2]) end
 return saved
end
if previous~=s.Previous or pending~='' and pending~=s.Hash then return redis.error_reply('STEP_ORDER_CONFLICT') end
local clock=redis.call('TIME');local now=tonumber(clock[1])*1000+math.floor(tonumber(clock[2])/1000)
local mode={};local disposition={}
for i,t in ipairs(s.Transitions) do
 local key=KEYS[i+3]
 if key~=t.Key then return redis.error_reply('KEY_SET_CONFLICT') end
 if matches(key,t.Before) then mode[i]='PRE';disposition[i]='APPLIED'
 elseif pending==s.Hash and matches(key,t.After) then mode[i]='POST';disposition[i]='APPLIED'
 elseif t.AllowDeadlineExpiry and t.Before.ExpiresAtUnixMilli<=now and redis.call('EXISTS',key)==0 then mode[i]='EXPIRED';disposition[i]='EXPIRED'
 else return redis.error_reply('PREIMAGE_CONFLICT') end
 if t.After.Kind~='none' and t.After.ExpiresAtUnixMilli>0 and t.After.ExpiresAtUnixMilli<=now then return redis.error_reply('POSTIMAGE_EXPIRED') end
end
if pending=='' then redis.call('SET',KEYS[2],s.Hash) end
for i,t in ipairs(s.Transitions) do
 if mode[i]=='PRE' then
  local key=KEYS[i+3];local result
  if t.Before.Kind=='set' then
   local keep={};if t.After.Kind=='set' then for _,m in ipairs(t.After.Members) do keep[m]=true end end
   local removed={};for _,m in ipairs(t.Before.Members) do if not keep[m] then table.insert(removed,m) end end
   if #removed>0 then result=redis.pcall('SREM',key,unpack(removed)) end
  elseif t.After.Kind=='none' then result=redis.pcall('DEL',key)
  elseif t.After.ExpiresAtUnixMilli>0 then result=redis.pcall('SET',key,t.After.Value,'PXAT',t.After.ExpiresAtUnixMilli)
  else result=redis.pcall('SET',key,t.After.Value) end
  if type(result)=='table' and result.err then return redis.error_reply('PARTIAL_STEP_REQUIRES_EXACT_RECOVERY') end
 end
end
for i,t in ipairs(s.Transitions) do if not matches(KEYS[i+3],t.After) then return redis.error_reply('POSTIMAGE_CONFLICT') end end
local receipt=s.Header..string.format('%.0f',now)..'\n'..table.concat(disposition,',')
redis.call('SET',KEYS[3],receipt);redis.call('SET',KEYS[1],receipt);redis.call('DEL',KEYS[2]);return receipt
`
