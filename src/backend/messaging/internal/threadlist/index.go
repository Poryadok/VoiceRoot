// Package threadlist implements the Messaging-owned immutable thread index.
// It is a storage primitive, not an authorization decision. Only the viewer
// projector may publish roots after the independent membership and message
// journal barriers have been satisfied.
package threadlist

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrInvalidEntry    = errors.New("invalid thread index entry")
	ErrInvalidPageSize = errors.New("invalid thread index page size")
	ErrInvalidNode     = errors.New("invalid thread index node")
)

// Entry is an already projected per-viewer summary. The projector, not this
// index, computes visibility and the exact reply count/latest reply/preview.
type Entry struct {
	ParentID    uuid.UUID
	LastReplyID uuid.UUID
	LastReplyAt time.Time
	ReplyCount  uint64
	Preview     string
}

type NodeID [32]byte
type Order uint8

const (
	ActivityOrder Order = 1
	ParentOrder   Order = 2
)

// Node is immutable. The content-addressed identity includes order, summary,
// children and balance metadata, so a persisted node cannot change in place.
type Node struct {
	Order  Order
	Entry  Entry
	Left   NodeID
	Right  NodeID
	Height int
	Size   int
}

func (n Node) ID() NodeID {
	b, err := json.Marshal(n)
	if err != nil {
		return NodeID{}
	}
	return sha256.Sum256(b)
}

func validEntry(e Entry) bool {
	year := e.LastReplyAt.UTC().Year()
	return e.ParentID != uuid.Nil && e.LastReplyID != uuid.Nil && !e.LastReplyAt.IsZero() && year >= 0 && year <= 9999 && e.ReplyCount > 0 && utf8.ValidString(e.Preview)
}

// NodeStore must belong to the same local transaction as root publication.
// Put is insert-only and idempotent for identical content; Get must fail for
// missing/corrupt nodes. No method queries the messages aggregate.
type NodeStore interface {
	Get(context.Context, NodeID) (Node, error)
	Put(context.Context, Node) (NodeID, error)
}

type Roots struct{ Activity, Parent NodeID }
type Index struct {
	Store NodeStore
	Roots Roots
}

func (idx Index) Upsert(ctx context.Context, e Entry) (Index, error) {
	if idx.Store == nil {
		return Index{}, ErrInvalidNode
	}
	if !validEntry(e) {
		return Index{}, ErrInvalidEntry
	}
	// PostgreSQL precision is microseconds. Canonicalize before both key
	// comparisons and content hashing (also strips Go's monotonic clock).
	e.LastReplyAt = time.UnixMicro(e.LastReplyAt.UnixMicro()).UTC()
	if !validEntry(e) {
		return Index{}, ErrInvalidEntry
	}
	a, p := avl{idx.Store, ActivityOrder}, avl{idx.Store, ParentOrder}
	old, found, err := p.find(ctx, idx.Roots.Parent, e)
	if err != nil {
		return Index{}, err
	}
	if found && old == e {
		return idx, nil
	}
	activity := idx.Roots.Activity
	if found {
		activity, err = a.remove(ctx, activity, old)
		if err != nil {
			return Index{}, err
		}
	}
	activity, err = a.insert(ctx, activity, e)
	if err != nil {
		return Index{}, err
	}
	parent, err := p.insert(ctx, idx.Roots.Parent, e)
	if err != nil {
		return Index{}, err
	}
	return Index{idx.Store, Roots{activity, parent}}, nil
}

func (idx Index) Delete(ctx context.Context, id uuid.UUID) (Index, error) {
	p := avl{idx.Store, ParentOrder}
	old, found, err := p.find(ctx, idx.Roots.Parent, Entry{ParentID: id})
	if err != nil {
		return Index{}, err
	}
	if !found {
		return idx, nil
	}
	activity, err := (avl{idx.Store, ActivityOrder}).remove(ctx, idx.Roots.Activity, old)
	if err != nil {
		return Index{}, err
	}
	parent, err := p.remove(ctx, idx.Roots.Parent, old)
	if err != nil {
		return Index{}, err
	}
	return Index{idx.Store, Roots{activity, parent}}, nil
}

// Page reads only O(height + limit) nodes from the immutable activity root.
// A cursor keeps its remaining-set roots; after emitting a page its owning
// transaction removes those parents using Delete. Never filter SQL candidates.
func (idx Index) Page(ctx context.Context, limit int) ([]Entry, error) {
	if limit < 1 || limit > 1000 {
		return nil, ErrInvalidPageSize
	}
	a := avl{idx.Store, ActivityOrder}
	stack := []Node{}
	id := idx.Roots.Activity
	result := make([]Entry, 0, limit)
	for id != (NodeID{}) || len(stack) > 0 {
		for id != (NodeID{}) {
			n, err := a.get(ctx, id)
			if err != nil {
				return nil, err
			}
			stack = append(stack, n)
			id = n.Left
		}
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		result = append(result, n.Entry)
		if len(result) == limit {
			break
		}
		id = n.Right
	}
	return result, nil
}

type avl struct {
	store NodeStore
	order Order
}

func compare(a, b Entry, order Order) int {
	if order == ActivityOrder {
		if a.LastReplyAt.Before(b.LastReplyAt) {
			return 1
		}
		if a.LastReplyAt.After(b.LastReplyAt) {
			return -1
		}
		return -bytes.Compare(a.ParentID[:], b.ParentID[:])
	}
	return bytes.Compare(a.ParentID[:], b.ParentID[:])
}

func (a avl) get(ctx context.Context, id NodeID) (Node, error) {
	if a.store == nil {
		return Node{}, ErrInvalidNode
	}
	n, err := a.store.Get(ctx, id)
	if err != nil {
		return Node{}, err
	}
	if !validNode(n) || n.Order != a.order || n.ID() != id {
		return Node{}, ErrInvalidNode
	}
	return n, nil
}

func (a avl) find(ctx context.Context, id NodeID, e Entry) (Entry, bool, error) {
	for id != (NodeID{}) {
		n, err := a.get(ctx, id)
		if err != nil {
			return Entry{}, false, err
		}
		c := compare(e, n.Entry, a.order)
		if c == 0 {
			return n.Entry, true, nil
		}
		if c < 0 {
			id = n.Left
		} else {
			id = n.Right
		}
	}
	return Entry{}, false, nil
}

func (a avl) metadata(ctx context.Context, id NodeID) (int, int, error) {
	if id == (NodeID{}) {
		return 0, 0, nil
	}
	n, err := a.get(ctx, id)
	return n.Height, n.Size, err
}

func (a avl) put(ctx context.Context, n Node) (NodeID, error) {
	lh, ls, err := a.metadata(ctx, n.Left)
	if err != nil {
		return NodeID{}, err
	}
	rh, rs, err := a.metadata(ctx, n.Right)
	if err != nil {
		return NodeID{}, err
	}
	n.Order = a.order
	n.Height = 1 + max(lh, rh)
	n.Size = 1 + ls + rs
	return a.store.Put(ctx, n)
}

func (a avl) balance(ctx context.Context, n Node) (NodeID, error) {
	lh, _, err := a.metadata(ctx, n.Left)
	if err != nil {
		return NodeID{}, err
	}
	rh, _, err := a.metadata(ctx, n.Right)
	if err != nil {
		return NodeID{}, err
	}
	if lh-rh > 1 {
		l, err := a.get(ctx, n.Left)
		if err != nil {
			return NodeID{}, err
		}
		ll, _, err := a.metadata(ctx, l.Left)
		if err != nil {
			return NodeID{}, err
		}
		lr, _, err := a.metadata(ctx, l.Right)
		if err != nil {
			return NodeID{}, err
		}
		if lr > ll {
			n.Left, err = a.rotateLeft(ctx, l)
			if err != nil {
				return NodeID{}, err
			}
		}
		return a.rotateRight(ctx, n)
	}
	if rh-lh > 1 {
		r, err := a.get(ctx, n.Right)
		if err != nil {
			return NodeID{}, err
		}
		rl, _, err := a.metadata(ctx, r.Left)
		if err != nil {
			return NodeID{}, err
		}
		rr, _, err := a.metadata(ctx, r.Right)
		if err != nil {
			return NodeID{}, err
		}
		if rl > rr {
			n.Right, err = a.rotateRight(ctx, r)
			if err != nil {
				return NodeID{}, err
			}
		}
		return a.rotateLeft(ctx, n)
	}
	return a.put(ctx, n)
}

func (a avl) rotateLeft(ctx context.Context, n Node) (NodeID, error) {
	r, err := a.get(ctx, n.Right)
	if err != nil {
		return NodeID{}, err
	}
	n.Right = r.Left
	r.Left, err = a.put(ctx, n)
	if err != nil {
		return NodeID{}, err
	}
	return a.put(ctx, r)
}
func (a avl) rotateRight(ctx context.Context, n Node) (NodeID, error) {
	l, err := a.get(ctx, n.Left)
	if err != nil {
		return NodeID{}, err
	}
	n.Left = l.Right
	l.Right, err = a.put(ctx, n)
	if err != nil {
		return NodeID{}, err
	}
	return a.put(ctx, l)
}

func (a avl) insert(ctx context.Context, id NodeID, e Entry) (NodeID, error) {
	if id == (NodeID{}) {
		return a.put(ctx, Node{Entry: e})
	}
	n, err := a.get(ctx, id)
	if err != nil {
		return NodeID{}, err
	}
	c := compare(e, n.Entry, a.order)
	if c < 0 {
		n.Left, err = a.insert(ctx, n.Left, e)
	} else if c > 0 {
		n.Right, err = a.insert(ctx, n.Right, e)
	} else {
		n.Entry = e
	}
	if err != nil {
		return NodeID{}, err
	}
	return a.balance(ctx, n)
}

func (a avl) remove(ctx context.Context, id NodeID, e Entry) (NodeID, error) {
	if id == (NodeID{}) {
		return id, nil
	}
	n, err := a.get(ctx, id)
	if err != nil {
		return NodeID{}, err
	}
	c := compare(e, n.Entry, a.order)
	if c < 0 {
		n.Left, err = a.remove(ctx, n.Left, e)
	} else if c > 0 {
		n.Right, err = a.remove(ctx, n.Right, e)
	} else {
		if n.Left == (NodeID{}) {
			return n.Right, nil
		}
		if n.Right == (NodeID{}) {
			return n.Left, nil
		}
		next, loadErr := a.get(ctx, n.Right)
		if loadErr != nil {
			return NodeID{}, loadErr
		}
		for next.Left != (NodeID{}) {
			next, loadErr = a.get(ctx, next.Left)
			if loadErr != nil {
				return NodeID{}, loadErr
			}
		}
		n.Entry = next.Entry
		n.Right, err = a.remove(ctx, n.Right, next.Entry)
	}
	if err != nil {
		return NodeID{}, err
	}
	return a.balance(ctx, n)
}
