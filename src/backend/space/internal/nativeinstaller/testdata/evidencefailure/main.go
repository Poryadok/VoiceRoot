// A test-only executable keeps real module build metadata while injecting an
// evidence-write failure into the production installer entrypoint.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"voice/backend/space/internal/nativeinstaller"
)

type failureWriter struct {
	at     string
	output io.Writer
}

func (w *failureWriter) Write(data []byte) (int, error) {
	var event struct {
		Event string `json:"event"`
	}
	if err := json.Unmarshal(data, &event); err != nil || event.Event == "" {
		return 0, errors.New("invalid installer event")
	}
	if event.Event == w.at {
		return 0, errors.New("controlled deployment evidence persistence failure")
	}
	return w.output.Write(data)
}

func main() {
	at := flag.String("at", "", "event at which evidence storage fails")
	driver := flag.String("migrate", "", "pinned migration driver")
	dir := flag.String("migrations-dir", "", "Space migration source")
	flag.Parse()
	valid := map[string]bool{"created": true, "migrated44": true, "trusted44": true,
		"migrated45": true, "trusted45": true, "migrated50": true,
		"trusted50": true, "trusted51": true, "complete": true}
	if flag.NArg() != 0 || !valid[*at] || *driver == "" || *dir == "" {
		fmt.Fprintln(os.Stderr, "invalid test fixture arguments")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	err := nativeinstaller.Run(ctx, nativeinstaller.Config{
		AdminURL: os.Getenv("SPACE_NATIVE_ADMIN_URL"), MigratePath: *driver,
		MigrationsDir: *dir, Mode: "provision", Output: &failureWriter{at: *at, output: os.Stdout},
	})
	if err == nil {
		fmt.Fprintln(os.Stderr, "expected evidence write failure")
	} else {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(1)
}
