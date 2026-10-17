package admissionproof

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

const (
	policyName  = "voice-nats-runner-connect-contract"
	bindingName = "voice-nats-runner-connect-contract"
	namespace   = "voice-staging"
	runner      = "system:serviceaccount:voice-staging:voice-nats-staging-runner"
	denyMessage = "runner exec command outside fixed phase protocol"
)

func TestConnectAdmissionEvaluatesPodExecOptions(t *testing.T) {
	assets := os.Getenv("KUBEBUILDER_ASSETS")
	if assets == "" {
		t.Fatal("KUBEBUILDER_ASSETS must point at the pinned Kubernetes v1.34.3 envtest binaries")
	}
	testEnv := &envtest.Environment{
		BinaryAssetsDirectory: assets,
		ControlPlane: envtest.ControlPlane{
			APIServer: &envtest.APIServer{Args: []string{
				"--advertise-address=127.0.0.1",
				"--enable-admission-plugins=ValidatingAdmissionPolicy",
			}},
		},
		ControlPlaneStartTimeout: 2 * time.Minute,
		ControlPlaneStopTimeout:  30 * time.Second,
	}
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
	_, err = client.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: namespace, Labels: map[string]string{"kubernetes.io/metadata.name": namespace}},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	kubelet := startFakeKubelet(t)
	createPodFixtures(ctx, t, client, kubelet)
	createConnectRoleFixture(ctx, t, client)
	createConnectPolicy(ctx, t, client)

	op := "0123456789ab"
	nonce := strings.Repeat("a", 32)
	podUID := "c5c81fc7-86d6-4d62-8b84-9912d0805bf2"
	name := "voice-nats-rollout-archive-" + op
	valid := execRequest{method: http.MethodPost, namespace: namespace, name: name, username: runner,
		container: "nats-phase", stdout: true,
		command: []string{"/voice-nats-runner/phase", "--create-and-stream", "--operation", op, "--nonce", nonce, "--pod-uid", podUID}}
	assertNotDeniedByVAP(t, cfg, valid)

	for _, tc := range []struct {
		name string
		edit func(*execRequest)
	}{
		{"arbitrary command", func(r *execRequest) { r.command = []string{"/bin/sh", "-c", "id"} }},
		{"wrong helper subcommand", func(r *execRequest) { r.command[1] = "--delete-everything" }},
		{"malformed operation", func(r *execRequest) { r.command[3] = "../../secret" }},
		{"wrong container", func(r *execRequest) { r.container = "nats" }},
		{"stdin enabled", func(r *execRequest) { r.stdin = true }},
		{"stderr enabled", func(r *execRequest) { r.stderr = true }},
		{"tty enabled", func(r *execRequest) { r.tty = true }},
		{"untrusted caller", func(r *execRequest) { r.username = "system:serviceaccount:voice-staging:default" }},
		{"wrong target name", func(r *execRequest) { r.name = "nats-0" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := valid.clone()
			tc.edit(&request)
			assertDeniedByVAP(t, cfg, request)
		})
	}

	// The same CEL policy runs on the actual kube-apiserver CONNECT handler.
	// A malformed policy is not accepted as a successful deny: it is caught
	// while installing the policy above, before this request matrix runs.
}

func createPodFixtures(ctx context.Context, t *testing.T, client *kubernetes.Clientset, kubelet net.Addr) {
	t.Helper()
	addr, portText, err := net.SplitHostPort(kubelet.String())
	if err != nil {
		t.Fatal(err)
	}
	var port int32
	if _, err := fmt.Sscan(portText, &port); err != nil {
		t.Fatal(err)
	}
	_, err = client.CoreV1().Nodes().Create(ctx, &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "admission-proof-node"},
		Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: addr}},
			DaemonEndpoints: corev1.NodeDaemonEndpoints{KubeletEndpoint: corev1.DaemonEndpoint{Port: port}}},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create private fake kubelet node: %v", err)
	}
	for _, name := range []string{"voice-nats-rollout-archive-0123456789ab", "nats-0"} {
		_, err := client.CoreV1().Pods(namespace).Create(ctx, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       corev1.PodSpec{NodeName: "admission-proof-node", Containers: []corev1.Container{{Name: "nats-phase", Image: "example.invalid/unused"}}},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Name: "nats-phase", Ready: true, ContainerID: "containerd://proof"}}},
		}, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("create CONNECT target Pod fixture %s: %v", name, err)
		}
	}
}

func startFakeKubelet(t *testing.T) net.Addr {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	return listener.Addr()
}

func createConnectRoleFixture(ctx context.Context, t *testing.T, client *kubernetes.Clientset) {
	t.Helper()
	roleName := "nats-admission-proof-exec"
	_, err := client.RbacV1().Roles(namespace).Create(ctx, &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: roleName},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{""}, Resources: []string{"pods/exec"}, Verbs: []string{"create", "get"}},
			{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"create"}},
			{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"create"}},
			{APIGroups: []string{"batch"}, Resources: []string{"jobs"}, Verbs: []string{"create"}},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("create minimal CONNECT test role: %v", err)
	}
	_, err = client.RbacV1().RoleBindings(namespace).Create(ctx, &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: roleName},
		Subjects: []rbacv1.Subject{
			{Kind: rbacv1.ServiceAccountKind, Name: "voice-nats-staging-runner", Namespace: namespace},
			{Kind: rbacv1.ServiceAccountKind, Name: "default", Namespace: namespace},
			{Kind: rbacv1.ServiceAccountKind, Name: "job-controller-fixture", Namespace: namespace},
			{Kind: rbacv1.ServiceAccountKind, Name: "job-controller-fixture", Namespace: namespace},
		},
		RoleRef: rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: roleName},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("bind minimal CONNECT test identities: %v", err)
	}
}

func createConnectPolicy(ctx context.Context, t *testing.T, client *kubernetes.Clientset) {
	t.Helper()
	failurePolicy := admissionv1.Fail
	matchPolicy := admissionv1.Equivalent
	scope := admissionv1.NamespacedScope
	operation := admissionv1.Connect
	resource := admissionv1.RuleWithOperations{Rule: admissionv1.Rule{
		APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"pods/exec"}, Scope: &scope,
	}, Operations: []admissionv1.OperationType{operation}}
	validation := admissionv1.Validation{Expression: strings.Join([]string{
		"request.userInfo.username == '" + runner + "'",
		"request.namespace == 'voice-staging'",
		"request.name.startsWith('voice-nats-rollout-')",
		"object.container == 'nats-phase'",
		"object.stdin == false && object.stdout == true && object.stderr == false && object.tty == false",
		"object.command.size() == 8",
		"object.command[0] == '/voice-nats-runner/phase'",
		"object.command[1] in ['--create-and-stream', '--stream-existing']",
		"object.command[2] == '--operation' && object.command[3].matches('^[0-9a-f]{12}$')",
		"object.command[4] == '--nonce' && object.command[5].matches('^[0-9a-f]{32}$')",
		"object.command[6] == '--pod-uid' && object.command[7].matches('^[0-9a-f-]{36}$')",
	}, " && "), Message: denyMessage}
	policy := &admissionv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: policyName},
		Spec: admissionv1.ValidatingAdmissionPolicySpec{
			FailurePolicy: &failurePolicy,
			MatchConstraints: &admissionv1.MatchResources{
				MatchPolicy:       &matchPolicy,
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}},
				ResourceRules:     []admissionv1.NamedRuleWithOperations{{RuleWithOperations: resource}},
			},
			Validations: []admissionv1.Validation{validation},
		},
	}
	if _, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(ctx, policy, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create VAP with upstream CEL type checking: %v", err)
	}
	servedPolicy, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, policyName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("read served VAP: %v", err)
	}
	if servedPolicy.Spec.FailurePolicy == nil || *servedPolicy.Spec.FailurePolicy != admissionv1.Fail ||
		servedPolicy.Spec.MatchConstraints == nil || len(servedPolicy.Spec.MatchConstraints.ResourceRules) != 1 ||
		len(servedPolicy.Spec.Validations) != 1 || servedPolicy.Spec.Validations[0].Expression != validation.Expression ||
		servedPolicy.Spec.Validations[0].Message != denyMessage {
		t.Fatalf("served VAP does not preserve fail-closed CONNECT contract: %#v", servedPolicy.Spec)
	}
	servedRule := servedPolicy.Spec.MatchConstraints.ResourceRules[0]
	if len(servedRule.Operations) != 1 || servedRule.Operations[0] != admissionv1.Connect ||
		len(servedRule.APIGroups) != 1 || servedRule.APIGroups[0] != "" ||
		len(servedRule.APIVersions) != 1 || servedRule.APIVersions[0] != "v1" ||
		len(servedRule.Resources) != 1 || servedRule.Resources[0] != "pods/exec" {
		t.Fatalf("served VAP does not match core/v1 pods/exec CONNECT: %#v", servedRule)
	}
	deny := admissionv1.Deny
	binding := &admissionv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: bindingName},
		Spec: admissionv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName: policyName,
			MatchResources: &admissionv1.MatchResources{
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}},
			},
			ValidationActions: []admissionv1.ValidationAction{deny},
		},
	}
	if _, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(ctx, binding, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create fail-closed VAP binding: %v", err)
	}
	servedBinding, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, bindingName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("read served VAP binding: %v", err)
	}
	if servedBinding.Spec.PolicyName != policyName || len(servedBinding.Spec.ValidationActions) != 1 ||
		servedBinding.Spec.ValidationActions[0] != admissionv1.Deny || servedBinding.Spec.MatchResources == nil {
		t.Fatalf("served VAP binding does not preserve Deny binding: %#v", servedBinding.Spec)
	}
	time.Sleep(time.Second)
}

type execRequest struct {
	method, namespace, name, username, container string
	command                                      []string
	stdin, stdout, stderr, tty                   bool
}

func (r execRequest) clone() execRequest {
	r.command = append([]string(nil), r.command...)
	return r
}

func issueExec(cfg *rest.Config, r execRequest) (int, string, error) {
	requestURL, err := url.Parse(strings.TrimRight(cfg.Host, "/") + "/api/v1/namespaces/" + url.PathEscape(r.namespace) + "/pods/" + url.PathEscape(r.name) + "/exec")
	if err != nil {
		return 0, "", err
	}
	query := url.Values{}
	query.Set("container", r.container)
	query.Set("stdin", fmt.Sprint(r.stdin))
	query.Set("stdout", fmt.Sprint(r.stdout))
	query.Set("stderr", fmt.Sprint(r.stderr))
	query.Set("tty", fmt.Sprint(r.tty))
	for _, arg := range r.command {
		query.Add("command", arg)
	}
	requestURL.RawQuery = query.Encode()
	req, err := http.NewRequest(r.method, requestURL.String(), nil)
	if err != nil {
		return 0, "", err
	}
	transportConfig := rest.CopyConfig(cfg)
	transportConfig.Impersonate.UserName = r.username
	transport, err := rest.TransportFor(transportConfig)
	if err != nil {
		return 0, "", err
	}
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	var body struct {
		Message string `json:"message"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body.Message, nil
}

func assertDeniedByVAP(t *testing.T, cfg *rest.Config, request execRequest) {
	t.Helper()
	status, message, err := issueExec(cfg, request)
	if err != nil {
		t.Fatal(err)
	}
	if status != http.StatusUnprocessableEntity || !strings.Contains(message, denyMessage) {
		t.Fatalf("request was not denied by VAP: status=%d message=%q", status, message)
	}
}

func assertNotDeniedByVAP(t *testing.T, cfg *rest.Config, request execRequest) {
	t.Helper()
	status, message, err := issueExec(cfg, request)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(message, denyMessage) {
		t.Fatalf("valid CONNECT request was denied by VAP: status=%d message=%q", status, message)
	}
	if status == http.StatusForbidden {
		t.Fatalf("valid CONNECT request did not pass admission: status=%d message=%q", status, message)
	}
}
