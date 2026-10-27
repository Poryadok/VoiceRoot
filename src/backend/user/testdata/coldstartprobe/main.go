// coldstartprobe is a negative/transport-only fixture client. It never supplies
// invented Auth or Role authority and does not turn connectivity into an allow.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/grpcclient"
	grpcsvc "voice/backend/user/internal/grpcsvc"
)

func main() {
	if len(os.Args) != 3 {
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, err := grpc.NewClient(grpcclient.DialTarget(os.Args[2]), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	conn.Connect()
	if err := grpcclient.WaitForReady(ctx, conn); err != nil {
		fmt.Println("TRANSPORT_UNAVAILABLE")
		os.Exit(1)
	}
	if os.Args[1] == "transport" {
		fmt.Println("TRANSPORT_READY_NOT_AUTHORITY")
		return
	}
	if os.Args[1] == "space-reconnect" {
		checker := grpcsvc.NewSpaceGRPCCoMembership(conn)
		// This is the real User adapter against the real ordinary Space RPC.
		// A forged raw Social marker must deny; no principal/allow is invented.
		for _, phase := range []struct {
			name string
			code codes.Code
		}{
			{"SPACE_DENIED_BEFORE", codes.Unauthenticated},
			{"SPACE_UNAVAILABLE", codes.Unavailable},
			{"SPACE_DENIED_AFTER_RECONNECT", codes.Unauthenticated},
		} {
			deadline := time.Now().Add(25 * time.Second)
			matched := false
			for time.Now().Before(deadline) {
				callCtx, done := context.WithTimeout(context.Background(), time.Second)
				callCtx = metadata.NewOutgoingContext(callCtx, metadata.Pairs("x-voice-internal-caller", "social"))
				allowed, callErr := checker.AreCoMembers(callCtx, uuid.MustParse("00000000-0000-4000-8000-000000000001"), uuid.MustParse("00000000-0000-4000-8000-000000000002"), nil)
				done()
				if allowed || callErr == nil {
					panic("ordinary Space must not grant authority")
				}
				if status.Code(callErr) == phase.code {
					matched = true
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			if !matched {
				panic("missing reconnect denial phase: " + phase.name)
			}
			fmt.Println(phase.name)
		}
		return
	}
	_, err = userv1.NewUserServiceClient(conn).GetProfile(ctx, &userv1.GetProfileRequest{By: &userv1.GetProfileRequest_ProfileId{ProfileId: "00000000-0000-4000-8000-000000000001"}})
	if status.Code(err) != codes.Unavailable {
		fmt.Printf("EXPECTED_STARTUP_UNAVAILABLE actual=%s\n", status.Code(err))
		os.Exit(1)
	}
	fmt.Println("STARTUP_RPC_DENIED_UNAVAILABLE")
}
