package admissionproof

import (
	"context"
	"encoding/json"
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
	jobPolicyName  = "voice-nats-archive-job-contract"
	jobBindingName = "voice-nats-archive-job-contract"
	jobDenyMessage = "archive Job differs from the closed producer contract"
	jobParamsName  = "voice-nats-archive-job-fixture"
)

func TestArchiveJobVAPValidatesProducerOutput(t *testing.T) {
	cfg, stop := startAdmissionAPIServer(t)
	defer stop()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = admin.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace, Labels: map[string]string{"kubernetes.io/metadata.name": namespace}}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	createConnectRoleFixture(ctx, t, admin)
	fixture := loadProducerFixture(t)
	if _, err = admin.CoreV1().ConfigMaps(namespace).Create(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: jobParamsName}, Data: map[string]string{"phase_image": testImage}}, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	installArchiveJobPolicy(ctx, t, admin, fixture)
	runner := clientAs(ctx, t, cfg, stagingRunner)
	base := producerJob(t, fixture)
	for _, tc := range []struct {
		name string
		edit func(*batchv1.Job)
	}{
		{"arbitrary image", func(j *batchv1.Job) { j.Spec.Template.Spec.Containers[0].Image = "attacker.invalid/phase:latest" }},
		{"arbitrary command", func(j *batchv1.Job) { j.Spec.Template.Spec.Containers[0].Command[0] = "/bin/sh" }},
		{"arbitrary arguments", func(j *batchv1.Job) { j.Spec.Template.Spec.Containers[0].Args[0] = "--delete-everything" }},
		{"wrong helper uid", func(j *batchv1.Job) { j.Spec.Template.Spec.SecurityContext.RunAsUser = i64ptr(65532) }},
		{"service account token enabled", func(j *batchv1.Job) { j.Spec.Template.Spec.AutomountServiceAccountToken = boolptr(true) }},
		{"wrong source claim", func(j *batchv1.Job) {
			j.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = "voice-nats-jsdata-other"
		}},
		{"source claim writable", func(j *batchv1.Job) {
			j.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ReadOnly = false
			j.Spec.Template.Spec.Containers[0].VolumeMounts[0].ReadOnly = false
		}},
		{"host path volume", func(j *batchv1.Job) {
			j.Spec.Template.Spec.Volumes = append(j.Spec.Template.Spec.Volumes, corev1.Volume{Name: "host", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/etc"}}})
		}},
		{"secret volume", func(j *batchv1.Job) {
			j.Spec.Template.Spec.Volumes = append(j.Spec.Template.Spec.Volumes, corev1.Volume{Name: "secret", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "arbitrary"}}})
		}},
		{"sidecar", func(j *batchv1.Job) {
			j.Spec.Template.Spec.Containers = append(j.Spec.Template.Spec.Containers, corev1.Container{Name: "sidecar", Image: "attacker.invalid/sidecar"})
		}},
		{"fsGroup", func(j *batchv1.Job) { j.Spec.Template.Spec.SecurityContext.FSGroup = i64ptr(10000) }},
		{"nodeName bypass", func(j *batchv1.Job) { j.Spec.Template.Spec.NodeName = "k3s-worker-1" }},
		{"wrong node affinity", func(j *batchv1.Job) {
			j.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values[0] = "other-node"
		}},
		{"wrong actor", func(j *batchv1.Job) {}},
	} {
		if !t.Run(tc.name, func(t *testing.T) {
			candidate := base.DeepCopy()
			tc.edit(candidate)
			client := runner
			if tc.name == "wrong actor" {
				client = clientAs(ctx, t, cfg, "system:serviceaccount:voice-staging:default")
			}
			if err := createJob(ctx, client, candidate); err == nil || !strings.Contains(err.Error(), jobPolicyName) {
				t.Fatalf("mutated archive Job was not denied by VAP: %v", err)
			}
		}) {
			return
		}
	}
	// Selector mismatch is rejected by the real Job API before a controller can
	// consume the template; it is a separate API validation assertion.
	selector := base.DeepCopy()
	selector.Spec.Template.Labels["voice.io/nats-phase-job"] = "other-job"
	if err := createJob(ctx, runner, selector); err == nil || !strings.Contains(err.Error(), "does not match template") {
		t.Fatalf("upstream Job selector validation did not reject mismatch: %v", err)
	}
	if err := createJob(ctx, runner, base); err != nil {
		t.Fatalf("producer archive Job rejected by VAP/Kubernetes: %v", err)
	}
	for _, removedPhase := range []string{"restore", "bind-restore", "verification-session"} {
		candidate := base.DeepCopy()
		candidate.Labels["voice.io/nats-job-phase"] = removedPhase
		candidate.Spec.Template.Labels["voice.io/nats-job-phase"] = removedPhase
		if err := createJob(ctx, runner, candidate); err == nil || !strings.Contains(err.Error(), jobPolicyName) {
			t.Errorf("removed Job phase %q was not denied by VAP: %v", removedPhase, err)
		}
	}
	census := decodeFixtureJob(t, fixture.CensusJob)
	for _, tc := range []struct {
		name string
		edit func(*batchv1.Job)
	}{
		{"source PVC co-mounted", func(j *batchv1.Job) {
			j.Spec.Template.Spec.Volumes = append(j.Spec.Template.Spec.Volumes, corev1.Volume{Name: "source", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "voice-nats-jsdata-d20261017a", ReadOnly: true}}})
		}},
		{"verification target changed", func(j *batchv1.Job) {
			j.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = "voice-nats-verify-ffffffffffff"
		}},
		{"verification target volume read only", func(j *batchv1.Job) { j.Spec.Template.Spec.Volumes[0].PersistentVolumeClaim.ReadOnly = true }},
		{"verification target mount read only", func(j *batchv1.Job) { j.Spec.Template.Spec.Containers[0].VolumeMounts[0].ReadOnly = true }},
		{"missing verification fsGroup", func(j *batchv1.Job) { j.Spec.Template.Spec.SecurityContext.FSGroup = nil }},
		{"wrong verification fsGroup", func(j *batchv1.Job) { j.Spec.Template.Spec.SecurityContext.FSGroup = i64ptr(10000) }},
		{"arbitrary census command", func(j *batchv1.Job) { j.Spec.Template.Spec.Containers[0].Args[0] = "--arbitrary" }},
		{"wrong census identity", func(j *batchv1.Job) { j.Spec.Template.Spec.Containers[0].Args[1] = strings.Repeat("f", 12) }},
		{"wrong actor", func(j *batchv1.Job) {}},
	} {
		t.Run("census "+tc.name, func(t *testing.T) {
			candidate := census.DeepCopy()
			tc.edit(candidate)
			client := runner
			if tc.name == "wrong actor" {
				client = clientAs(ctx, t, cfg, "system:serviceaccount:voice-staging:default")
			}
			err := createJob(ctx, client, candidate)
			if err == nil {
				_ = admin.BatchV1().Jobs(namespace).Delete(ctx, candidate.Name, metav1.DeleteOptions{})
				t.Fatalf("mutated census Job was not denied by VAP")
			}
			if !strings.Contains(err.Error(), jobPolicyName) {
				t.Fatalf("mutated census Job failed outside the VAP: %v", err)
			}
		})
	}
	if err := createJob(ctx, runner, census); err != nil {
		t.Fatalf("producer census Job rejected by VAP/Kubernetes: %v", err)
	}
}

func producerJob(t *testing.T, fixture producerFixture) *batchv1.Job {
	t.Helper()
	data, err := json.Marshal(fixture.Job)
	if err != nil {
		t.Fatal(err)
	}
	var job batchv1.Job
	if err := json.Unmarshal(data, &job); err != nil {
		t.Fatal(err)
	}
	return &job
}
func createJob(ctx context.Context, client *kubernetes.Clientset, job *batchv1.Job) error {
	_, err := client.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	return err
}

func installArchiveJobPolicy(ctx context.Context, t *testing.T, client *kubernetes.Clientset, fixture producerFixture) {
	t.Helper()
	fail, equivalent, scope := admissionv1.Fail, admissionv1.Equivalent, admissionv1.NamespacedScope
	validations := []admissionv1.Validation{{Expression: "request.namespace == 'voice-staging'", Message: jobDenyMessage}, {Expression: "request.userInfo.username == '" + stagingRunner + "'", Message: jobDenyMessage}}
	for _, job := range []map[string]interface{}{fixture.Job, fixture.CensusJob} {
		phase := job["metadata"].(map[string]interface{})["labels"].(map[string]interface{})["voice.io/nats-job-phase"].(string)
		for _, expr := range exactJobCELExpressions(job) {
			if strings.HasPrefix(expr, "object.spec.template.spec.containers[0].image == ") {
				expr = "has(object.spec.template.spec.containers[0].image) && object.spec.template.spec.containers[0].image == params.data['phase_image']"
			}
			validations = append(validations, admissionv1.Validation{Expression: "object.metadata.labels['voice.io/nats-job-phase'] != '" + phase + "' || (" + expr + ")", Message: jobDenyMessage})
		}
	}
	for _, expr := range []string{"object.metadata.labels['voice.io/nats-job-phase'] in ['archive','census']", "!has(object.spec.template.spec.nodeName)", "object.metadata.labels['voice.io/nats-job-phase'] != 'archive' || !has(object.spec.template.spec.securityContext.fsGroup)", "object.metadata.labels['voice.io/nats-job-phase'] != 'census' || (!has(object.spec.template.spec.volumes[0].persistentVolumeClaim.readOnly) || object.spec.template.spec.volumes[0].persistentVolumeClaim.readOnly == false)", "object.metadata.labels['voice.io/nats-job-phase'] != 'census' || (!has(object.spec.template.spec.containers[0].volumeMounts[0].readOnly) || object.spec.template.spec.containers[0].volumeMounts[0].readOnly == false)", "!has(object.spec.template.spec.hostNetwork) || object.spec.template.spec.hostNetwork == false", "!has(object.spec.template.spec.shareProcessNamespace) || object.spec.template.spec.shareProcessNamespace == false", "!has(object.spec.template.spec.initContainers) || object.spec.template.spec.initContainers.size() == 0", "!has(object.spec.template.spec.ephemeralContainers) || object.spec.template.spec.ephemeralContainers.size() == 0"} {
		validations = append(validations, admissionv1.Validation{Expression: expr, Message: jobDenyMessage})
	}
	paramKind := &admissionv1.ParamKind{APIVersion: "v1", Kind: "ConfigMap"}
	policy := &admissionv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: jobPolicyName}, Spec: admissionv1.ValidatingAdmissionPolicySpec{FailurePolicy: &fail, ParamKind: paramKind,
		MatchConstraints: &admissionv1.MatchResources{MatchPolicy: &equivalent, NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}}, ResourceRules: []admissionv1.NamedRuleWithOperations{{RuleWithOperations: admissionv1.RuleWithOperations{Rule: admissionv1.Rule{APIGroups: []string{"batch"}, APIVersions: []string{"v1"}, Resources: []string{"jobs"}, Scope: &scope}, Operations: []admissionv1.OperationType{admissionv1.Create}}}}}, Validations: validations}}
	if _, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(ctx, policy, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create archive Job VAP: %v", err)
	}
	notFound, deny := admissionv1.DenyAction, admissionv1.Deny
	binding := &admissionv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: jobBindingName}, Spec: admissionv1.ValidatingAdmissionPolicyBindingSpec{PolicyName: jobPolicyName,
		MatchResources: &admissionv1.MatchResources{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}}}, ParamRef: &admissionv1.ParamRef{Name: jobParamsName, Namespace: namespace, ParameterNotFoundAction: &notFound}, ValidationActions: []admissionv1.ValidationAction{deny}}}
	if _, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(ctx, binding, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create archive Job Deny binding: %v", err)
	}
	served, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, jobPolicyName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if served.Spec.FailurePolicy == nil || *served.Spec.FailurePolicy != admissionv1.Fail || len(served.Spec.Validations) != len(validations) {
		t.Fatalf("served archive Job policy differs from source: %#v", served.Spec)
	}
	servedBinding, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, jobBindingName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(servedBinding.Spec.ValidationActions) != 1 || servedBinding.Spec.ValidationActions[0] != admissionv1.Deny || servedBinding.Spec.ParamRef == nil || servedBinding.Spec.ParamRef.ParameterNotFoundAction == nil || *servedBinding.Spec.ParamRef.ParameterNotFoundAction != admissionv1.DenyAction {
		t.Fatalf("served archive Job binding is not fail-closed: %#v", servedBinding.Spec)
	}
	time.Sleep(time.Second)
}
