# Private MM admission wire

The existing descriptor/Observe V1 payloads, Binding, Origin and admission-stamp
hashes are unchanged. An operational client now additionally requires the
`MM_ADMISSION_UDS_MTLS_LAUNCH_V1` transport profile and ALPN
`voice-mm-admission-readback/1`; plaintext has no fallback.

Protected bootstrap calls `LoadValidatedPeerBinding(descriptor, expectations)`.
It reads only `/run/voice/installation/mm-readback-peers-v1.json`. Expectations
must come from independent protected driver observations of both launches,
namespaces, socket mount and key/FD/no-exec custody. They must not be derived from
the public pin file or from an Observe response. Then call
`NewReadbackClient(descriptor, validatedBinding, ownLaunchTLS, timeout)` using only
that process's fresh launch identity. A `Client` literal is nonoperational.

`ReadbackPeerBindingV1` contains public certificates and DRIVER_OBSERVED
provenance. Its hash domain is `voice.mm.readback-peer-binding.v1` followed by
NUL. The binary grammar follows the struct's declared field order, excluding
`PeerBindingHash`: uint32 version/PID/UID/GID in big endian; IDs as 16 bytes;
hashes as 32 bytes; strings and certificate DER prefixed by uint32 byte length.
Each kernel tuple is PID, UID, GID, namespace ID. Scope is a string. Certificates
are bounded canonical base64 in JSON and individually bind exact DER and SPKI
hashes. The strict decoder rejects missing, duplicate, unknown and trailing data.
The deterministic golden is
`17adc525f22b8d1afc5c0cd33bc3ac2f0f8932f988c8e9542f981fb434fc53b5`.

`PeerCredentials` captures only SO_PEERCRED on the connected FD. It never reads
another UID's executable. TLS uses normal certificate verification, isolated
exact certificate pools, Ed25519 per-launch certificates, exact leaf/SPKI pins,
TLS1.3 only, no sessions, and one framed exchange per connection. Raw EOF is
distinguished from authenticated close-notify despite Go TLS's web compatibility
behavior. A readback response must bind the expected consumer launch observation.

Linux/amd64 `DenyFurtherExec` applies no-new-privileges and a TSYNC seccomp filter
denying execve, execveat and the x32 ABI while preserving Go thread creation.
Unsupported platforms reject. A caller must install and actually verify the
sandbox before independent launch registration. This helper alone does not
establish private-key/FD isolation or qualify a native/installed launch.

The controller-side server must find a current independent launch registration
by exact kernel tuple before `AuthenticateReadbackServer`, and recheck it after
decoding the authenticated request. A TLS session does not create registration,
mint admission, establish database continuity or qualify an installed cut.
