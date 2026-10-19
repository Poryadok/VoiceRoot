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

The executable native bootstrap entrypoint is now
`LoadNativeReadbackClient(ctx, exactDescriptorBytes, timeout)`. It consumes fixed
inherited **FD3** exactly once. FD3 must be a private Unix stream from the launch
driver; SO_PEERCRED on this same channel must identify UID0 with a positive PID
in the consumer's namespace. Serving remains nonroot. A different UID, regular
file, missing FD, unmappable driver PID or unsupported sandbox rejects; no env,
arbitrary file or public-pin fallback exists. The loader reinstates CLOEXEC,
installs the mandatory no-exec sandbox before accepting keys, reads one bounded
frame plus EOF within two seconds (or the earlier context deadline), and closes
the original and duplicated private descriptors before returning a client.

The profile is `MM_NATIVE_BOOTSTRAP_PRIVATE_FD_V1`. The private JSON envelope has
exact tags: `version`, `profile`, `descriptor_bytes_sha256`, `recipient`
(actual PID/UID/GID), `independent_launch_expectations`,
`own_client_certificate_der`, and `own_client_private_key_pkcs8`. Expectations
use the exact seven snake-case tags in `privateLaunchExpectations`. Certificate
and PKCS8 bytes use bounded canonical base64. Framing is the existing U32BE JSON
frame; no bytes may follow it. The descriptor hash binds the exact protected
bytes, and the recipient must be this process. The envelope contains only that
replica's fresh Ed25519 client identity, never controller/private authority keys.
The loader obtains public pins only after independent private inputs arrive,
checks all expectations against those pins, then uses the existing exact client
constructor. It does not obtain expectations from the public file or Observe.

The trusted driver creates a per-launch CLOEXEC socketpair using
`mmadmissionctl.NewNativeBootstrapChannel`, passes only `Child` as
`exec.Cmd.ExtraFiles[0]`, and closes its copy of `Child` immediately after Start.
After independently observing the actual launch and installing protected public
pins, it calls `Deliver(ctx, descriptorBytes, actualRecipient, expectations,
ownClientTLS)`. Delivery is private and ends with write shutdown/close. This lets
the process start closed while the driver observes its real PID; no pre-minted
PID or file-as-authority is needed. Independent launch/storage/upstream custody
is still a prerequisite for the driver; the serialization helper cannot create
it. First supported topology shares compatible PID/user namespaces; cross-
namespace driver mapping and installed qualification remain separate gates.

Native roots must validate descriptor/config/executable/argv/local database and
the required service factories in their existing startup sequence. They call the
loader while domain ingress is closed, then perform STARTUP observation before
JWKS/runtime preparation and retain the existing OPEN/fresh admission barriers.
No HTTP/RPC administration or new network listener is introduced by this ABI.

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
