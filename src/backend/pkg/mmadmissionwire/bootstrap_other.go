//go:build !linux

package mmadmissionwire

import (
	"context"
	"time"
)

func LoadNativeReadbackClient(context.Context, []byte, time.Duration) (Client, error) {
	return Client{}, ErrInvalid
}
