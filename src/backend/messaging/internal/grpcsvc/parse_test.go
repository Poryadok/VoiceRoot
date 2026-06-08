package grpcsvc

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestParseUUIDField(t *testing.T) {
	t.Parallel()
	valid := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")

	tests := []struct {
		name    string
		value   string
		wantErr codes.Code
	}{
		{"empty", "", codes.InvalidArgument},
		{"whitespace", "   ", codes.InvalidArgument},
		{"invalid", "not-a-uuid", codes.InvalidArgument},
		{"valid", valid.String(), codes.OK},
		{"trimmed", "  " + valid.String() + "  ", codes.OK},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseUUIDField("field_name", tc.value)
			if tc.wantErr == codes.OK {
				require.NoError(t, err)
				require.Equal(t, valid, got)
				return
			}
			require.Error(t, err)
			require.Equal(t, tc.wantErr, status.Code(err))
		})
	}
}
