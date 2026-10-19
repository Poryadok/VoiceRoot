package threadlist

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type nodeDB interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// PostgresNodes uses a pgx transaction during projection, so node inserts and
// publication of both roots roll back together with the message journal.
// A pool can be supplied for reading an already committed immutable snapshot.
type PostgresNodes struct{ DB nodeDB }

func (s PostgresNodes) Get(ctx context.Context, id NodeID) (Node, error) {
	if s.DB == nil || id == (NodeID{}) {
		return Node{}, ErrInvalidNode
	}
	var data []byte
	if err := s.DB.QueryRow(ctx, "SELECT payload FROM thread_list_nodes WHERE node_id=$1", id[:]).Scan(&data); err != nil {
		return Node{}, err
	}
	var n Node
	if err := json.Unmarshal(data, &n); err != nil {
		return Node{}, fmt.Errorf("thread index node decode: %w", err)
	}
	canonical, err := json.Marshal(n)
	if err != nil {
		return Node{}, err
	}
	if !validNode(n) || !bytes.Equal(data, canonical) || n.ID() != id {
		return Node{}, ErrInvalidNode
	}
	return n, nil
}

func validNode(n Node) bool {
	return (n.Order == ActivityOrder || n.Order == ParentOrder) && n.Height > 0 && n.Size > 0 && validEntry(n.Entry) && n.Entry.LastReplyAt.Location() == time.UTC && n.Entry.LastReplyAt.Equal(time.UnixMicro(n.Entry.LastReplyAt.UnixMicro()).UTC())
}

func (s PostgresNodes) Put(ctx context.Context, n Node) (NodeID, error) {
	if s.DB == nil || !validNode(n) {
		return NodeID{}, ErrInvalidNode
	}
	data, err := json.Marshal(n)
	if err != nil {
		return NodeID{}, err
	}
	id := n.ID()
	result, err := s.DB.Exec(ctx, "INSERT INTO thread_list_nodes(node_id,payload) VALUES($1,$2) ON CONFLICT(node_id) DO NOTHING", id[:], data)
	if err != nil {
		return NodeID{}, err
	}
	if result.RowsAffected() == 0 {
		// Do not silently treat a corrupted row/hash conflict as a valid retry.
		old, err := s.Get(ctx, id)
		if err != nil {
			return NodeID{}, err
		}
		previous, err := json.Marshal(old)
		if err != nil {
			return NodeID{}, err
		}
		if !bytes.Equal(previous, data) {
			return NodeID{}, ErrInvalidNode
		}
	}
	return id, nil
}
