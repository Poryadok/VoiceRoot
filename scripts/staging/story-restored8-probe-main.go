package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"math"
	"os"
	"strconv"
	"time"
	storyv1 "voice.app/voice/story/v1"
)

func stop(phase string, err error) {
	r := map[string]any{"status": "FAIL", "phase": phase}
	if err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) {
			r["sqlstate"] = pg.Code
			r["constraint"] = pg.ConstraintName
		} else {
			r["grpc_code"] = status.Code(err).String()
		}
	}
	_ = json.NewEncoder(os.Stdout).Encode(r)
	os.Exit(17)
}
func check(ok bool, phase string) {
	if !ok {
		stop(phase, nil)
	}
}
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	pool, e := pgxpool.New(ctx, os.Getenv("FIXTURE_STORY"))
	if e != nil {
		stop("db_connect", e)
	}
	defer pool.Close()
	baseline, parseErr := strconv.Atoi(os.Getenv("FIXTURE_BASELINE_STORIES"))
	check(parseErr == nil && baseline >= 0, "baseline_stories_input")
	baselineUnrelated, parseErr := strconv.Atoi(os.Getenv("FIXTURE_BASELINE_UNRELATED"))
	check(parseErr == nil && baselineUnrelated >= 0, "baseline_unrelated_input")
	var count int
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM stories`).Scan(&count); e != nil {
		stop("count_before", e)
	}
	if len(os.Args) > 1 && os.Args[1] == "wrong" {
		check(count == baseline+2, "wrong_expected_row_count")
		stop("negative_control_not_detected", nil)
	}
	check(count == baseline, "retained_story_count")
	conn, e := grpc.NewClient("127.0.0.1:9090", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if e != nil {
		stop("grpc_connect", e)
	}
	defer func() { _ = conn.Close() }()
	client := storyv1.NewStoryServiceClient(conn)
	account, author := uuid.New(), uuid.New()
	privacy, closePrivacy, e := startPrivacyFixture(author)
	if e != nil {
		stop("privacy_fixture_listen", e)
	}
	defer closePrivacy()
	rpcctx := metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-user-id", account.String(), "x-voice-profile-id", author.String()))
	text := "stories live"
	response, e := client.CreateStory(rpcctx, &storyv1.CreateStoryRequest{Type: "text", TextContent: &text, Visibility: "friends"})
	if e != nil {
		stop("create_story", e)
	}
	check(privacy.calls.Load() > 0, "privacy_rpc_observed")
	story := response.GetStory()
	id, e := uuid.Parse(story.GetId())
	if e != nil {
		stop("response_uuid", nil)
	}
	check(id != uuid.Nil && story.GetAuthorProfileId() == author.String() && story.GetType() == "text", "response_identity_type")
	var sqlAuthor uuid.UUID
	var kind, sqlText, visibility string
	var media *uuid.UUID
	var mentions []byte
	var lfp bool
	var created, expires, archived time.Time
	var deleted, expired *time.Time
	e = pool.QueryRow(ctx, `SELECT author_profile_id,type,text_content,visibility,media_file_id,mention_profile_ids,is_looking_for_party,created_at,expires_at,archived_until,deleted_at,expired_at FROM stories WHERE id=$1`, id).Scan(&sqlAuthor, &kind, &sqlText, &visibility, &media, &mentions, &lfp, &created, &expires, &archived, &deleted, &expired)
	if e != nil {
		stop("sql_readback", e)
	}
	var decoded []any
	if e = json.Unmarshal(mentions, &decoded); e != nil {
		stop("mentions_json", nil)
	}
	check(sqlAuthor == author && kind == "text" && sqlText == text && visibility == "friends" && media == nil && len(decoded) == 0 && !lfp && deleted == nil && expired == nil, "sql_writer_fields")
	check(!created.IsZero() && math.Abs(expires.Sub(created).Seconds()-86400) < 5 && archived.Equal(expires.Add(30*24*time.Hour)), "sql_ttl_archive")
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM stories`).Scan(&count); e != nil {
		stop("count_after", e)
	}
	check(count == baseline+1, "one_story_inserted")
	deniedctx := metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-user-id", account.String()))
	_, e = client.CreateStory(deniedctx, &storyv1.CreateStoryRequest{Type: "text", TextContent: &text, Visibility: "friends"})
	check(status.Code(e) == codes.Unauthenticated, "missing_profile_unauthenticated")
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM stories`).Scan(&count); e != nil {
		stop("count_after_negative", e)
	}
	check(count == baseline+1, "negative_no_new_row")
	var unrelated int
	e = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM story_views)+(SELECT count(*) FROM story_reactions)+(SELECT count(*) FROM highlights)+(SELECT count(*) FROM highlight_stories)+(SELECT count(*) FROM story_media_deletion_outbox)`).Scan(&unrelated)
	if e != nil {
		stop("unrelated_counts", e)
	}
	check(unrelated == baselineUnrelated, "no_unrelated_rows")
	var after []byte
	e = pool.QueryRow(ctx, `SELECT row_to_json(s)::text FROM stories s WHERE id=$1`, id).Scan(&after)
	if e != nil {
		stop("row_retention", e)
	}
	check(len(bytes.TrimSpace(after)) > 0, "row_retained")
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"status": "PASS", "contract": "restored-data8-v1", "baseline_story_count": baseline, "new_story_id": id.String(), "create_grpc_code": "OK", "response_identity_type_verified": true, "sql_fields_verified": true, "story_count": count, "default_ttl_24h_verified": true, "archive_plus30d_verified": true, "missing_profile_code": "Unauthenticated", "negative_count_unchanged": true, "unrelated_row_count": unrelated, "row_retained": true})
}
