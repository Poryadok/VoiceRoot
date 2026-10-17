package admissionproof

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const (
	testOperation = "0123456789ab"
	testNonce     = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testPodUID    = "c5c81fc7-86d6-4d62-8b84-9912d0805bf2"
	testImage     = "ghcr.io/poryadok/voice-nats-phase@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

type producerFixture struct {
	Job                map[string]interface{}            `json:"job"`
	CensusJob          map[string]interface{}            `json:"census_job"`
	RestoreJob         map[string]interface{}            `json:"restore_job"`
	BindJobs           map[string]map[string]interface{} `json:"bind_jobs"`
	ExecVectors        map[string][]string               `json:"exec_vectors"`
	CensusExecVectors  map[string][]string               `json:"census_exec_vectors"`
	RestoreExecVectors map[string][]string               `json:"restore_exec_vectors"`
	Binding            map[string]string                 `json:"binding"`
	CensusBinding      map[string]string                 `json:"census_binding"`
	RestoreBinding     map[string]string                 `json:"restore_binding"`
	PodUID             string                            `json:"pod_uid"`
}

func loadProducerFixture(t *testing.T) producerFixture {
	t.Helper()
	sourceDir := os.Getenv("NATS_VERIFICATION_JOB_SOURCE_DIR")
	if sourceDir == "" {
		workingDir, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		sourceDir = filepath.Join(workingDir, "testdata", "producer-95f3d4c49")
	}
	sourceBytes, err := os.ReadFile(filepath.Join(sourceDir, "verification_job.py"))
	if err != nil {
		t.Fatalf("read committed producer source: %v", err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(sourceBytes)); got != "186bfb42890f4f508af4134d3c0b5116732c037e4310bbce8c303109a5854df0" {
		t.Fatalf("producer source bytes do not match commit 95f3d4c49: sha256=%s", got)
	}
	program := `import json, verification_job as j
archive_request = {'operation': '0123456789ab', 'nonce': 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'phase': 'archive'}
census_request = {'operation': '0123456789ab', 'nonce': 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'phase': 'census'}
restore_request = {'operation': '0123456789ab', 'nonce': 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'phase': 'restore'}
archive_binding = {'source_claim':'voice-nats-jsdata-d20261017a','source_uid':'11111111-1111-4111-8111-111111111111','source_pv_name':'pv-source','source_pv_uid':'33333333-3333-4333-8333-333333333333','source_node_name':'k3s-worker-1'}
census_binding = {'source_claim':'voice-nats-jsdata-d20261017a','source_uid':'11111111-1111-4111-8111-111111111111','source_pv_name':'pv-source','source_pv_uid':'33333333-3333-4333-8333-333333333333','source_node_name':'k3s-worker-1','verification_claim':'voice-nats-verify-0123456789ab','verification_uid':'22222222-2222-4222-8222-222222222222','verification_pv_name':'pv-verification','verification_pv_uid':'44444444-4444-4444-8444-444444444444','verification_node_name':'k3s-worker-1'}
restore_binding = {'restore_claim':'voice-nats-restore-0123456789ab-deadbeef','restore_attempt':'deadbeef','restore_uid':'55555555-5555-4555-8555-555555555555','restore_pv_name':'pv-restore','restore_pv_uid':'66666666-6666-4666-8666-666666666666','restore_node_name':'k3s-worker-1'}
pod_uid = 'c5c81fc7-86d6-4d62-8b84-9912d0805bf2'
image = 'ghcr.io/poryadok/voice-nats-phase@sha256:' + 'a' * 64
archive = {'sha256': 'b' * 64, 'bytes': 4096}
bind_jobs = {}
for phase, role, claim in [('bind-verification','verification','voice-nats-verify-0123456789ab'),('bind-restore','restore','voice-nats-restore-0123456789ab-deadbeef')]:
    bind_request = {'operation': archive_request['operation'], 'nonce': archive_request['nonce'], 'phase': phase}
    bind_jobs[phase] = j.document(bind_request, {'role':role,'claim_name':claim,'target_uid':'55555555-5555-4555-8555-555555555555','node_name':'k3s-worker-1'}, image)
print(json.dumps({'job':j.document(archive_request,archive_binding,image),'census_job':j.document(census_request,census_binding,image),'restore_job':j.document(restore_request,restore_binding,image),'bind_jobs':bind_jobs,'exec_vectors':j.exec_vectors(archive_request,pod_uid,archive_binding),'census_exec_vectors':j.exec_vectors(census_request,pod_uid,census_binding,archive),'restore_exec_vectors':j.exec_vectors(restore_request,pod_uid,restore_binding,archive),'binding':archive_binding,'census_binding':census_binding,'restore_binding':restore_binding,'pod_uid':pod_uid}))`
	cmd := exec.Command("python3", "-c", program)
	cmd.Dir = sourceDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("run committed verification_job producer from %s: %v: %s", sourceDir, err, stderr.String())
	}
	var fixture producerFixture
	if err := json.Unmarshal(stdout.Bytes(), &fixture); err != nil {
		t.Fatalf("decode producer Job fixture: %v: %s", err, stdout.String())
	}
	if fixture.Job == nil || fixture.CensusJob == nil || fixture.RestoreJob == nil || len(fixture.BindJobs) != 2 || fixture.BindJobs["bind-verification"] == nil || fixture.BindJobs["bind-restore"] == nil || fixture.ExecVectors["archive_create_and_stream"] == nil || fixture.ExecVectors["archive_same_pod_readback"] == nil || len(fixture.CensusExecVectors) != 4 || len(fixture.RestoreExecVectors) != 2 {
		t.Fatalf("producer omitted Job or fixed exec vectors: %s", fmt.Sprintf("%s", stdout.String()))
	}
	return fixture
}

func TestFixtureConsumesCommittedProducer(t *testing.T) {
	fixture := loadProducerFixture(t)
	metadata := fixture.Job["metadata"].(map[string]interface{})
	if metadata["namespace"] != "voice-staging" {
		t.Fatalf("producer Job namespace = %v", metadata["namespace"])
	}
	if got := len(fixture.ExecVectors["archive_create_and_stream"]); got != 13 {
		t.Fatalf("producer archive-create vector has %d argv elements, want 13", got)
	}
	if got := len(fixture.ExecVectors["archive_same_pod_readback"]); got != 13 {
		t.Fatalf("producer archive-readback vector has %d argv elements, want 13", got)
	}
	if got := fixture.Job["kind"]; got != "Job" {
		t.Fatalf("producer output is not Job: %v", got)
	}
	if got := fixture.CensusJob["kind"]; got != "Job" {
		t.Fatalf("producer census output is not Job: %v", got)
	}
	if got := fixture.RestoreJob["kind"]; got != "Job" {
		t.Fatalf("producer restore output is not Job: %v", got)
	}
	if got := len(fixture.CensusExecVectors["census_receive_archive"]); got != 21 {
		t.Fatalf("producer census receive vector has %d argv elements, want 21", got)
	}
}
