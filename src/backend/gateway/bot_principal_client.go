package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

func isBotLifecycleRequest(r *http.Request) bool {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "bots" {
		return false
	}
	if r.Method == http.MethodDelete {
		return len(parts) == 4 || (len(parts) == 6 && parts[4] == "spaces")
	}
	if r.Method == http.MethodPatch {
		return len(parts) == 7 && parts[4] == "chats" && parts[6] == "enabled"
	}
	return r.Method == http.MethodPost && len(parts) == 7 && parts[4] == "spaces" && parts[6] == "install"
}

func (t *transcoder) botLifecycleContext(r *http.Request, request proto.Message, method string) (context.Context, error) {
	return t.auditInitiatingContext(r, request, method, uuid.NewString())
}
