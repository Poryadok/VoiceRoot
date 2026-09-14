package grpcsvc_test

import (
	"context"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/pkg/integrationtest"
	grpcsvc "voice/backend/story/internal/grpcsvc"
	"voice/backend/story/internal/store"
	"voice/backend/story/internal/storyevents"

	storyv1 "voice.app/voice/story/v1"
)

type storyMediaEvents struct{ created, lfpCreated int }

func (e *storyMediaEvents) PublishStoryCreated(context.Context, string, string, string, string, []string) error {
	e.created++
	return nil
}
func (e *storyMediaEvents) PublishStoryViewed(context.Context, string, string) error { return nil }
func (e *storyMediaEvents) PublishStoryReacted(context.Context, string, string, string) error {
	return nil
}
func (e *storyMediaEvents) PublishStoryExpired(context.Context, string) error { return nil }
func (e *storyMediaEvents) PublishStoryHighlightCreated(context.Context, string, string) error {
	return nil
}
func (e *storyMediaEvents) PublishStoryLfpCreated(context.Context, string, string, string) error {
	e.lfpCreated++
	return nil
}
func (e *storyMediaEvents) PublishStoryLfpResponse(context.Context, string, string, string, string) error {
	return nil
}
func (e *storyMediaEvents) Close() error { return nil }

var _ storyevents.Publisher = (*storyMediaEvents)(nil)

func TestCreateStoryMediaRED_closedInputAndNoWriteOrEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	mediaID := uuid.NewString()
	photo := storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO
	video := storyv1.StoryMediaType_STORY_MEDIA_TYPE_VIDEO
	text := storyv1.StoryMediaType_STORY_MEDIA_TYPE_TEXT
	unknown := storyv1.StoryMediaType(99)
	unspecified := storyv1.StoryMediaType_STORY_MEDIA_TYPE_UNSPECIFIED
	calls := 0
	validator := mediaValidatorFunc(func(context.Context, uuid.UUID, uuid.UUID, storyv1.StoryMediaType) error { calls++; return nil })
	client, pool, events, author := startStoryMediaRED(t, validator)
	for _, tc := range []struct {
		name    string
		request *storyv1.CreateStoryRequest
		want    codes.Code
	}{
		{"unknown legacy type", &storyv1.CreateStoryRequest{Type: "clip"}, codes.InvalidArgument},
		{"legacy alias is not canonical", &storyv1.CreateStoryRequest{Type: "PHOTO", MediaFileId: &mediaID}, codes.InvalidArgument},
		{"unknown enum", &storyv1.CreateStoryRequest{TypeEnum: &unknown, MediaFileId: &mediaID}, codes.InvalidArgument},
		{"unspecified enum cannot fall back", &storyv1.CreateStoryRequest{Type: "photo", TypeEnum: &unspecified, MediaFileId: &mediaID}, codes.InvalidArgument},
		{"whitespace media", &storyv1.CreateStoryRequest{TypeEnum: &photo, MediaFileId: stringp(" \t ")}, codes.InvalidArgument},
		{"empty media", &storyv1.CreateStoryRequest{TypeEnum: &photo, MediaFileId: stringp("")}, codes.InvalidArgument},
		{"mismatched string and enum", &storyv1.CreateStoryRequest{Type: "video", TypeEnum: &photo, MediaFileId: &mediaID}, codes.InvalidArgument},
		{"photo needs media", &storyv1.CreateStoryRequest{TypeEnum: &photo}, codes.InvalidArgument},
		{"video needs media", &storyv1.CreateStoryRequest{TypeEnum: &video}, codes.InvalidArgument},
		{"text forbids media", &storyv1.CreateStoryRequest{TypeEnum: &text, TextContent: stringp("hello"), MediaFileId: &mediaID}, codes.InvalidArgument},
		{"media uuid is strict", &storyv1.CreateStoryRequest{TypeEnum: &photo, MediaFileId: stringp("not-a-uuid")}, codes.InvalidArgument},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.CreateStory(withProfile(context.Background(), uuid.New(), author), tc.request)
			assert.Equal(t, tc.want, status.Code(err))
			require.Equal(t, 0, storyMediaRowCount(t, pool, author))
			require.Zero(t, events.created)
			require.Zero(t, calls)
		})
	}
}

func TestCreateStoryMediaRED_unavailableAttestationDoesNotPersistOrPublish(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	client, pool, events, author := startStoryMediaRED(t)
	mediaID := uuid.NewString()
	_, err := client.CreateStory(withProfile(context.Background(), uuid.New(), author), &storyv1.CreateStoryRequest{Type: "photo", MediaFileId: &mediaID})
	require.Equal(t, codes.Unavailable, status.Code(err), "photo/video require File's protected attestation; an absent client is not an allow")
	require.Equal(t, 0, storyMediaRowCount(t, pool, author))
	require.Zero(t, events.created)
}

func TestCreateLookingForPartyMediaRED_temporaryNoMediaPolicyGate(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	calls := 0
	validator := mediaValidatorFunc(func(context.Context, uuid.UUID, uuid.UUID, storyv1.StoryMediaType) error { calls++; return nil })
	client, pool, events, author := startStoryMediaRED(t, validator)
	for _, mediaID := range []string{uuid.NewString(), "invalid-uuid", " \t "} {
		_, err := client.CreateLookingForParty(withProfile(context.Background(), uuid.New(), author), &storyv1.CreateLookingForPartyRequest{CriteriaJson: `{}`, MediaFileId: &mediaID})
		require.Equal(t, codes.FailedPrecondition, status.Code(err), "until product defines LFP media kind/duration, media must fail before File, DB or event")
		require.Equal(t, 0, storyMediaRowCount(t, pool, author))
		require.Zero(t, events.lfpCreated)
		require.Zero(t, calls)
	}
}

func startStoryMediaRED(t *testing.T, validators ...grpcsvc.FileMediaValidator) (storyv1.StoryServiceClient, *pgxpool.Pool, *storyMediaEvents, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "storymediared", "")
	_, err := pool.Exec(ctx, migrationSQL(t))
	require.NoError(t, err)
	events := &storyMediaEvents{}
	svc := grpcsvc.NewStoryGRPC(&store.StoryStore{Pool: pool})
	if len(validators) > 0 {
		svc.Files = validators[0]
	}
	svc.Events = events
	lis := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	storyv1.RegisterStoryServiceServer(server, svc)
	go func() { _ = server.Serve(lis) }()
	conn, err := grpc.NewClient("passthrough:///story-media-red", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { server.Stop(); _ = conn.Close(); _ = lis.Close() })
	return storyv1.NewStoryServiceClient(conn), pool, events, uuid.New()
}

func storyMediaRowCount(t *testing.T, pool *pgxpool.Pool, author uuid.UUID) int {
	t.Helper()
	var count int
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT count(*) FROM stories WHERE author_profile_id = $1", author).Scan(&count))
	return count
}
func stringp(value string) *string { return &value }

type mediaValidatorFunc func(context.Context, uuid.UUID, uuid.UUID, storyv1.StoryMediaType) error

func (f mediaValidatorFunc) ValidateStoryMedia(ctx context.Context, file, author uuid.UUID, expected storyv1.StoryMediaType) error {
	return f(ctx, file, author, expected)
}

func TestStoryMediaAttestationFailureDoesNotPersistOrPublish(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	for _, dependency := range []codes.Code{codes.InvalidArgument, codes.NotFound, codes.PermissionDenied, codes.FailedPrecondition, codes.Unauthenticated, codes.DeadlineExceeded, codes.Internal, codes.Unavailable, codes.Canceled, codes.Unknown} {
		t.Run(dependency.String(), func(t *testing.T) {
			calls := 0
			validator := mediaValidatorFunc(func(context.Context, uuid.UUID, uuid.UUID, storyv1.StoryMediaType) error {
				calls++
				return status.Error(dependency, "secret dependency details")
			})
			client, pool, events, author := startStoryMediaRED(t, validator)
			id := uuid.NewString()
			_, err := client.CreateStory(withProfile(context.Background(), uuid.New(), author), &storyv1.CreateStoryRequest{Type: "photo", MediaFileId: &id})
			want := codes.Unavailable
			switch dependency {
			case codes.InvalidArgument, codes.NotFound, codes.PermissionDenied, codes.FailedPrecondition:
				want = dependency
			}
			require.Equal(t, want, status.Code(err))
			require.NotContains(t, err.Error(), "secret")
			require.Equal(t, 1, calls)
			require.Zero(t, storyMediaRowCount(t, pool, author))
			require.Zero(t, events.created)
		})
	}
}

func TestStoryMediaAttestationSuccessUsesExactAuthorFileAndType(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	for _, expected := range []storyv1.StoryMediaType{storyv1.StoryMediaType_STORY_MEDIA_TYPE_PHOTO, storyv1.StoryMediaType_STORY_MEDIA_TYPE_VIDEO} {
		t.Run(expected.String(), func(t *testing.T) {
			fileID := uuid.New()
			var seenAuthor, seenFile uuid.UUID
			var seenType storyv1.StoryMediaType
			calls := 0
			validator := mediaValidatorFunc(func(_ context.Context, file, author uuid.UUID, typ storyv1.StoryMediaType) error {
				calls++
				seenFile, seenType = file, typ
				seenAuthor = author
				return nil
			})
			client, pool, events, author := startStoryMediaRED(t, validator)
			id := fileID.String()
			result, err := client.CreateStory(withProfile(context.Background(), uuid.New(), author), &storyv1.CreateStoryRequest{TypeEnum: &expected, MediaFileId: &id})
			require.NoError(t, err)
			require.Equal(t, id, result.GetStory().GetMediaFileId())
			require.Equal(t, author, seenAuthor)
			require.Equal(t, fileID, seenFile)
			require.Equal(t, expected, seenType)
			require.Equal(t, 1, calls)
			require.Equal(t, 1, storyMediaRowCount(t, pool, author))
			require.Equal(t, 1, events.created)
		})
	}
}

func TestTextAndMediaLessLFPDoNotCallFile(t *testing.T) {
	if testing.Short() {
		t.Skip("requires isolated PostgreSQL")
	}
	validator := mediaValidatorFunc(func(context.Context, uuid.UUID, uuid.UUID, storyv1.StoryMediaType) error {
		t.Error("unexpected File call")
		return status.Error(codes.Internal, "unexpected")
	})
	client, pool, events, author := startStoryMediaRED(t, validator)
	ctx := withProfile(context.Background(), uuid.New(), author)
	_, err := client.CreateStory(ctx, &storyv1.CreateStoryRequest{Type: "text", TextContent: stringp("hello")})
	require.NoError(t, err)
	_, err = client.CreateLookingForParty(ctx, &storyv1.CreateLookingForPartyRequest{CriteriaJson: `{}`})
	require.NoError(t, err)
	require.Equal(t, 2, storyMediaRowCount(t, pool, author))
	require.Equal(t, 1, events.created)
	require.Equal(t, 1, events.lfpCreated)
}
