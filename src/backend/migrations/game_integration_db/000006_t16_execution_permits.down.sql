DROP TABLE player_binding_execution_permits;
ALTER TABLE player_bindings DROP CONSTRAINT player_bindings_status_check;
UPDATE player_bindings SET status='revoked' WHERE status='revoking';
ALTER TABLE player_bindings ADD CONSTRAINT player_bindings_status_check
    CHECK (status IN ('pending', 'active', 'revoked'));
