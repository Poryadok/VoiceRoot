package admissionproof

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestProducer3e8CensusConnectVectorsReachUpstreamAdmission(t *testing.T) {
	assets := os.Getenv("KUBEBUILDER_ASSETS")
	if assets == "" {
		t.Fatal("KUBEBUILDER_ASSETS must point at the pinned Kubernetes v1.34.3 envtest binaries")
	}
	testEnv := &envtest.Environment{BinaryAssetsDirectory: assets,
		ControlPlane: envtest.ControlPlane{APIServer: &envtest.APIServer{Args: []string{
			"--advertise-address=127.0.0.1", "--enable-admission-plugins=ValidatingAdmissionPolicy",
		}}}, ControlPlaneStartTimeout: 2 * time.Minute, ControlPlaneStopTimeout: 30 * time.Second}
	cfg, err := testEnv.Start()
	if err != nil {
		t.Fatalf("start upstream Kubernetes API server: %v", err)
	}
	t.Cleanup(func() {
		if err := testEnv.Stop(); err != nil {
			t.Errorf("stop upstream Kubernetes API server: %v", err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = admin.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace,
		Labels: map[string]string{"kubernetes.io/metadata.name": namespace}}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	createConnectRoleFixture(ctx, t, admin)
	fixture := loadProducer3e8CensusFixture(t)
	archiveJobName := fixture.Job["metadata"].(map[string]interface{})["name"].(string)
	archivePodName := archiveJobName + "-proof"
	censusJobName := fixture.CensusJob["metadata"].(map[string]interface{})["name"].(string)
	censusPodName := censusJobName + "-proof"
	kubelet := startFakeKubelet(t)
	createFakeNode(ctx, t, admin, kubelet)
	createProducerPodFixture(ctx, t, admin, fixture.Job, archivePodName)
	createProducerPodFixture(ctx, t, admin, fixture.CensusJob, censusPodName)
	installProducerConnectPolicy(ctx, t, admin, fixture, map[string]string{"archive": archivePodName, "census": censusPodName})

	request := func(vector string, stdin bool) execRequest {
		return execRequest{method: http.MethodPost, namespace: namespace, name: censusPodName, username: runner,
			container: "nats-phase", stdin: stdin, stdout: true, command: fixture.CensusExecVectors[vector]}
	}
	for _, tc := range []struct {
		name   string
		vector string
		stdin  bool
	}{
		{"load inputs", "census_load_inputs", true},
		{"read input receipt", "census_read_input_receipt", false},
		{"consume inputs", "census_consume_inputs", false},
		{"receive archive", "census_receive_archive", true},
		{"same Pod readback", "census_same_pod_readback", false},
	} {
		t.Run("allow "+tc.name, func(t *testing.T) {
			assertProducerConnectAllowed(t, cfg, request(tc.vector, tc.stdin))
		})
	}
	consume := request("census_consume_inputs", false)
	for _, tc := range []struct {
		name string
		edit func(*execRequest)
	}{
		{"shell command", func(r *execRequest) { r.command = []string{"/bin/sh", "-c", "id"} }},
		{"unexpected stdin", func(r *execRequest) { r.stdin = true }},
		{"operation drift", func(r *execRequest) { r.command[4] = strings.Repeat("f", 12) }},
		{"wrong container", func(r *execRequest) { r.container = "nats" }},
		{"stderr enabled", func(r *execRequest) { r.stderr = true }},
		{"wrong actor", func(r *execRequest) { r.username = "system:serviceaccount:voice-staging:default" }},
		{"wrong target Pod", func(r *execRequest) { r.name = archivePodName }},
	} {
		t.Run("deny consume inputs "+tc.name, func(t *testing.T) {
			candidate := consume.clone()
			tc.edit(&candidate)
			assertDeniedByMessage(t, cfg, candidate, producerConnectDenyMessage)
		})
	}
}

func loadProducer3e8CensusFixture(t *testing.T) producerFixture {
	t.Helper()
	workingDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	sourceDir := filepath.Join(workingDir, "testdata", "producer-3e8bae3")
	sourceBytes, err := os.ReadFile(filepath.Join(sourceDir, "verification_job.py"))
	if err != nil {
		t.Fatalf("read immutable producer 3e8bae3 source: %v", err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(sourceBytes)); got != "2544a7e0e0fa3433d007cc794b4f636cb2ffd3708bbc447ea1e89f30d0dc21bd" {
		t.Fatalf("producer source bytes do not match 3e8bae3: sha256=%s", got)
	}
	program := `import json, verification_job as j
archive_request = {'operation':'0123456789ab','nonce':'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','phase':'archive'}
census_request = {'operation':'0123456789ab','nonce':'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','phase':'census'}
archive_binding = {'source_claim':'voice-nats-jsdata-d20261017a','source_uid':'11111111-1111-4111-8111-111111111111','source_pv_name':'pv-source','source_pv_uid':'33333333-3333-4333-8333-333333333333','source_node_name':'k3s-node-actual-1','source_hostname':'k3s-host-label-1'}
census_binding = {'source_claim':'voice-nats-jsdata-d20261017a','source_uid':'11111111-1111-4111-8111-111111111111','source_pv_name':'pv-source','source_pv_uid':'33333333-3333-4333-8333-333333333333','source_node_name':'k3s-node-actual-1','source_hostname':'k3s-host-label-1','verification_claim':'voice-nats-verify-0123456789ab','verification_uid':'22222222-2222-4222-8222-222222222222','verification_pv_name':'pv-verification','verification_pv_uid':'44444444-4444-4444-8444-444444444444','verification_node_name':'k3s-node-actual-1','verification_hostname':'k3s-host-label-1'}
pod_uid = 'c5c81fc7-86d6-4d62-8b84-9912d0805bf2'
input_pod_uid = '77777777-7777-4777-8777-777777777777'
image = 'ghcr.io/poryadok/voice-nats-phase@sha256:' + 'a' * 64
archive = {'sha256':'b' * 64,'bytes':4096}
session_inputs = {'attempt':'deadbeef','sha256':'c' * 64,'bytes':1024}
vectors = j.exec_vectors(census_request,pod_uid,census_binding,archive,session_inputs=session_inputs,input_pod_uid=input_pod_uid)
print(json.dumps({'job':j.document(archive_request,archive_binding,image),'census_job':j.document(census_request,census_binding,image),'exec_vectors':j.exec_vectors(archive_request,pod_uid,archive_binding,archive),'census_exec_vectors':vectors,'pod_uid':pod_uid}))`
	cmd := exec.Command("python3", "-c", program)
	cmd.Dir = sourceDir
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("run committed producer 3e8bae3: %v: %s", err, stderr.String())
	}
	var fixture producerFixture
	if err := json.Unmarshal([]byte(stdout.String()), &fixture); err != nil {
		t.Fatalf("decode current census producer output: %v", err)
	}
	if fixture.Job == nil || fixture.CensusJob == nil || len(fixture.CensusExecVectors) != 5 {
		t.Fatalf("producer 3e8bae3 omitted expected Job or census vectors: %s", stdout.String())
	}
	return fixture
}
