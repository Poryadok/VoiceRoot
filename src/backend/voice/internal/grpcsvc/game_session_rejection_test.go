package grpcsvc

import (
	"testing"

	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/gisowner"
	"voice/backend/pkg/principal"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestVoiceConflictRejectionsBindProvisionAndRosterRequests(t *testing.T) {
	const operationID = "6f2f5cd2-8712-484e-87d6-2da943a6f589"
	provision := &callsv1.ProvisionGameSessionRoomRequest{OperationId: operationID}
	provisionHash, err := principal.RequestHash(provision)
	if err != nil {
		t.Fatal(err)
	}
	provisionErr := annotateVoiceOwnerConflict(status.Error(codes.AlreadyExists, "conflict"), provision, gisowner.VoiceProvisionRPC)
	if category, ok := gisowner.Match(provisionErr, gisowner.VoiceDomain, gisowner.VoiceProvisionRPC, operationID, provisionHash); !ok || category != gisowner.CategoryOperationConflict {
		t.Fatalf("provision Match() = %q, %v; want typed conflict", category, ok)
	}
	roster := &callsv1.ApplyGameSessionRosterRequest{OperationId: operationID}
	rosterHash, err := principal.RequestHash(roster)
	if err != nil {
		t.Fatal(err)
	}
	rosterErr := annotateVoiceOwnerConflict(status.Error(codes.AlreadyExists, "conflict"), roster, gisowner.VoiceRosterRPC)
	if category, ok := gisowner.Match(rosterErr, gisowner.VoiceDomain, gisowner.VoiceRosterRPC, operationID, rosterHash); !ok || category != gisowner.CategoryOperationConflict {
		t.Fatalf("roster Match() = %q, %v; want typed conflict", category, ok)
	}
}

func TestVoiceTransientStatusIsNotTypedAsPermanent(t *testing.T) {
	const operationID = "6f2f5cd2-8712-484e-87d6-2da943a6f589"
	request := &callsv1.ProvisionGameSessionRoomRequest{OperationId: operationID}
	hash, err := principal.RequestHash(request)
	if err != nil {
		t.Fatal(err)
	}
	transient := status.Error(codes.Unavailable, "temporary owner failure")
	if got := annotateVoiceOwnerConflict(transient, request, gisowner.VoiceProvisionRPC); got != transient {
		t.Fatalf("annotateVoiceOwnerConflict() changed transient status: %v", got)
	}
	if category, ok := gisowner.Match(transient, gisowner.VoiceDomain, gisowner.VoiceProvisionRPC, operationID, hash); ok {
		t.Fatalf("transient status matched permanent category %q", category)
	}
}
