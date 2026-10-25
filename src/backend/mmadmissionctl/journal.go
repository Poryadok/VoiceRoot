// Package mmadmissionctl implements the MM-only external admission journal.
package mmadmissionctl

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	_ "github.com/mattn/go-sqlite3"
	"math"
	"net/url"
	"os"
	w "voice/backend/pkg/mmadmissionwire"
)

var ErrConflict = errors.New("MM admission revision or operation conflict")
var ErrPending = errors.New("MM admission effect requires reconciliation")

type Journal struct {
	db   *sql.DB
	path string
}
type Intent struct {
	TargetOperationID w.ID
	OperationID       w.ID
	ExpectedRevision  uint64
	RequestHash       w.Hash
	Kind              string
}
type Attempt struct {
	Replay   bool
	Pending  bool
	Revision uint64
	Result   []byte
}

func CreateLocalJournal(path string) (*Journal, error) {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return nil, e
	}
	_ = f.Close()
	j, e := OpenJournal(path)
	if e != nil {
		return nil, e
	}
	_, e = j.db.Exec(`CREATE TABLE head(singleton INTEGER PRIMARY KEY CHECK(singleton=1),revision INTEGER NOT NULL,pending TEXT NOT NULL,snapshot BLOB NOT NULL);INSERT INTO head VALUES(1,0,'',X'');CREATE TABLE intents(operation_id TEXT PRIMARY KEY,expected_revision INTEGER NOT NULL,request_hash BLOB NOT NULL,kind TEXT NOT NULL);CREATE TABLE results(operation_id TEXT PRIMARY KEY REFERENCES intents(operation_id),revision INTEGER NOT NULL,result BLOB NOT NULL);CREATE TABLE effects(sequence INTEGER PRIMARY KEY,operation_id TEXT NOT NULL REFERENCES intents(operation_id),phase TEXT NOT NULL,payload BLOB NOT NULL);`)
	if e != nil {
		_ = j.Close()
		return nil, e
	}
	return j, nil
}
func OpenJournal(path string) (*Journal, error) {
	st, e := os.Stat(path)
	if e != nil || !st.Mode().IsRegular() {
		return nil, ErrPending
	}
	u := url.URL{Scheme: "file", Path: path}
	db, e := sql.Open("sqlite3", u.String()+"?mode=rw&_journal_mode=WAL&_synchronous=FULL&_foreign_keys=on&_busy_timeout=1000&_txlock=immediate")
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	if e = db.Ping(); e != nil {
		_ = db.Close()
		return nil, e
	}
	return &Journal{db: db, path: path}, nil
}
func (j *Journal) Close() error { return j.db.Close() }
func (j *Journal) Begin(ctx context.Context, in Intent) (Attempt, error) {
	if in.OperationID == (w.ID{}) || in.RequestHash == (w.Hash{}) || in.ExpectedRevision > math.MaxInt64-2 {
		return Attempt{}, ErrConflict
	}
	switch in.Kind {
	case "PREPARE", "CLOSE", "RECONCILE", "ARM", "OPEN":
	default:
		return Attempt{}, ErrConflict
	}
	tx, e := j.db.BeginTx(ctx, nil)
	if e != nil {
		return Attempt{}, e
	}
	defer tx.Rollback()
	var oldhash []byte
	var oldrev uint64
	var kind string
	e = tx.QueryRowContext(ctx, `SELECT expected_revision,request_hash,kind FROM intents WHERE operation_id=?`, in.OperationID.String()).Scan(&oldrev, &oldhash, &kind)
	if e == nil {
		if oldrev != in.ExpectedRevision || kind != in.Kind || !bytes.Equal(oldhash, in.RequestHash[:]) {
			return Attempt{}, ErrConflict
		}
		var a Attempt
		a.Replay = true
		e = tx.QueryRowContext(ctx, `SELECT revision,result FROM results WHERE operation_id=?`, in.OperationID.String()).Scan(&a.Revision, &a.Result)
		if errors.Is(e, sql.ErrNoRows) {
			a.Pending = true
			e = tx.QueryRowContext(ctx, `SELECT revision FROM head WHERE singleton=1`).Scan(&a.Revision)
		}
		return a, e
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return Attempt{}, e
	}
	var revision uint64
	var pending string
	if e = tx.QueryRowContext(ctx, `SELECT revision,pending FROM head WHERE singleton=1`).Scan(&revision, &pending); e != nil {
		return Attempt{}, e
	}
	if pending != "" && (in.Kind != "RECONCILE" || in.TargetOperationID.String() != pending) {
		return Attempt{}, ErrPending
	}
	if revision != in.ExpectedRevision {
		return Attempt{}, ErrConflict
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO intents(operation_id,expected_revision,request_hash,kind)VALUES(?,?,?,?)`, in.OperationID.String(), in.ExpectedRevision, in.RequestHash[:], in.Kind); e != nil {
		return Attempt{}, e
	}
	if in.TargetOperationID != (w.ID{}) {
		if in.Kind != "RECONCILE" {
			return Attempt{}, ErrConflict
		}
		var count int
		if e = tx.QueryRowContext(ctx, `SELECT count(*) FROM intents WHERE operation_id=? AND NOT EXISTS(SELECT 1 FROM results WHERE operation_id=?)`, in.TargetOperationID.String(), in.TargetOperationID.String()).Scan(&count); e != nil || count != 1 {
			return Attempt{}, ErrConflict
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO effects(operation_id,phase,payload)VALUES(?,'RECONCILE_TARGET',?)`, in.OperationID.String(), in.TargetOperationID[:]); e != nil {
			return Attempt{}, e
		}
	}
	if _, e = tx.ExecContext(ctx, `UPDATE head SET revision=revision+1,pending=? WHERE singleton=1`, in.OperationID.String()); e != nil {
		return Attempt{}, e
	}
	if e = tx.Commit(); e != nil {
		return Attempt{}, e
	}
	return Attempt{Revision: revision + 1, Pending: true}, nil
}

type Head struct {
	Revision uint64
	Pending  string
	Snapshot []byte
}

func (j *Journal) SetPendingSnapshot(ctx context.Context, op w.ID, snapshot []byte) error {
	if len(snapshot) > w.MaxFrame {
		return ErrConflict
	}
	r, e := j.db.ExecContext(ctx, `UPDATE head SET revision=revision+1,snapshot=? WHERE singleton=1 AND pending=? AND revision<9223372036854775807`, snapshot, op.String())
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n != 1 {
		return ErrConflict
	}
	return nil
}

func (j *Journal) Read(ctx context.Context) (Head, error) {
	var h Head
	e := j.db.QueryRowContext(ctx, `SELECT revision,pending,snapshot FROM head WHERE singleton=1`).Scan(&h.Revision, &h.Pending, &h.Snapshot)
	return h, e
}

// RecordEffect is append-only. Intent is durable before an external action;
// verified observations are durable afterwards. It never clears pending custody.
func (j *Journal) RecordEffect(ctx context.Context, op w.ID, phase string, payload []byte) error {
	if phase == "" || len(phase) > 64 || len(payload) > w.MaxFrame {
		return ErrConflict
	}
	tx, e := j.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var pending string
	if e = tx.QueryRowContext(ctx, `SELECT pending FROM head WHERE singleton=1`).Scan(&pending); e != nil {
		return e
	}
	if pending != op.String() {
		return ErrConflict
	}
	if _, e = tx.ExecContext(ctx, `INSERT INTO effects(operation_id,phase,payload)VALUES(?,?,?)`, op.String(), phase, payload); e != nil {
		return e
	}
	return tx.Commit()
}
func (j *Journal) Complete(ctx context.Context, op w.ID, snapshot, result []byte) (uint64, error) {
	if len(snapshot) > w.MaxFrame || len(result) > w.MaxFrame {
		return 0, ErrConflict
	}
	tx, e := j.db.BeginTx(ctx, nil)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback()
	var revision uint64
	var pending string
	if e = tx.QueryRowContext(ctx, `SELECT revision,pending FROM head WHERE singleton=1`).Scan(&revision, &pending); e != nil {
		return 0, e
	}
	if pending != op.String() {
		return 0, ErrConflict
	}
	if revision >= math.MaxInt64 {
		return 0, ErrConflict
	}
	revision++
	if _, e = tx.ExecContext(ctx, `INSERT INTO results(operation_id,revision,result)VALUES(?,?,?)`, op.String(), revision, result); e != nil {
		return 0, e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE head SET revision=?,pending='',snapshot=? WHERE singleton=1`, revision, snapshot); e != nil {
		return 0, e
	}
	if e = tx.Commit(); e != nil {
		return 0, e
	}
	return revision, nil
}
