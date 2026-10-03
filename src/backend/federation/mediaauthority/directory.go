package mediaauthority

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"
)

// RefreshDirectory consumes the controller's atomic files. Invalid files cannot
// replace authority; missing files never remove expiry or renew an old lease.
func RefreshDirectory(registry *Registry, directory string, now time.Time) {
	files, err := filepath.Glob(filepath.Join(directory, "*.json"))
	if err != nil || len(files) > 4096 {
		return
	}
	for _, name := range files {
		info, err := os.Lstat(name)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 16<<20 {
			continue
		}
		file, err := os.Open(name)
		if err != nil {
			continue
		}
		var bundle Bundle
		decoder := json.NewDecoder(io.LimitReader(file, (16<<20)+1))
		decoder.DisallowUnknownFields()
		err = decoder.Decode(&bundle)
		complete := decoder.Decode(new(any)) == io.EOF
		_ = file.Close()
		if err == nil && complete && filepath.Base(name) == bundle.Scope.SpaceID+".json" {
			_ = registry.Apply(bundle, now)
		}
	}
}
