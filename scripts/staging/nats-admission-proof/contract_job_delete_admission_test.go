package admissionproof

import (
	"context"
	"strings"
	"testing"
	"time"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

const (
	contractDeletePolicy = "voice-nats-contract-job-delete"
	contractDeleteBind   = "voice-nats-contract-job-delete"
	contractDeleteDeny   = "contract Job deletion is outside the fixed close contract"
	contractOpLabel      = "voice.io/nats-operation"
	contractNonceLabel   = "voice.io/nats-job-nonce"
	contractPhaseLabel   = "voice.io/nats-job-phase"
	contractAttemptLabel = "voice.io/nats-verification-attempt"
	contractSelectorLabel = "voice.io/nats-phase-job"
)

func TestContractJobDeleteUsesExactActorAndUIDPrecondition(t *testing.T) {
	cfg, stop := startAdmissionAPIServer(t)
	defer stop()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	admin, err := kubernetes.NewForConfig(cfg)
	if err != nil { t.Fatal(err) }
	_, err = admin.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace, Labels: map[string]string{"kubernetes.io/metadata.name": namespace}}}, metav1.CreateOptions{})
	if err != nil { t.Fatal(err) }
	_, err = admin.RbacV1().Roles(namespace).Create(ctx, &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "contract-delete-proof"}, Rules: []rbacv1.PolicyRule{{APIGroups: []string{"batch"}, Resources: []string{"jobs"}, Verbs: []string{"get", "delete"}}}}, metav1.CreateOptions{})
	if err != nil { t.Fatal(err) }
	_, err = admin.RbacV1().RoleBindings(namespace).Create(ctx, &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "contract-delete-runner"}, Subjects: []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: "voice-nats-staging-runner", Namespace: namespace}}, RoleRef: rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: "contract-delete-proof"}}, metav1.CreateOptions{})
	if err != nil { t.Fatal(err) }
	_, err = admin.RbacV1().RoleBindings(namespace).Create(ctx, &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "contract-delete-foreign"}, Subjects: []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: "fixture:other-runner"}}, RoleRef: rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: "contract-delete-proof"}}, metav1.CreateOptions{})
	if err != nil { t.Fatal(err) }
	installContractDeleteVAP(ctx, t, admin)
	runner := clientAs(ctx, t, cfg, stagingRunner)
	job := contractJob("voice-nats-contract-migration-0123456789abcdef", "contract-migration")
	stored, err := admin.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil { t.Fatal(err) }
	uid := stored.UID
	if err := runner.BatchV1().Jobs(namespace).Delete(ctx, stored.Name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}}); err != nil { t.Fatalf("exact UID-preconditioned contract Job delete denied: %v", err) }
	if _, err := admin.BatchV1().Jobs(namespace).Get(ctx, stored.Name, metav1.GetOptions{}); err == nil { t.Fatal("contract Job remained after successful UID delete") }

	t.Run("wrong UID remains apiserver conflict, not VAP proof", func(t *testing.T) {
		current, err := admin.BatchV1().Jobs(namespace).Create(ctx, contractJob("voice-nats-contract-migration-1111111111111111", "contract-migration"), metav1.CreateOptions{})
		if err != nil { t.Fatal(err) }
		wrong := types.UID("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
		err = runner.BatchV1().Jobs(namespace).Delete(ctx, current.Name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &wrong}})
		if err == nil { t.Fatal("wrong UID delete precondition passed") }
		if _, err := admin.BatchV1().Jobs(namespace).Get(ctx, current.Name, metav1.GetOptions{}); err != nil { t.Fatalf("wrong UID delete removed Job: %v", err) }
	})
	t.Run("wrong phase and deterministic name denied by VAP", func(t *testing.T) {
		badPhase := contractJob("voice-nats-contract-migration-2222222222222222", "archive")
		created, err := admin.BatchV1().Jobs(namespace).Create(ctx, badPhase, metav1.CreateOptions{})
		if err != nil { t.Fatal(err) }
		if err := runner.BatchV1().Jobs(namespace).Delete(ctx, created.Name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: uidPtr(created.UID)}}); err == nil || !strings.Contains(err.Error(), contractDeleteDeny) { t.Fatalf("wrong phase Job delete was not denied: %v", err) }
		wrongName := contractJob("voice-nats-archive-3333333333333333", "contract-migration")
		created, err = admin.BatchV1().Jobs(namespace).Create(ctx, wrongName, metav1.CreateOptions{})
		if err != nil { t.Fatal(err) }
		if err := runner.BatchV1().Jobs(namespace).Delete(ctx, created.Name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: uidPtr(created.UID)}}); err == nil || !strings.Contains(err.Error(), contractDeleteDeny) { t.Fatalf("wrong-name Job delete was not denied: %v", err) }
	})
	t.Run("wrong actor denied by VAP", func(t *testing.T) {
		current, err := admin.BatchV1().Jobs(namespace).Create(ctx, contractJob("voice-nats-contract-migration-4444444444444444", "contract-migration"), metav1.CreateOptions{})
		if err != nil { t.Fatal(err) }
		other := clientAs(ctx, t, cfg, "fixture:other-runner")
		if err := other.BatchV1().Jobs(namespace).Delete(ctx, current.Name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: uidPtr(current.UID)}}); err == nil || !strings.Contains(err.Error(), contractDeleteDeny) { t.Fatalf("wrong actor Job delete was not denied: %v", err) }
	})
}

func contractJob(name, phase string) *batchv1.Job {
	labels := map[string]string{contractOpLabel: testOperation, contractNonceLabel: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", contractPhaseLabel: phase, contractSelectorLabel: name, contractAttemptLabel: "deadbeef"}
	backoff := int32(0)
	return &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels}, Spec: batchv1.JobSpec{
		BackoffLimit: &backoff, Selector: &metav1.LabelSelector{MatchLabels: map[string]string{contractSelectorLabel: name}},
		Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labels}, Spec: corev1.PodSpec{RestartPolicy: corev1.RestartPolicyNever, Containers: []corev1.Container{{Name: "control", Image: "fixture@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}}},
	}}
}

func installContractDeleteVAP(ctx context.Context, t *testing.T, c *kubernetes.Clientset) {
	t.Helper()
	fail, equivalent, scope := admissionv1.Fail, admissionv1.Equivalent, admissionv1.NamespacedScope
	expr := `request.namespace == 'voice-staging' && request.userInfo.username == '` + stagingRunner + `' && request.name.matches('^voice-nats-contract-migration-[a-f0-9]{16}$') && oldObject.metadata.name == request.name && oldObject.metadata.labels['voice.io/nats-job-phase'] == 'contract-migration' && oldObject.metadata.labels['voice.io/nats-phase-job'] == request.name && oldObject.metadata.labels['voice.io/nats-operation'].matches('^[a-f0-9]{12}$') && oldObject.metadata.labels['voice.io/nats-job-nonce'].matches('^[a-f0-9]{32}$') && oldObject.metadata.labels['voice.io/nats-verification-attempt'].matches('^[a-f0-9]{8}$') && oldObject.spec.backoffLimit == 0 && oldObject.spec.selector.matchLabels['voice.io/nats-phase-job'] == request.name && oldObject.spec.template.metadata.labels == oldObject.metadata.labels`
	policy := &admissionv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: contractDeletePolicy}, Spec: admissionv1.ValidatingAdmissionPolicySpec{FailurePolicy: &fail, MatchConstraints: &admissionv1.MatchResources{MatchPolicy: &equivalent, NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}}, ResourceRules: []admissionv1.NamedRuleWithOperations{{RuleWithOperations: admissionv1.RuleWithOperations{Rule: admissionv1.Rule{APIGroups: []string{"batch"}, APIVersions: []string{"v1"}, Resources: []string{"jobs"}, Scope: &scope}, Operations: []admissionv1.OperationType{admissionv1.Delete}}}}}, Validations: []admissionv1.Validation{{Expression: expr, Message: contractDeleteDeny}}}}
	if _, err := c.AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(ctx, policy, metav1.CreateOptions{}); err != nil { t.Fatalf("install contract Job DELETE VAP: %v", err) }
	deny := admissionv1.Deny
	binding := &admissionv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: contractDeleteBind}, Spec: admissionv1.ValidatingAdmissionPolicyBindingSpec{PolicyName: contractDeletePolicy, MatchResources: &admissionv1.MatchResources{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}}}, ValidationActions: []admissionv1.ValidationAction{deny}}}
	if _, err := c.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(ctx, binding, metav1.CreateOptions{}); err != nil { t.Fatalf("bind contract Job DELETE VAP: %v", err) }
	time.Sleep(time.Second)
}

