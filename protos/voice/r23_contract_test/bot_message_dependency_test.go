package r23contracttest

import (
	"encoding/json"
	"os/exec"
	"testing"
)

// The send-only binding adds a file dependency, not an R23 lifecycle DTO or RPC.
// Removing exactly that import must recover the independently frozen projection.
func TestR23BotMessageDependencyOnlyExtension(t *testing.T) {
	baseline := map[string]string{
		"voice/bot/v1/bot.proto":             "81451ccaaffc39873ec7debc85530be9bfe5caed05f7cc08916cb070271d84c7",
		"voice/messaging/v1/messaging.proto": "fed44d432fd4760910f642e21f72ac00eb60b934898264e11460475d529778cd",
	}
	cmd := exec.Command("buf", "build", "protos", "--as-file-descriptor-set", "-o", "-#format=json")
	cmd.Dir = repoRoot(t)
	raw, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var descriptors descriptorSet
	if err := json.Unmarshal(raw, &descriptors); err != nil {
		t.Fatal(err)
	}
	actual := map[string]fileDescriptor{}
	for _, file := range descriptors.Files {
		actual[file.Name] = file
	}
	for _, spec := range loadManifest(t).Files {
		oldHash, selected := baseline[spec.Path]
		if !selected {
			continue
		}
		t.Run(spec.Path, func(t *testing.T) {
			// projectActual/fingerprint normalize slices in place. Independent deep
			// copies ensure source projection cannot silently mutate the oracle.
			clone := func() fileSpec {
				data, err := json.Marshal(spec)
				if err != nil {
					t.Fatal(err)
				}
				var copy fileSpec
				if err := json.Unmarshal(data, &copy); err != nil {
					t.Fatal(err)
				}
				return copy
			}
			for name, projection := range map[string]fileSpec{"manifest": clone(), "source": projectActual(clone(), actual[spec.Path])} {
				without := make([]string, 0, len(projection.Dependencies))
				found := 0
				for _, dependency := range projection.Dependencies {
					if dependency == "voice/common/v1/bot_message.proto" {
						found++
						continue
					}
					without = append(without, dependency)
				}
				if found != 1 {
					t.Errorf("%s must contain exactly one approved Bot message dependency, got %d", name, found)
				}
				projection.Dependencies = without
				if got := fingerprint(projection); got != oldHash {
					t.Errorf("%s selected R23 semantics changed: %s, want %s", name, got, oldHash)
				}
			}
		})
		delete(baseline, spec.Path)
	}
	if len(baseline) != 0 {
		t.Fatalf("missing selected contracts: %v", baseline)
	}
}
