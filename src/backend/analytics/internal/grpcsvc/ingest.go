package grpcsvc

import (
	"context"

	analyticsv1 "voice.app/voice/analytics/v1"
	"voice/backend/analytics/internal/buffer"
	"voice/backend/analytics/internal/metrics"
)

// IngestGRPC implements AnalyticsIngestService.
type IngestGRPC struct {
	analyticsv1.UnimplementedAnalyticsIngestServiceServer
	Buffer *buffer.Accumulator
}

func (s *IngestGRPC) IngestEvent(_ context.Context, req *analyticsv1.IngestEventRequest) (*analyticsv1.IngestEventResponse, error) {
	if s != nil && s.Buffer != nil && req.GetEvent() != nil {
		s.Buffer.AppendProto(req.GetEvent())
		metrics.EventsIngested.Inc()
	}
	return &analyticsv1.IngestEventResponse{}, nil
}

func (s *IngestGRPC) IngestBatch(_ context.Context, req *analyticsv1.IngestBatchRequest) (*analyticsv1.IngestBatchResponse, error) {
	if s != nil && s.Buffer != nil {
		for _, ev := range req.GetEvents() {
			if ev != nil {
				s.Buffer.AppendProto(ev)
				metrics.EventsIngested.Inc()
			}
		}
	}
	return &analyticsv1.IngestBatchResponse{}, nil
}
