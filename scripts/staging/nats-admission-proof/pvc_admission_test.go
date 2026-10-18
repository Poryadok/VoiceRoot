package admissionproof

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

const (
	pvcPolicyName  = "voice-nats-scratch-pvc-contract"
	pvcBindingName = "voice-nats-scratch-pvc-contract"
	pvcParamName   = "voice-nats-admission-fixture"
	pvcDenyMessage = "PVC differs from the required verification storage contract"
	stagingRunner  = "system:serviceaccount:voice-staging:voice-nats-staging-runner"
	fixturePVCSize = "8589934592"
	fixturePVCMax  = "17179869184"
)

func TestScratchPVCVAPUsesRequiredFixtureParameters(t *testing.T) {
	cfg, stop := startAdmissionAPIServer(t)
	defer stop()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	_, err = admin.RbacV1().Roles(namespace).Create(ctx, &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "admission-proof-pvc"},
		Rules: []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"persistentvolumeclaims"}, Verbs: []string{"create"}}}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = admin.RbacV1().RoleBindings(namespace).Create(ctx, &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "admission-proof-pvc"},
		Subjects: []rbacv1.Subject{
			{Kind: rbacv1.ServiceAccountKind, Name: "voice-nats-staging-runner", Namespace: namespace},
			{Kind: rbacv1.ServiceAccountKind, Name: "default", Namespace: namespace},
		},
		RoleRef: rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: "admission-proof-pvc"}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = admin.CoreV1().ConfigMaps(namespace).Create(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: pvcParamName}, Data: map[string]string{
		"verification_pvc_size_bytes": fixturePVCSize,
		"verification_pvc_max_bytes":  fixturePVCMax,
		"verification_storage_class":  "local-path",
	}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	base := rolePVC()
	sourceFixture := base.DeepCopy()
	sourceFixture.Name = "voice-nats-jsdata-d20261017a"
	if _, err := admin.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, sourceFixture, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create source PVC fixture before policy installation: %v", err)
	}
	installScratchPVCPolicy(ctx, t, admin)
	runnerConfig := *cfg
	runnerConfig.Impersonate.UserName = stagingRunner
	runnerClient, err := kubernetes.NewForConfig(&runnerConfig)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runnerClient.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, base.DeepCopy(), metav1.CreateOptions{}); err != nil {
		t.Fatalf("trusted fixture scratch PVC rejected: %v", err)
	}
	for _, tc := range []struct {
		name string
		edit func(*corev1.PersistentVolumeClaim)
	}{
		{"wrong caller", func(p *corev1.PersistentVolumeClaim) {}},
		{"wrong claim name", func(p *corev1.PersistentVolumeClaim) { p.Name = "voice-nats-verify-0123456789ac" }},
		{"wrong PVC role", func(p *corev1.PersistentVolumeClaim) { p.Labels["voice.io/nats-pvc-role"] = "restore" }},
		{"wrong operation label", func(p *corev1.PersistentVolumeClaim) { p.Labels["voice.io/nats-operation"] = "eeeeeeeeeeee" }},
		{"missing nonce label", func(p *corev1.PersistentVolumeClaim) { delete(p.Labels, "voice.io/nats-job-nonce") }},
		{"extra label", func(p *corev1.PersistentVolumeClaim) { p.Labels["attacker.invalid/extra"] = "yes" }},
		{"annotation", func(p *corev1.PersistentVolumeClaim) {
			p.Annotations = map[string]string{"attacker.invalid/extra": "yes"}
		}},
		{"wrong storage class", func(p *corev1.PersistentVolumeClaim) { class := "other"; p.Spec.StorageClassName = &class }},
		{"wrong exact trusted request", func(p *corev1.PersistentVolumeClaim) {
			p.Spec.Resources.Requests[corev1.ResourceStorage] = resourceQuantity("8589934593")
		}},
		{"claim exceeds required maximum", func(p *corev1.PersistentVolumeClaim) {
			p.Spec.Resources.Requests[corev1.ResourceStorage] = resourceQuantity("17179869185")
		}},
		{"wrong access mode", func(p *corev1.PersistentVolumeClaim) {
			p.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
		}},
		{"extra access mode", func(p *corev1.PersistentVolumeClaim) {
			p.Spec.AccessModes = append(p.Spec.AccessModes, corev1.ReadOnlyMany)
		}},
		{"wrong volume mode", func(p *corev1.PersistentVolumeClaim) { mode := corev1.PersistentVolumeBlock; p.Spec.VolumeMode = &mode }},
		{"bound volume name", func(p *corev1.PersistentVolumeClaim) { p.Spec.VolumeName = "attacker-pv" }},
		{"selector", func(p *corev1.PersistentVolumeClaim) {
			p.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"x": "y"}}
		}},
		{"data source", func(p *corev1.PersistentVolumeClaim) {
			p.Spec.DataSource = &corev1.TypedLocalObjectReference{APIGroup: stringptr("snapshot.storage.k8s.io"), Kind: "VolumeSnapshot", Name: "attacker"}
		}},
		{"storage limit", func(p *corev1.PersistentVolumeClaim) {
			p.Spec.Resources.Limits = corev1.ResourceList{corev1.ResourceStorage: resourceQuantity(fixturePVCSize)}
		}},
		{"volume attributes class", func(p *corev1.PersistentVolumeClaim) { class := "attacker"; p.Spec.VolumeAttributesClassName = &class }},
		{"restore attempt label on verification PVC", func(p *corev1.PersistentVolumeClaim) { p.Labels["voice.io/nats-restore-attempt"] = "abcdef12" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := base.DeepCopy()
			candidate.Name = "voice-nats-verify-" + strings.Repeat("f", 12)
			candidate.Labels["voice.io/nats-operation"] = strings.Repeat("f", 12)
			tc.edit(candidate)
			user := stagingRunner
			if tc.name == "wrong caller" {
				user = "system:serviceaccount:voice-staging:default"
			}
			if err := createPVCAs(ctx, t, cfg, user, candidate); err == nil || !strings.Contains(err.Error(), pvcDenyMessage) {
				t.Fatalf("mutated PVC was not denied by VAP: %v", err)
			}
		})
	}
	// Existing source PVCs are selected by name and never created by this runner.
	source := base.DeepCopy()
	source.Name = "voice-nats-jsdata-d20261017a"
	if err := createPVCAs(ctx, t, cfg, stagingRunner, source); err == nil || !strings.Contains(err.Error(), pvcDenyMessage) {
		t.Fatalf("source PVC CREATE was not denied: %v", err)
	}
	for _, name := range []string{"voice-nats-jsdata-d20261017a", base.Name} {
		stored, err := admin.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			t.Fatalf("read protected PVC fixture %s: %v", name, err)
		}
		changed := stored.DeepCopy()
		changed.Annotations = map[string]string{"attacker.invalid/change": "yes"}
		if _, err := admin.CoreV1().PersistentVolumeClaims(namespace).Update(ctx, changed, metav1.UpdateOptions{}); err == nil || !strings.Contains(err.Error(), pvcDenyMessage) {
			t.Fatalf("protected PVC UPDATE was not denied for %s: %v", name, err)
		}
		if err := admin.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err == nil || !strings.Contains(err.Error(), pvcDenyMessage) {
			t.Fatalf("protected PVC DELETE was not denied for %s: %v", name, err)
		}
	}
	if err := admin.CoreV1().ConfigMaps(namespace).Delete(ctx, pvcParamName, metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	missingParams := base.DeepCopy()
	missingParams.Name = "voice-nats-verify-ffffffffffff"
	missingParams.Labels["voice.io/nats-operation"] = "ffffffffffff"
	if err := createPVCAs(ctx, t, cfg, stagingRunner, missingParams); err == nil {
		t.Fatal("PVC CREATE was admitted after required trusted storage parameters disappeared")
	}
}

func startAdmissionAPIServer(t *testing.T) (*rest.Config, func()) {
	t.Helper()
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
	stop := func() {
		if err := testEnv.Stop(); err != nil {
			t.Errorf("stop upstream Kubernetes API server: %v", err)
		}
	}
	return cfg, stop
}

func rolePVC() *corev1.PersistentVolumeClaim {
	class := "local-path"
	mode := corev1.PersistentVolumeFilesystem
	op := testOperation
	name := "voice-nats-verify-" + op
	size := fixturePVCSize
	labels := map[string]string{"voice.io/nats-operation": op, "voice.io/nats-job-nonce": testNonce, "voice.io/nats-pvc-role": "verification"}
	return &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: namespace, Labels: labels,
	}, Spec: corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, VolumeMode: &mode, StorageClassName: &class,
		Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resourceQuantity(size)}},
	}}
}

func installScratchPVCPolicy(ctx context.Context, t *testing.T, client *kubernetes.Clientset) {
	t.Helper()
	fail, equivalent, scope := admissionv1.Fail, admissionv1.Equivalent, admissionv1.NamespacedScope
	paramKind := &admissionv1.ParamKind{APIVersion: "v1", Kind: "ConfigMap"}
	validations := make([]admissionv1.Validation, 0)
	for i, expression := range scratchPVCExpressions() {
		validations = append(validations, admissionv1.Validation{Expression: expression, Message: pvcDenyMessage + " [clause " + strconv.Itoa(i+1) + "]"})
	}
	policy := &admissionv1.ValidatingAdmissionPolicy{ObjectMeta: metav1.ObjectMeta{Name: pvcPolicyName}, Spec: admissionv1.ValidatingAdmissionPolicySpec{
		FailurePolicy: &fail, ParamKind: paramKind, MatchConstraints: &admissionv1.MatchResources{MatchPolicy: &equivalent,
			NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}},
			ResourceRules: []admissionv1.NamedRuleWithOperations{{RuleWithOperations: admissionv1.RuleWithOperations{
				Rule:       admissionv1.Rule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"persistentvolumeclaims"}, Scope: &scope},
				Operations: []admissionv1.OperationType{admissionv1.Create, admissionv1.Update, admissionv1.Delete},
			}}},
		}, Validations: validations,
	}}
	if _, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(ctx, policy, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create parameterized scratch PVC VAP: %v", err)
	}
	deny, notFound := admissionv1.Deny, admissionv1.DenyAction
	binding := &admissionv1.ValidatingAdmissionPolicyBinding{ObjectMeta: metav1.ObjectMeta{Name: pvcBindingName}, Spec: admissionv1.ValidatingAdmissionPolicyBindingSpec{
		PolicyName: pvcPolicyName, MatchResources: &admissionv1.MatchResources{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": namespace}}},
		ParamRef:          &admissionv1.ParamRef{Name: pvcParamName, Namespace: namespace, ParameterNotFoundAction: &notFound},
		ValidationActions: []admissionv1.ValidationAction{deny},
	}}
	if _, err := client.AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(ctx, binding, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create parameterized fail-closed PVC binding: %v", err)
	}
	time.Sleep(time.Second)
}

func scratchPVCExpressions() []string {
	create := []string{
		"request.namespace == 'voice-staging' && request.userInfo.username == '" + stagingRunner + "'",
		"object.metadata.labels['voice.io/nats-pvc-role'] == 'verification'",
		"object.metadata.name == 'voice-nats-verify-' + object.metadata.labels['voice.io/nats-operation'] && !('voice.io/nats-restore-attempt' in object.metadata.labels) && object.metadata.labels.size() == 3",
		"object.metadata.labels['voice.io/nats-operation'].matches('^[a-f0-9]{12}$')",
		"object.metadata.labels['voice.io/nats-job-nonce'].matches('^[a-f0-9]{32}$')",
		"!has(object.metadata.annotations) || object.metadata.annotations.size() == 0",
		"!has(object.metadata.ownerReferences) || object.metadata.ownerReferences.size() == 0",
		"!has(object.metadata.finalizers) || object.metadata.finalizers.size() == 0 || (object.metadata.finalizers.size() == 1 && object.metadata.finalizers[0] == 'kubernetes.io/pvc-protection')",
		"object.spec.accessModes == ['ReadWriteOnce']",
		"object.spec.volumeMode == 'Filesystem'",
		"object.spec.storageClassName == params.data['verification_storage_class']",
		"object.spec.resources.requests.size() == 1",
		"!has(object.spec.resources.limits) || object.spec.resources.limits.size() == 0",
		"object.spec.resources.requests.storage == params.data['verification_pvc_size_bytes']",
		"quantity(object.spec.resources.requests.storage).isInteger() && quantity(object.spec.resources.requests.storage).asInteger() > 0",
		"quantity(object.spec.resources.requests.storage).asInteger() <= int(params.data['verification_pvc_max_bytes'])",
		"int(params.data['verification_pvc_max_bytes']) > 0",
		"!has(object.spec.volumeName) && !has(object.spec.selector) && !has(object.spec.volumeAttributesClassName)",
		"!has(object.spec.dataSource) && !has(object.spec.dataSourceRef)",
	}
	out := make([]string, 0, len(create)+1)
	for _, expression := range create {
		out = append(out, "request.operation != 'CREATE' || ("+expression+")")
	}
	out = append(out, "request.operation == 'CREATE' || (request.operation in ['UPDATE', 'DELETE'] && !(oldObject.metadata.name.matches('^voice-nats-jsdata-d[0-9]{8}[a-z0-9]{1,8}$') || oldObject.metadata.name.matches('^voice-nats-verify-[a-f0-9]{12}$')))")
	return out
}

func createPVCAs(ctx context.Context, t *testing.T, cfg *rest.Config, username string, pvc *corev1.PersistentVolumeClaim) error {
	t.Helper()
	userConfig := rest.CopyConfig(cfg)
	userConfig.Impersonate.UserName = username
	client, err := kubernetes.NewForConfig(userConfig)
	if err != nil {
		return err
	}
	_, err = client.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
	return err
}

func resourceQuantity(value string) resource.Quantity { return resource.MustParse(value) }

func stringptr(value string) *string { return &value }
