package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/file/internal/r2file"
)

const GIFProcessorVersion = "gif-mp4-v1"

var ErrGIFConfirmationConflict = errors.New("GIF confirmation hash conflicts with source")

// Representation describes delivered bytes independently of immutable upload facts.
type Representation struct {
	Key              string `json:"key"`
	MIMEType         string `json:"mime_type"`
	SizeBytes        int64  `json:"size_bytes"`
	SHA256           string `json:"sha256"`
	Filename         string `json:"filename"`
	ProcessorVersion string `json:"processor_version"`
}

func loadGIFRepresentations(ctx context.Context, q fileRowQueryer, row *FileRow) error {
	if row.UploadIntent != "gif" {
		return nil
	}
	var full, thumb []byte
	err := q.QueryRow(ctx, `SELECT full_descriptor,thumbnail_descriptor FROM file_gif_outputs WHERE file_id=$1`, row.ID).Scan(&full, &thumb)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	row.DeliveredFull, row.DeliveredThumbnail = &Representation{}, &Representation{}
	if err = json.Unmarshal(full, row.DeliveredFull); err != nil {
		return err
	}
	return json.Unmarshal(thumb, row.DeliveredThumbnail)
}

// ConfirmGIFUpload serializes confirmation with deletion and creates exactly one
// durable job. The caller has verified the source bytes against sha.
func (s *FilesStore) ConfirmGIFUpload(ctx context.Context, id uuid.UUID, sha string) (FileRow, error) {
	tx, err := BeginInventoryMutation(ctx, s.Pool, true)
	if err != nil {
		return FileRow{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	before, err := getFileByID(ctx, tx, id, " FOR NO KEY UPDATE")
	if err != nil {
		return FileRow{}, err
	}
	if before.UploadIntent != "gif" || before.IsE2E {
		return FileRow{}, ErrGIFConfirmationConflict
	}
	var catalog bool
	if err = tx.QueryRow(ctx, `SELECT COALESCE((to_jsonb(f)->>'catalog_asset')::boolean,false) FROM files f WHERE id=$1`, id).Scan(&catalog); err != nil {
		return FileRow{}, err
	}
	if catalog {
		return FileRow{}, ErrGIFConfirmationConflict
	}
	if before.Status != "pending_upload" {
		if before.SHA256Hash == nil || *before.SHA256Hash != sha {
			return FileRow{}, ErrGIFConfirmationConflict
		}
		return before, tx.Commit(ctx)
	}
	_, err = tx.Exec(ctx, `UPDATE files SET sha256_hash=$2,status='processing',scan_result='skipped',updated_at=clock_timestamp() WHERE id=$1`, id, sha)
	if err != nil {
		return FileRow{}, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO file_gif_jobs(file_id,processor_version) VALUES($1,$2)`, id, GIFProcessorVersion)
	if err != nil {
		return FileRow{}, err
	}
	after, err := getFileByID(ctx, tx, id, "")
	if err != nil {
		return FileRow{}, err
	}
	if err = recordPreviewChange(ctx, tx, before, after); err != nil {
		return FileRow{}, err
	}
	return after, tx.Commit(ctx)
}

type GIFJob struct {
	Row                   FileRow
	Token                 uuid.UUID
	FullKey, ThumbnailKey string
	TrackedWrites         bool
}

// ClaimGIFJob fences every attempt with a fresh UUID and records its output
// keys before the first object write. A codec/request is bounded to 120 seconds;
// its database lease is180 seconds. A tracked STARTED/UNSETTLED write remains
// pinned independently of these scheduling bounds; elapsed time is not fencing.
func (s *FilesStore) ClaimGIFJob(ctx context.Context) (*GIFJob, error) {
	tx, err := BeginInventoryMutation(ctx, s.Pool, true)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var id uuid.UUID
	err = tx.QueryRow(ctx, `SELECT f.id FROM files f JOIN file_gif_jobs j ON j.file_id=f.id
 WHERE f.status='processing' AND j.processor_version=$1 AND j.state IN ('queued','running')
 AND j.next_attempt_at<=clock_timestamp() AND (j.lease_until IS NULL OR j.lease_until<=clock_timestamp())
 ORDER BY j.next_attempt_at,f.id FOR NO KEY UPDATE OF f SKIP LOCKED LIMIT 1`, GIFProcessorVersion).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	row, err := getFileByID(ctx, tx, id, "")
	if err != nil {
		return nil, err
	}
	token := uuid.New()
	prefix := "processed/" + id.String() + "/gif/" + token.String()
	job := &GIFJob{Row: row, Token: token, FullKey: prefix + "/full.mp4", ThumbnailKey: prefix + "/thumb.webp"}
	job.TrackedWrites, err = gifTrackedModeTx(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE file_gif_jobs SET state='running',attempt=attempt+1,attempt_token=$2,
 lease_until=clock_timestamp()+interval '180 seconds',updated_at=clock_timestamp() WHERE file_id=$1`, id, token)
	if err != nil {
		return nil, err
	}
	for _, key := range []string{job.FullKey, job.ThumbnailKey} {
		if _, err = tx.Exec(ctx, `INSERT INTO file_gif_cleanup(object_key,file_id,attempt_token,not_before) VALUES($1,$2,$3,clock_timestamp()+interval '240 seconds')`, key, id, token); err != nil {
			return nil, err
		}
	}
	return job, tx.Commit(ctx)
}

// FinishGIFJob atomically publishes only the winning live attempt. A stale or
// deleted attempt leaves its declared keys eligible for durable cleanup.
func (s *FilesStore) FinishGIFJob(ctx context.Context, job *GIFJob, full, thumb *Representation, width, height, duration int32, failed bool) (bool, error) {
	tx, err := BeginInventoryMutation(ctx, s.Pool, true)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	before, err := getFileByID(ctx, tx, job.Row.ID, " FOR NO KEY UPDATE")
	if err != nil {
		return false, err
	}
	if before.Status != "processing" {
		return false, nil
	}
	var live bool
	err = tx.QueryRow(ctx, `SELECT state='running' AND attempt_token=$2 AND lease_until>clock_timestamp() FROM file_gif_jobs WHERE file_id=$1 FOR UPDATE`, before.ID, job.Token).Scan(&live)
	if err != nil {
		return false, err
	}
	if !live {
		return false, nil
	}
	nextStatus := "ready"
	jobStatus := "complete"
	if failed {
		nextStatus = "failed"
		jobStatus = "failed"
	} else {
		if !validGIFRepresentation(full, job.FullKey, "video/mp4") || !validGIFRepresentation(thumb, job.ThumbnailKey, "image/webp") || width <= 0 || height <= 0 || duration <= 0 {
			return false, fmt.Errorf("invalid GIF processing result")
		}
		if err = requireGIFTrackedResultsTx(ctx, tx, job, full, thumb); err != nil {
			if errors.Is(err, ErrGIFPublicationLost) {
				if _, retireErr := retireGIFPublicationTx(ctx, tx, before); retireErr != nil {
					return false, retireErr
				}
				if commitErr := tx.Commit(ctx); commitErr != nil {
					return false, commitErr
				}
			}
			return false, err
		}
		f, err := json.Marshal(full)
		if err != nil {
			return false, err
		}
		t, err := json.Marshal(thumb)
		if err != nil {
			return false, err
		}
		_, err = tx.Exec(ctx, `INSERT INTO file_gif_outputs(file_id,full_descriptor,thumbnail_descriptor,processor_version) VALUES($1,$2,$3,$4)`, before.ID, f, t, GIFProcessorVersion)
		if err != nil {
			return false, err
		}
		_, err = tx.Exec(ctx, `UPDATE files SET converted_r2_key=$2,thumbnail_r2_key=$3,file_type='video',width=$4,height=$5,duration_seconds=$6 WHERE id=$1`, before.ID, full.Key, thumb.Key, width, height, duration)
		if err != nil {
			return false, err
		}
		_, err = tx.Exec(ctx, `UPDATE file_gif_cleanup SET retained=true WHERE file_id=$1 AND attempt_token=$2`, before.ID, job.Token)
		if err != nil {
			return false, err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE files SET status=$2,updated_at=clock_timestamp() WHERE id=$1`, before.ID, nextStatus)
	if err != nil {
		return false, err
	}
	_, err = tx.Exec(ctx, `UPDATE file_gif_jobs SET state=$2,lease_until=NULL,updated_at=clock_timestamp() WHERE file_id=$1`, before.ID, jobStatus)
	if err != nil {
		return false, err
	}
	if err = queueGIFOriginalCleanupTx(ctx, tx, before); err != nil {
		return false, err
	}
	after, err := getFileByID(ctx, tx, before.ID, "")
	if err != nil {
		return false, err
	}
	if err = recordPreviewChange(ctx, tx, before, after); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func validGIFRepresentation(r *Representation, key, mime string) bool {
	if r == nil || r.Key != key || r.MIMEType != mime || r.SizeBytes <= 0 || r.SizeBytes > 5*1024*1024 || r.ProcessorVersion != GIFProcessorVersion || len(r.SHA256) != 64 || r.Filename == "" || strings.ContainsAny(r.Filename, "\r\n/\\") {
		return false
	}
	for _, c := range r.SHA256 {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// Terminal deletion and successful/failed processing share this authority. A
// signed PUT can finish after normal blob GC. recheck_until is a legacy original
// marker/scheduling hint, never proof that an accepted remote PUT has drained.
func queueGIFOriginalCleanupTx(ctx context.Context, tx pgx.Tx, row FileRow) error {
	if row.UploadIntent != "gif" {
		return nil
	}
	_, err := tx.Exec(ctx, `INSERT INTO file_gif_cleanup(object_key,file_id,recheck_until) VALUES($1,$2,$3) ON CONFLICT(object_key) DO NOTHING`, row.R2Key, row.ID, row.CreatedAt.Add(r2file.DefaultURLTTL+time.Minute))
	return err
}

func (s *FilesStore) RetryGIFJob(ctx context.Context, job *GIFJob) error {
	_, err := s.Pool.Exec(ctx, `UPDATE file_gif_jobs SET next_attempt_at=clock_timestamp()+interval '30 seconds',updated_at=clock_timestamp()
 WHERE file_id=$1 AND state='running' AND attempt_token=$2`, job.Row.ID, job.Token)
	return err
}

// CleanupGIFOnce retains active attempts and committed winner keys. It locks
// the file before checking the attempt, matching completion/deletion ordering.
func (s *FilesStore) CleanupGIFOnce(ctx context.Context, deleter r2file.ObjectDeleter) (bool, error) {
	if deleter == nil {
		return false, nil
	}
	tx, err := BeginInventoryMutation(ctx, s.Pool, true)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var id uuid.UUID
	var key string
	var publicOriginal bool
	ledger, err := trackedWriteLedgerExists(ctx, tx)
	if err != nil {
		return false, err
	}
	writeGuard := ""
	if ledger {
		writeGuard = ` AND NOT EXISTS(SELECT 1 FROM file_catalog_writes w WHERE (w.file_id=f.id OR w.object_key=c.object_key) AND w.state IN ('STARTED','UNSETTLED'))`
	}
	// Original rows have a trusted store-owned marker distinct from derivative
	// attempt rows. Recover old clock-completed public originals as well: neither
	// URL expiration nor a successful DELETE fences a previously accepted PUT.
	publicOriginalSQL := `(c.attempt_token IS NULL AND c.recheck_until IS NOT NULL AND NOT COALESCE((to_jsonb(f)->>'catalog_asset')::boolean,false))`
	err = tx.QueryRow(ctx, `SELECT f.id,c.object_key,`+publicOriginalSQL+` FROM file_gif_cleanup c JOIN files f ON f.id=c.file_id
 LEFT JOIN file_gif_jobs j ON j.file_id=f.id
 WHERE NOT c.retained AND (c.completed_at IS NULL OR `+publicOriginalSQL+`) AND c.not_before<=clock_timestamp()
	AND NOT EXISTS(SELECT 1 FROM files alias WHERE alias.id<>f.id AND alias.r2_key=c.object_key AND alias.status NOT IN ('deleted','expired','failed'))
 AND (c.attempt_token IS NULL OR j.attempt_token IS DISTINCT FROM c.attempt_token OR j.state<>'running' OR j.lease_until<=clock_timestamp())
 `+writeGuard+` ORDER BY c.not_before,c.object_key FOR NO KEY UPDATE OF f SKIP LOCKED LIMIT 1`).Scan(&id, &key, &publicOriginal)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// A shared original remains under the existing blob authority; do not infer
	// sole ownership from an uploader or from the processing job.
	var shared bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM files WHERE id<>$1 AND r2_key=$2 AND status NOT IN ('deleted','expired','failed'))`, id, key).Scan(&shared)
	if err != nil {
		return false, err
	}
	if shared {
		return false, nil
	}
	// Recheck after the File lock with a fresh snapshot. A Start that won the
	// lock before this cleanup may have submitted a PUT since candidate selection.
	var blob uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT b.blob_id FROM files f JOIN file_blobs b ON b.blob_id=f.blob_id WHERE f.id=$1 FOR UPDATE OF b`, id).Scan(&blob); err != nil {
		return false, err
	}
	collect, err := CanCollectTrackedObjectTx(ctx, tx, id, key)
	if err != nil || !collect {
		return false, err
	}
	deleteCtx, cancel := context.WithTimeout(guardedInventoryDeleteContext(ctx), 5*time.Second)
	defer cancel()
	if err = deleter.DeleteObject(deleteCtx, key); err != nil {
		return false, err
	}
	// Public upload clients have no trusted storage completion/fence channel.
	// Keep their original discoverable after URL expiry and repeat deletion at
	// the existing cadence; do not claim finite erasure from an age or HEAD check.
	// Exact catalog/derivative custody still passes the physical guards above.
	_, err = tx.Exec(ctx, `UPDATE file_gif_cleanup SET completed_at=CASE WHEN $2 THEN NULL ELSE clock_timestamp() END,not_before=clock_timestamp()+interval '1 minute' WHERE object_key=$1`, key, publicOriginal)
	if err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}
