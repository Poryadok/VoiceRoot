-- Roll back the binary first. Reapplying this migration starts a new baseline.
DROP TABLE auto_mod_scan_stats;
