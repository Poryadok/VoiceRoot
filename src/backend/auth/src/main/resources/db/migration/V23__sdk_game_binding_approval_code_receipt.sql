ALTER TABLE sdk_authorizations
  ADD COLUMN game_binding_approval_code_nonce BYTEA,
  ADD COLUMN game_binding_approval_code_ciphertext BYTEA,
  ADD CONSTRAINT sdk_authorizations_binding_code_cipher_pair_check CHECK (
    (game_binding_approval_code_nonce IS NULL) = (game_binding_approval_code_ciphertext IS NULL)
    AND (game_binding_approval_code_nonce IS NULL OR game_binding_intent)
    AND (game_binding_approval_code_nonce IS NULL OR octet_length(game_binding_approval_code_nonce)=12)
  );
