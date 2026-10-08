package registry

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestWinningCloseReconcilesInFlightCreateOwnerStages(t *testing.T) {
	stages := []struct {
		name       string
		ownerStage string
		prior      int
	}{
		{name: "chat create", ownerStage: "chat_create", prior: 0},
		{name: "chat roster", ownerStage: "chat_roster", prior: 1},
		{name: "voice provision", ownerStage: "voice_provision", prior: 2},
		{name: "role apply", ownerStage: "role_apply", prior: 3},
	}
	for _, stage := range stages {
		t.Run(stage.name, func(t *testing.T) {
			store, ctx := startT31SessionStore(t)
			owners := newSessionOwnerScript()
			p := newT31TestSessionPrincipal(t, ctx, store)
			o := NewSessionOrchestrator(store, owners.adapters())
			created, err := o.CreateSession(ctx, p, CreateSessionInput{
				OperationID: uuid.New(), Kind: "party", ExternalKey: "close-race-" + stage.ownerStage,
				DisplayName: "Raid", RosterRevision: 1, RosterComplete: true,
			})
			require.NoError(t, err)
			for i := 0; i < stage.prior; i++ {
				_, err = o.AdvanceOne(ctx, created.OperationID)
				require.NoError(t, err)
			}

			committed := make(chan struct{})
			release := make(chan struct{})
			owners.committed[stage.ownerStage] = committed
			owners.release[stage.ownerStage] = release
			workerResult := make(chan error, 1)
			go func() {
				_, advanceErr := o.AdvanceOne(ctx, created.OperationID)
				workerResult <- advanceErr
			}()
			select {
			case <-committed: // the fake owner has committed, but its reply is held
			case <-time.After(10 * time.Second):
				t.Fatal("owner call did not reach its durable commit point")
			}

			closeOperationID := uuid.New()
			closed, err := o.CloseSession(ctx, p, created.SessionID, closeOperationID)
			require.NoError(t, err)
			require.Equal(t, "pending", closed.Status)
			var createStatus string
			require.NoError(t, store.Pool.QueryRow(ctx, `SELECT status FROM gis_session_operations WHERE operation_id=$1`, created.OperationID).Scan(&createStatus))
			require.Equal(t, "failed", createStatus, "close revokes create-worker custody before replay")

			close(release)
			require.Error(t, <-workerResult, "the original create worker must lose its lease-fenced write")
			advanceSessionUntil(t, ctx, o, p, closed.OperationID, "closed")

			calls := owners.calls[stage.ownerStage]
			require.Len(t, calls, 2, "close must replay the exact owner operation to recover the committed receipt")
			require.Equal(t, calls[0].OperationID, calls[1].OperationID)
			require.Equal(t, calls[0].RequestHash, calls[1].RequestHash)
			require.Equal(t, 1, owners.sideEffects[sessionOwnerEffectKey{operationID: calls[0].OperationID, requestHash: calls[0].RequestHash}])
			create, err := o.GetOperation(ctx, p, created.OperationID)
			require.NoError(t, err)
			require.Equal(t, "failed", create.Status)
			require.Nil(t, create.ActiveEventID)
			_, err = o.AdvanceOne(ctx, created.OperationID)
			require.Error(t, err, "terminal create state must fence any later grant application")
			closedAgain, err := o.CloseSession(ctx, p, created.SessionID, closeOperationID)
			require.NoError(t, err)
			require.Equal(t, "succeeded", closedAgain.Status, "terminal close replay is read-only")
			var receiptCount int
			require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_session_owner_receipts
				WHERE operation_id=$1 AND stage=$2 AND owner_operation_id=$3`, created.OperationID, stage.ownerStage, calls[0].OperationID).Scan(&receiptCount))
			require.Equal(t, 1, receiptCount, "the close operation must persist the recovered owner receipt against the create operation")
			if stage.prior < 2 {
				require.Empty(t, owners.calls["voice_close"])
				require.Empty(t, owners.calls["role_revoke"])
			} else {
				require.Len(t, owners.calls["voice_close"], 1)
				require.Len(t, owners.calls["role_revoke"], 1)
			}
		})
	}
}

func TestWinningCloseFencesPendingCreateWithoutVoiceResource(t *testing.T) {
	store, ctx := startT31SessionStore(t)
	owners := newSessionOwnerScript()
	p := newT31TestSessionPrincipal(t, ctx, store)
	o := NewSessionOrchestrator(store, owners.adapters())
	created, err := o.CreateSession(ctx, p, CreateSessionInput{OperationID: uuid.New(), Kind: "party", ExternalKey: "close-before-first-stage", DisplayName: "Raid", RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{}})
	require.NoError(t, err)
	closed, err := o.CloseSession(ctx, p, created.SessionID, uuid.New())
	require.NoError(t, err)
	require.Equal(t, "succeeded", closed.Status)
	require.Equal(t, "closed", closed.SessionStatus)
	create, err := o.GetOperation(ctx, p, created.OperationID)
	require.NoError(t, err)
	require.Equal(t, "failed", create.Status)
	require.Equal(t, "failed", create.Stage)
	_, err = o.AdvanceOne(ctx, created.OperationID)
	require.Error(t, err, "terminal create operation must no longer be claimable")
	require.Empty(t, owners.calls["chat_create"])
	require.Empty(t, owners.calls["voice_provision"])
}

func TestWinningClosePersistsTypedOwnerRejectionCustody(t *testing.T) {
	cases := []struct {
		name, stage, category string
		prior                 int
		noEffect              bool
	}{
		{name: "chat create conflict is unresolved", stage: "chat_create", category: "operation_conflict"},
		{name: "chat resource conflict is unresolved", stage: "chat_create", category: "resource_conflict"},
		{name: "chat roster missing is no effect", stage: "chat_roster", category: "resource_missing", prior: 1, noEffect: true},
		{name: "chat roster operation conflict is unresolved", stage: "chat_roster", category: "operation_conflict", prior: 1},
		{name: "chat roster resource conflict is unresolved", stage: "chat_roster", category: "resource_conflict", prior: 1},
		{name: "voice provision conflict is unresolved", stage: "voice_provision", category: "operation_conflict", prior: 2},
		{name: "role revoked is no effect and still closes voice", stage: "role_apply", category: "terminal_revoked", prior: 3, noEffect: true},
		{name: "role operation conflict is unresolved", stage: "role_apply", category: "operation_conflict", prior: 3},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			store, ctx := startT31SessionStore(t)
			owners := newSessionOwnerScript()
			adapters := owners.adapters()
			p := newT31TestSessionPrincipal(t, ctx, store)
			o := NewSessionOrchestrator(store, adapters)
			created, err := o.CreateSession(ctx, p, CreateSessionInput{
				OperationID: uuid.New(), Kind: "party", ExternalKey: "close-rejection-" + testCase.stage,
				DisplayName: "Raid", RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{},
			})
			require.NoError(t, err)
			for i := 0; i < testCase.prior; i++ {
				_, err = o.AdvanceOne(ctx, created.OperationID)
				require.NoError(t, err)
			}

			entered, release := make(chan struct{}), make(chan struct{})
			var rejectedRequests []SessionOwnerRequest
			reject := func(_ context.Context, request SessionOwnerRequest) (SessionOwnerReceipt, error) {
				rejectedRequests = append(rejectedRequests, request)
				if len(rejectedRequests) == 1 {
					close(entered)
					<-release
				}
				return SessionOwnerReceipt{}, &PermanentOwnerRejection{Category: testCase.category}
			}
			switch testCase.stage {
			case "chat_create":
				adapters.CreateChat = reject
			case "chat_roster":
				adapters.SyncChatRoster = reject
			case "voice_provision":
				adapters.ProvisionVoice = reject
			case "role_apply":
				adapters.ApplyRoleGrants = reject
			}
			o = NewSessionOrchestrator(store, adapters)
			workerResult := make(chan error, 1)
			go func() {
				_, advanceErr := o.AdvanceOne(ctx, created.OperationID)
				workerResult <- advanceErr
			}()
			select {
			case <-entered:
			case <-time.After(10 * time.Second):
				t.Fatal("owner rejection call did not start")
			}

			closeID := uuid.New()
			closing, err := o.CloseSession(ctx, p, created.SessionID, closeID)
			require.NoError(t, err)
			require.Equal(t, "create_reconcile_pending:"+stageBefore(testCase.stage), closing.Stage)
			close(release)
			<-workerResult

			closeKey := SessionOperationKey{ApplicationID: p.ApplicationID, EnvironmentID: p.EnvironmentID, OperationID: closeID}
			classified, err := o.AdvanceScoped(ctx, closeKey)
			require.NoError(t, err)
			require.Len(t, rejectedRequests, 2, "close must replay the same rejected owner operation")
			require.Equal(t, rejectedRequests[0].OperationID, rejectedRequests[1].OperationID)
			require.Equal(t, rejectedRequests[0].RequestHash, rejectedRequests[1].RequestHash)
			require.NotEmpty(t, rejectedRequests[0].RequestHash)

			var receiptCount int
			require.NoError(t, store.Pool.QueryRow(ctx, `SELECT count(*) FROM gis_session_owner_receipts
				WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3 AND stage=$4`,
				p.ApplicationID, p.EnvironmentID, created.OperationID, testCase.stage).Scan(&receiptCount))
			require.Zero(t, receiptCount, "a typed rejection must never create an owner receipt")

			if !testCase.noEffect {
				require.Equal(t, "pending", classified.Status)
				require.Equal(t, "closing", classified.SessionStatus)
				require.Equal(t, "create_reconcile_unresolved:"+stageBefore(testCase.stage)+":"+testCase.category, classified.Stage)
				var suspended bool
				require.NoError(t, store.Pool.QueryRow(ctx, `SELECT next_attempt_at='infinity'::timestamptz
					FROM gis_session_operations WHERE application_id=$1 AND environment_id=$2 AND operation_id=$3`,
					p.ApplicationID, p.EnvironmentID, closeID).Scan(&suspended))
				require.True(t, suspended, "ambiguous immutable rejection must not be background-retried")
				replayed, replayErr := o.AdvanceScoped(ctx, closeKey)
				require.NoError(t, replayErr)
				require.Equal(t, classified.Stage, replayed.Stage)
				require.Len(t, rejectedRequests, 2, "explicit replay must return unresolved custody without another owner call")
				return
			}

			require.Equal(t, "pending", classified.Status)
			require.True(t, strings.HasPrefix(classified.Stage, "create_reconcile_no_effect:"))
			advanced, err := o.AdvanceScoped(ctx, closeKey)
			require.NoError(t, err)
			if testCase.stage == "role_apply" {
				require.Equal(t, "voice_close_pending", advanced.Stage)
				advanceSessionUntil(t, ctx, o, p, closeID, "closed")
			} else {
				require.Equal(t, "closed", advanced.Stage)
				require.Equal(t, "succeeded", advanced.Status)
			}
			createAfterClose, err := o.GetOperation(ctx, p, created.OperationID)
			require.NoError(t, err)
			require.Equal(t, "failed", createAfterClose.Status)
			_, err = o.AdvanceOne(ctx, created.OperationID)
			require.Error(t, err, "a create rejected by the winning close must never reapply owner effects")
			require.Len(t, rejectedRequests, 2)
			if testCase.stage == "role_apply" {
				require.Len(t, owners.calls["voice_close"], 1)
				require.Len(t, owners.calls["role_revoke"], 1)
				require.Nil(t, createAfterClose.RoleGrantReceiptID)
			}
		})
	}
}

func TestWinningCloseRetriesGenericOwnerFailureWithSameRequest(t *testing.T) {
	store, ctx := startT31SessionStore(t)
	owners := newSessionOwnerScript()
	adapters := owners.adapters()
	p := newT31TestSessionPrincipal(t, ctx, store)
	o := NewSessionOrchestrator(store, adapters)
	created, err := o.CreateSession(ctx, p, CreateSessionInput{
		OperationID: uuid.New(), Kind: "party", ExternalKey: "close-transport-retry",
		DisplayName: "Raid", RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{},
	})
	require.NoError(t, err)
	_, err = o.AdvanceOne(ctx, created.OperationID)
	require.NoError(t, err, "Chat create receipt advances to the roster stage")

	entered, release := make(chan struct{}), make(chan struct{})
	var requests []SessionOwnerRequest
	transient := func(call int, request SessionOwnerRequest) (SessionOwnerReceipt, error) {
		requests = append(requests, request)
		if call == 1 {
			close(entered)
			<-release
			return SessionOwnerReceipt{}, errors.New("owner transport timeout")
		}
		if call == 2 {
			return SessionOwnerReceipt{}, errors.New("owner unavailable")
		}
		return owners.result("chat_roster", request)
	}
	adapters.SyncChatRoster = func(_ context.Context, request SessionOwnerRequest) (SessionOwnerReceipt, error) {
		return transient(len(requests)+1, request)
	}
	o = NewSessionOrchestrator(store, adapters)
	workerResult := make(chan error, 1)
	go func() {
		_, advanceErr := o.AdvanceOne(ctx, created.OperationID)
		workerResult <- advanceErr
	}()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("roster owner call did not start")
	}
	closed, err := o.CloseSession(ctx, p, created.SessionID, uuid.New())
	require.NoError(t, err)
	close(release)
	<-workerResult

	closeKey := SessionOperationKey{ApplicationID: p.ApplicationID, EnvironmentID: p.EnvironmentID, OperationID: closed.OperationID}
	_, err = o.AdvanceScoped(ctx, closeKey)
	require.Error(t, err, "generic transport failures remain retryable")
	require.Len(t, requests, 2)
	require.Equal(t, requests[0].OperationID, requests[1].OperationID)
	require.Equal(t, requests[0].RequestHash, requests[1].RequestHash)
	require.Equal(t, "create_reconcile_pending:chat_ready", mustGetOperation(t, o, ctx, p, closed.OperationID).Stage)

	resolved, err := o.AdvanceScoped(ctx, closeKey)
	require.NoError(t, err)
	require.Equal(t, "closed", resolved.Stage)
	require.Len(t, requests, 3)
	require.Equal(t, requests[0].OperationID, requests[2].OperationID)
	require.Equal(t, requests[0].RequestHash, requests[2].RequestHash)
}

func stageBefore(ownerStage string) string {
	switch ownerStage {
	case "chat_create":
		return "accepted"
	case "chat_roster":
		return "chat_ready"
	case "voice_provision":
		return "roster_ready"
	case "role_apply":
		return "voice_ready"
	default:
		return ""
	}
}

func mustGetOperation(t *testing.T, o *SessionOrchestrator, ctx context.Context, p SessionPrincipal, operationID uuid.UUID) SessionOperation {
	t.Helper()
	operation, err := o.GetOperation(ctx, p, operationID)
	require.NoError(t, err)
	return operation
}

func TestPermanentTypedOwnerRejectionsDurablyFailAndCleanUp(t *testing.T) {
	stages := []struct {
		name, stage, category string
		prior                 int
		voiceExists           bool
	}{
		{name: "chat create conflict", stage: "chat_create", category: "operation_conflict"},
		{name: "chat roster missing after create receipt", stage: "chat_roster", category: "resource_missing", prior: 1},
		{name: "voice provision conflict", stage: "voice_provision", category: "operation_conflict", prior: 2},
		{name: "role apply after voice provision", stage: "role_apply", category: "terminal_revoked", prior: 3, voiceExists: true},
	}
	for _, rejected := range stages {
		t.Run(rejected.name, func(t *testing.T) {
			store, ctx := startT31SessionStore(t)
			owners := newSessionOwnerScript()
			adapters := owners.adapters()
			ownerCall := func(stage string) func(context.Context, SessionOwnerRequest) (SessionOwnerReceipt, error) {
				return func(_ context.Context, request SessionOwnerRequest) (SessionOwnerReceipt, error) {
					owners.calls[stage] = append(owners.calls[stage], request)
					return SessionOwnerReceipt{}, &PermanentOwnerRejection{Category: rejected.category}
				}
			}
			switch rejected.stage {
			case "chat_create":
				adapters.CreateChat = ownerCall(rejected.stage)
			case "chat_roster":
				adapters.SyncChatRoster = ownerCall(rejected.stage)
			case "voice_provision":
				adapters.ProvisionVoice = ownerCall(rejected.stage)
			case "role_apply":
				adapters.ApplyRoleGrants = ownerCall(rejected.stage)
			}
			p := newT31TestSessionPrincipal(t, ctx, store)
			o := NewSessionOrchestrator(store, adapters)
			created, err := o.CreateSession(ctx, p, CreateSessionInput{
				OperationID: uuid.New(), Kind: "party", ExternalKey: "permanent-rejection-" + rejected.stage,
				DisplayName: "Raid", RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{},
			})
			require.NoError(t, err)
			for index := 0; index < rejected.prior; index++ {
				_, err = o.AdvanceOne(ctx, created.OperationID)
				require.NoError(t, err)
			}
			failedCreate, err := o.AdvanceOne(ctx, created.OperationID)
			require.NoError(t, err, "permanent owner rejection must create a durable failure operation")
			require.Equal(t, "failed", failedCreate.Status)
			if rejected.voiceExists {
				require.Equal(t, "closing", failedCreate.SessionStatus, "terminal cleanup remains visible until its receipts are durable")
			} else {
				require.Equal(t, "failed", failedCreate.SessionStatus)
			}

			var failureOperationID uuid.UUID
			var errorCode string
			require.NoError(t, store.Pool.QueryRow(ctx, `SELECT operation_id FROM gis_session_operations
				WHERE session_id=$1 AND operation_kind='failure'`, created.SessionID).Scan(&failureOperationID))
			require.NoError(t, store.Pool.QueryRow(ctx, `SELECT error_code FROM gis_session_operations WHERE operation_id=$1`, created.OperationID).Scan(&errorCode))
			require.Equal(t, "OWNER_"+strings.ToUpper(rejected.stage)+"_"+strings.ToUpper(rejected.category), errorCode)
			advanceSessionUntil(t, ctx, o, p, failureOperationID, "failed")
			if rejected.voiceExists {
				require.Len(t, owners.calls["voice_close"], 1, "failure cleanup must close the provisioned room")
				require.Len(t, owners.calls["role_revoke"], 1, "failure cleanup must revoke grants")
			} else {
				require.Empty(t, owners.calls["voice_close"])
				require.Empty(t, owners.calls["role_revoke"])
			}
			require.Len(t, owners.calls[rejected.stage], 1, "terminal rejection must not replay the refused stage")
			_, err = o.AdvanceOne(ctx, created.OperationID)
			require.Error(t, err, "terminal create replay must not resurrect owner effects")
			_, err = o.AdvanceOne(ctx, failureOperationID)
			require.Error(t, err, "terminal failure replay must not repeat cleanup")
			require.Len(t, owners.calls[rejected.stage], 1)
		})
	}
}

func TestSessionCloseAndPermanentFailureHaveOneTerminalizationWinner(t *testing.T) {
	store, ctx := startT31SessionStore(t)
	owners := newSessionOwnerScript()
	p := newT31TestSessionPrincipal(t, ctx, store)
	o := NewSessionOrchestrator(store, owners.adapters())
	created, err := o.CreateSession(ctx, p, CreateSessionInput{OperationID: uuid.New(), Kind: "party", ExternalKey: "terminal-race", DisplayName: "Raid", RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{}})
	require.NoError(t, err)
	advanceSessionUntil(t, ctx, o, p, created.OperationID, "active")
	closeID, failureID := uuid.New(), uuid.New()
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	results := make(chan SessionOperation, 2)
	errs := make(chan error, 2)
	go func() {
		defer wg.Done()
		<-start
		op, e := o.CloseSession(ctx, p, created.SessionID, closeID)
		results <- op
		errs <- e
	}()
	go func() {
		defer wg.Done()
		<-start
		op, e := o.FailSession(ctx, p, created.SessionID, failureID, "OWNER_REJECTED")
		results <- op
		errs <- e
	}()
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var operations []SessionOperation
	for op := range results {
		operations = append(operations, op)
	}
	require.Len(t, operations, 2)
	var winner uuid.UUID
	var terminalKind string
	var finalStatus string
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT terminalization_operation_id,terminalization_kind,session_status FROM gis_sessions WHERE id=$1`, created.SessionID).Scan(&winner, &terminalKind, &finalStatus))
	require.Contains(t, []uuid.UUID{closeID, failureID}, winner)
	terminalState := "closed"
	if terminalKind == "failure" {
		terminalState = "failed"
	}
	advanceSessionUntil(t, ctx, o, p, operations[0].OperationID, terminalState)
	advanceSessionUntil(t, ctx, o, p, operations[1].OperationID, terminalState)
	for _,op:=range operations{if op.OperationID!=winner{_,e:=o.AdvanceOne(ctx,op.OperationID);require.NoError(t,e,"the coalesced loser must durably resolve after the winner commits")}}
	require.NoError(t, store.Pool.QueryRow(ctx, `SELECT session_status FROM gis_sessions WHERE id=$1`, created.SessionID).Scan(&finalStatus))
	if winner == closeID {
		require.Equal(t, "close", terminalKind)
		require.Equal(t, "closed", finalStatus)
	} else {
		require.Equal(t, "failure", terminalKind)
		require.Equal(t, "failed", finalStatus)
	}
	require.Len(t, owners.calls["voice_close"], 1, "the losing terminalization must not invoke another Voice close ID")
	require.Len(t, owners.calls["role_revoke"], 1, "the losing terminalization must not invoke another Role revoke ID")
	resolved := make([]SessionOperation, 0, 2)
	for _, op := range operations {
		value, e := o.GetOperation(ctx, p, op.OperationID)
		require.NoError(t, e)
		require.Equal(t,"succeeded",value.Status)
		resolved = append(resolved, value)
	}
	require.NotNil(t, resolved[0].VoiceCloseReceiptID)
	require.NotNil(t, resolved[1].VoiceCloseReceiptID)
	require.Equal(t, *resolved[0].VoiceCloseReceiptID, *resolved[1].VoiceCloseReceiptID)
	require.NotNil(t, resolved[0].RoleRevokeReceiptID)
	require.NotNil(t, resolved[1].RoleRevokeReceiptID)
	require.Equal(t, *resolved[0].RoleRevokeReceiptID, *resolved[1].RoleRevokeReceiptID)
	createAfterTerminal, err := o.GetOperation(ctx, p, created.OperationID)
	require.NoError(t, err)
	if terminalKind == "close" {
		require.Equal(t, "succeeded", createAfterTerminal.Status, "close must not rewrite a completed create operation")
	} else {
		require.Equal(t, "failed", createAfterTerminal.Status, "failure terminalization must remain visible on the create operation")
	}
	require.NotNil(t, createAfterTerminal.VoiceCloseReceiptID)
	require.NotNil(t, createAfterTerminal.RoleRevokeReceiptID)
	first, second := owners.calls["voice_close"][0], owners.calls["role_revoke"][0]
	require.Equal(t, winner, firstSessionTerminalizer(ctx, store, created.SessionID))
	require.NotEqual(t, uuid.Nil, first.OperationID)
	require.NotEqual(t, uuid.Nil, second.OperationID)
}

func firstSessionTerminalizer(ctx context.Context, store *Store, session uuid.UUID) uuid.UUID {
	var id uuid.UUID
	_ = store.Pool.QueryRow(ctx, `SELECT terminalization_operation_id FROM gis_sessions WHERE id=$1`, session).Scan(&id)
	return id
}

func TestSessionWorkerRestartReclaimsExpiredStageLease(t *testing.T) {
	store, ctx := startT31SessionStore(t)
	owners := newSessionOwnerScript()
	p := newT31TestSessionPrincipal(t, ctx, store)
	o := NewSessionOrchestrator(store, owners.adapters())
	accepted, err := o.CreateSession(ctx, p, CreateSessionInput{OperationID: uuid.New(), Kind: "party", ExternalKey: "restart-lease", DisplayName: "Raid", RosterRevision: 1, RosterComplete: true, Members: []uuid.UUID{}})
	require.NoError(t, err)
	_, err = store.Pool.Exec(ctx, `UPDATE gis_session_operations SET lease_owner=$2,lease_until=now()-interval '1 second' WHERE operation_id=$1`, accepted.OperationID, uuid.New())
	require.NoError(t, err)
	restarted := NewSessionOrchestrator(&Store{Pool: store.Pool}, owners.adapters())
	advanced, err := restarted.AdvanceOne(ctx, accepted.OperationID)
	require.NoError(t, err)
	require.Equal(t, "chat_ready", advanced.Stage)
	require.Len(t, owners.calls["chat_create"], 1)
}
