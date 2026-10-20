package principalruntime

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/principalgrpc"
)

// Dispatch recorder only: no receipt/storage/global fence authority is claimed.
// The actual44 store/handler and247 persisted-root composition are separate.
func (s *recordingRoleServer) RegisterBotGlobalTerminal(ctx context.Context,_ *rolev1.RegisterBotGlobalTerminalRequest)(*rolev1.RegisterBotGlobalTerminalResponse,error){
	s.calls.Add(1);p,_:=principal.FromContext(ctx);s.principals<-p
	return &rolev1.RegisterBotGlobalTerminalResponse{},nil
}

func TestBotGlobalTerminalProtectedDispatch(t *testing.T){
	f:=newRuntimeFixture(t,2);f.config.BotLifecycleEnabled=true;f.config.JWKSURLs["bot"]=f.config.JWKSURLs["space"]
	address,recorder,roots:=startRuntimeListener(t,f)
	client:=runtimeListenerClient(t,address,runtimeTLSCredentials(f,roots))
	method:=rolev1.RoleService_RegisterBotGlobalTerminal_FullMethodName
	request:=&rolev1.RegisterBotGlobalTerminalRequest{ProtocolVersion:1,BotId:uuid.NewString(),DecisionId:uuid.NewString()}
	hash,err:=principal.RequestHash(request);require.NoError(t,err)
	token:=issueRuntimeToken(t,f.key,"bot","current","role",method,request.DecisionId,hash)
	for _,name:=range []string{"valid","replay","wrong_issuer","wrong_rpc","wrong_hash","wrong_root","unknown_body","raw_identity","zero_bot","wrong_protocol"}{
		t.Run(name,func(t *testing.T){
			r:=proto.Clone(request).(*rolev1.RegisterBotGlobalTerminalRequest)
			issuer,rpc,requestID,digest:="bot",method,request.DecisionId,hash
			switch name{case "wrong_issuer":issuer="space";case "wrong_rpc":rpc=rolev1.RoleService_ApplyBotInstallationRemoval_FullMethodName;case "wrong_hash":digest=runtimeHash;case "wrong_root":requestID=uuid.NewString();case "unknown_body":r.ProtoReflect().SetUnknown([]byte{0xa0,0x06,0x01});case "zero_bot":r.BotId=uuid.Nil.String();case "wrong_protocol":r.ProtocolVersion=2}
			if name=="unknown_body"||name=="zero_bot"||name=="wrong_protocol"{digest,err=principal.RequestHash(r);require.NoError(t,err)}
			signed:=issueRuntimeToken(t,f.key,issuer,"current","role",rpc,requestID,digest)
			if name=="valid"||name=="replay"{signed=token}
			md:=metadata.Pairs("authorization","Bearer "+signed,"x-request-id",requestID)
			if name=="raw_identity"{md.Set("x-voice-profile-id",uuid.NewString())}
			ctx,cancel:=context.WithTimeout(metadata.NewOutgoingContext(context.Background(),md),2*time.Second);defer cancel()
			_,err=client.RegisterBotGlobalTerminal(ctx,r)
			if name=="valid"{require.NoError(t,err);p:=<-recorder.principals;require.Equal(t,"service:bot",p.Subject);require.Equal(t,request.DecisionId,p.RequestID)}else{require.Error(t,err)}
			require.EqualValues(t,1,recorder.calls.Load())
		})
	}
}

func TestBotGlobalTerminalOrdinaryDeniesBeforeHandler(t *testing.T){
	called:=false
	_,err:=principalgrpc.OwnershipUnaryInterceptor(nil)(context.Background(),&rolev1.RegisterBotGlobalTerminalRequest{},&grpc.UnaryServerInfo{FullMethod:rolev1.RoleService_RegisterBotGlobalTerminal_FullMethodName},func(context.Context,any)(any,error){called=true;return nil,nil})
	require.Error(t,err);require.False(t,called)
}
