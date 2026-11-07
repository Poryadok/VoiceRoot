package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthJavaCandidateSourceBinding(t *testing.T) {
	repo := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	require.NoError(t, os.MkdirAll(filepath.Join(repo, "src/backend/auth"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".gitignore"), []byte("target/\n"), 0644))
	authSource := filepath.Join(repo, "src/backend/auth/pom.xml")
	require.NoError(t, os.WriteFile(authSource, []byte("original\n"), 0644))
	peerSource := filepath.Join(repo, "src/backend/voice/pb/voice/auth/peer.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(peerSource), 0755))
	require.NoError(t, os.WriteFile(peerSource, []byte("original\n"), 0644))
	git("add", ".")
	git("-c", "user.name=CI", "-c", "user.email=ci@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture")
	head := git("rev-parse", "HEAD")
	testClasses := filepath.Join(repo, "src/backend/auth/target/test-classes")
	classes := filepath.Join(repo, "src/backend/auth/target/classes")
	require.NoError(t, os.MkdirAll(testClasses, 0755))
	require.NoError(t, os.MkdirAll(classes, 0755))
	classpath := testClasses + ":" + classes
	require.NoError(t, authJavaCandidateSource(repo, head, classpath))
	require.Error(t, authJavaCandidateSource(repo, strings.Repeat("a", 40), classpath))
	require.Error(t, authJavaCandidateSource(repo, head, classes+":"+testClasses))
	require.NoError(t, os.WriteFile(authSource, []byte("changed\n"), 0644))
	require.Error(t, authJavaCandidateSource(repo, head, classpath))
	require.NoError(t, os.WriteFile(authSource, []byte("original\n"), 0644))
	require.NoError(t, os.WriteFile(peerSource, []byte("changed\n"), 0644))
	require.Error(t, authJavaCandidateSource(repo, head, classpath))
}
