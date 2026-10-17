package admissionproof

import (
	"context"
	"testing"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func clientAs(ctx context.Context, t *testing.T, cfg *rest.Config, username string) *kubernetes.Clientset {
	t.Helper()
	copy := rest.CopyConfig(cfg)
	copy.Impersonate.UserName = username
	client, err := kubernetes.NewForConfig(copy)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func i64ptr(value int64) *int64 { return &value }
func boolptr(value bool) *bool  { return &value }
