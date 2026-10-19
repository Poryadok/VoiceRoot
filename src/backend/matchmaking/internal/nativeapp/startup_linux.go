//go:build linux

package nativeapp

import (
	"context"
	"crypto/sha256"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"os"
	"slices"
	"syscall"
	"time"
	w "voice/backend/pkg/mmadmissionwire"
)

type closedBootstrap struct {
	config nativeConfig
	client w.Client
	pool   *pgxpool.Pool
	stamp  w.AdmissionStampV1
	launch w.Hash
}

// readNativeDescriptor is deliberately fixed-path and Linux-only. The private
// FD loader independently binds these exact bytes to the admitted launch.
func readNativeDescriptor() ([]byte, error) {
	return readProtectedDescriptor(w.DescriptorPath)
}

func readProtectedDescriptor(path string) ([]byte, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, errUnavailable
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > w.MaxFrame || info.Mode().Perm()&0022 != 0 {
		return nil, errUnavailable
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 || stat.Nlink != 1 {
		return nil, errUnavailable
	}
	var fs syscall.Statfs_t
	if syscall.Fstatfs(fd, &fs) != nil || fs.Flags&syscall.MS_RDONLY == 0 {
		return nil, errUnavailable
	}
	raw, err := io.ReadAll(io.LimitReader(file, w.MaxFrame+1))
	if err != nil || len(raw) == 0 || len(raw) > w.MaxFrame {
		return nil, errUnavailable
	}
	return raw, nil
}

func checkNativeExecutable(descriptor w.DescriptorV1, argv []string) error {
	if len(argv) != 1 || !slices.Equal(argv, descriptor.AdmittedArgv) || argv[0] != "/service-native" {
		return errUnavailable
	}
	file, err := os.Open("/proc/self/exe")
	if err != nil {
		return errUnavailable
	}
	defer file.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return errUnavailable
	}
	var actual w.Hash
	copy(actual[:], hash.Sum(nil))
	if actual != descriptor.AdmittedExecutableHash {
		return errUnavailable
	}
	return nil
}

// openClosedBootstrap performs no listener, worker, broker delivery or domain
// mutation. ARMED permits only this finite bootstrap; every forward action
// still requires a new OPEN observation and its FIRST local SQL admission.
func openClosedBootstrap(ctx context.Context, cfg nativeConfig) (*closedBootstrap, error) {
	if ctx == nil || ctx.Err() != nil || checkCodeContracts(cfg) != nil || checkNativeExecutable(cfg.descriptor, os.Args) != nil {
		return nil, errUnavailable
	}
	client, err := w.LoadNativeReadbackClient(ctx, cfg.rawDescriptor, 2*time.Second)
	if err != nil {
		return nil, errUnavailable
	}
	startupCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	var observation w.StartupObservation
	for {
		observation, err = client.ObserveStartup(startupCtx)
		if err == nil {
			break
		}
		select {
		case <-startupCtx.Done():
			return nil, errUnavailable
		case <-time.After(100 * time.Millisecond):
		}
	}
	if observation.Binding() != cfg.descriptor.Binding || observation.Stamp().Validate() != nil || observation.LaunchObservationHash() == (w.Hash{}) {
		return nil, errUnavailable
	}
	config, err := pgxpool.ParseConfig(cfg.values["DATABASE_URL"])
	if err != nil {
		return nil, errUnavailable
	}
	config.MinConns = 0
	config.MaxConns = 8
	config.ConnConfig.ConnectTimeout = 3 * time.Second
	pool, err := pgxpool.NewWithConfig(startupCtx, config)
	if err != nil {
		return nil, errUnavailable
	}
	if checkNativeDatabase(startupCtx, pool, cfg.descriptor, observation.Stamp()) != nil {
		pool.Close()
		return nil, errUnavailable
	}
	return &closedBootstrap{config: cfg, client: client, pool: pool, stamp: observation.Stamp(), launch: observation.LaunchObservationHash()}, nil
}

func (b *closedBootstrap) Close() {
	if b != nil && b.pool != nil {
		b.pool.Close()
	}
}
