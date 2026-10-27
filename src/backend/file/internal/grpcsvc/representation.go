package grpcsvc

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strings"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/store"
)

func representationToProto(r *store.Representation) *filev1.FileRepresentation {
	if r == nil {
		return nil
	}
	return &filev1.FileRepresentation{MimeType: r.MIMEType, SizeBytes: r.SizeBytes, Sha256Hash: r.SHA256, Filename: r.Filename, ProcessorVersion: r.ProcessorVersion}
}

func selectedRepresentation(row store.FileRow, variant filev1.FileURLVariant) (store.Representation, error) {
	if err := validateFileURLVariant(variant); err != nil {
		return store.Representation{}, err
	}
	if row.UploadIntent == "gif" {
		r := row.DeliveredFull
		key := row.ConvertedR2Key
		mime := "video/mp4"
		if variant == filev1.FileURLVariant_FILE_URL_VARIANT_THUMBNAIL {
			r = row.DeliveredThumbnail
			key = row.ThumbnailR2Key
			mime = "image/webp"
		}
		if row.Status != "ready" || r == nil || key == nil || r.Key != *key || r.Key == "" || r.MIMEType != mime || r.SizeBytes <= 0 || r.SizeBytes > 5*1024*1024 || len(r.SHA256) != 64 || r.Filename == "" || strings.ContainsAny(r.Filename, "\r\n/\\") {
			return store.Representation{}, status.Error(codes.FailedPrecondition, "GIF representation unavailable")
		}
		return *r, nil
	}
	key, err := fileURLKey(row, variant)
	if err != nil {
		return store.Representation{}, err
	}
	r := store.Representation{Key: key, MIMEType: row.MimeType, SizeBytes: row.SizeBytes, Filename: row.OriginalName}
	if variant == filev1.FileURLVariant_FILE_URL_VARIANT_THUMBNAIL {
		r.MIMEType = "image/webp"
		r.SizeBytes = 5 * 1024 * 1024
		r.Filename = "thumbnail.webp"
	}
	return r, nil
}
