//go:build linux

// voice-party-fencer is an installer-launched finite one-shot process. It has
// no listener, arbitrary SQL/DSN/path flags, executor-controlled configuration
// or serving mode. Installation custody must supply its fixed protected mounts.
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"voice/backend/voice/internal/fencecustody"
)

func main() {
	operation := flag.String("operation", "", "fence, observe, read-receipt, or read-observation")
	id := flag.String("id", "", "installed command or proof UUID")
	hash := flag.String("hash", "", "expected domain-separated SHA256 hex")
	flag.Parse()
	key, err := uuid.Parse(*id)
	if err != nil || key == uuid.Nil || flag.NArg() != 0 {
		fail()
	}
	raw, err := hex.DecodeString(*hash)
	if err != nil || len(raw) != 32 {
		fail()
	}
	var digest [32]byte
	copy(digest[:], raw)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	var result any
	switch *operation {
	case "fence":
		result, err = fencecustody.FencePreparedPostgresRole(ctx, fencecustody.CommandRef{CommandID: key, Hash: digest})
	case "observe":
		result, err = fencecustody.ReobservePreparedPostgresFence(ctx, fencecustody.CommandRef{CommandID: key, Hash: digest})
	case "read-receipt":
		result, err = fencecustody.ReadPostgresFenceReceipt(ctx, fencecustody.ReceiptRef{ReceiptID: key, Hash: digest})
	case "read-observation":
		var observation fencecustody.PostgresFenceObservation
		var receipt fencecustody.PostgresFenceReceipt
		observation, receipt, err = fencecustody.ReadPostgresFenceObservation(ctx, fencecustody.ObservationRef{ObservationID: key, Hash: digest})
		result = struct {
			Observation fencecustody.PostgresFenceObservation
			Receipt     fencecustody.PostgresFenceReceipt
		}{observation, receipt}
	default:
		fail()
	}
	if err != nil || json.NewEncoder(os.Stdout).Encode(result) != nil {
		fail()
	}
}

func fail() {
	// Never expose driver/configuration errors or administrative credentials.
	fmt.Fprintln(os.Stderr, "fencer operation unavailable")
	os.Exit(1)
}
