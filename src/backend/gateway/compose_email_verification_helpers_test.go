package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestComposeMailStubBaseURL_UsesExplicitURLThenIsolatedPort(t *testing.T) {
	t.Setenv("VOICE_AUTH_MAIL_STUB_URL", " http://127.0.0.1:29999/ ")
	t.Setenv("VERIFICATION_STUB_PORT", "26666")
	require.Equal(t, "http://127.0.0.1:29999", composeMailStubBaseURL())

	t.Setenv("VOICE_AUTH_MAIL_STUB_URL", "")
	require.Equal(t, "http://127.0.0.1:26666", composeMailStubBaseURL())
}

func TestLiveComposeRateLimitPatterns_ClearOTPBucket(t *testing.T) {
	require.Contains(t, liveComposeRateLimitPatterns, "ratelimit:OTP:*")
}

func TestWaitComposeVerificationCode_PollsRecipientScopedMail(t *testing.T) {
	const email = "regular-user@voice-qa.test"
	requests := 0
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/emails/latest", r.URL.Path)
		require.Equal(t, email, r.URL.Query().Get("to"))
		requests++
		if requests == 1 {
			http.Error(w, `{"error":"email_not_found"}`, http.StatusNotFound)
			return
		}
		_, _ = fmt.Fprint(w, `{"to":["regular-user@voice-qa.test"],"text":"Your Voice verification code is 123456."}`)
	}))
	defer stub.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	code, err := waitComposeVerificationCode(ctx, stub.Client(), stub.URL, email, 2, time.Millisecond)
	require.NoError(t, err)
	require.Equal(t, "123456", code)
	require.Equal(t, 2, requests)
}

func TestComposeVerificationCodeFromMail_RejectsWrongRecipientAndMissingCode(t *testing.T) {
	_, err := composeVerificationCodeFromMail("regular-user@voice-qa.test", []byte(`{"to":["other@voice-qa.test"],"text":"Your Voice verification code is 123456."}`))
	require.Error(t, err)

	_, err = composeVerificationCodeFromMail("regular-user@voice-qa.test", []byte(`{"to":["regular-user@voice-qa.test"],"text":"No code here."}`))
	require.Error(t, err)
}
