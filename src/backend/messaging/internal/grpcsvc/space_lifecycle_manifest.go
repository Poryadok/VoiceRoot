package grpcsvc

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	messagingv1 "voice.app/voice/messaging/v1"
)

const spacePurgeManifestPageSize = 1000

func validateSpacePurgeManifestPageRequest(req *messagingv1.ImportSpacePurgeManifestPageRequest) error {
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 || req.GetProtocolVersion() != 1 || req.GetScheduleGeneration() == 0 || req.GetScheduleGeneration() > math.MaxInt64 || req.GetPage() == nil {
		return errors.New("invalid Space purge manifest page request")
	}
	if _, err := canonicalSpaceLifecycleUUID(req.GetSpaceId()); err != nil {
		return errors.New("invalid Space purge manifest page request")
	}
	if _, err := canonicalSpaceLifecycleUUID(req.GetDeletionOperationId()); err != nil {
		return errors.New("invalid Space purge manifest page request")
	}
	page := req.GetPage()
	if page.GetProtocolVersion() != 1 || page.GetManifest() == nil || page.GetManifest().GetManifestId() == "" ||
		len(page.GetManifest().GetManifestSha256()) != sha256.Size || hasAnyUnknown(page.GetManifest()) {
		return errors.New("invalid Chat manifest binding")
	}
	if page.GetManifest().GetItemCount() > math.MaxInt64 || len(page.GetItemIds()) > spacePurgeManifestPageSize {
		return errors.New("invalid Chat manifest page size")
	}
	pageCount := (page.GetManifest().GetItemCount() + spacePurgeManifestPageSize - 1) / spacePurgeManifestPageSize
	if pageCount == 0 {
		pageCount = 1
	}
	if page.GetPageIndex() >= pageCount {
		return errors.New("Chat manifest page index is outside the manifest")
	}
	wantItemCount := spacePurgeManifestPageSize
	if page.GetPageIndex()+1 == pageCount {
		wantItemCount = int(page.GetManifest().GetItemCount() - page.GetPageIndex()*spacePurgeManifestPageSize)
	}
	finalPage := page.GetPageIndex()+1 == pageCount
	if len(page.GetItemIds()) != wantItemCount || req.GetSealsManifest() != finalPage ||
		finalPage && page.GetNextPageToken() != "" || !finalPage && page.GetNextPageToken() == "" {
		return errors.New("Chat manifest page chain or seal marker is inconsistent")
	}
	hash := sha256.New()
	hash.Write([]byte("voice.chat.v1.SpaceDeletionManifestPage\x00"))
	hash.Write(page.GetManifest().GetManifestSha256())
	var integer [8]byte
	binary.BigEndian.PutUint64(integer[:], page.GetPageIndex())
	hash.Write(integer[:])
	binary.BigEndian.PutUint64(integer[:], uint64(len(page.GetItemIds())))
	hash.Write(integer[:])
	var previous uuid.UUID
	for i, raw := range page.GetItemIds() {
		id, err := canonicalSpaceLifecycleUUID(raw)
		if err != nil || i > 0 && bytes.Compare(previous[:], id[:]) >= 0 {
			return errors.New("Chat manifest page IDs must be canonical and raw-UUID sorted")
		}
		previous = id
		hash.Write(id[:])
	}
	if len(page.GetPageSha256()) != sha256.Size {
		return errors.New("invalid Chat manifest page hash")
	}
	if !bytes.Equal(hash.Sum(nil), page.GetPageSha256()) {
		return errors.New("Chat manifest page hash mismatch")
	}
	return nil
}

func hasAnyUnknown(message proto.Message) bool {
	if message == nil {
		return false
	}
	reflection := message.ProtoReflect()
	if len(reflection.GetUnknown()) != 0 {
		return true
	}
	unknown := false
	reflection.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.IsList() && field.Message() != nil {
			list := value.List()
			for i := 0; i < list.Len(); i++ {
				if hasAnyUnknown(list.Get(i).Message().Interface()) {
					unknown = true
					return false
				}
			}
		} else if field.IsMap() && field.MapValue().Message() != nil {
			value.Map().Range(func(_ protoreflect.MapKey, entry protoreflect.Value) bool {
				if hasAnyUnknown(entry.Message().Interface()) {
					unknown = true
					return false
				}
				return true
			})
		} else if field.Message() != nil && hasAnyUnknown(value.Message().Interface()) {
			unknown = true
		}
		return !unknown
	})
	return unknown
}

func domainSHA(name string, wire []byte) []byte {
	input := make([]byte, 0, len(name)+1+len(wire))
	input = append(input, name...)
	input = append(input, 0)
	input = append(input, wire...)
	sum := sha256.Sum256(input)
	return sum[:]
}
