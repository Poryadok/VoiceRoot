package main

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	voicejwt "voice/backend/pkg/jwt"
)

type wsSessionEpochPolicy struct {
	Strict              bool
	Floor               sessionEpochFloor
	Now                 func() time.Time
	CloseWriter         func(*websocket.Conn, int, string) error
	OnAuthorizedInbound func(string)
	BeforeWrite         func(string)
}

func (p wsSessionEpochPolicy) authorizeUpgrade(claims voicejwt.Claims) bool {
	return p.authorizeClaims(claims)
}

func (p wsSessionEpochPolicy) newConnectionGuard(conn *websocket.Conn, claims voicejwt.Claims, writeMu *sync.Mutex) *wsSessionEpochGuard {
	return &wsSessionEpochGuard{policy: p, conn: conn, claims: claims, writeMu: writeMu}
}

func (p wsSessionEpochPolicy) authorizeClaims(claims voicejwt.Claims) bool {
	if !p.Strict {
		return true
	}
	now := p.Now
	if now == nil {
		now = time.Now
	}
	if claims.UserID == "" || claims.SessionEpoch <= 0 || claims.ExpiresAt.IsZero() || !now().Before(claims.ExpiresAt) {
		return false
	}
	if p.Floor == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	minimum, err := p.Floor.Minimum(ctx, claims.UserID)
	return err == nil && minimum > 0 && claims.SessionEpoch >= minimum
}

type wsSessionEpochGuard struct {
	policy  wsSessionEpochPolicy
	conn    *websocket.Conn
	claims  voicejwt.Claims
	closed  sync.Once
	writeMu *sync.Mutex
}

func (g *wsSessionEpochGuard) authorizeInbound(op string) bool {
	if !g.authorize(false, op) {
		return false
	}
	if g.policy.OnAuthorizedInbound != nil {
		g.policy.OnAuthorizedInbound(op)
	}
	return true
}

func (g *wsSessionEpochGuard) authorizeWrite(op string) bool {
	return g.authorize(true, op)
}

func (g *wsSessionEpochGuard) authorizeSideEffect() bool {
	return g.authorize(false, "")
}

func (g *wsSessionEpochGuard) authorize(beforeWrite bool, op string) bool {
	if beforeWrite && g.policy.BeforeWrite != nil {
		g.policy.BeforeWrite(op)
	}
	if g.policy.authorizeClaims(g.claims) {
		return true
	}
	g.close()
	return false
}

// authorizeWriteLocked must be called while the connection's Gorilla write
// mutex is held. Keeping validation and the subsequent write in that same
// critical section prevents a revoked connection from racing a local frame.
func (g *wsSessionEpochGuard) authorizeWriteLocked(op string) bool {
	if g.policy.BeforeWrite != nil {
		g.policy.BeforeWrite(op)
	}
	if g.policy.authorizeClaims(g.claims) {
		return true
	}
	g.closeLocked()
	return false
}

func (g *wsSessionEpochGuard) close() {
	if g.writeMu != nil {
		g.writeMu.Lock()
		defer g.writeMu.Unlock()
	}
	g.closeLocked()
}

func (g *wsSessionEpochGuard) closeLocked() {
	g.closed.Do(func() {
		closeWriter := g.policy.CloseWriter
		if closeWriter == nil {
			closeWriter = writeSessionRevokedClose
		}
		_ = closeWriter(g.conn, websocket.ClosePolicyViolation, "session_revoked")
	})
}

func writeSessionRevokedClose(conn *websocket.Conn, code int, reason string) error {
	return conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), time.Now().Add(time.Second))
}
