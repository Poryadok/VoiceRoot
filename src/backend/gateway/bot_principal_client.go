package main

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

func (t *transcoder) botLifecycleContext(r *http.Request, request proto.Message, method string) (context.Context, error) {
	return t.auditInitiatingContext(r, request, method, uuid.NewString())
}
