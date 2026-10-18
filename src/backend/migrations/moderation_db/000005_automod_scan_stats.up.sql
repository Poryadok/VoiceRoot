-- Real scan evaluations start here; action history cannot reconstruct clean scans.
CREATE TABLE auto_mod_scan_stats (
    singleton BOOLEAN PRIMARY KEY DEFAULT true CHECK (singleton),
    messages_checked BIGINT NOT NULL DEFAULT 0 CHECK (messages_checked >= 0),
    recorded_since TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO auto_mod_scan_stats (singleton) VALUES (true);
