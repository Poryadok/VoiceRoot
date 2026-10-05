//go:build !voice_compose_fcm_diagnostic

package main

import (
	"context"
	"voice/backend/notification/internal/chatmembers"
	"voice/backend/notification/internal/delivery"
)

type composeFcmTrace struct{}

func newComposeFcmObserver() *composeFcmObserver { return nil }

type composeFcmObserver struct{}

func (*composeFcmObserver) begin(string, string, string, string) *composeFcmTrace { return nil }
func (*composeFcmObserver) finish(*composeFcmTrace, string)                       {}
func (*composeFcmTrace) members([]chatmembers.Member, error)                      {}
func (*composeFcmTrace) baseFor(map[string]delivery.DeliveryDecision)             {}
func (*composeFcmTrace) finalFor(map[string]delivery.DeliveryDecision)            {}
func (*composeFcmTrace) context(ctx context.Context) context.Context              { return ctx }
