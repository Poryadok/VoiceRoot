These are public, disposable TLS fixtures for loopback integration tests only.
The private key is intentionally committed test data and must never be deployed.
The server certificate is self-signed, has SAN `localhost`, and expires in 2126.
The unrelated CA certificate tests rejection of an untrusted server certificate.
