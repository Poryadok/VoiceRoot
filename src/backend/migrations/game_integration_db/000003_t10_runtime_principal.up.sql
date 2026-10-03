GRANT CONNECT ON DATABASE game_integration_db TO gameintegration_runtime;
GRANT USAGE ON SCHEMA public TO gameintegration_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO gameintegration_runtime;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO gameintegration_runtime;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO gameintegration_runtime;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO gameintegration_runtime;
