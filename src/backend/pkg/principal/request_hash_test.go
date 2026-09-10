package principal

import (
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

func TestRequestHash_UsesDeterministicProtoSHA256(t *testing.T) {
	first, err := structpb.NewStruct(map[string]any{"a": "one", "b": float64(2)})
	if err != nil {
		t.Fatal(err)
	}
	second, err := structpb.NewStruct(map[string]any{"b": float64(2), "a": "one"})
	if err != nil {
		t.Fatal(err)
	}
	firstHash, err := RequestHash(first)
	if err != nil {
		t.Fatal(err)
	}
	secondHash, err := RequestHash(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstHash != secondHash || len(firstHash) != len("sha256:")+64 {
		t.Fatalf("hashes = %q, %q", firstHash, secondHash)
	}
	if _, err := RequestHash(nil); err == nil {
		t.Fatal("nil protobuf accepted")
	}
}
