package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
)

// Reserved for a separately reviewed custody enrollment. A caller-supplied key
// or digest is not an authority. Until this public policy digest is approved in
// a reviewed source change, the production entrypoint fails closed. Disposable
// fixture testing remains available and is always typed FIXTURE_PASS.
const approvedTrustPolicySHA = ""

func main() {
	if e := run(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, "NATS_POPULATED_ACL_PROOF=FAIL code="+e.Error())
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) > 0 && args[0] == "runtime" {
		if len(args) != 2 {
			return failure("runtime_arguments_invalid")
		}
		return runtimePhase(args[1])
	}
	flags := flag.NewFlagSet("nats-populated-proof", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	dir := flags.String("inputs", "", "protected witnessed bundle directory")
	acl := flags.String("acl", "", "reviewed ACL manifest")
	trust := flags.String("approved-trust", "", "independently approved external custody policy")
	fixture := flags.Bool("fixture", false, "disposable testing only; never production PASS")
	if e := flags.Parse(args); e != nil {
		return failure("arguments_invalid")
	}
	if flags.NArg() != 0 {
		return failure("arguments_invalid")
	}
	if !*fixture && !digestPattern.MatchString(approvedTrustPolicySHA) {
		return failure("custody_trust_anchor_not_enrolled")
	}
	in, e := loadInput(*dir, *acl, *trust, approvedTrustPolicySHA, *fixture, time.Now())
	if e != nil {
		return e
	}
	exe, e := os.Executable()
	if e != nil {
		return failure("helper_invalid")
	}
	if e = runSandbox(in, realDocker{}, exe); e != nil {
		return e
	}
	// This marker deliberately cannot be parsed as the existing LIVE proof. It
	// records limited clone scope and never updates any staging gate variable.
	result := struct{ Schema, Kind, Scope, Generation, ReleaseSHA, ACLSHA, ManifestSHA, Outcome string }{schema, in.Manifest.Kind, "isolated-config-clone", in.Manifest.Generation, in.Manifest.ReleaseSHA, in.Manifest.ACLSHA, in.ManifestSHA, "PASS"}
	if *fixture {
		result.Outcome = "FIXTURE_PASS"
	}
	raw, e := json.Marshal(result)
	if e != nil {
		return failure("evidence_encode_failed")
	}
	fmt.Println("NATS_POPULATED_ACL_PROOF=" + string(raw))
	return nil
}
