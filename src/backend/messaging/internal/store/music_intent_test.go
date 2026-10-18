package store

import (
	"encoding/json"
	"github.com/google/uuid"
	"strings"
	"testing"
)

func TestMusicIntentIgnoresOnlyServerMetadata(t *testing.T) {
	file, chat := uuid.New(), uuid.New()
	row := MessageRow{ContentType: "music", AttachmentsJSON: `[{"type":"music","file_id":"` + file.String() + `","title":"old","duration_seconds":3}]`, MusicBinding: &MusicAttachmentBinding{FileID: file, SourceChatID: chat, Revision: 1}}
	first, err := attachmentIntentIdentity(row, nil)
	if err != nil {
		t.Fatal(err)
	}
	row.AttachmentsJSON = `[{"type":"music","file_id":"` + file.String() + `","title":"new","album":"new","duration_seconds":4}]`
	row.MusicBinding.Revision = 2
	next, err := attachmentIntentIdentity(row, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(next) {
		t.Fatal("server refresh changed client intent")
	}
	row.Content = "different request"
	changed, _ := attachmentIntentIdentity(row, nil)
	if string(first) == string(changed) {
		t.Fatal("caller content no longer bound")
	}
	row.Content = ""
	row.AttachmentsJSON = `[{"type":"music","file_id":"` + uuid.New().String() + `"}]`
	changed, _ = attachmentIntentIdentity(row, nil)
	if string(first) == string(changed) {
		t.Fatal("file identity no longer bound")
	}
}

func TestPrivateMusicBindingDoesNotChangeLegacyRowJSON(t *testing.T) {
	raw, err := json.Marshal(MessageRow{ContentType: "document"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "MusicBinding") {
		t.Fatal("private field changed legacy intent serialization")
	}
}
