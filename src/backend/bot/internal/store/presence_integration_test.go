package store_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/bot/internal/store"
	"voice/backend/pkg/integrationtest"
)

func botPresenceMigrationSQL(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "bot_db")
	initSQL, err := os.ReadFile(filepath.Join(dir, "000001_init.up.sql"))
	require.NoError(t, err)
	presenceSQL, err := os.ReadFile(filepath.Join(dir, "000002_bot_presence.up.sql"))
	require.NoError(t, err)
	requestsSQL, err := os.ReadFile(filepath.Join(dir, "000006_bot_chat_create_requests.up.sql"))
	require.NoError(t, err)
	return string(initSQL) + "\n" + string(presenceSQL) + "\n" + string(requestsSQL)
}

func startBotStoreWithPresence(t *testing.T) *store.BotStore {
	t.Helper()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "botdb_presence", "")
	_, err := pool.Exec(ctx, botPresenceMigrationSQL(t))
	require.NoError(t, err)
	return &store.BotStore{Pool: pool}
}

// TestIncrementDailyChatCreates_concurrent proves Postgres upsert is atomic under concurrent writers.
func TestIncrementDailyChatCreates_concurrent(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startBotStoreWithPresence(t)
	owner := uuid.New()
	row, _, err := st.CreateBot(ctx, owner, "LimitBot", "", `["TEXT_CHAT_CREATE_IN_SPACE"]`, uuid.Nil)
	require.NoError(t, err)

	const workers = 20
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			_, incErr := st.IncrementDailyChatCreates(ctx, row.ID)
			require.NoError(t, incErr)
		}()
	}
	wg.Wait()

	count, err := st.IncrementDailyChatCreates(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, workers+1, count, "atomic upsert must count every increment")
}

func TestReserveDailyChatCreate_concurrentLimit(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startBotStoreWithPresence(t)
	owner := uuid.New()
	row, _, err := st.CreateBot(ctx, owner, "LimitBot", "", `[]`, uuid.Nil)
	require.NoError(t, err)
	for range 9 {
		_, err := st.IncrementDailyChatCreates(ctx, row.ID)
		require.NoError(t, err)
	}

	const workers = 2
	var wg sync.WaitGroup
	var mu sync.Mutex
	var reserved int
	var rejected int
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			_, ok, reserveErr := st.ReserveDailyChatCreate(ctx, row.ID)
			require.NoError(t, reserveErr)
			mu.Lock()
			defer mu.Unlock()
			if ok {
				reserved++
			} else {
				rejected++
			}
		}()
	}
	wg.Wait()

	require.Equal(t, 1, reserved)
	require.Equal(t, 1, rejected)
	count, err := st.DailyChatCreateCount(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, 10, count, "concurrent reservations must never over-admit")
}

func TestReleaseDailyChatCreate_restoresDefiniteRejectionCapacity(t *testing.T) {
	ctx := context.Background()
	st := startBotStoreWithPresence(t)
	owner := uuid.New()
	row, _, err := st.CreateBot(ctx, owner, "LimitBot", "", `[]`, uuid.Nil)
	require.NoError(t, err)
	for range 9 {
		_, err := st.IncrementDailyChatCreates(ctx, row.ID)
		require.NoError(t, err)
	}

	day, ok, err := st.ReserveDailyChatCreate(ctx, row.ID)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, st.ReleaseDailyChatCreate(ctx, row.ID, day))
	count, err := st.DailyChatCreateCount(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, 9, count)
}

func TestReserveDailyChatCreateForRequest_replaysWithoutDoubleReservation(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startBotStoreWithPresence(t)
	owner := uuid.New()
	row, _, err := st.CreateBot(ctx, owner, "IdempotentLimitBot", "", `[]`, uuid.Nil)
	require.NoError(t, err)
	for range 9 {
		_, err := st.IncrementDailyChatCreates(ctx, row.ID)
		require.NoError(t, err)
	}
	requestID := uuid.New()
	requestHash := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	day, admitted, replayed, err := st.ReserveDailyChatCreateForRequest(ctx, row.ID, requestID, requestHash)
	require.NoError(t, err)
	require.True(t, admitted)
	require.False(t, replayed)
	require.NotZero(t, day)

	_, admitted, replayed, err = st.ReserveDailyChatCreateForRequest(ctx, row.ID, requestID, requestHash)
	require.NoError(t, err)
	require.False(t, admitted)
	require.True(t, replayed)
	count, err := st.DailyChatCreateCount(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, 10, count, "retrying the same request must not reserve a second slot")
	_, err = st.FinishDailyChatCreateRequestAttempt(ctx, row.ID, requestID, store.BotChatCreateAttemptRejected)
	require.NoError(t, err)
	_, err = st.FinishDailyChatCreateRequestAttempt(ctx, row.ID, requestID, store.BotChatCreateAttemptSucceeded)
	require.NoError(t, err)

	_, _, _, err = st.ReserveDailyChatCreateForRequest(ctx, row.ID, requestID, "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	require.ErrorIs(t, err, store.ErrBotChatCreateRequestConflict)
}

func TestReserveDailyChatCreateForRequest_twoDistinctRequestsAtNineAdmitOne(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startBotStoreWithPresence(t)
	owner := uuid.New()
	row, _, err := st.CreateBot(ctx, owner, "ConcurrentIdempotentLimitBot", "", `[]`, uuid.Nil)
	require.NoError(t, err)
	for range 9 {
		_, err := st.IncrementDailyChatCreates(ctx, row.ID)
		require.NoError(t, err)
	}

	start := make(chan struct{})
	type outcome struct {
		admitted bool
		err      error
	}
	outcomes := make(chan outcome, 2)
	for range 2 {
		requestID := uuid.New()
		go func(id uuid.UUID) {
			<-start
			_, admitted, replayed, reserveErr := st.ReserveDailyChatCreateForRequest(ctx, row.ID, id, "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd")
			if replayed && reserveErr == nil {
				reserveErr = errors.New("distinct request unexpectedly replayed")
			}
			outcomes <- outcome{admitted: admitted, err: reserveErr}
		}(requestID)
	}
	close(start)
	first, second := <-outcomes, <-outcomes
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.NotEqual(t, first.admitted, second.admitted, "only one distinct request can reserve the tenth slot")
	count, err := st.DailyChatCreateCount(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, 10, count)
}

func TestFinishDailyChatCreateRequestAttempt_releasesOnlyAfterAllDefiniteRejections(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startBotStoreWithPresence(t)
	otherRepository := &store.BotStore{Pool: st.Pool}
	owner := uuid.New()
	row, _, err := st.CreateBot(ctx, owner, "ReleaseIdempotentBot", "", `[]`, uuid.Nil)
	require.NoError(t, err)
	requestID := uuid.New()
	hash := "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	_, admitted, _, err := st.ReserveDailyChatCreateForRequest(ctx, row.ID, requestID, hash)
	require.NoError(t, err)
	require.True(t, admitted)
	_, admitted, replayed, err := otherRepository.ReserveDailyChatCreateForRequest(ctx, row.ID, requestID, hash)
	require.NoError(t, err)
	require.False(t, admitted)
	require.True(t, replayed)

	released, err := st.FinishDailyChatCreateRequestAttempt(ctx, row.ID, requestID, store.BotChatCreateAttemptRejected)
	require.NoError(t, err)
	require.False(t, released, "one rejection cannot release while another same-key RPC is in flight")
	count, err := st.DailyChatCreateCount(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	_, err = otherRepository.FinishDailyChatCreateRequestAttempt(ctx, row.ID, requestID, store.BotChatCreateAttemptSucceeded)
	require.NoError(t, err)
	_, err = st.FinishDailyChatCreateRequestAttempt(ctx, row.ID, requestID, store.BotChatCreateAttemptRejected)
	require.NoError(t, err)
	count, err = st.DailyChatCreateCount(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, 1, count, "a late rejection cannot release quota after another repository records success")

	secondID := uuid.New()
	_, admitted, _, err = st.ReserveDailyChatCreateForRequest(ctx, row.ID, secondID, hash)
	require.NoError(t, err)
	require.True(t, admitted)
	_, _, _, err = otherRepository.ReserveDailyChatCreateForRequest(ctx, row.ID, secondID, hash)
	require.NoError(t, err)
	_, err = st.FinishDailyChatCreateRequestAttempt(ctx, row.ID, secondID, store.BotChatCreateAttemptSucceeded)
	require.NoError(t, err)
	_, err = otherRepository.FinishDailyChatCreateRequestAttempt(ctx, row.ID, secondID, store.BotChatCreateAttemptRejected)
	require.NoError(t, err)
	count, err = st.DailyChatCreateCount(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, 2, count, "the inverse completion order also keeps the single reservation")

	thirdID := uuid.New()
	_, admitted, _, err = st.ReserveDailyChatCreateForRequest(ctx, row.ID, thirdID, hash)
	require.NoError(t, err)
	require.True(t, admitted)
	_, _, _, err = otherRepository.ReserveDailyChatCreateForRequest(ctx, row.ID, thirdID, hash)
	require.NoError(t, err)
	_, err = otherRepository.FinishDailyChatCreateRequestAttempt(ctx, row.ID, thirdID, store.BotChatCreateAttemptRejected)
	require.NoError(t, err)
	_, err = st.FinishDailyChatCreateRequestAttempt(ctx, row.ID, thirdID, store.BotChatCreateAttemptRejected)
	require.NoError(t, err)
	count, err = st.DailyChatCreateCount(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, 2, count, "all attempts may release exactly one slot after definite rejection")
	_, _, _, err = otherRepository.ReserveDailyChatCreateForRequest(ctx, row.ID, thirdID, "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee")
	require.ErrorIs(t, err, store.ErrBotChatCreateRequestConflict, "a released reservation must retain its request hash tombstone")
	_, admitted, replayed, err = otherRepository.ReserveDailyChatCreateForRequest(ctx, row.ID, thirdID, hash)
	require.NoError(t, err)
	require.True(t, admitted, "the same hash may acquire a new slot after a definite rejection")
	require.True(t, replayed)
	_, err = otherRepository.FinishDailyChatCreateRequestAttempt(ctx, row.ID, thirdID, store.BotChatCreateAttemptRejected)
	require.NoError(t, err)
	count, err = st.DailyChatCreateCount(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, 2, count, "same-hash definite retry releases only its newly reserved slot")

	fourthID := uuid.New()
	_, admitted, _, err = st.ReserveDailyChatCreateForRequest(ctx, row.ID, fourthID, hash)
	require.NoError(t, err)
	require.True(t, admitted)
	_, err = st.FinishDailyChatCreateRequestAttempt(ctx, row.ID, fourthID, store.BotChatCreateAttemptUncertain)
	require.NoError(t, err)
	_, _, replayed, err = otherRepository.ReserveDailyChatCreateForRequest(ctx, row.ID, fourthID, hash)
	require.NoError(t, err)
	require.True(t, replayed)
	_, err = otherRepository.FinishDailyChatCreateRequestAttempt(ctx, row.ID, fourthID, store.BotChatCreateAttemptRejected)
	require.NoError(t, err)
	count, err = st.DailyChatCreateCount(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, 3, count, "a later definite rejection cannot clear an earlier uncertain outcome")
}

func TestReserveDailyChatCreateForRequest_tombstoneKeepsHashAndUsesRetryDay(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	st := startBotStoreWithPresence(t)
	owner := uuid.New()
	row, _, err := st.CreateBot(ctx, owner, "TombstoneRolloverBot", "", `[]`, uuid.Nil)
	require.NoError(t, err)
	requestID := uuid.New()
	hash := "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	reservedDay, admitted, _, err := st.ReserveDailyChatCreateForRequest(ctx, row.ID, requestID, hash)
	require.NoError(t, err)
	require.True(t, admitted)
	_, err = st.Pool.Exec(ctx, `UPDATE bot_daily_chat_creates SET day=CURRENT_DATE-1 WHERE bot_id=$1 AND day=$2`, row.ID, reservedDay)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE bot_chat_create_requests SET day=CURRENT_DATE-1 WHERE bot_id=$1 AND request_id=$2`, row.ID, requestID)
	require.NoError(t, err)

	released, err := st.FinishDailyChatCreateRequestAttempt(ctx, row.ID, requestID, store.BotChatCreateAttemptRejected)
	require.NoError(t, err)
	require.True(t, released)
	var previousDayCount int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count FROM bot_daily_chat_creates WHERE bot_id=$1 AND day=CURRENT_DATE-1`, row.ID).Scan(&previousDayCount))
	require.Equal(t, 0, previousDayCount, "release decrements the attempt's original quota day")

	_, _, _, err = st.ReserveDailyChatCreateForRequest(ctx, row.ID, requestID, "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	require.ErrorIs(t, err, store.ErrBotChatCreateRequestConflict, "hash tombstone remains after release and day rollover")
	newDay, admitted, replayed, err := st.ReserveDailyChatCreateForRequest(ctx, row.ID, requestID, hash)
	require.NoError(t, err)
	require.True(t, admitted)
	require.True(t, replayed)
	require.Equal(t, reservedDay, newDay, "a re-admitted same-hash attempt reserves against the current day")
	count, err := st.DailyChatCreateCount(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	_, err = st.FinishDailyChatCreateRequestAttempt(ctx, row.ID, requestID, store.BotChatCreateAttemptSucceeded)
	require.NoError(t, err)
}

func TestLookupGameIntegrationBotAuthorityReturnsOnlyStoredOwnerAndLifecycle(t *testing.T) {
	ctx := context.Background()
	st := startBotStoreWithPresence(t)
	owner := uuid.New()
	bot, _, err := st.CreateBot(ctx, owner, "AuthorityBot", "", `[]`, uuid.Nil)
	require.NoError(t, err)

	gotOwner, status, found, err := st.LookupGameIntegrationBotAuthority(ctx, bot.ID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, owner, gotOwner)
	require.Equal(t, "live", status)

	missingOwner, missingStatus, missing, err := st.LookupGameIntegrationBotAuthority(ctx, uuid.New())
	require.NoError(t, err)
	require.False(t, missing)
	require.Equal(t, uuid.Nil, missingOwner)
	require.Empty(t, missingStatus)

	_, err = st.Pool.Exec(ctx, `UPDATE bots SET status='disabled' WHERE id=$1`, bot.ID)
	require.NoError(t, err)
	gotOwner, status, found, err = st.LookupGameIntegrationBotAuthority(ctx, bot.ID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, owner, gotOwner)
	require.Equal(t, "disabled", status, "GIS proof handler must reject a non-live Bot")
}
