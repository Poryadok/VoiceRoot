//go:build !linux

package nativeapp

import (
	"context"
	"log/slog"
)

func Run(context.Context, *slog.Logger) error { return errUnavailable }
