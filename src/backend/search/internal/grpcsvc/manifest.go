package grpcsvc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
)

type importedChatManifest struct {
	Binding *commonv1.ManifestBinding
	Pages   []*chatv1.SpacePurgeManifestPage
	ChatIDs []uuid.UUID
}

func (s *SearchGRPC) importChatManifest(ctx context.Context, spaceID, operationID uuid.UUID, generation uint64, root *commonv1.ManifestBinding) (*importedChatManifest, error) {
	if s.ChatManifest == nil {
		return nil, errors.New("chat manifest source unavailable")
	}
	if validateManifest(root) != nil || root.GetItemCount() == 0 {
		return nil, errors.New("root manifest unavailable")
	}
	var pages []*chatv1.SpacePurgeManifestPage
	var ids []uuid.UUID
	var binding *commonv1.ManifestBinding
	seenTokens := map[string]struct{}{"": {}}
	token := ""
	for pageIndex := uint64(0); ; pageIndex++ {
		response, err := s.ChatManifest.GetSpacePurgeManifestPage(ctx, &chatv1.GetSpacePurgeManifestPageRequest{ProtocolVersion: 1, SpaceId: spaceID.String(), DeletionOperationId: operationID.String(), Generation: generation, ManifestId: root.GetManifestId(), PageToken: token})
		if err != nil {
			return nil, err
		}
		page := response.GetPage()
		if err := validateManifestPage(page, root.GetManifestId(), pageIndex); err != nil {
			return nil, err
		}
		if !proto.Equal(root, page.GetManifest()) {
			return nil, errors.New("chat manifest does not match Space root")
		}
		if binding == nil {
			binding = proto.Clone(page.GetManifest()).(*commonv1.ManifestBinding)
		} else if !proto.Equal(binding, page.GetManifest()) {
			return nil, errors.New("chat manifest binding changed between pages")
		}
		for _, raw := range page.GetItemIds() {
			id, err := canonicalUUID(raw)
			if err != nil {
				return nil, err
			}
			if len(ids) > 0 && bytes.Compare(ids[len(ids)-1][:], id[:]) >= 0 {
				return nil, errors.New("chat manifest items are not unique raw-UUID sorted")
			}
			ids = append(ids, id)
		}
		pages = append(pages, proto.Clone(page).(*chatv1.SpacePurgeManifestPage))
		next := page.GetNextPageToken()
		if next == "" {
			break
		}
		if _, exists := seenTokens[next]; exists {
			return nil, errors.New("chat manifest page token cycle")
		}
		seenTokens[next] = struct{}{}
		token = next
		if pageIndex >= 1_000_000 {
			return nil, errors.New("chat manifest exceeds page limit")
		}
	}
	if binding.GetItemCount() == 0 || uint64(len(ids)) != binding.GetItemCount() {
		return nil, errors.New("chat manifest is empty or incomplete")
	}
	if !bytes.Equal(chatManifestSHA(spaceID, operationID, generation, ids), binding.GetManifestSha256()) {
		return nil, errors.New("chat manifest hash mismatch")
	}
	return &importedChatManifest{Binding: proto.Clone(binding).(*commonv1.ManifestBinding), Pages: pages, ChatIDs: ids}, nil
}

func validateManifestPage(page *chatv1.SpacePurgeManifestPage, manifestID string, index uint64) error {
	if page == nil || page.GetProtocolVersion() != 1 || page.GetPageIndex() != index || len(page.GetItemIds()) > 1000 || page.GetManifest() == nil {
		return errors.New("invalid Chat manifest page")
	}
	if hasUnknown(page.GetManifest()) {
		return errors.New("chat manifest page contains unknown fields")
	}
	b := page.GetManifest()
	if b.GetManifestId() != manifestID || len(b.GetManifestSha256()) != sha256.Size {
		return errors.New("chat manifest binding mismatch")
	}
	if len(page.GetPageSha256()) != sha256.Size {
		return errors.New("invalid Chat manifest page hash")
	}
	clone := proto.Clone(page).(*chatv1.SpacePurgeManifestPage)
	clone.PageSha256 = nil
	wire, err := proto.MarshalOptions{Deterministic: true}.Marshal(clone)
	if err != nil {
		return err
	}
	want := domainSeparatedSHA(string(page.ProtoReflect().Descriptor().FullName()), wire)
	if !bytes.Equal(want, page.GetPageSha256()) {
		return errors.New("chat manifest page hash mismatch")
	}
	return nil
}

func chatManifestSHA(spaceID, operationID uuid.UUID, generation uint64, ids []uuid.UUID) []byte {
	payload := make([]byte, 0, len("voice.chat.v1.SpaceDeletionChatManifest")+1+16+16+8+16*len(ids))
	payload = append(payload, "voice.chat.v1.SpaceDeletionChatManifest"...)
	payload = append(payload, 0)
	payload = append(payload, operationID[:]...)
	payload = append(payload, spaceID[:]...)
	var generationBytes [8]byte
	binary.BigEndian.PutUint64(generationBytes[:], generation)
	payload = append(payload, generationBytes[:]...)
	for _, id := range ids {
		payload = append(payload, id[:]...)
	}
	sum := sha256.Sum256(payload)
	return sum[:]
}

// validatePersistedManifestEvidence replays every durable manifest invariant
// inside the lifecycle transaction. A sealed bit or an item count alone is not
// enough evidence to authorize an irreversible purge after storage corruption.
func validatePersistedManifestEvidence(ctx context.Context, tx pgx.Tx, spaceID, operationID uuid.UUID, expected *commonv1.ManifestBinding) error {
	if err := validateManifest(expected); err != nil {
		return err
	}
	var generation, itemCount, pageCount uint64
	var manifestID string
	var manifestHash []byte
	var sealed bool
	if err := tx.QueryRow(ctx, `SELECT generation,manifest_id,manifest_sha256,item_count,page_count,sealed FROM search_space_chat_manifests WHERE space_id=$1 AND deletion_operation_id=$2`, spaceID, operationID).Scan(&generation, &manifestID, &manifestHash, &itemCount, &pageCount, &sealed); err != nil {
		return errors.New("complete chat manifest missing")
	}
	if !sealed || itemCount == 0 || pageCount == 0 || manifestID != expected.GetManifestId() || len(manifestHash) != sha256.Size {
		return errors.New("persisted chat manifest binding mismatch")
	}
	storedBinding := &commonv1.ManifestBinding{ManifestId: manifestID, ManifestSha256: bytes.Clone(manifestHash), ItemCount: itemCount}
	if !proto.Equal(storedBinding, expected) {
		return errors.New("persisted chat manifest does not match Space root")
	}

	rows, err := tx.Query(ctx, `SELECT page_index,page_bytes,page_sha256,item_count,next_page_token FROM search_space_chat_manifest_pages WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=$3 ORDER BY page_index`, spaceID, operationID, generation)
	if err != nil {
		return err
	}
	ids := make([]uuid.UUID, 0)
	var seenPages uint64
	seenTokens := map[string]struct{}{"": {}}
	for rows.Next() {
		var pageIndex, storedItems uint64
		var pageBytes, pageHash []byte
		var nextToken string
		if err = rows.Scan(&pageIndex, &pageBytes, &pageHash, &storedItems, &nextToken); err != nil {
			rows.Close()
			return err
		}
		page := &chatv1.SpacePurgeManifestPage{}
		if pageIndex != seenPages || proto.Unmarshal(pageBytes, page) != nil || validateManifestPage(page, manifestID, pageIndex) != nil || !proto.Equal(page.GetManifest(), storedBinding) || !bytes.Equal(pageHash, page.GetPageSha256()) || storedItems != uint64(len(page.GetItemIds())) || nextToken != page.GetNextPageToken() {
			rows.Close()
			return errors.New("persisted chat manifest page mismatch")
		}
		if pageIndex+1 < pageCount && nextToken == "" || pageIndex+1 == pageCount && nextToken != "" {
			rows.Close()
			return errors.New("persisted chat manifest page chain is incomplete")
		}
		if nextToken != "" {
			if _, duplicate := seenTokens[nextToken]; duplicate {
				rows.Close()
				return errors.New("persisted chat manifest page token cycle")
			}
			seenTokens[nextToken] = struct{}{}
		}
		canonicalBytes, marshalErr := proto.MarshalOptions{Deterministic: true}.Marshal(page)
		if marshalErr != nil || !bytes.Equal(canonicalBytes, pageBytes) {
			rows.Close()
			return errors.New("persisted chat manifest page is not canonical")
		}
		for _, raw := range page.GetItemIds() {
			id, parseErr := canonicalUUID(raw)
			if parseErr != nil || len(ids) > 0 && bytes.Compare(ids[len(ids)-1][:], id[:]) >= 0 {
				rows.Close()
				return errors.New("persisted chat manifest items are invalid")
			}
			ids = append(ids, id)
		}
		seenPages++
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if seenPages != pageCount || uint64(len(ids)) != itemCount || !bytes.Equal(chatManifestSHA(spaceID, operationID, generation, ids), manifestHash) {
		return errors.New("persisted chat manifest is incomplete")
	}

	itemRows, err := tx.Query(ctx, `SELECT chat_id FROM search_space_chat_manifest_items WHERE space_id=$1 AND deletion_operation_id=$2 AND generation=$3 ORDER BY page_index,item_index`, spaceID, operationID, generation)
	if err != nil {
		return err
	}
	seenItems := 0
	for itemRows.Next() {
		var id uuid.UUID
		if err = itemRows.Scan(&id); err != nil || seenItems >= len(ids) || id != ids[seenItems] {
			itemRows.Close()
			return errors.New("persisted chat manifest item projection mismatch")
		}
		seenItems++
	}
	if err = itemRows.Err(); err != nil {
		itemRows.Close()
		return err
	}
	itemRows.Close()
	if seenItems != len(ids) {
		return errors.New("persisted chat manifest item projection is incomplete")
	}
	return nil
}
