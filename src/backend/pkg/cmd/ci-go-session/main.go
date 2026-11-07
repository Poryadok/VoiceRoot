// ci-go-session predicts the pinned testcontainers-go session for a parent.
// Prediction alone does not prove that child resources belong to a run.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/shirou/gopsutil/v3/process"
	_ "github.com/testcontainers/testcontainers-go"
)

type identity struct {
	ParentPID int32  `json:"parent_pid"`
	CreatedMS int64  `json:"created_ms"`
	Session   string `json:"session"`
}

func validateDependencies(info *debug.BuildInfo) error {
	want := map[string]string{
		"github.com/testcontainers/testcontainers-go": "v0.35.0",
		"github.com/shirou/gopsutil/v3":               "v3.23.12",
	}
	if info == nil {
		return fmt.Errorf("build dependency information unavailable")
	}
	for _, dep := range info.Deps {
		if version, ok := want[dep.Path]; ok {
			if dep.Version != version || dep.Replace != nil {
				return fmt.Errorf("unsupported dependency: %s", dep.Path)
			}
			delete(want, dep.Path)
		}
	}
	if len(want) != 0 {
		return fmt.Errorf("required dependency information missing")
	}
	return nil
}

func sessionForParent(pid int32) (identity, error) {
	if pid <= 0 {
		return identity{}, fmt.Errorf("parent PID must be positive")
	}
	p, err := process.NewProcess(pid)
	if err != nil {
		return identity{}, fmt.Errorf("inspect parent process: %w", err)
	}
	created, err := p.CreateTime()
	if err != nil || created <= 0 {
		return identity{}, fmt.Errorf("parent creation time unavailable")
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("testcontainers-go:%d:%d", pid, created)))
	return identity{ParentPID: pid, CreatedMS: created, Session: fmt.Sprintf("%x", sum)}, nil
}

func main() {
	pid := flag.Int64("pid", 0, "live parent process ID")
	flag.Parse()
	if flag.NArg() != 0 || *pid <= 0 || *pid > 1<<31-1 {
		fmt.Fprintln(os.Stderr, "a positive 32-bit --pid is required")
		os.Exit(2)
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Fprintln(os.Stderr, "build dependency information unavailable")
		os.Exit(1)
	}
	if err := validateDependencies(info); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	value, err := sessionForParent(int32(*pid))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(value); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
