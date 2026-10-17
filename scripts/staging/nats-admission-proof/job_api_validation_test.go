package admissionproof

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// The producer output is posted unchanged to a real kube-apiserver so built-in
// Job selector/template validation catches producer/API contract drift.
func TestProducerJobPassesKubernetesValidation(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	fixture := loadProducerFixture(t)
	data, err := json.Marshal(fixture.Job)
	if err != nil {
		t.Fatal(err)
	}
	var job batchv1.Job
	if err := json.Unmarshal(data, &job); err != nil {
		t.Fatal(err)
	}
	if _, err := client.BatchV1().Jobs(namespace).Create(ctx, &job, metav1.CreateOptions{}); err != nil {
		t.Fatalf("committed producer Job rejected by upstream Kubernetes v1.34.3: %v", err)
	}
}
