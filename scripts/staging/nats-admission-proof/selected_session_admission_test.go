package admissionproof

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	selectedJobPolicyName  = "voice-nats-selected-session-job-contract"
	selectedPodPolicyName  = "voice-nats-selected-session-pod-contract"
	selectedExecPolicyName = "voice-nats-selected-session-exec-contract"
	selectedJobParams      = "voice-nats-selected-session-params"
	selectedJobDeny        = "selected-session Job differs from the committed producer contract"
	selectedPodDeny        = "selected-session Pod differs from the committed producer contract"
	selectedExecDeny       = "exec differs from the fixed selected-session lifecycle contract"
	jobControllerFixture   = "system:serviceaccount:voice-staging:job-controller-fixture"
)

type selectedFixture struct {
	ArchiveJob    map[string]interface{}          `json:"archive_job"`
	CensusJob     map[string]interface{}          `json:"census_job"`
	BindJob       map[string]interface{}          `json:"bind_job"`
	Job           map[string]interface{}          `json:"job"`
	ExecContracts map[string]selectedExecContract `json:"exec_contracts"`
}

type selectedExecContract struct {
	Container string   `json:"container"`
	Command   []string `json:"command"`
	Stdin     bool     `json:"stdin"`
	Stdout    bool     `json:"stdout"`
	Stderr    bool     `json:"stderr"`
	TTY       bool     `json:"tty"`
}

func loadSelectedFixture(t *testing.T) selectedFixture {
	t.Helper()
	dir := filepath.Join("testdata", "producer-dff102a")
	source, err := os.ReadFile(filepath.Join(dir, "verification_job.py"))
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(source)); got != "fb720abf8e6886a83ada988efae9416d4ed140338bf01f9780e9bf10c8fe2be9" {
		t.Fatalf("contract-migration source drift: sha256=%s", got)
	}
	program := `import json, verification_job as j
base={'operation':'0123456789ab','nonce':'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'}
r=base|{'phase':'contract-migration'}
b={'source_claim':'voice-nats-jsdata-d20261017a','source_uid':'11111111-1111-4111-8111-111111111111','source_pv_name':'pv-source','source_pv_uid':'33333333-3333-4333-8333-333333333333','source_node_name':'k3s-node-actual-1','source_hostname':'k3s-host-label-1','verification_claim':'voice-nats-verify-0123456789ab','verification_uid':'22222222-2222-4222-8222-222222222222','verification_pv_name':'pv-verification','verification_pv_uid':'44444444-4444-4444-8444-444444444444','verification_node_name':'k3s-node-actual-1','verification_hostname':'k3s-host-label-1','contract_attempt':'deadbeef','input_pod_uid':'77777777-7777-4777-8777-777777777777','input_attempt':'cafebabe','input_bundle_sha256':'a'*64,'input_bundle_bytes':4096,'archive_sha256':'b'*64,'archive_bytes':8192,'kernel_bundle_sha256':'c'*64,'plan_sha256':'d'*64,'plan_bytes':2048,'server_config_sha256':'e'*64}
image='ghcr.io/poryadok/voice-nats-phase@sha256:'+'a'*64
vectors=j.exec_vectors(r,'c5c81fc7-86d6-4d62-8b84-9912d0805bf2',b)
contracts={name:{'container':'control','command':command,'stdin':name=='load-plan','stdout':True,'stderr':False,'tty':False} for name,command in vectors.items()}
print(json.dumps({'job':j.document(r,b,image),'exec_contracts':contracts}))`
	cmd := exec.Command("python3", "-c", program)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("run committed selected-session producer: %v: %s", err, stderr.String())
	}
	var got selectedFixture
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("decode selected producer: %v: %s", err, stdout.String())
	}
	if got.Job == nil || len(got.ExecContracts) != 4 {
		t.Fatalf("producer did not return contract Job and fixed exec vectors")
	}
	return got
}

func TestSelectedSessionJobPodAndConnectVAP(t *testing.T) {
	cfg, stop := startAdmissionAPIServer(t)
	defer stop()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	admin, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = admin.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace, Labels: map[string]string{"kubernetes.io/metadata.name": namespace}}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	fixture := loadSelectedFixture(t)
	_, err = admin.CoreV1().ConfigMaps(namespace).Create(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: selectedJobParams}, Data: map[string]string{"phase_image": testImage, "job_controller_username": jobControllerFixture}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	createSelectedSessionRole(ctx, t, admin)
	installSelectedJobVAP(ctx, t, admin, fixture.Job)
	runner := clientAs(ctx, t, cfg, stagingRunner)
	job := decodeFixtureJob(t, fixture.Job)
	for _, tc := range []struct {
		name string
		edit func(*batchv1.Job)
	}{
		{"source PVC co-mounted into control", func(j *batchv1.Job) {
			j.Spec.Template.Spec.Containers[0].VolumeMounts = append(j.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{Name: "source", MountPath: "/data", ReadOnly: true})
		}},
		{"source claim rebound", func(j *batchv1.Job) {
			j.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = "voice-nats-jsdata-d20261017b"
		}},
		{"PV hostname affinity changed to node name", func(j *batchv1.Job) {
			j.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values[0] = "k3s-node-actual-1"
		}},
		{"spec nodeName used for WFFC", func(j *batchv1.Job) { j.Spec.Template.Spec.NodeName = "k3s-node-actual-1" }},
		{"Secret volume injected", func(j *batchv1.Job) {
			j.Spec.Template.Spec.Volumes = append(j.Spec.Template.Spec.Volumes, corev1.Volume{Name: "credential", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "unrelated"}}})
		}},
		{"private bundle subPath changed", func(j *batchv1.Job) {
			j.Spec.Template.Spec.Containers[0].VolumeMounts[0].SubPath = "inputs/public/server.conf"
		}},
		{"control state subPath changed", func(j *batchv1.Job) { j.Spec.Template.Spec.Containers[0].VolumeMounts[1].SubPath = "inputs/private" }},
		{"NATS sidecar sees private state", func(j *batchv1.Job) {
			j.Spec.Template.Spec.InitContainers[0].VolumeMounts = append(j.Spec.Template.Spec.InitContainers[0].VolumeMounts, corev1.VolumeMount{Name: "scratch", MountPath: "/state", SubPath: "state/0123456789ab/deadbeef"})
		}},
		{"arbitrary control command", func(j *batchv1.Job) { j.Spec.Template.Spec.Containers[0].Command[0] = "/bin/sh" }},
		{"arbitrary control args", func(j *batchv1.Job) { j.Spec.Template.Spec.Containers[0].Args[0] = "--shell" }},
		{"arbitrary NATS image", func(j *batchv1.Job) { j.Spec.Template.Spec.InitContainers[0].Image = "attacker.invalid/nats:latest" }},
		{"arbitrary control image", func(j *batchv1.Job) { j.Spec.Template.Spec.Containers[0].Image = "attacker.invalid/control:latest" }},
		{"NATS identity changed", func(j *batchv1.Job) {
			uid := int64(65532)
			j.Spec.Template.Spec.InitContainers[0].SecurityContext.RunAsUser = &uid
		}},
		{"extra environment credential", func(j *batchv1.Job) {
			j.Spec.Template.Spec.Containers[0].Env = append(j.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{Name: "TOKEN", Value: "secret"})
		}},
		{"service account token enabled", func(j *batchv1.Job) { j.Spec.Template.Spec.AutomountServiceAccountToken = boolptr(true) }},
		{"host path", func(j *batchv1.Job) {
			j.Spec.Template.Spec.Volumes = append(j.Spec.Template.Spec.Volumes, corev1.Volume{Name: "host", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/"}}})
		}},
		{"network host", func(j *batchv1.Job) { j.Spec.Template.Spec.HostNetwork = true }},
	} {
		t.Run("Job DENY "+tc.name, func(t *testing.T) {
			candidate := job.DeepCopy()
			tc.edit(candidate)
			err := createJob(ctx, runner, candidate)
			if err == nil || !strings.Contains(err.Error(), selectedJobPolicyName) {
				t.Fatalf("mutated selected-session Job not denied by VAP: %v", err)
			}
		})
	}
	if err := createJob(ctx, runner, job); err != nil {
		t.Fatalf("committed contract-migration Job rejected: %v", err)
	}
	stored, err := admin.BatchV1().Jobs(namespace).Get(ctx, job.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pod := selectedPodFromJob(stored)
	installSelectedPodVAP(ctx, t, admin, job)
	controller := clientAs(ctx, t, cfg, jobControllerFixture)
	for _, tc := range []struct {
		name string
		edit func(*corev1.Pod)
	}{
		{"source PVC co-mounted into control", func(p *corev1.Pod) {
			p.Spec.Containers[0].VolumeMounts = append(p.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{Name: "source", MountPath: "/data", ReadOnly: true})
		}},
		{"private bundle changed", func(p *corev1.Pod) { p.Spec.Containers[0].VolumeMounts[0].SubPath = "inputs/public/server.conf" }},
		{"NATS sidecar removed", func(p *corev1.Pod) { p.Spec.InitContainers = nil }},
		{"control image changed", func(p *corev1.Pod) { p.Spec.Containers[0].Image = "attacker.invalid/image:latest" }},
		{"owner is not Job controller", func(p *corev1.Pod) { p.OwnerReferences[0].Controller = boolptr(false) }},
		{"owner Job name mismatch", func(p *corev1.Pod) { p.OwnerReferences[0].Name = "unrelated-job" }},
		{"owner UID malformed", func(p *corev1.Pod) { p.OwnerReferences[0].UID = "bad" }},
		{"POD_UID does not use metadata.uid", func(p *corev1.Pod) { p.Spec.Containers[0].Env[0].ValueFrom.FieldRef.FieldPath = "metadata.name" }},
		{"POD_UID literal override", func(p *corev1.Pod) {
			p.Spec.Containers[0].Env[0].ValueFrom = nil
			p.Spec.Containers[0].Env[0].Value = testPodUID
		}},
		{"wrong actor", func(p *corev1.Pod) {}},
	} {
		t.Run("Pod DENY "+tc.name, func(t *testing.T) {
			candidate := pod.DeepCopy()
			tc.edit(candidate)
			client := controller
			if tc.name == "wrong actor" {
				client = runner
			}
			if _, err := client.CoreV1().Pods(namespace).Create(ctx, candidate, metav1.CreateOptions{}); err == nil || !strings.Contains(err.Error(), selectedPodPolicyName) {
				t.Fatalf("mutated selected-session Pod not denied by VAP: %v", err)
			}
		})
	}
	if _, err := controller.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		t.Fatalf("committed contract-migration Pod rejected: %v", err)
	}

	kubelet := startFakeKubelet(t)
	createFakeNode(ctx, t, admin, kubelet)
	installSelectedExecVAP(ctx, t, admin, stored.Name, pod.Name, fixture.ExecContracts)
	for name, contract := range fixture.ExecContracts {
		request := execRequest{method: http.MethodPost, namespace: namespace, name: pod.Name, username: stagingRunner, container: contract.Container, stdin: contract.Stdin, stdout: contract.Stdout, stderr: contract.Stderr, tty: contract.TTY, command: contract.Command}
		status, message, err := issueExec(cfg, request)
		if err != nil {
			t.Fatalf("execute accepted fixed CONNECT %s: %v", name, err)
		}
		if status == http.StatusForbidden || status == http.StatusUnprocessableEntity {
			t.Fatalf("fixed CONNECT %s was not admitted/reached handler: status=%d message=%s", name, status, message)
		}
	}
	base := execRequest{method: http.MethodPost, namespace: namespace, name: pod.Name, username: stagingRunner, container: "control", stdout: true, command: fixture.ExecContracts["broker-start"].Command}
	for _, tc := range []struct {
		name string
		edit func(*execRequest)
	}{
		{"shell command", func(r *execRequest) { r.command = []string{"/bin/sh", "-c", "id"} }},
		{"wrong container", func(r *execRequest) { r.container = "nats" }},
		{"stream mode changed", func(r *execRequest) { r.stdin = true }},
		{"wrong pod", func(r *execRequest) { r.name = "unrelated-pod" }},
		{"wrong actor", func(r *execRequest) { r.username = "system:serviceaccount:voice-staging:default" }},
	} {
		t.Run("CONNECT DENY "+tc.name, func(t *testing.T) {
			request := base.clone()
			tc.edit(&request)
			assertDeniedByMessage(t, cfg, request, selectedExecDeny)
		})
	}
}

func createSelectedSessionRole(ctx context.Context, t *testing.T, c *kubernetes.Clientset) {
	t.Helper()
	_, err := c.RbacV1().Roles(namespace).Create(ctx, &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "selected-session-proof"}, Rules: []rbacv1.PolicyRule{
		{APIGroups: []string{"batch"}, Resources: []string{"jobs"}, Verbs: []string{"create"}},
		{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"create"}},
		{APIGroups: []string{""}, Resources: []string{"pods/exec"}, Verbs: []string{"create", "get"}},
	}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.RbacV1().RoleBindings(namespace).Create(ctx, &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "selected-session-proof"}, Subjects: []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: "voice-nats-staging-runner", Namespace: namespace}, {Kind: rbacv1.ServiceAccountKind, Name: "job-controller-fixture", Namespace: namespace}, {Kind: rbacv1.ServiceAccountKind, Name: "default", Namespace: namespace}}, RoleRef: rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: "selected-session-proof"}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
}

func installSelectedJobVAP(ctx context.Context, t *testing.T, c *kubernetes.Clientset, raw map[string]interface{}) {
	t.Helper()
	fail, eq, scope := admissionv1.Fail, admissionv1.Equivalent, admissionv1.NamespacedScope
	validations := []admissionv1.Validation{{Expression: "request.namespace == 'voice-staging'", Message: selectedJobDeny}, {Expression: "request.userInfo.username == '" + stagingRunner + "'", Message: selectedJobDeny}}
	for _, expr := range exactJobCELExpressions(raw) {
		if strings.Contains(expr, ".image == ") {
			parts := strings.SplitN(expr, " == ", 2)
			expr = "has(" + parts[0] + ") && " + parts[0] + " == params.data['phase_image']"
		}
		validations = append(validations, admissionv1.Validation{Expression: expr, Message: selectedJobDeny})
	}
	for _, expr := range []string{"object.metadata.labels['voice.io/nats-job-phase'] == 'contract-migration'", "object.spec.template.spec.initContainers.size() == 1", "object.spec.template.spec.containers.size() == 1", "object.spec.template.spec.containers[0].env[0].name == 'POD_UID' && object.spec.template.spec.containers[0].env[0].valueFrom.fieldRef.fieldPath == 'metadata.uid'", "!has(object.spec.template.spec.nodeName)", "!has(object.spec.template.spec.hostNetwork) || object.spec.template.spec.hostNetwork == false", "!has(object.spec.template.spec.shareProcessNamespace) || object.spec.template.spec.shareProcessNamespace == false"} {
		validations = append(validations, admissionv1.Validation{Expression: expr, Message: selectedJobDeny})
	}
	policy := &admissionv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: selectedJobPolicyName}, Spec: admissionv1.ValidatingAdmissionPolicySpec{FailurePolicy: &fail, ParamKind: &admissionv1.ParamKind{APIVersion: "v1", Kind: "ConfigMap"}, MatchConstraints: &admissionv1.MatchResources{MatchPolicy: &eq, NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}}, ResourceRules: []admissionv1.NamedRuleWithOperations{{RuleWithOperations: admissionv1.RuleWithOperations{Rule: admissionv1.Rule{APIGroups: []string{"batch"}, APIVersions: []string{"v1"}, Resources: []string{"jobs"}, Scope: &scope}, Operations: []admissionv1.OperationType{admissionv1.Create}}}}}, Validations: validations}}
	if _, err := c.AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(ctx, policy, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	deny, notFound := admissionv1.Deny, admissionv1.DenyAction
	binding := &admissionv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: selectedJobPolicyName}, Spec: admissionv1.ValidatingAdmissionPolicyBindingSpec{PolicyName: selectedJobPolicyName, MatchResources: &admissionv1.MatchResources{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}}}, ParamRef: &admissionv1.ParamRef{Name: selectedJobParams, Namespace: namespace, ParameterNotFoundAction: &notFound}, ValidationActions: []admissionv1.ValidationAction{deny}}}
	if _, err := c.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(ctx, binding, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
}

func selectedPodFromJob(job *batchv1.Job) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: job.Name + "-abcde", Namespace: namespace, Labels: job.Spec.Template.Labels, OwnerReferences: []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "Job", Name: job.Name, UID: job.UID, Controller: boolptr(true), BlockOwnerDeletion: boolptr(true)}}}, Spec: *job.Spec.Template.Spec.DeepCopy()}
}

func installSelectedPodVAP(ctx context.Context, t *testing.T, c *kubernetes.Clientset, job *batchv1.Job) {
	pod := selectedPodFromJob(job)
	b, _ := json.Marshal(pod)
	var expected map[string]interface{}
	_ = json.Unmarshal(b, &expected)
	fail, eq, scope := admissionv1.Fail, admissionv1.Equivalent, admissionv1.NamespacedScope
	validations := []admissionv1.Validation{{Expression: "request.namespace == 'voice-staging'", Message: selectedPodDeny}, {Expression: "request.userInfo.username == params.data['job_controller_username']", Message: selectedPodDeny}, {Expression: "object.metadata.ownerReferences.size() == 1 && object.metadata.ownerReferences[0].apiVersion == 'batch/v1' && object.metadata.ownerReferences[0].kind == 'Job' && object.metadata.ownerReferences[0].name == '" + job.Name + "' && object.metadata.ownerReferences[0].controller == true && object.metadata.ownerReferences[0].blockOwnerDeletion == true && object.metadata.ownerReferences[0].uid.matches('^[a-f0-9]{8}(-[a-f0-9]{4}){3}-[a-f0-9]{12}$')", Message: selectedPodDeny}, {Expression: "object.metadata.name.matches('^" + job.Name + "-[a-z0-9]{5}$')", Message: selectedPodDeny}}
	for index, expr := range exactJobCELExpressions(expected) {
		if strings.HasPrefix(expr, "object.metadata.name == ") || strings.HasPrefix(expr, "object.metadata.ownerReferences[") {
			continue
		}
		if strings.Contains(expr, ".image == ") {
			parts := strings.SplitN(expr, " == ", 2)
			expr = "has(" + parts[0] + ") && " + parts[0] + " == params.data['phase_image']"
		}
		validations = append(validations, admissionv1.Validation{Expression: expr, Message: fmt.Sprintf("%s [exact-%d]", selectedPodDeny, index)})
	}
	policy := &admissionv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: selectedPodPolicyName}, Spec: admissionv1.ValidatingAdmissionPolicySpec{FailurePolicy: &fail, ParamKind: &admissionv1.ParamKind{APIVersion: "v1", Kind: "ConfigMap"}, MatchConstraints: &admissionv1.MatchResources{MatchPolicy: &eq, NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}}, ResourceRules: []admissionv1.NamedRuleWithOperations{{RuleWithOperations: admissionv1.RuleWithOperations{Rule: admissionv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"pods"}, Scope: &scope}, Operations: []admissionv1.OperationType{admissionv1.Create}}}}}, Validations: validations}}
	if _, err := c.AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(ctx, policy, metav1.CreateOptions{}); err != nil {
		t.Fatalf("install Pod VAP: %v", err)
	}
	deny, notFound := admissionv1.Deny, admissionv1.DenyAction
	binding := &admissionv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: selectedPodPolicyName}, Spec: admissionv1.ValidatingAdmissionPolicyBindingSpec{PolicyName: selectedPodPolicyName, MatchResources: &admissionv1.MatchResources{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}}}, ParamRef: &admissionv1.ParamRef{Name: selectedJobParams, Namespace: namespace, ParameterNotFoundAction: &notFound}, ValidationActions: []admissionv1.ValidationAction{deny}}}
	if _, err := c.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(ctx, binding, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
}

func installSelectedExecVAP(ctx context.Context, t *testing.T, c *kubernetes.Clientset, jobName, podName string, contracts map[string]selectedExecContract) {
	q := strconv.Quote
	rules := make([]string, 0, len(contracts))
	for _, contract := range contracts {
		cmd, _ := json.Marshal(contract.Command)
		terms := []string{"object.container == " + q(contract.Container), "object.command == " + string(cmd), "object.stdout == " + strconv.FormatBool(contract.Stdout), "(!has(object.stdin) || object.stdin == " + strconv.FormatBool(contract.Stdin) + ")", "(!has(object.stderr) || object.stderr == " + strconv.FormatBool(contract.Stderr) + ")", "(!has(object.tty) || object.tty == " + strconv.FormatBool(contract.TTY) + ")"}
		rules = append(rules, "("+strings.Join(terms, " && ")+")")
	}
	expr := "request.namespace == " + q(namespace) + " && request.userInfo.username == " + q(stagingRunner) + " && request.name == " + q(podName) + " && object.command.size() > 1 && (" + strings.Join(rules, " || ") + ")"
	fail, eq, scope := admissionv1.Fail, admissionv1.Equivalent, admissionv1.NamespacedScope
	policy := &admissionv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: selectedExecPolicyName}, Spec: admissionv1.ValidatingAdmissionPolicySpec{FailurePolicy: &fail, MatchConstraints: &admissionv1.MatchResources{MatchPolicy: &eq, NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}}, ResourceRules: []admissionv1.NamedRuleWithOperations{{RuleWithOperations: admissionv1.RuleWithOperations{Rule: admissionv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"pods/exec"}, Scope: &scope}, Operations: []admissionv1.OperationType{admissionv1.Connect}}}}}, Validations: []admissionv1.Validation{{Expression: expr, Message: selectedExecDeny}}}}
	if _, err := c.AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(ctx, policy, metav1.CreateOptions{}); err != nil {
		t.Fatalf("install CONNECT VAP for Job %s: %v", jobName, err)
	}
	deny := admissionv1.Deny
	binding := &admissionv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: selectedExecPolicyName}, Spec: admissionv1.ValidatingAdmissionPolicyBindingSpec{PolicyName: selectedExecPolicyName, MatchResources: &admissionv1.MatchResources{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}}}, ValidationActions: []admissionv1.ValidationAction{deny}}}
	if _, err := c.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(ctx, binding, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)
}
