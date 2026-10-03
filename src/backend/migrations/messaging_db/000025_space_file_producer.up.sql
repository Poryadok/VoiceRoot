CREATE TABLE messaging_space_file_producers (
 space_id UUID NOT NULL,
 deletion_operation_id UUID NOT NULL,
 schedule_generation BIGINT NOT NULL CHECK(schedule_generation>0),
 references_bytes BYTEA NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(space_id,deletion_operation_id,schedule_generation)
);
