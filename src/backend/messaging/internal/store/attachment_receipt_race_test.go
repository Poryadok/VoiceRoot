package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	filev1 "voice.app/voice/file/v1"
)

func TestAttachmentConcurrentReceiptConflictSurvivesEqualEnrichedIdentity(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL attachment operation lock and immutable receipt")
	}
	st, row, request := attachmentIntentFixture(t)
	row.AdmissionRequestRPC = "/voice.messaging.v1.MessagingService/SendMessage"
	row.AdmissionRequestHash = "sha256:" + strings.Repeat("a", 64)
	changed := row
	changed.ID = uuid.New()
	changed.AdmissionRequestHash = "sha256:" + strings.Repeat("b", 64)
	// File canonicalization has already replaced the caller's different tags.
	require.Equal(t, row.AttachmentsJSON, changed.AttachmentsJSON)
	entered, release, attempted := make(chan struct{}), make(chan struct{}), make(chan struct{})
	first, second := make(chan error, 1), make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func() {
		_, err := st.InsertMessageWithReferences(ctx, row, nil, request, func(context.Context, *filev1.AcquireFileReferencesRequest) error {
			close(entered)
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		first <- err
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	go func() {
		close(attempted)
		_, err := st.InsertMessageWithReferences(ctx, changed, nil, request, func(context.Context, *filev1.AcquireFileReferencesRequest) error {
			return errors.New("changed request must not reacquire")
		})
		second <- err
	}()
	<-attempted
	close(release)
	require.NoError(t, <-first)
	err := <-second
	require.True(t, errors.Is(err, ErrAttachmentIntentConflict) || errors.Is(err, ErrSendReceiptConflict), "changed retained request must conflict: %v", err)
	// JSON-equivalent caller retries use the same normalized pre-enrichment hash,
	// independently covered by the handler fingerprint normalization regression.
	equivalent := row
	equivalent.ID = uuid.New()
	var formatted bytes.Buffer
	require.NoError(t, json.Indent(&formatted, []byte(row.AttachmentsJSON), "", "  "))
	equivalent.AttachmentsJSON = formatted.String()
	saved, err := st.InsertMessageWithReferences(ctx, equivalent, nil, request, func(context.Context, *filev1.AcquireFileReferencesRequest) error {
		t.Error("committed equivalent retry reacquired")
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, row.ID, saved.ID)
	var messages, receipts, intents int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM messages),(SELECT count(*) FROM message_send_receipts),(SELECT count(*) FROM messaging_attachment_send_intents WHERE state='COMMITTED')`).Scan(&messages, &receipts, &intents))
	require.Equal(t, []int{1, 1, 1}, []int{messages, receipts, intents})
}
