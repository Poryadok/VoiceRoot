package admissionproof

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	controllerFixtureUser = "system:serviceaccount:voice-staging:job-controller-fixture"
	podParamsName         = "voice-nats-admission-fixture"
	podPolicyID           = "voice-nats-helper-pod-contract"
	podBindingID          = "voice-nats-helper-pod-contract"
	podPolicyMessage      = "helper Pod differs from the admitted Job template"
)

func TestProducerBinderPodsPassVAPAndKubernetes(t *testing.T) {
	cfg, stop := startAdmissionAPIServer(t)
	defer stop()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = admin.CoreV1().Namespaces().Create(ctx, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace, Labels: map[string]string{"kubernetes.io/metadata.name": namespace}}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	createConnectRoleFixture(ctx, t, admin)
	fixture := loadProducerFixture(t)
	if _, err = admin.CoreV1().ConfigMaps(namespace).Create(ctx, &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: podParamsName}, Data: map[string]string{"phase_image": testImage, "job_controller_username": controllerFixtureUser}}, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	installBinderPodVAP(ctx, t, admin, fixture)
	controller := clientAs(ctx, t, cfg, controllerFixtureUser)
	for phase, raw := range fixture.BindJobs {
		t.Run(phase, func(t *testing.T) {
			job := decodeFixtureJob(t, raw)
			base := podFromBinderJob(job)
			for _, tc := range []struct {
				name string
				edit func(*v1.Pod)
			}{
				{"arbitrary image", func(p *v1.Pod) { p.Spec.Containers[0].Image = "attacker.invalid/phase:latest" }},
				{"arbitrary command", func(p *v1.Pod) { p.Spec.Containers[0].Command[0] = "/bin/sh" }},
				{"wrong PVC", func(p *v1.Pod) { p.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = "voice-nats-jsdata-d20261017a" }},
				{"writable PVC mount", func(p *v1.Pod) { p.Spec.Containers[0].VolumeMounts[0].ReadOnly = false }},
				{"nodeName bypass", func(p *v1.Pod) { p.Spec.NodeName = "k3s-worker-1" }},
				{"source PVC co-mount", func(p *v1.Pod) {
					p.Spec.Volumes = append(p.Spec.Volumes, v1.Volume{Name: "source", VolumeSource: v1.VolumeSource{PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{ClaimName: "voice-nats-jsdata-d20261017a", ReadOnly: true}}})
				}},
				{"wrong owner Job", func(p *v1.Pod) { p.OwnerReferences[0].Name = "voice-nats-bind-unowned-0123456789abcdef" }},
				{"not Job controller", func(p *v1.Pod) { p.OwnerReferences[0].Controller = boolptr(false) }},
				{"wrong actor", func(p *v1.Pod) {}},
			} {
				if !t.Run(tc.name, func(t *testing.T) {
					candidate := base.DeepCopy()
					tc.edit(candidate)
					actor := controller
					if tc.name == "wrong actor" {
						actor = clientAs(ctx, t, cfg, stagingRunner)
					}
					if _, err := actor.CoreV1().Pods(namespace).Create(ctx, candidate, metav1.CreateOptions{}); err == nil || !strings.Contains(err.Error(), podPolicyID) {
						t.Fatalf("mutated Pod was not denied by VAP: %v", err)
					}
				}) {
					return
				}
			}
			if _, err := controller.CoreV1().Pods(namespace).Create(ctx, base, metav1.CreateOptions{}); err != nil {
				t.Fatalf("producer Job-template Pod rejected: %v", err)
			}
		})
	}
	censusJob := decodeFixtureJob(t, fixture.CensusJob)
	censusPod := podFromBinderJob(censusJob)
	for _, tc := range []struct {
		name string
		edit func(*v1.Pod)
	}{
		{"wrong image", func(p *v1.Pod) { p.Spec.Containers[0].Image = "attacker.invalid/phase:latest" }},
		{"arbitrary command", func(p *v1.Pod) { p.Spec.Containers[0].Args[0] = "--arbitrary" }},
		{"source PVC co-mounted", func(p *v1.Pod) {
			p.Spec.Volumes = append(p.Spec.Volumes, v1.Volume{Name: "source", VolumeSource: v1.VolumeSource{PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{ClaimName: "voice-nats-jsdata-d20261017a", ReadOnly: true}}})
		}},
		{"wrong verification PVC", func(p *v1.Pod) { p.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = "voice-nats-verify-ffffffffffff" }},
		{"wrong fsGroup", func(p *v1.Pod) { p.Spec.SecurityContext.FSGroup = i64ptr(10000) }},
		{"wrong owner Job", func(p *v1.Pod) { p.OwnerReferences[0].Name = "voice-nats-census-unowned" }},
		{"wrong actor", func(p *v1.Pod) {}},
	} {
		t.Run("census Pod "+tc.name, func(t *testing.T) {
			candidate := censusPod.DeepCopy()
			tc.edit(candidate)
			actor := controller
			if tc.name == "wrong actor" {
				actor = clientAs(ctx, t, cfg, stagingRunner)
			}
			if _, err := actor.CoreV1().Pods(namespace).Create(ctx, candidate, metav1.CreateOptions{}); err == nil || !strings.Contains(err.Error(), podPolicyID) {
				t.Fatalf("mutated census Pod was not denied by VAP: %v", err)
			}
		})
	}
	if _, err := controller.CoreV1().Pods(namespace).Create(ctx, censusPod, metav1.CreateOptions{}); err != nil {
		t.Fatalf("producer census Pod rejected: %v", err)
	}
	restorePod := podFromBinderJob(decodeFixtureJob(t, fixture.RestoreJob))
	for _, tc := range []struct {
		name string
		edit func(*v1.Pod)
	}{
		{"source PVC co-mounted", func(p *v1.Pod) {
			p.Spec.Volumes = append(p.Spec.Volumes, v1.Volume{Name: "source", VolumeSource: v1.VolumeSource{PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{ClaimName: "voice-nats-jsdata-d20261017a", ReadOnly: true}}})
		}},
		{"wrong restore claim attempt", func(p *v1.Pod) {
			p.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = "voice-nats-restore-0123456789ab-deadbeee"
		}},
		{"wrong restore fsGroup", func(p *v1.Pod) { p.Spec.SecurityContext.FSGroup = i64ptr(65532) }},
		{"wrong restore UID", func(p *v1.Pod) { p.Spec.SecurityContext.RunAsUser = i64ptr(65532) }},
		{"wrong actor", func(p *v1.Pod) {}},
	} {
		t.Run("restore Pod "+tc.name, func(t *testing.T) {
			candidate := restorePod.DeepCopy()
			tc.edit(candidate)
			actor := controller
			if tc.name == "wrong actor" {
				actor = clientAs(ctx, t, cfg, stagingRunner)
			}
			if _, err := actor.CoreV1().Pods(namespace).Create(ctx, candidate, metav1.CreateOptions{}); err == nil || !strings.Contains(err.Error(), podPolicyID) {
				t.Fatalf("mutated restore Pod was not denied by VAP: %v", err)
			}
		})
	}
	if _, err := controller.CoreV1().Pods(namespace).Create(ctx, restorePod, metav1.CreateOptions{}); err != nil {
		t.Fatalf("producer restore Pod rejected: %v", err)
	}
	if err := admin.CoreV1().ConfigMaps(namespace).Delete(ctx, podParamsName, metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	missing := podFromBinderJob(decodeFixtureJob(t, fixture.BindJobs["bind-verification"]))
	missing.Name += "-missing"
	missing.OwnerReferences[0].Name += "-missing"
	time.Sleep(time.Second)
	if _, err := controller.CoreV1().Pods(namespace).Create(ctx, missing, metav1.CreateOptions{}); err == nil {
		t.Fatal("Pod admitted after required installed policy parameters disappeared")
	}
}

func podFromBinderJob(job *batchv1.Job) *v1.Pod {
	return &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: job.Name + "-proof1", Namespace: namespace,
		Labels:          map[string]string{"voice.io/nats-operation": testOperation, "voice.io/nats-job-nonce": testNonce, "voice.io/nats-job-phase": job.Labels["voice.io/nats-job-phase"], "voice.io/nats-phase-job": job.Name},
		OwnerReferences: []metav1.OwnerReference{{APIVersion: "batch/v1", Kind: "Job", Name: job.Name, UID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Controller: boolptr(true), BlockOwnerDeletion: boolptr(true)}}}, Spec: *job.Spec.Template.Spec.DeepCopy()}
}

func installBinderPodVAP(ctx context.Context, t *testing.T, client *kubernetes.Clientset, fixture producerFixture) {
	t.Helper()
	fail, equivalent, scope := admissionv1.Fail, admissionv1.Equivalent, admissionv1.NamespacedScope
	validations := []admissionv1.Validation{
		{Expression: "request.namespace == 'voice-staging'", Message: podPolicyMessage},
		{Expression: "request.userInfo.username == params.data['job_controller_username']", Message: podPolicyMessage},
		{Expression: "!has(object.spec.nodeName)", Message: podPolicyMessage},
		{Expression: "!has(object.spec.hostNetwork) || object.spec.hostNetwork == false", Message: podPolicyMessage},
		{Expression: "!has(object.spec.shareProcessNamespace) || object.spec.shareProcessNamespace == false", Message: podPolicyMessage},
		{Expression: "!has(object.spec.initContainers) || object.spec.initContainers.size() == 0", Message: podPolicyMessage},
		{Expression: "!has(object.spec.ephemeralContainers) || object.spec.ephemeralContainers.size() == 0", Message: podPolicyMessage},
		{Expression: "object.metadata.ownerReferences.size() == 1 && object.metadata.ownerReferences[0].apiVersion == 'batch/v1' && object.metadata.ownerReferences[0].kind == 'Job' && object.metadata.ownerReferences[0].controller == true && object.metadata.ownerReferences[0].blockOwnerDeletion == true", Message: podPolicyMessage},
		{Expression: "object.metadata.ownerReferences[0].uid.matches('^[0-9a-f-]{36}$') && object.metadata.ownerReferences[0].name == object.metadata.labels['voice.io/nats-phase-job'] && object.metadata.name.startsWith(object.metadata.ownerReferences[0].name + '-')", Message: podPolicyMessage},
	}
	jobs := map[string]map[string]interface{}{
		"archive": fixture.Job,
		"census":  fixture.CensusJob,
		"restore": fixture.RestoreJob,
	}
	for phase, raw := range fixture.BindJobs {
		jobs[phase] = raw
	}
	for phase, raw := range jobs {
		job := decodeFixtureJob(t, raw)
		encoded, _ := json.Marshal(podFromBinderJob(job))
		var expected map[string]interface{}
		_ = json.Unmarshal(encoded, &expected)
		for _, expr := range exactJobCELExpressions(expected) {
			if strings.HasPrefix(expr, "object.metadata.name == ") || strings.HasPrefix(expr, "object.metadata.ownerReferences[0].uid == ") {
				continue
			}
			if strings.HasPrefix(expr, "object.spec.containers[0].image == ") {
				expr = "has(object.spec.containers[0].image) && object.spec.containers[0].image == params.data['phase_image']"
			}
			validations = append(validations, admissionv1.Validation{Expression: "object.metadata.labels['voice.io/nats-job-phase'] != '" + phase + "' || (" + expr + ")", Message: podPolicyMessage})
		}
	}
	paramKind := &admissionv1.ParamKind{APIVersion: "v1", Kind: "ConfigMap"}
	policy := &admissionv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: podPolicyID}, Spec: admissionv1.ValidatingAdmissionPolicySpec{FailurePolicy: &fail, ParamKind: paramKind,
		MatchConstraints: &admissionv1.MatchResources{MatchPolicy: &equivalent, NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}}, ResourceRules: []admissionv1.NamedRuleWithOperations{{RuleWithOperations: admissionv1.RuleWithOperations{Rule: admissionv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"pods"}, Scope: &scope}, Operations: []admissionv1.OperationType{admissionv1.Create}}}}}, Validations: validations}}
	if _, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(ctx, policy, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create helper Pod VAP: %v", err)
	}
	notFound, deny := admissionv1.DenyAction, admissionv1.Deny
	binding := &admissionv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: podBindingID}, Spec: admissionv1.ValidatingAdmissionPolicyBindingSpec{PolicyName: podPolicyID,
		MatchResources: &admissionv1.MatchResources{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}}}, ParamRef: &admissionv1.ParamRef{Name: podParamsName, Namespace: namespace, ParameterNotFoundAction: &notFound}, ValidationActions: []admissionv1.ValidationAction{deny}}}
	if _, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(ctx, binding, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create helper Pod Deny binding: %v", err)
	}
	served, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Get(ctx, podPolicyID, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if served.Spec.FailurePolicy == nil || *served.Spec.FailurePolicy != admissionv1.Fail || len(served.Spec.Validations) != len(validations) {
		t.Fatalf("served helper Pod VAP differs from source: %#v", served.Spec)
	}
	servedBinding, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Get(ctx, podBindingID, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(servedBinding.Spec.ValidationActions) != 1 || servedBinding.Spec.ValidationActions[0] != admissionv1.Deny || servedBinding.Spec.ParamRef == nil || servedBinding.Spec.ParamRef.ParameterNotFoundAction == nil || *servedBinding.Spec.ParamRef.ParameterNotFoundAction != admissionv1.DenyAction {
		t.Fatalf("helper Pod binding is not fail-closed: %#v", servedBinding.Spec)
	}
	time.Sleep(time.Second)
}
