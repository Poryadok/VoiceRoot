package admissionproof

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

const (
	writePolicy            = "voice-nats-api-write-contract"
	writeBind              = "voice-nats-api-write-contract"
	writeDeny              = "runner Kubernetes write is outside the fixed rollout CAS contract"
	rolloutFinalizerPrefix = "voice.io/nats-preservation-"
)

// The proof exercises the exact resource verbs used by the typed consumer:
// JSON Patch for finalizers, deployments and the generation marker, and
// DeleteOptions with UID/resourceVersion preconditions for owned Pods.
// VAP sees the old/new objects, not JSON Patch test operations; apiserver
// optimistic concurrency remains independently exercised by these requests.
func TestTypedRolloutWritesAreClosedByVAP(t *testing.T) {
	cfg, stop := startAdmissionAPIServer(t)
	defer stop()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	admin, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = admin.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: namespace, Labels: map[string]string{"kubernetes.io/metadata.name": namespace},
	}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	createWriteRole(ctx, t, admin)
	installWritePolicy(ctx, t, admin)
	runner := clientAs(ctx, t, cfg, stagingRunner)

	t.Run("Pod finalizer add remove and UID-RV delete", func(t *testing.T) {
		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "voice-nats-pvc-candidate-proof", Namespace: namespace,
			Labels: map[string]string{"app": "voice-nats-pvc-candidate"}, Annotations: map[string]string{}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "nats", Image: "nats:fixture"}}}}
		stored, err := admin.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
		if err != nil {
			t.Fatal(err)
		}
		fin := rolloutFinalizerPrefix + testOperation
		patchPodFinalizers(t, ctx, runner, stored, []string{fin})
		stored, err = admin.CoreV1().Pods(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		patchPodFinalizers(t, ctx, runner, stored, []string{})

		foreign, err := admin.CoreV1().Pods(namespace).Create(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name: "voice-auth-foreign-finalizer", Namespace: namespace, Finalizers: []string{"example.net/keep"}},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "auth", Image: "fixture"}}}}, metav1.CreateOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if err := patchPodFinalizersErr(ctx, runner, foreign, nil); err == nil || !strings.Contains(err.Error(), writeDeny) {
			t.Fatalf("foreign finalizer removal was not denied: %v", err)
		}
		swap, err := admin.CoreV1().Pods(namespace).Create(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name: "voice-auth-finalizer-swap", Namespace: namespace, Finalizers: []string{rolloutFinalizerPrefix + "bbbbbbbbbbbb"}},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "auth", Image: "fixture"}}}}, metav1.CreateOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if err := patchPodFinalizersErr(ctx, runner, swap, []string{rolloutFinalizerPrefix + testOperation}); err == nil || !strings.Contains(err.Error(), writeDeny) {
			t.Fatalf("finalizer swap was not denied: %v", err)
		}

		deleted, err := admin.CoreV1().Pods(namespace).Create(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name: "voice-auth-proof", Namespace: namespace, Labels: map[string]string{"app": "voice-auth"}},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "auth", Image: "fixture"}}}}, metav1.CreateOptions{})
		if err != nil {
			t.Fatal(err)
		}
		rv := deleted.ResourceVersion
		err = runner.CoreV1().Pods(namespace).Delete(ctx, deleted.Name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{
			UID: uidPtr(deleted.UID), ResourceVersion: &rv,
		}})
		if err != nil {
			t.Fatalf("fixed UID/RV Pod delete rejected: %v", err)
		}
		bad, err := admin.CoreV1().Pods(namespace).Create(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name: "voice-bot-proof", Namespace: namespace, Labels: map[string]string{"app": "voice-bot"}},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "bot", Image: "fixture"}}}}, metav1.CreateOptions{})
		if err != nil {
			t.Fatal(err)
		}
		wrongUID := types.UID("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")
		err = runner.CoreV1().Pods(namespace).Delete(ctx, bad.Name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{
			UID: &wrongUID, ResourceVersion: &bad.ResourceVersion,
		}})
		if err == nil {
			t.Fatal("mismatched delete UID passed the apiserver precondition")
		}
	})

	t.Run("canonical deployment replica CAS", func(t *testing.T) {
		dep := rolloutDeployment("voice-nats-pvc-candidate", 1)
		stored, err := admin.AppsV1().Deployments(namespace).Create(ctx, dep, metav1.CreateOptions{})
		if err != nil {
			t.Fatal(err)
		}
		patch := []map[string]interface{}{
			{"op": "test", "path": "/metadata/uid", "value": string(stored.UID)},
			{"op": "test", "path": "/metadata/resourceVersion", "value": stored.ResourceVersion},
			{"op": "test", "path": "/spec/replicas", "value": int32(1)},
			{"op": "replace", "path": "/spec/replicas", "value": int32(0)},
		}
		if _, err := patchDeployment(ctx, runner, dep.Name, patch); err != nil {
			t.Fatalf("canonical replica CAS rejected: %v", err)
		}
		current, err := admin.AppsV1().Deployments(namespace).Get(ctx, dep.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		bad := []map[string]interface{}{{"op": "test", "path": "/metadata/uid", "value": string(current.UID)}, {"op": "replace", "path": "/spec/replicas", "value": int32(9)}}
		if _, err := patchDeployment(ctx, runner, dep.Name, bad); err == nil || !strings.Contains(err.Error(), writeDeny) {
			t.Fatalf("out-of-contract replica count was not denied: %v", err)
		}
	})

	t.Run("generation marker canonical transition preserves selected claim", func(t *testing.T) {
		claim := "voice-nats-jsdata-d20261017a"
		cm, err := admin.CoreV1().ConfigMaps(namespace).Create(ctx, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "voice-nats-generation", Namespace: namespace, Labels: map[string]string{}, Annotations: map[string]string{}},
			Data:       map[string]string{"generation": "fixture-generation", "phase": "active", "dataPVC": claim}}, metav1.CreateOptions{})
		if err != nil {
			t.Fatal(err)
		}
		patch := []map[string]interface{}{{"op": "test", "path": "/metadata/uid", "value": string(cm.UID)},
			{"op": "test", "path": "/metadata/resourceVersion", "value": cm.ResourceVersion},
			{"op": "test", "path": "/data/phase", "value": "active"}, {"op": "test", "path": "/data/dataPVC", "value": claim},
			{"op": "replace", "path": "/data/phase", "value": "rollout-capturing"},
			{"op": "add", "path": "/data/knownRolloutOperation", "value": testOperation}}
		if _, err := patchConfigMap(ctx, runner, cm.Name, patch); err != nil {
			t.Fatalf("canonical marker transition rejected: %v", err)
		}
		changed, err := admin.CoreV1().ConfigMaps(namespace).Get(ctx, cm.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		badClaim := []map[string]interface{}{{"op": "test", "path": "/metadata/uid", "value": string(changed.UID)},
			{"op": "test", "path": "/metadata/resourceVersion", "value": changed.ResourceVersion},
			{"op": "test", "path": "/data/phase", "value": "rollout-capturing"},
			{"op": "test", "path": "/data/dataPVC", "value": claim},
			{"op": "test", "path": "/data/knownRolloutOperation", "value": testOperation},
			{"op": "replace", "path": "/data/phase", "value": "rollout-prepared"},
			{"op": "replace", "path": "/data/dataPVC", "value": "voice-nats-jsdata-d20261018b"}}
		if _, err := patchConfigMap(ctx, runner, cm.Name, badClaim); err == nil || !strings.Contains(err.Error(), writeDeny) {
			t.Fatalf("marker claim switch was not denied: %v", err)
		}
	})

	t.Run("voice-user bootstrap install and restore", func(t *testing.T) {
		dep := userDeployment()
		stored, err := admin.AppsV1().Deployments(namespace).Create(ctx, dep, metav1.CreateOptions{})
		if err != nil {
			t.Fatal(err)
		}
		installed := bootstrapTemplate(stored.Spec.Template, testOperation)
		patch := []map[string]interface{}{{"op": "test", "path": "/metadata/uid", "value": string(stored.UID)},
			{"op": "test", "path": "/metadata/resourceVersion", "value": stored.ResourceVersion},
			{"op": "test", "path": "/spec/replicas", "value": int32(0)},
			{"op": "test", "path": "/spec/template", "value": templateMap(t, stored.Spec.Template)},
			{"op": "replace", "path": "/spec/template", "value": templateMap(t, installed)}}
		if _, err := patchDeployment(ctx, runner, dep.Name, patch); err != nil {
			t.Fatalf("bootstrap install rejected: %v", err)
		}
		current, err := admin.AppsV1().Deployments(namespace).Get(ctx, dep.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		restored, err := runner.AppsV1().Deployments(namespace).Patch(ctx, dep.Name, types.JSONPatchType,
			bootstrapRestorePatch(t, current, stored.Spec.Template), metav1.PatchOptions{})
		if err != nil {
			t.Fatalf("bootstrap exact restore rejected: %v", err)
		}
		if !templateEqual(t, restored.Spec.Template, stored.Spec.Template) {
			t.Fatal("bootstrap restore did not restore original template")
		}
		current, err = admin.AppsV1().Deployments(namespace).Get(ctx, dep.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		wrong := bootstrapTemplate(current.Spec.Template, testOperation)
		wrong.Spec.Containers[0].Image = "attacker/changed-image"
		if _, err := patchDeployment(ctx, runner, dep.Name, []map[string]interface{}{{"op": "test", "path": "/metadata/uid", "value": string(current.UID)}, {"op": "test", "path": "/metadata/resourceVersion", "value": current.ResourceVersion}, {"op": "test", "path": "/spec/replicas", "value": int32(0)}, {"op": "test", "path": "/spec/template", "value": templateMap(t, current.Spec.Template)}, {"op": "replace", "path": "/spec/template", "value": templateMap(t, wrong)}}); err == nil || !strings.Contains(err.Error(), writeDeny) {
			t.Fatalf("bootstrap image mutation was not denied: %v", err)
		}
	})
}

func createWriteRole(ctx context.Context, t *testing.T, c *kubernetes.Clientset) {
	t.Helper()
	_, err := c.RbacV1().Roles(namespace).Create(ctx, &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "write-proof"}, Rules: []rbacv1.PolicyRule{
		{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "patch", "delete"}},
		{APIGroups: []string{""}, Resources: []string{"configmaps"}, Verbs: []string{"get", "patch"}},
		{APIGroups: []string{"apps"}, Resources: []string{"deployments"}, Verbs: []string{"get", "patch"}},
	}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.RbacV1().RoleBindings(namespace).Create(ctx, &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "write-proof"},
		Subjects: []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: "voice-nats-staging-runner", Namespace: namespace}},
		RoleRef:  rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: "write-proof"}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
}

func installWritePolicy(ctx context.Context, t *testing.T, c *kubernetes.Clientset) {
	t.Helper()
	fail, equivalent, scope := admissionv1.Fail, admissionv1.Equivalent, admissionv1.NamespacedScope
	all := "request.namespace == 'voice-staging' && request.userInfo.username == '" + stagingRunner + "'"
	baseRules := []admissionv1.NamedRuleWithOperations{
		{RuleWithOperations: admissionv1.RuleWithOperations{Rule: admissionv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"pods"}, Scope: &scope}, Operations: []admissionv1.OperationType{admissionv1.Update, admissionv1.Delete}}},
		{RuleWithOperations: admissionv1.RuleWithOperations{Rule: admissionv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"configmaps"}, Scope: &scope}, Operations: []admissionv1.OperationType{admissionv1.Update}}},
		{RuleWithOperations: admissionv1.RuleWithOperations{Rule: admissionv1.Rule{APIGroups: []string{"apps"}, APIVersions: []string{"v1"}, Resources: []string{"deployments"}, Scope: &scope}, Operations: []admissionv1.OperationType{admissionv1.Update}}},
	}
	validations := []admissionv1.Validation{
		{Expression: all, Message: writeDeny + " [actor]"},
		{Expression: `request.resource.resource != 'pods' || request.operation != 'UPDATE' || ((request.name.startsWith('voice-nats-pvc-candidate-') || request.name.matches('^voice-(analytics|auth|bot|chat|file|matchmaking|messaging|moderation|notification|realtime|role|search|social|space|story|subscription|user|voice)-[a-z0-9-]+$')) && oldObject.metadata.uid == object.metadata.uid && oldObject.spec == object.spec && ((!has(oldObject.metadata.labels) && !has(object.metadata.labels)) || (has(oldObject.metadata.labels) && has(object.metadata.labels) && oldObject.metadata.labels == object.metadata.labels)) && ((!has(oldObject.metadata.annotations) && !has(object.metadata.annotations)) || (has(oldObject.metadata.annotations) && has(object.metadata.annotations) && oldObject.metadata.annotations == object.metadata.annotations)) && ((!has(oldObject.metadata.ownerReferences) && !has(object.metadata.ownerReferences)) || (has(oldObject.metadata.ownerReferences) && has(object.metadata.ownerReferences) && oldObject.metadata.ownerReferences == object.metadata.ownerReferences)) && (has(object.metadata.finalizers) ? object.metadata.finalizers : []).size() <= (has(oldObject.metadata.finalizers) ? oldObject.metadata.finalizers : []).size()+1 && (has(oldObject.metadata.finalizers) ? oldObject.metadata.finalizers : []).size() <= (has(object.metadata.finalizers) ? object.metadata.finalizers : []).size()+1 && (has(object.metadata.finalizers) ? object.metadata.finalizers : []).all(f, f in (has(oldObject.metadata.finalizers) ? oldObject.metadata.finalizers : []) || (f.startsWith('voice.io/nats-preservation-') && f.matches('^voice.io/nats-preservation-[a-f0-9]{12}$'))) && (has(oldObject.metadata.finalizers) ? oldObject.metadata.finalizers : []).all(f, f in (has(object.metadata.finalizers) ? object.metadata.finalizers : []) || (f.startsWith('voice.io/nats-preservation-') && f.matches('^voice.io/nats-preservation-[a-f0-9]{12}$'))))`, Message: writeDeny + " [pod-finalizer]"},
		{Expression: `request.resource.resource != 'pods' || request.operation != 'UPDATE' || (((has(object.metadata.finalizers) ? object.metadata.finalizers : []).filter(f, !(f in (has(oldObject.metadata.finalizers) ? oldObject.metadata.finalizers : []))).size() == 0 && (has(oldObject.metadata.finalizers) ? oldObject.metadata.finalizers : []).filter(f, !(f in (has(object.metadata.finalizers) ? object.metadata.finalizers : []))).size() == 0) || ((has(object.metadata.finalizers) ? object.metadata.finalizers : []).filter(f, !(f in (has(oldObject.metadata.finalizers) ? oldObject.metadata.finalizers : []))).size() == 1 && (has(oldObject.metadata.finalizers) ? oldObject.metadata.finalizers : []).filter(f, !(f in (has(object.metadata.finalizers) ? object.metadata.finalizers : []))).size() == 0) || ((has(object.metadata.finalizers) ? object.metadata.finalizers : []).filter(f, !(f in (has(oldObject.metadata.finalizers) ? oldObject.metadata.finalizers : []))).size() == 0 && (has(oldObject.metadata.finalizers) ? oldObject.metadata.finalizers : []).filter(f, !(f in (has(object.metadata.finalizers) ? object.metadata.finalizers : []))).size() == 1))`, Message: writeDeny + " [pod-finalizer-delta]"},
		{Expression: `request.resource.resource != 'pods' || request.operation != 'DELETE' || (request.name.matches('^(voice-nats-pvc-candidate|voice-(analytics|auth|bot|chat|file|matchmaking|messaging|moderation|notification|realtime|role|search|social|space|story|subscription|user|voice))-[a-z0-9-]+$'))`, Message: writeDeny + " [pod-delete]"},
		{Expression: `request.resource.resource != 'configmaps' || (request.name == 'voice-nats-generation' && object.metadata.uid == oldObject.metadata.uid && ((!has(oldObject.metadata.labels) && !has(object.metadata.labels)) || (has(oldObject.metadata.labels) && has(object.metadata.labels) && oldObject.metadata.labels == object.metadata.labels)) && object.data.dataPVC == oldObject.data.dataPVC && object.data.generation == oldObject.data.generation && ((oldObject.data.phase == 'active' && object.data.phase == 'rollout-capturing' && !('knownRolloutOperation' in oldObject.data) && object.data.knownRolloutOperation.matches('^[a-f0-9]{12}$') && object.data.size() == oldObject.data.size()+1) || (oldObject.data.phase == 'rollout-capturing' && object.data.phase == 'rollout-prepared' && object.data.knownRolloutOperation == oldObject.data.knownRolloutOperation && object.data.size() == oldObject.data.size()) || (oldObject.data.phase == 'rollout-prepared' && object.data.phase == 'rollout-applying' && object.data.knownRolloutOperation == oldObject.data.knownRolloutOperation && object.data.size() == oldObject.data.size()) || (oldObject.data.phase == 'rollout-applying' && object.data.phase == 'rollout-verified' && object.data.knownRolloutOperation == oldObject.data.knownRolloutOperation && object.data.size() == oldObject.data.size()) || (oldObject.data.phase == 'rollout-verified' && object.data.phase == 'active' && !('knownRolloutOperation' in object.data) && object.data.size()+1 == oldObject.data.size())) && oldObject.data.size() in [3,4] && object.data.size() in [3,4])`, Message: writeDeny + " [marker]"},
		{Expression: `request.resource.resource != 'deployments' || (request.name in ['voice-nats-pvc-candidate','voice-analytics','voice-auth','voice-bot','voice-chat','voice-file','voice-matchmaking','voice-messaging','voice-moderation','voice-notification','voice-realtime','voice-role','voice-search','voice-social','voice-space','voice-story','voice-subscription','voice-user','voice-voice'] && object.metadata.uid == oldObject.metadata.uid && ((!has(oldObject.metadata.labels) && !has(object.metadata.labels)) || (has(oldObject.metadata.labels) && has(object.metadata.labels) && oldObject.metadata.labels == object.metadata.labels)) && ((!has(oldObject.metadata.annotations) && !has(object.metadata.annotations)) || (has(oldObject.metadata.annotations) && has(object.metadata.annotations) && oldObject.metadata.annotations == object.metadata.annotations)) && object.spec.selector == oldObject.spec.selector && object.spec.strategy == oldObject.spec.strategy && object.spec.replicas in [0,1] && (object.spec.template == oldObject.spec.template || (request.name == 'voice-user' && object.spec.replicas == oldObject.spec.replicas && object.spec.template.spec.containers.size() == oldObject.spec.template.spec.containers.size() && ((has(object.spec.template.spec.volumes) ? object.spec.template.spec.volumes : []) == (has(oldObject.spec.template.spec.volumes) ? oldObject.spec.template.spec.volumes : [])) && ((has(object.spec.template.spec.initContainers) ? object.spec.template.spec.initContainers : []) == (has(oldObject.spec.template.spec.initContainers) ? oldObject.spec.template.spec.initContainers : [])) && object.spec.template.metadata.labels == oldObject.spec.template.metadata.labels && object.spec.template.metadata.annotations.size() <= oldObject.spec.template.metadata.annotations.size()+1 && oldObject.spec.template.metadata.annotations.size() <= object.spec.template.metadata.annotations.size()+1 && object.spec.template.metadata.annotations.all(k,v, k == 'voice.io/nats-user-space-bootstrap' || oldObject.spec.template.metadata.annotations[k] == v) && object.spec.template.spec.containers.all(c, c.name != 'user' || (c.image == oldObject.spec.template.spec.containers.filter(x, x.name == 'user')[0].image && c.command == oldObject.spec.template.spec.containers.filter(x, x.name == 'user')[0].command && c.args == oldObject.spec.template.spec.containers.filter(x, x.name == 'user')[0].args && c.volumeMounts == oldObject.spec.template.spec.containers.filter(x, x.name == 'user')[0].volumeMounts && c.securityContext == oldObject.spec.template.spec.containers.filter(x, x.name == 'user')[0].securityContext && c.ports == oldObject.spec.template.spec.containers.filter(x, x.name == 'user')[0].ports && c.resources == oldObject.spec.template.spec.containers.filter(x, x.name == 'user')[0].resources && (((!('voice.io/nats-user-space-bootstrap' in oldObject.spec.template.metadata.annotations)) && object.spec.template.metadata.annotations['voice.io/nats-user-space-bootstrap'].matches('^[a-f0-9]{12}$') && c.env.size() == oldObject.spec.template.spec.containers.filter(x, x.name == 'user')[0].env.size()+1 && c.env[c.env.size()-1].name == 'SPACE_GRPC_ADDR' && oldObject.spec.template.spec.containers.filter(x, x.name == 'user')[0].env.all(v, v in c.env)) || (oldObject.spec.template.metadata.annotations['voice.io/nats-user-space-bootstrap'].matches('^[a-f0-9]{12}$') && !('voice.io/nats-user-space-bootstrap' in object.spec.template.metadata.annotations) && oldObject.spec.template.spec.containers.filter(x, x.name == 'user')[0].env.size() == c.env.size()+1 && oldObject.spec.template.spec.containers.filter(x, x.name == 'user')[0].env.all(v, v in c.env || v.name == 'SPACE_GRPC_ADDR') && c.env.all(v, v in oldObject.spec.template.spec.containers.filter(x, x.name == 'user')[0].env))))))))`, Message: writeDeny + " [deployment]"},
	}
	policy := &admissionv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: writePolicy}, Spec: admissionv1.ValidatingAdmissionPolicySpec{FailurePolicy: &fail,
		MatchConstraints: &admissionv1.MatchResources{MatchPolicy: &equivalent, NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}}, ResourceRules: baseRules}, Validations: validations}}
	if _, err := c.AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(ctx, policy, metav1.CreateOptions{}); err != nil {
		t.Fatalf("install typed write VAP: %v", err)
	}
	deny := admissionv1.Deny
	binding := &admissionv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: writeBind}, Spec: admissionv1.ValidatingAdmissionPolicyBindingSpec{PolicyName: writePolicy, MatchResources: &admissionv1.MatchResources{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}}}, ValidationActions: []admissionv1.ValidationAction{deny}}}
	if _, err := c.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(ctx, binding, metav1.CreateOptions{}); err != nil {
		t.Fatalf("bind typed write VAP: %v", err)
	}
	time.Sleep(time.Second)
}

func patchPodFinalizers(t *testing.T, ctx context.Context, c *kubernetes.Clientset, p *corev1.Pod, next []string) {
	t.Helper()
	if err := patchPodFinalizersErr(ctx, c, p, next); err != nil {
		t.Fatalf("owned finalizer patch: %v", err)
	}
}

func patchPodFinalizersErr(ctx context.Context, c *kubernetes.Clientset, p *corev1.Pod, next []string) error {
	patch := []map[string]interface{}{{"op": "test", "path": "/metadata/uid", "value": string(p.UID)}, {"op": "test", "path": "/metadata/resourceVersion", "value": p.ResourceVersion}}
	if len(p.Finalizers) > 0 {
		patch = append(patch, map[string]interface{}{"op": "test", "path": "/metadata/finalizers", "value": p.Finalizers}, map[string]interface{}{"op": "replace", "path": "/metadata/finalizers", "value": next})
	} else {
		patch = append(patch, map[string]interface{}{"op": "add", "path": "/metadata/finalizers", "value": next})
	}
	data, _ := json.Marshal(patch)
	_, err := c.CoreV1().Pods(namespace).Patch(ctx, p.Name, types.JSONPatchType, data, metav1.PatchOptions{})
	return err
}

func patchDeployment(ctx context.Context, c *kubernetes.Clientset, name string, patch []map[string]interface{}) (*appsv1.Deployment, error) {
	data, _ := json.Marshal(patch)
	return c.AppsV1().Deployments(namespace).Patch(ctx, name, types.JSONPatchType, data, metav1.PatchOptions{})
}
func patchConfigMap(ctx context.Context, c *kubernetes.Clientset, name string, patch []map[string]interface{}) (*corev1.ConfigMap, error) {
	data, _ := json.Marshal(patch)
	return c.CoreV1().ConfigMaps(namespace).Patch(ctx, name, types.JSONPatchType, data, metav1.PatchOptions{})
}
func uidPtr(v types.UID) *types.UID { return &v }
func rolloutDeployment(name string, replicas int32) *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: map[string]string{}, Annotations: map[string]string{}}, Spec: appsv1.DeploymentSpec{
		Replicas: &replicas, Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": name}},
		Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": name}, Annotations: map[string]string{}},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "main", Image: "fixture"}}}},
	}}
}
func userDeployment() *appsv1.Deployment {
	d := rolloutDeployment("voice-user", 0)
	d.Spec.Template.Annotations["fixture"] = "stable"
	d.Spec.Template.Spec.Volumes = []corev1.Volume{{Name: "config", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}}
	d.Spec.Template.Spec.Containers = []corev1.Container{{Name: "user", Image: "voice-user@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Command: []string{"/voice-user"}, Args: []string{"serve"}, VolumeMounts: []corev1.VolumeMount{{Name: "config", MountPath: "/config", ReadOnly: true}},
		SecurityContext: &corev1.SecurityContext{RunAsNonRoot: boolptr(true)}, Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 8080}},
		Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10m")}},
		Env:       []corev1.EnvVar{{Name: "PORT", Value: "8080"}}}}
	return d
}
func bootstrapTemplate(in corev1.PodTemplateSpec, op string) corev1.PodTemplateSpec {
	out := in.DeepCopy()
	if out.Annotations == nil {
		out.Annotations = map[string]string{}
	}
	out.Annotations["voice.io/nats-user-space-bootstrap"] = op
	for i := range out.Spec.Containers {
		if out.Spec.Containers[i].Name == "user" {
			out.Spec.Containers[i].Env = append(out.Spec.Containers[i].Env, corev1.EnvVar{Name: "SPACE_GRPC_ADDR", Value: ""})
		}
	}
	return *out
}
func templateMap(t *testing.T, v corev1.PodTemplateSpec) map[string]interface{} {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]interface{}
	if err = json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
func templateEqual(t *testing.T, a, b corev1.PodTemplateSpec) bool {
	t.Helper()
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}
func bootstrapRestorePatch(t *testing.T, current *appsv1.Deployment, original corev1.PodTemplateSpec) []byte {
	t.Helper()
	rows := []map[string]interface{}{{"op": "test", "path": "/metadata/uid", "value": string(current.UID)}, {"op": "test", "path": "/metadata/resourceVersion", "value": current.ResourceVersion}, {"op": "test", "path": "/spec/replicas", "value": int32(0)}, {"op": "test", "path": "/spec/template", "value": templateMap(t, current.Spec.Template)}, {"op": "replace", "path": "/spec/template", "value": templateMap(t, original)}}
	b, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
