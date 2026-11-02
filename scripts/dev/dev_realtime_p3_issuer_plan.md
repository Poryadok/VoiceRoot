# Delegated DEV Realtime P3 issuance

The pmd operator needs a fixed Realtime P3 operation through the installed DEV
peer issuer. The existing principal CA and key remain in their original
root-owned archive. The existing Realtime P3 generator creates exactly six
Secrets and one immutable ConfigMap; existing or uncertain members are not
silently replaced.

- [ ] Establish an independently protected pre-drain capability for the
  current principal/media CA identities and seven-member metadata, without
  requiring a future Space server certificate.
- [ ] After Space creation, bind the actual Space leaf and current seven-member
  absence to a fixed root callback and the existing create-only Session.
- [ ] Generate under the existing one-shot P3 reservation, compare guarded
  metadata and trust both before and after, validate the existing P3 packet,
  and require sealed-FD ACK and terminal success before Kubernetes effects.
- [ ] Ensure cancellable worker handles, unknown outcome HOLD, no replay, and
  isolated end-to-end tests with real Journal/Fence, crypto and socket.
- [ ] Independently review all changed executable source and repin; rebuild and
  independently inspect the one-time installer bundle before use.

No implementation or fixture authorizes live issuance, installation, CA
rotation, or export of the root CA key. Actual TT, Kubernetes and CA custody
remain external inputs.
