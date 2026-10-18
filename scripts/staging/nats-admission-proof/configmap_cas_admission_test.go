package admissionproof

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

const (
	bootstrapCASPolicy = "voice-nats-bootstrap-configmap-cas"
	bootstrapCASBind   = "voice-nats-bootstrap-configmap-cas"
	bootstrapCASDeny   = "bootstrap ConfigMap update is outside the fixed script custody CAS"
	bootstrapCASOp     = "voice-nats-preservation/contract-operation"
	bootstrapCASName   = "voice-nats-realtime-bootstrap"
)

func TestBootstrapConfigMapCASUsesRealAPIServerAndClosedVAP(t *testing.T) {
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
	_, err = admin.RbacV1().Roles(namespace).Create(ctx, &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "bootstrap-cas-proof"}, Rules: []rbacv1.PolicyRule{{
		APIGroups: []string{""}, Resources: []string{"configmaps"}, Verbs: []string{"get", "patch"},
	}}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = admin.RbacV1().RoleBindings(namespace).Create(ctx, &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "bootstrap-cas-proof"},
		Subjects: []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: "voice-nats-staging-runner", Namespace: namespace}},
		RoleRef: rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: "bootstrap-cas-proof"},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	installBootstrapCASVAP(ctx, t, admin)
	runner := clientAs(ctx, t, cfg, stagingRunner)
	oldScript, targetScript := "#!/bin/sh\nold\n", "#!/bin/sh\nnew\n"
	stored, err := admin.CoreV1().ConfigMaps(namespace).Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: bootstrapCASName, Namespace: namespace,
			Labels: map[string]string{"app": "nats"}, Annotations: map[string]string{"owner": "preserved"}},
		Data: map[string]string{"bootstrap.sh": oldScript, "other": "keep"},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	patch := bootstrapCASPatch(stored, targetScript, map[string]string{"owner": "preserved", bootstrapCASOp: testOperation})
	changed, err := patchBootstrapConfigMap(ctx, runner, stored.Name, patch)
	if err != nil {
		t.Fatalf("fixed bootstrap script CAS was denied: %v", err)
	}
	if changed.UID != stored.UID || changed.ResourceVersion == stored.ResourceVersion ||
		changed.Data["bootstrap.sh"] != targetScript || changed.Data["other"] != "keep" ||
		changed.Annotations["owner"] != "preserved" || changed.Annotations[bootstrapCASOp] != testOperation || changed.Labels["app"] != "nats" {
		t.Fatalf("CAS did not preserve object custody: %#v", changed)
	}

	t.Run("unrelated data mutation denied", func(t *testing.T) {
		current, err := admin.CoreV1().ConfigMaps(namespace).Get(ctx, stored.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		bad := []map[string]interface{}{{"op": "test", "path": "/metadata/uid", "value": string(current.UID)},
			{"op": "test", "path": "/metadata/resourceVersion", "value": current.ResourceVersion},
			{"op": "replace", "path": "/data/other", "value": "attacker"}}
		if _, err := patchBootstrapConfigMap(ctx, runner, current.Name, bad); err == nil || !strings.Contains(err.Error(), bootstrapCASDeny) {
			t.Fatalf("unrelated ConfigMap data update was not denied: %v", err)
		}
	})
	t.Run("unrelated annotation mutation denied", func(t *testing.T) {
		current, err := admin.CoreV1().ConfigMaps(namespace).Get(ctx, stored.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		bad := bootstrapCASPatch(current, targetScript, map[string]string{"owner": "preserved", bootstrapCASOp: testOperation, "attacker": "yes"})
		if _, err := patchBootstrapConfigMap(ctx, runner, current.Name, bad); err == nil || !strings.Contains(err.Error(), bootstrapCASDeny) {
			t.Fatalf("unrelated annotation update was not denied: %v", err)
		}
	})
	t.Run("noncanonical ConfigMap name denied", func(t *testing.T) {
		foreign, err := admin.CoreV1().ConfigMaps(namespace).Create(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name: "voice-auth-bootstrap", Namespace: namespace, Annotations: map[string]string{"owner": "preserved"},
		}, Data: map[string]string{"bootstrap.sh": oldScript}}, metav1.CreateOptions{})
		if err != nil {
			t.Fatal(err)
		}
		patch := bootstrapCASPatch(foreign, targetScript, map[string]string{"owner": "preserved", bootstrapCASOp: testOperation})
		if _, err := patchBootstrapConfigMap(ctx, runner, foreign.Name, patch); err == nil || !strings.Contains(err.Error(), bootstrapCASDeny) {
			t.Fatalf("noncanonical ConfigMap update was not denied: %v", err)
		}
	})
	t.Run("stale UID and resourceVersion are enforced by apiserver", func(t *testing.T) {
		current, err := admin.CoreV1().ConfigMaps(namespace).Get(ctx, stored.Name, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		staleUID := bootstrapCASPatch(current, "stale uid", map[string]string{"owner": "preserved", bootstrapCASOp: testOperation})
		staleUID[0]["value"] = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
		if _, err := patchBootstrapConfigMap(ctx, runner, current.Name, staleUID); err == nil {
			t.Fatal("stale UID test operation passed the apiserver")
		}
		staleRV := bootstrapCASPatch(current, "stale version", map[string]string{"owner": "preserved", bootstrapCASOp: testOperation})
		staleRV[1]["value"] = "1"
		if _, err := patchBootstrapConfigMap(ctx, runner, current.Name, staleRV); err == nil {
			t.Fatal("stale resourceVersion test operation passed the apiserver")
		}
	})
}

func installBootstrapCASVAP(ctx context.Context, t *testing.T, c *kubernetes.Clientset) {
	t.Helper()
	fail, equivalent, scope := admissionv1.Fail, admissionv1.Equivalent, admissionv1.NamespacedScope
	shape := `object.metadata.uid == oldObject.metadata.uid && object.immutable == oldObject.immutable && object.data.size() == oldObject.data.size() && oldObject.data.all(k, v, k == 'bootstrap.sh' || object.data[k] == v) && oldObject.data.all(k, v, k in object.data) && ((!has(oldObject.metadata.labels) && !has(object.metadata.labels)) || (has(oldObject.metadata.labels) && has(object.metadata.labels) && oldObject.metadata.labels == object.metadata.labels)) && ((!has(oldObject.metadata.ownerReferences) && !has(object.metadata.ownerReferences)) || (has(oldObject.metadata.ownerReferences) && has(object.metadata.ownerReferences) && oldObject.metadata.ownerReferences == object.metadata.ownerReferences)) && ((!has(oldObject.metadata.finalizers) && !has(object.metadata.finalizers)) || (has(oldObject.metadata.finalizers) && has(object.metadata.finalizers) && oldObject.metadata.finalizers == object.metadata.finalizers)) && 'bootstrap.sh' in object.data && has(object.metadata.annotations) && object.metadata.annotations['voice-nats-preservation/contract-operation'].matches('^[a-f0-9]{12}$') && object.metadata.annotations.size() <= (has(oldObject.metadata.annotations) ? oldObject.metadata.annotations.size() : 0) + 1 && object.metadata.annotations.all(k, v, k == 'voice-nats-preservation/contract-operation' || (has(oldObject.metadata.annotations) && oldObject.metadata.annotations[k] == v)) && (!has(oldObject.metadata.annotations) || oldObject.metadata.annotations.all(k, v, k == 'voice-nats-preservation/contract-operation' || (has(object.metadata.annotations) && object.metadata.annotations[k] == v)))`
	policy := &admissionv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: bootstrapCASPolicy}, Spec: admissionv1.ValidatingAdmissionPolicySpec{
		FailurePolicy: &fail,
		MatchConstraints: &admissionv1.MatchResources{MatchPolicy: &equivalent,
			NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}},
			ResourceRules: []admissionv1.NamedRuleWithOperations{{RuleWithOperations: admissionv1.RuleWithOperations{
				Rule: admissionv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"configmaps"}, Scope: &scope},
				Operations: []admissionv1.OperationType{admissionv1.Update},
			}}},
		},
		Validations: []admissionv1.Validation{
			{Expression: "request.namespace == 'voice-staging' && request.userInfo.username == '" + stagingRunner + "' && request.name in ['voice-nats-realtime-bootstrap','voice-nats-analytics-chat-bootstrap','voice-nats-search-bootstrap']", Message: bootstrapCASDeny + " [actor-name]"},
			{Expression: shape, Message: bootstrapCASDeny + " [shape]"},
		},
	}}
	if _, err := c.AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(ctx, policy, metav1.CreateOptions{}); err != nil {
		t.Fatalf("install bootstrap ConfigMap CAS VAP: %v", err)
	}
	deny := admissionv1.Deny
	binding := &admissionv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: bootstrapCASBind}, Spec: admissionv1.ValidatingAdmissionPolicyBindingSpec{
		PolicyName: bootstrapCASPolicy,
		MatchResources: &admissionv1.MatchResources{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}}},
		ValidationActions: []admissionv1.ValidationAction{deny},
	}}
	if _, err := c.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(ctx, binding, metav1.CreateOptions{}); err != nil {
		t.Fatalf("bind bootstrap ConfigMap CAS VAP: %v", err)
	}
	time.Sleep(time.Second)
}

func bootstrapCASPatch(current *corev1.ConfigMap, target string, annotations map[string]string) []map[string]interface{} {
	annOp := "add"
	if current.Annotations != nil {
		annOp = "replace"
	}
	return []map[string]interface{}{
		{"op": "test", "path": "/metadata/uid", "value": string(current.UID)},
		{"op": "test", "path": "/metadata/resourceVersion", "value": current.ResourceVersion},
		{"op": "replace", "path": "/data/bootstrap.sh", "value": target},
		{"op": annOp, "path": "/metadata/annotations", "value": annotations},
	}
}

func patchBootstrapConfigMap(ctx context.Context, c *kubernetes.Clientset, name string, patch []map[string]interface{}) (*corev1.ConfigMap, error) {
	data, err := json.Marshal(patch)
	if err != nil {
		return nil, err
	}
	return c.CoreV1().ConfigMaps(namespace).Patch(ctx, name, types.JSONPatchType, data, metav1.PatchOptions{})
}
