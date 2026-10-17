package admissionproof

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	binderPolicyName  = "voice-nats-binder-job-contract"
	binderBindingName = "voice-nats-binder-job-contract"
	binderDenyMessage = "binder Job differs from the closed producer contract"
	binderParamsName  = "voice-nats-binder-job-fixture"
)

func TestProducerBinderJobsPassVAPAndKubernetes(t *testing.T) {
	cfg, stop := startAdmissionAPIServer(t)
	defer stop()
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
	fixture := loadProducerFixture(t)
	if _, err := admin.CoreV1().ConfigMaps(namespace).Create(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: binderParamsName}, Data: map[string]string{"phase_image": testImage}}, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	installBinderJobPolicy(ctx, t, admin, fixture)
	client := clientAs(ctx, t, cfg, stagingRunner)

	for phase, raw := range fixture.BindJobs {
		t.Run(phase, func(t *testing.T) {
			base := decodeFixtureJob(t, raw)
			for _, tc := range []struct {
				name string
				edit func(*batchv1.Job)
			}{
				{"arbitrary image", func(j *batchv1.Job) { j.Spec.Template.Spec.Containers[0].Image = "attacker.invalid/phase:latest" }},
				{"arbitrary args", func(j *batchv1.Job) {
					j.Spec.Template.Spec.Containers[0].Args[len(j.Spec.Template.Spec.Containers[0].Args)-1] = "arbitrary"
				}},
				{"wrong target claim", func(j *batchv1.Job) {
					j.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = "voice-nats-jsdata-d20261017a"
				}},
				{"target mount writable", func(j *batchv1.Job) { j.Spec.Template.Spec.Containers[0].VolumeMounts[0].ReadOnly = false }},
				{"target volume writable", func(j *batchv1.Job) { j.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ReadOnly = false }},
				{"source claim co-mounted", func(j *batchv1.Job) {
					j.Spec.Template.Spec.Volumes = append(j.Spec.Template.Spec.Volumes, corev1.Volume{Name: "source", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "voice-nats-jsdata-d20261017a", ReadOnly: true}}})
				}},
				{"filesystem group", func(j *batchv1.Job) { j.Spec.Template.Spec.SecurityContext.FSGroup = i64ptr(65532) }},
				{"token enabled", func(j *batchv1.Job) { j.Spec.Template.Spec.AutomountServiceAccountToken = boolptr(true) }},
				{"nodeName bypasses scheduler", func(j *batchv1.Job) { j.Spec.Template.Spec.NodeName = "k3s-worker-1" }},
				{"wrong node affinity", func(j *batchv1.Job) {
					j.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values[0] = "other-node"
				}},
				{"extra container", func(j *batchv1.Job) {
					j.Spec.Template.Spec.Containers = append(j.Spec.Template.Spec.Containers, corev1.Container{Name: "sidecar", Image: "attacker.invalid/sidecar"})
				}},
				{"wrong actor", func(j *batchv1.Job) {}},
			} {
				if !t.Run(tc.name, func(t *testing.T) {
					candidate := base.DeepCopy()
					tc.edit(candidate)
					requestClient := client
					if tc.name == "wrong actor" {
						requestClient = clientAs(ctx, t, cfg, "system:serviceaccount:voice-staging:default")
					}
					if err := createJob(ctx, requestClient, candidate); err == nil || !strings.Contains(err.Error(), binderPolicyName) {
						t.Fatalf("mutated binder Job was not denied by VAP: %v", err)
					}
				}) {
					return
				}
			}
			if err := createJob(ctx, client, base); err != nil {
				t.Fatalf("producer binder Job rejected: %v", err)
			}
		})
	}
}

func decodeFixtureJob(t *testing.T, raw map[string]interface{}) *batchv1.Job {
	t.Helper()
	b, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var job batchv1.Job
	if err := json.Unmarshal(b, &job); err != nil {
		t.Fatal(err)
	}
	return &job
}

func installBinderJobPolicy(ctx context.Context, t *testing.T, client *kubernetes.Clientset, fixture producerFixture) {
	t.Helper()
	fail, equivalent, scope := admissionv1.Fail, admissionv1.Equivalent, admissionv1.NamespacedScope
	validations := []admissionv1.Validation{{Expression: "request.namespace == 'voice-staging'", Message: binderDenyMessage},
		{Expression: "request.userInfo.username == '" + stagingRunner + "'", Message: binderDenyMessage}}
	for phase, raw := range fixture.BindJobs {
		for _, expression := range exactJobCELExpressions(raw) {
			if strings.HasPrefix(expression, "object.spec.template.spec.containers[0].image == ") {
				expression = "has(object.spec.template.spec.containers[0].image) && object.spec.template.spec.containers[0].image == params.data['phase_image']"
			}
			validations = append(validations, admissionv1.Validation{Expression: "object.metadata.labels['voice.io/nats-job-phase'] != '" + phase + "' || (" + expression + ")", Message: binderDenyMessage})
		}
	}
	for _, expression := range []string{
		"!has(object.spec.template.spec.nodeName)",
		"!has(object.spec.template.spec.securityContext.fsGroup)",
		"!has(object.spec.template.spec.hostNetwork) || object.spec.template.spec.hostNetwork == false",
		"!has(object.spec.template.spec.shareProcessNamespace) || object.spec.template.spec.shareProcessNamespace == false",
		"!has(object.spec.template.spec.initContainers) || object.spec.template.spec.initContainers.size() == 0",
		"!has(object.spec.template.spec.ephemeralContainers) || object.spec.template.spec.ephemeralContainers.size() == 0",
		"!has(object.spec.template.spec.volumes[0].persistentVolumeClaim.readOnly) || object.spec.template.spec.volumes[0].persistentVolumeClaim.readOnly == true",
	} {
		validations = append(validations, admissionv1.Validation{Expression: expression, Message: binderDenyMessage})
	}
	paramKind := &admissionv1.ParamKind{APIVersion: "v1", Kind: "ConfigMap"}
	policy := &admissionv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: binderPolicyName}, Spec: admissionv1.ValidatingAdmissionPolicySpec{
		FailurePolicy: &fail, ParamKind: paramKind, MatchConstraints: &admissionv1.MatchResources{MatchPolicy: &equivalent,
			NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}},
			ResourceRules: []admissionv1.NamedRuleWithOperations{{RuleWithOperations: admissionv1.RuleWithOperations{
				Rule:       admissionv1.Rule{APIGroups: []string{"batch"}, APIVersions: []string{"v1"}, Resources: []string{"jobs"}, Scope: &scope},
				Operations: []admissionv1.OperationType{admissionv1.Create}}}},
		}, Validations: validations}}
	if _, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(ctx, policy, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create binder Job VAP: %v", err)
	}
	deny := admissionv1.Deny
	notFound := admissionv1.DenyAction
	binding := &admissionv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: binderBindingName}, Spec: admissionv1.ValidatingAdmissionPolicyBindingSpec{
		PolicyName: binderPolicyName, MatchResources: &admissionv1.MatchResources{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}}},
		ParamRef: &admissionv1.ParamRef{Name: binderParamsName, Namespace: namespace, ParameterNotFoundAction: &notFound}, ValidationActions: []admissionv1.ValidationAction{deny}}}
	if _, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(ctx, binding, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create binder VAP Deny binding: %v", err)
	}
	served, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, binderPolicyName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if served.Spec.FailurePolicy == nil || *served.Spec.FailurePolicy != admissionv1.Fail || len(served.Spec.Validations) != len(validations) {
		t.Fatalf("served binder VAP differs from fail-closed source: %#v", served.Spec)
	}
	servedBinding, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, binderBindingName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(servedBinding.Spec.ValidationActions) != 1 || servedBinding.Spec.ValidationActions[0] != admissionv1.Deny {
		t.Fatalf("served binding is not Deny: %#v", servedBinding.Spec)
	}
	if servedBinding.Spec.ParamRef == nil || servedBinding.Spec.ParamRef.ParameterNotFoundAction == nil || *servedBinding.Spec.ParamRef.ParameterNotFoundAction != admissionv1.DenyAction {
		t.Fatalf("served binding does not fail closed on missing image parameter: %#v", servedBinding.Spec)
	}
	time.Sleep(time.Second)
}

func exactJobCELExpressions(job map[string]interface{}) []string {
	var clauses []string
	var walk func(string, interface{})
	walk = func(path string, value interface{}) {
		switch v := value.(type) {
		case map[string]interface{}:
			keys := make([]string, 0, len(v))
			for key := range v {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			if isCELMapPath(path) {
				clauses = append(clauses, fmt.Sprintf("%s.size() == %d", path, len(keys)))
			}
			for _, key := range keys {
				child := path + "." + key
				if isCELMapPath(path) || strings.ContainsAny(key, "-./") {
					child = path + "[" + strconv.Quote(key) + "]"
				}
				walk(child, v[key])
			}
		case []interface{}:
			clauses = append(clauses, fmt.Sprintf("%s.size() == %d", path, len(v)))
			for i, item := range v {
				walk(fmt.Sprintf("%s[%d]", path, i), item)
			}
		default:
			encoded, _ := json.Marshal(v)
			if v == false {
				clauses = append(clauses, "!has("+path+") || "+path+" == false")
			} else if v == true {
				clauses = append(clauses, "has("+path+") && "+path+" == true")
			} else {
				clauses = append(clauses, path+" == "+string(encoded))
			}
		}
	}
	walk("object", job)
	return clauses
}

func isCELMapPath(path string) bool {
	return strings.HasSuffix(path, ".metadata.labels") || strings.HasSuffix(path, ".selector.matchLabels") ||
		strings.HasSuffix(path, ".resources.requests") || strings.HasSuffix(path, ".resources.limits") ||
		strings.HasSuffix(path, ".resources")
}
