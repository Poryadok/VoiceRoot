package store

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAttachmentIntentBindsPreEnrichmentRequestReceipt(t *testing.T) {
	row := MessageRow{ID: uuid.New(), AttachmentsJSON: `[{"type":"music","title":"File canonical title"}]`, AdmissionRequestHash: "sha256:" + strings.Repeat("a", 64), AdmissionRequestRPC: "/voice.messaging.v1.MessagingService/SendMessage"}
	a, err := attachmentIntentIdentity(row, nil)
	require.NoError(t, err)
	changed := row
	changed.AdmissionRequestHash = "sha256:" + strings.Repeat("b", 64)
	b, err := attachmentIntentIdentity(changed, nil)
	require.NoError(t, err)
	require.NotEqual(t, string(a), string(b), "equal enriched attachments must not erase different original request fingerprints")
	equivalent := row
	equivalent.ID = uuid.New()
	equivalent.AttachmentsJSON = `[ { "title":"File canonical title", "type":"music" } ]`
	b, err = attachmentIntentIdentity(equivalent, nil)
	require.NoError(t, err)
	require.Equal(t, string(a), string(b), "normalized equivalent retries keep the immutable request fingerprint")
	legacy := row
	legacy.ID, legacy.AdmissionRequestHash, legacy.AdmissionRequestRPC = uuid.Nil, "", ""
	a, err = attachmentIntentIdentity(legacy, nil)
	require.NoError(t, err)
	b, err = json.Marshal(struct {
		Row   MessageRow
		Space *uuid.UUID
	}{legacy, nil})
	require.NoError(t, err)
	require.Equal(t, string(b), string(a), "historical intents retain their original serialized shape")
}
