package store

import (
	"context"
	"errors"
	"reflect"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/messaging/internal/messageevents"
)

func validateForwardCommentary(row MessageRow) error {
	c := row.ForwardCommentary
	if c == nil {
		return nil
	}
	expected := MessageRow{ID: c.ID, ChatID: row.ChatID, ChatType: row.ChatType, SenderProfileID: row.SenderProfileID, Content: c.Content, Type: "regular", AttachmentsJSON: "[]", MentionsJSON: "[]", GhostOnly: row.GhostOnly, ContentType: "text", PostedAsChat: row.PostedAsChat, DisplayChatID: row.DisplayChatID}
	if c.ID == uuid.Nil || c.ID == row.ID || row.ClientMessageID != nil || strings.TrimSpace(c.Content) == "" || !reflect.DeepEqual(*c, expected) {
		return errors.New("invalid forward commentary companion")
	}
	return nil
}

func insertForwardCommentaryDB(ctx context.Context, tx pgx.Tx, row MessageRow) error {
	if err := validateForwardCommentary(row); err != nil {
		return err
	}
	if row.ForwardCommentary == nil {
		return nil
	}
	saved, err := insertMessageDB(ctx, tx, *row.ForwardCommentary)
	if err != nil {
		return err
	}
	if saved.ID != row.ForwardCommentary.ID {
		return errors.New("forward commentary identity conflict")
	}
	return nil
}

func compoundEventMatches(row MessageRow, event messageevents.OutboxEvent) bool {
	return event.ChatID == row.ChatID && (event.MessageID == row.ID || row.ForwardCommentary != nil && event.MessageID == row.ForwardCommentary.ID)
}
