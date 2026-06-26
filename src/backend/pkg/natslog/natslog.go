package natslog

import (
	"context"
	"log/slog"
	"strings"

	"github.com/nats-io/nats.go"

	"voice/backend/pkg/correlation"
)

// RequestIDFromMsg reads X-Request-Id from NATS message headers.
func RequestIDFromMsg(msg *nats.Msg) string {
	if msg == nil || msg.Header == nil {
		return ""
	}
	return strings.TrimSpace(msg.Header.Get(correlation.RequestIDHeader))
}

// PublishAttrs returns common attrs for a NATS publish log line.
func PublishAttrs(subject, requestID string, extra ...slog.Attr) []slog.Attr {
	attrs := []slog.Attr{
		slog.String("event", "nats_publish"),
		slog.String("subject", subject),
		slog.String("request_id", requestID),
	}
	return append(attrs, extra...)
}

// ConsumeAttrs returns common attrs for a NATS consume log line.
func ConsumeAttrs(msg *nats.Msg, extra ...slog.Attr) []slog.Attr {
	subject := ""
	if msg != nil {
		subject = msg.Subject
	}
	attrs := []slog.Attr{
		slog.String("event", "nats_consume"),
		slog.String("subject", subject),
		slog.String("request_id", RequestIDFromMsg(msg)),
	}
	return append(attrs, extra...)
}

// LogPublish writes a structured publish log line.
func LogPublish(logger *slog.Logger, subject, requestID, msg string, attrs ...slog.Attr) {
	if logger == nil {
		return
	}
	all := PublishAttrs(subject, requestID, attrs...)
	logger.LogAttrs(context.Background(), slog.LevelInfo, msg, all...)
}

// LogPublishError writes a structured publish failure log line (event=nats_publish, level=error).
func LogPublishError(logger *slog.Logger, subject, requestID string, publishErr error, attrs ...slog.Attr) {
	if logger == nil || publishErr == nil {
		return
	}
	all := PublishAttrs(subject, requestID, append(attrs, slog.String("error", publishErr.Error()))...)
	logger.LogAttrs(context.Background(), slog.LevelError, "nats publish failed", all...)
}

// LogConsume writes a structured consume log line.
func LogConsume(logger *slog.Logger, msg *nats.Msg, level slog.Level, text string, attrs ...slog.Attr) {
	if logger == nil {
		return
	}
	all := ConsumeAttrs(msg, attrs...)
	logger.LogAttrs(context.Background(), level, text, all...)
}

// SetRequestIDHeader sets X-Request-Id on a NATS message header map.
func SetRequestIDHeader(hdr nats.Header, requestID string) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || hdr == nil {
		return
	}
	hdr.Set(correlation.RequestIDHeader, requestID)
}
