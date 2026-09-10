DROP TABLE IF EXISTS ownership_transfer_proofs;
DROP TRIGGER IF EXISTS backup_codes_security_revision ON backup_codes;
DROP FUNCTION IF EXISTS auth_backup_security_revision();
DROP TRIGGER IF EXISTS accounts_security_revision ON accounts;
DROP FUNCTION IF EXISTS auth_advance_security_revision();
ALTER TABLE accounts DROP COLUMN IF EXISTS security_revision;
