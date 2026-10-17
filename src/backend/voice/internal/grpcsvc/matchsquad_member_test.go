package grpcsvc

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	callsv1 "voice.app/voice/calls/v1"
)

type memberControllerStub struct{ calls int }

func (s *memberControllerStub) Join(context.Context, *callsv1.JoinMatchSquadRoomRequest) (*callsv1.JoinMatchSquadRoomResponse, error) {
	s.calls++
	return &callsv1.JoinMatchSquadRoomResponse{}, nil
}
func (s *memberControllerStub) GetJoinToken(context.Context, *callsv1.GetMatchSquadJoinTokenRequest) (*callsv1.GetMatchSquadJoinTokenResponse, error) {
	s.calls++
	return &callsv1.GetMatchSquadJoinTokenResponse{}, nil
}
func (s *memberControllerStub) Leave(context.Context, *callsv1.LeaveMatchSquadRoomRequest) (*callsv1.LeaveMatchSquadRoomResponse, error) {
	s.calls++
	return &callsv1.LeaveMatchSquadRoomResponse{}, nil
}

func TestMatchSquadMemberGRPCRejectsUnknownFieldsBeforeController(t *testing.T) {
	controller := &memberControllerStub{}
	server := &MatchSquadMemberGRPC{Controller: controller}
	if _, err := server.JoinMatchSquadRoom(context.Background(), nil); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("nil join request code = %v, want InvalidArgument", status.Code(err))
	}
	request := &callsv1.GetMatchSquadJoinTokenRequest{}
	request.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
	if _, err := server.GetMatchSquadJoinToken(context.Background(), request); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("unknown-field token request code = %v, want InvalidArgument", status.Code(err))
	}
	leave := &callsv1.LeaveMatchSquadRoomRequest{}
	leave.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
	if _, err := server.LeaveMatchSquadRoom(context.Background(), leave); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("unknown-field leave request code = %v, want InvalidArgument", status.Code(err))
	}
	if controller.calls != 0 {
		t.Fatalf("controller calls = %d, want 0", controller.calls)
	}
}

func TestMatchSquadMemberGRPCFailsClosedWithoutController(t *testing.T) {
	server := &MatchSquadMemberGRPC{}
	if _, err := server.JoinMatchSquadRoom(context.Background(), &callsv1.JoinMatchSquadRoomRequest{}); status.Code(err) != codes.Unavailable {
		t.Fatalf("join code = %v, want Unavailable", status.Code(err))
	}
}
