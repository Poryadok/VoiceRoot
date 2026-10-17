package admissionproof

import (
	"context"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

const (
	producerConnectPolicyName  = "voice-nats-helper-connect-contract"
	producerConnectBindingName = "voice-nats-helper-connect-contract"
	producerConnectDenyMessage = "exec does not match the fixed helper protocol"
)

func TestProducerConnectVAPEvaluatesBothFixedVectors(t *testing.T) {
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
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace,
		Labels: map[string]string{"kubernetes.io/metadata.name": namespace}}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	createConnectRoleFixture(ctx, t, client)
	fixture := loadProducerFixture(t)
	archiveJobName := fixture.Job["metadata"].(map[string]interface{})["name"].(string)
	archivePodName := archiveJobName + "-proof"
	censusJobName := fixture.CensusJob["metadata"].(map[string]interface{})["name"].(string)
	censusPodName := censusJobName + "-proof"
	kubelet := startFakeKubelet(t)
	createFakeNode(ctx, t, client, kubelet)
	createProducerPodFixture(ctx, t, client, fixture.Job, archivePodName)
	createProducerPodFixture(ctx, t, client, fixture.CensusJob, censusPodName)
	installProducerConnectPolicy(ctx, t, client, fixture, map[string]string{"archive": archivePodName, "census": censusPodName})

	create := execRequest{method: http.MethodPost, namespace: namespace, name: archivePodName, username: runner,
		container: "nats-phase", stdout: true, command: fixture.ExecVectors["archive_create_and_stream"]}
	readback := execRequest{method: http.MethodPost, namespace: namespace, name: archivePodName, username: runner,
		container: "nats-phase", stdout: true, command: fixture.ExecVectors["archive_same_pod_readback"]}
	censusLoad := execRequest{method: http.MethodPost, namespace: namespace, name: censusPodName, username: runner,
		container: "nats-phase", stdin: true, stdout: true, command: fixture.CensusExecVectors["census_load_inputs"]}
	censusInputReadback := execRequest{method: http.MethodPost, namespace: namespace, name: censusPodName, username: runner,
		container: "nats-phase", stdout: true, command: fixture.CensusExecVectors["census_read_input_receipt"]}
	censusReceive := execRequest{method: http.MethodPost, namespace: namespace, name: censusPodName, username: runner,
		container: "nats-phase", stdin: true, stdout: true, command: fixture.CensusExecVectors["census_receive_archive"]}
	censusReadback := execRequest{method: http.MethodPost, namespace: namespace, name: censusPodName, username: runner,
		container: "nats-phase", stdout: true, command: fixture.CensusExecVectors["census_same_pod_readback"]}
	assertProducerConnectAllowed(t, cfg, create)
	assertProducerConnectAllowed(t, cfg, readback)
	assertProducerConnectAllowed(t, cfg, censusLoad)
	assertProducerConnectAllowed(t, cfg, censusInputReadback)
	assertProducerConnectAllowed(t, cfg, censusReceive)
	assertProducerConnectAllowed(t, cfg, censusReadback)
	for _, tc := range []struct {
		name string
		base execRequest
		edit func(*execRequest)
	}{
		{"shell command", create, func(r *execRequest) { r.command = []string{"/bin/sh", "-c", "id"} }},
		{"wrong archive mode", create, func(r *execRequest) { r.command[2] = "--serve" }},
		{"wrong readback mode", readback, func(r *execRequest) { r.command[2] = "--unknown" }},
		{"operation drift", create, func(r *execRequest) { r.command[4] = strings.Repeat("f", 12) }},
		{"nonce drift", create, func(r *execRequest) { r.command[6] = strings.Repeat("c", 32) }},
		{"wrong Pod UID", create, func(r *execRequest) { r.command[8] = "not-a-uid" }},
		{"wrong source PVC UID", create, func(r *execRequest) { r.command[10] = "55555555-5555-4555-8555-555555555555" }},
		{"wrong source PV UID", create, func(r *execRequest) { r.command[12] = "66666666-6666-4666-8666-666666666666" }},
		{"wrong container", create, func(r *execRequest) { r.container = "nats" }},
		{"stdin enabled", create, func(r *execRequest) { r.stdin = true }},
		{"stderr enabled", create, func(r *execRequest) { r.stderr = true }},
		{"tty enabled", create, func(r *execRequest) { r.tty = true }},
		{"wrong caller", create, func(r *execRequest) { r.username = "system:serviceaccount:voice-staging:default" }},
		{"wrong target Pod", create, func(r *execRequest) { r.name = "nats-0" }},
		{"census shell command", censusReceive, func(r *execRequest) { r.command = []string{"/bin/sh", "-c", "id"} }},
		{"census missing stdin", censusReceive, func(r *execRequest) { r.stdin = false }},
		{"census operation drift", censusReceive, func(r *execRequest) { r.command[2] = strings.Repeat("f", 12) }},
		{"census wrong target Pod", censusReadback, func(r *execRequest) { r.name = archivePodName }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := tc.base.clone()
			tc.edit(&request)
			assertDeniedByMessage(t, cfg, request, producerConnectDenyMessage)
		})
	}
}

func assertDeniedByMessage(t *testing.T, cfg *rest.Config, request execRequest, expected string) {
	t.Helper()
	status, message, err := issueExec(cfg, request)
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusUnprocessableEntity || !strings.Contains(message, expected) {
		t.Fatalf("request was not denied by VAP: status=%d message=%q", status, message)
	}
}

func assertProducerConnectAllowed(t *testing.T, cfg *rest.Config, request execRequest) {
	t.Helper()
	status, message, err := issueExec(cfg, request)
	if err != nil {
		t.Fatal(err)
	}
	if status == http.StatusUnprocessableEntity {
		t.Fatalf("fixed producer exec vector was rejected by VAP or API validation: status=%d message=%q", status, message)
	}
	if status == http.StatusForbidden {
		t.Fatalf("fixed producer exec vector did not reach the Kubernetes exec handler: status=%d message=%q", status, message)
	}
}

func createFakeNode(ctx context.Context, t *testing.T, client *kubernetes.Clientset, kubelet net.Addr) {
	t.Helper()
	addr, portText, err := net.SplitHostPort(kubelet.String())
	if err != nil {
		t.Fatal(err)
	}
	var port int32
	if _, err := strconv.Atoi(portText); err != nil {
		t.Fatal(err)
	}
	parsedPort, _ := strconv.Atoi(portText)
	port = int32(parsedPort)
	_, err = client.CoreV1().Nodes().Create(ctx, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "admission-proof-node"},
		Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: addr}},
			DaemonEndpoints: corev1.NodeDaemonEndpoints{KubeletEndpoint: corev1.DaemonEndpoint{Port: port}}}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create private fake kubelet Node: %v", err)
	}
}

func createProducerPodFixture(ctx context.Context, t *testing.T, client *kubernetes.Clientset, raw map[string]interface{}, name string) {
	t.Helper()
	job := decodeFixtureJob(t, raw)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace,
		Labels: job.Spec.Template.Labels, OwnerReferences: []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "Job",
			Name: job.Name, UID: "77777777-7777-4777-8777-777777777777", Controller: boolptr(true), BlockOwnerDeletion: boolptr(true)}}},
		Spec: *job.Spec.Template.Spec.DeepCopy()}
	pod.Spec.NodeName = "admission-proof-node"
	if _, err := client.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create helper Pod fixture from producer template: %v", err)
	}
}

func installProducerConnectPolicy(ctx context.Context, t *testing.T, client *kubernetes.Clientset, fixture producerFixture, podNames map[string]string) {
	t.Helper()
	fail := admissionv1.Fail
	equivalent := admissionv1.Equivalent
	namespaced := admissionv1.NamespacedScope
	resourceRule := admissionv1.NamedRuleWithOperations{RuleWithOperations: admissionv1.RuleWithOperations{
		Rule:       admissionv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"pods/exec"}, Scope: &namespaced},
		Operations: []admissionv1.OperationType{admissionv1.Connect}}}
	validation := admissionv1.Validation{Expression: producerConnectExpression(fixture, podNames), Message: producerConnectDenyMessage}
	policy := &admissionv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: producerConnectPolicyName},
		Spec: admissionv1.ValidatingAdmissionPolicySpec{FailurePolicy: &fail, MatchConstraints: &admissionv1.MatchResources{
			MatchPolicy: &equivalent, NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}},
			ResourceRules: []admissionv1.NamedRuleWithOperations{resourceRule}}, Validations: []admissionv1.Validation{validation}}}
	if _, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(ctx, policy, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create producer-vector CONNECT VAP: %v", err)
	}
	binding := &admissionv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: producerConnectBindingName},
		Spec: admissionv1.ValidatingAdmissionPolicyBindingSpec{PolicyName: producerConnectPolicyName,
			MatchResources:    &admissionv1.MatchResources{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}}},
			ValidationActions: []admissionv1.ValidationAction{admissionv1.Deny}}}
	if _, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(ctx, binding, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create producer-vector CONNECT Deny binding: %v", err)
	}
	if _, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, producerConnectPolicyName, metav1.GetOptions{}); err != nil {
		t.Fatalf("read served CONNECT VAP: %v", err)
	}
	if _, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, producerConnectBindingName, metav1.GetOptions{}); err != nil {
		t.Fatalf("read served CONNECT binding: %v", err)
	}
	time.Sleep(time.Second)
}

func producerConnectExpression(fixture producerFixture, podNames map[string]string) string {
	q := strconv.Quote
	list := func(values []string) string {
		parts := make([]string, len(values))
		for i, value := range values {
			parts[i] = q(value)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	common := "request.userInfo.username == " + q(runner) + " && request.namespace == " + q(namespace) +
		" && object.container == 'nats-phase' && object.stdout == true && (!has(object.stderr) || object.stderr == false) && (!has(object.tty) || object.tty == false)"
	archiveName := "request.name == " + q(podNames["archive"])
	archiveCreate := "(!has(object.stdin) || object.stdin == false) && object.command == " + list(fixture.ExecVectors["archive_create_and_stream"])
	archiveReadback := "(!has(object.stdin) || object.stdin == false) && object.command == " + list(fixture.ExecVectors["archive_same_pod_readback"])
	censusName := "request.name == " + q(podNames["census"])
	censusLoad := "has(object.stdin) && object.stdin == true && object.command == " + list(fixture.CensusExecVectors["census_load_inputs"])
	censusInputReceipt := "(!has(object.stdin) || object.stdin == false) && object.command == " + list(fixture.CensusExecVectors["census_read_input_receipt"])
	censusReceive := "has(object.stdin) && object.stdin == true && object.command == " + list(fixture.CensusExecVectors["census_receive_archive"])
	censusReadback := "(!has(object.stdin) || object.stdin == false) && object.command == " + list(fixture.CensusExecVectors["census_same_pod_readback"])
	return "(" + common + ") && ((" + archiveName + " && ((" + archiveCreate + ") || (" + archiveReadback + "))) || (" + censusName + " && ((" + censusLoad + ") || (" + censusInputReceipt + ") || (" + censusReceive + ") || (" + censusReadback + "))))"
}
