package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"runtime"
	"strconv"
)

// Input and output are private inherited pipes owned by the captured root
// controller. No seed is accepted in arguments/environment or written to logs.
func renewBootstrapPipe(input io.Reader, output io.Writer) error {
	raw, e := io.ReadAll(io.LimitReader(input, 1048577))
	if e != nil || len(raw) == 0 || len(raw) > 1048576 {
		return renewalFailure()
	}
	defer func() {
		for i := range raw {
			raw[i] = 0
		}
	}()
	var r struct {
		Schema     string `json:"schema"`
		Old        string `json:"old_creds"`
		Account    string `json:"account_jwt"`
		Operator   string `json:"operator_jwt"`
		Seed       string `json:"signer_seed"`
		ActorSHA   string `json:"actor_sha256"`
		AccountSHA string `json:"account_sha256"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&r) != nil || decoder.Decode(new(any)) != io.EOF || r.Schema != "voice-nats-existing-bootstrap-renewal-v1" {
		return renewalFailure()
	}
	result, e := renewExistingBootstrap([]byte(r.Old), r.Account, r.Operator, []byte(r.Seed), r.ActorSHA, r.AccountSHA)
	if e != nil {
		return renewalFailure()
	}
	defer func() {
		for i := range result {
			result[i] = 0
		}
	}()
	n, e := output.Write(result)
	if e != nil || n != len(result) {
		return renewalFailure()
	}
	return nil
}
func runRenewBootstrapPipe(descriptor string) (err error) {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 {
		return renewalFailure()
	}
	fd, e := strconv.Atoi(descriptor)
	if e != nil || fd < 3 || fd > 1024 || strconv.Itoa(fd) != descriptor {
		return renewalFailure()
	}
	output := os.NewFile(uintptr(fd), "private-renewal-result")
	if output == nil {
		return renewalFailure()
	}
	defer func() {
		if output.Close() != nil {
			err = renewalFailure()
		}
	}()
	s, e := output.Stat()
	if e != nil || s.Mode()&os.ModeNamedPipe == 0 {
		return renewalFailure()
	}
	return renewBootstrapPipe(os.Stdin, output)
}
