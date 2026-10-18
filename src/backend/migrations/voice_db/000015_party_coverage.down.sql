BEGIN;
DROP TABLE voice_party_coverage_head;
DROP TABLE voice_party_coverage_receipts;
DROP TABLE voice_party_coverage_steps;
DROP TABLE voice_party_coverage_operations;
DROP FUNCTION voice_party_coverage_no_rewrite();
COMMIT;
