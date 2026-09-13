package indexer

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

// jetStreamMsgAck supports JetStream ack/nak/term without importing test doubles.
type jetStreamMsgAck interface {
	Ack(...nats.AckOpt) error
	Nak(...nats.AckOpt) error
	NakWithDelay(time.Duration, ...nats.AckOpt) error
	Term(...nats.AckOpt) error
}

// jetStreamConsumeAck keeps transient failures pending on the durable. Search
// has neither a DLQ nor a historical backfill contract, so retry exhaustion
// must never turn a recoverable index update into permanent data loss.
func jetStreamConsumeAck(msg jetStreamMsgAck, handlerErr error, stream string, metrics *ConsumerMetrics, logger *slog.Logger) {
	if msg == nil {
		return
	}
	if handlerErr != nil {
		if isPermanentConsumeError(handlerErr) {
			recordDisposition(msg.Term(), stream, consumerOutcomeTerminalSemanticPoison, metrics, logger)
			return
		}
		recordDisposition(msg.NakWithDelay(jetStreamRetryDelay(msg)), stream, consumerOutcomeRetry, metrics, logger)
		return
	}
	recordDisposition(msg.Ack(), stream, consumerOutcomeAck, metrics, logger)
}

func consumeJetStreamMessage(msg jetStreamMsgAck, stream string, metrics *ConsumerMetrics, logger *slog.Logger, handle func() error) {
	jetStreamConsumeAck(msg, handle(), stream, metrics, logger)
}

// jetStreamTermAck permanently drops a poison message that cannot be processed.
func jetStreamTermAck(msg jetStreamMsgAck, stream string, metrics *ConsumerMetrics, logger *slog.Logger) {
	if msg == nil {
		return
	}
	recordDisposition(msg.Term(), stream, consumerOutcomeTerminalMalformed, metrics, logger)
}

func recordDisposition(err error, stream, outcome string, metrics *ConsumerMetrics, logger *slog.Logger) {
	if err == nil {
		metrics.observe(stream, outcome)
		if outcome == consumerOutcomeTerminalMalformed || outcome == consumerOutcomeTerminalSemanticPoison {
			logConsumerOutcome(logger, stream, outcome)
		}
		return
	}
	metrics.observe(stream, consumerOutcomeDispositionFailure)
	if logger != nil {
		logger.Warn("search indexer JetStream disposition failed", slog.String("stream", stream), slog.String("outcome", outcome), slog.Any("error", err))
	}
}

func logConsumerOutcome(logger *slog.Logger, stream, outcome string) {
	if logger != nil {
		logger.Warn("search indexer terminal JetStream outcome", slog.String("stream", stream), slog.String("outcome", outcome))
	}
}

type jetStreamMetadata interface {
	Metadata() (*nats.MsgMetadata, error)
}

func jetStreamRetryDelay(msg jetStreamMsgAck) time.Duration {
	attempt := 1
	if withMetadata, ok := msg.(jetStreamMetadata); ok {
		if metadata, err := withMetadata.Metadata(); err == nil && metadata != nil && metadata.NumDelivered > 0 {
			attempt = int(metadata.NumDelivered)
		}
	}
	index := attempt - 1
	if index >= len(searchConsumerRetryBackoff) {
		index = len(searchConsumerRetryBackoff) - 1
	}
	return searchConsumerRetryBackoff[index]
}

type permanentConsumeError struct{ err error }

func (e *permanentConsumeError) Error() string { return e.err.Error() }
func (e *permanentConsumeError) Unwrap() error { return e.err }

func newPermanentConsumeError(format string, args ...any) error {
	return &permanentConsumeError{err: fmt.Errorf(format, args...)}
}

func isPermanentConsumeError(err error) bool {
	var permanent *permanentConsumeError
	return errors.As(err, &permanent)
}

// ensure *nats.Msg implements jetStreamMsgAck at compile time.
var _ jetStreamMsgAck = (*nats.Msg)(nil)
