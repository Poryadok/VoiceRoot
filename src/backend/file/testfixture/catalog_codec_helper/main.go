// Command catalog_codec_helper is an engineering-only isolated native codec
// runner. It has no service listener, provider adapter or production config.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"voice/backend/file/internal/gifproc"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	body, err := io.ReadAll(io.LimitReader(os.Stdin, (50<<20)+1))
	if err != nil || len(body) > 50<<20 {
		fmt.Fprintln(os.Stderr, "invalid codec input")
		os.Exit(1)
	}
	out, err := gifproc.Convert(ctx, body)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err = json.NewEncoder(os.Stdout).Encode(out); err != nil {
		fmt.Fprintln(os.Stderr, "codec output failed")
		os.Exit(1)
	}
}
