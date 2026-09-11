BEGIN;

CREATE TABLE voice_room_instances (
    room_id UUID PRIMARY KEY,
    space_id UUID NOT NULL,
    voice_room_id UUID NOT NULL,
    livekit_room_name TEXT NOT NULL UNIQUE,
    state TEXT NOT NULL,
    roster_version BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    closed_at TIMESTAMPTZ NULL,
    CONSTRAINT voice_room_instances_room_id_not_nil CHECK (room_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_room_instances_space_id_not_nil CHECK (space_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_room_instances_voice_room_id_not_nil CHECK (voice_room_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_room_instances_livekit_name_nonblank CHECK (btrim(livekit_room_name) <> ''),
    CONSTRAINT voice_room_instances_state_check CHECK (state IN ('active', 'closed')),
    CONSTRAINT voice_room_instances_roster_version_check CHECK (roster_version >= 0),
    CONSTRAINT voice_room_instances_closed_at_check CHECK (
        (state = 'active' AND closed_at IS NULL)
        OR (state = 'closed' AND closed_at IS NOT NULL)
    )
);

CREATE UNIQUE INDEX voice_room_instances_one_active_logical_room
    ON voice_room_instances (voice_room_id)
    WHERE state = 'active';
CREATE INDEX voice_room_instances_path_idx
    ON voice_room_instances (space_id, voice_room_id);
CREATE INDEX voice_room_instances_recovery_idx
    ON voice_room_instances (state, updated_at);

CREATE TABLE voice_room_memberships (
    profile_id UUID PRIMARY KEY,
    room_id UUID NOT NULL,
    media_epoch UUID NOT NULL UNIQUE,
    space_access_epoch BIGINT NOT NULL,
    role_policy_epoch BIGINT NOT NULL,
    authorization_digest BYTEA NOT NULL,
    can_join BOOLEAN NOT NULL,
    can_publish_audio BOOLEAN NOT NULL,
    can_publish_video BOOLEAN NOT NULL,
    can_publish_screen_share BOOLEAN NOT NULL,
    can_subscribe BOOLEAN NOT NULL,
    can_mute_others BOOLEAN NOT NULL,
    can_deafen_others BOOLEAN NOT NULL,
    can_move_others BOOLEAN NOT NULL,
    can_use_ptt BOOLEAN NOT NULL,
    priority_speaker BOOLEAN NOT NULL,
    latest_grant_expires_at TIMESTAMPTZ NULL,
    joined_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT voice_room_memberships_profile_id_not_nil CHECK (profile_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_room_memberships_room_id_not_nil CHECK (room_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_room_memberships_media_epoch_not_nil CHECK (media_epoch <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_room_memberships_space_access_epoch_check CHECK (space_access_epoch > 0),
    CONSTRAINT voice_room_memberships_role_policy_epoch_check CHECK (role_policy_epoch > 0),
    CONSTRAINT voice_room_memberships_authorization_digest_check CHECK (octet_length(authorization_digest) = 32),
    CONSTRAINT voice_room_memberships_room_fk FOREIGN KEY (room_id)
        REFERENCES voice_room_instances (room_id) ON DELETE RESTRICT
);

CREATE INDEX voice_room_memberships_room_idx ON voice_room_memberships (room_id);

CREATE TABLE voice_lifecycle_operations (
    actor_profile_id UUID NOT NULL,
    operation_id UUID NOT NULL,
    schema_version SMALLINT NOT NULL,
    method TEXT NOT NULL,
    fingerprint BYTEA NOT NULL,
    binding_bytes BYTEA NOT NULL,
    binding_hash BYTEA NOT NULL,
    actor_account_id UUID NOT NULL,
    subject_profile_id UUID NOT NULL,
    space_id UUID NOT NULL,
    source_voice_room_id UUID NULL,
    destination_voice_room_id UUID NULL,
    source_room_id UUID NULL,
    destination_room_id UUID NULL,
    source_media_epoch UUID NULL,
    destination_media_epoch UUID NULL,
    space_access_epoch BIGINT NULL,
    subject_role_policy_epoch BIGINT NULL,
    actor_source_role_policy_epoch BIGINT NULL,
    actor_destination_role_policy_epoch BIGINT NULL,
    authorization_digest BYTEA NULL,
    can_join BOOLEAN NULL,
    can_publish_audio BOOLEAN NULL,
    can_publish_video BOOLEAN NULL,
    can_publish_screen_share BOOLEAN NULL,
    can_subscribe BOOLEAN NULL,
    can_mute_others BOOLEAN NULL,
    can_deafen_others BOOLEAN NULL,
    can_move_others BOOLEAN NULL,
    can_use_ptt BOOLEAN NULL,
    priority_speaker BOOLEAN NULL,
    actor_can_move_source BOOLEAN NULL,
    actor_can_move_destination BOOLEAN NULL,
    decided_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    redis_owner_token BYTEA NOT NULL,
    state TEXT NOT NULL,
    lease_owner UUID NULL,
    lease_until TIMESTAMPTZ NULL,
    lease_fence BIGINT NOT NULL DEFAULT 0,
    quarantine_class TEXT NULL,
    quarantine_detail TEXT NULL,
    quarantined_at TIMESTAMPTZ NULL,
    receipt_outcome TEXT NULL,
    receipt_source_voice_room_id UUID NULL,
    receipt_destination_voice_room_id UUID NULL,
    receipt_room_id UUID NULL,
    receipt_source_roster_version BIGINT NULL,
    receipt_destination_roster_version BIGINT NULL,
    receipt_media_epoch UUID NULL,
    receipt_space_access_epoch BIGINT NULL,
    receipt_role_policy_epoch BIGINT NULL,
    receipt_authorization_digest BYTEA NULL,
    receipt_bytes BYTEA NULL,
    receipt_hash BYTEA NULL,
    completed_at TIMESTAMPTZ NULL,
    replay_until TIMESTAMPTZ NULL,
    PRIMARY KEY (actor_profile_id, operation_id),
    CONSTRAINT voice_lifecycle_operations_actor_profile_id_not_nil CHECK (actor_profile_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_operations_operation_id_not_nil CHECK (operation_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_operations_actor_account_id_not_nil CHECK (actor_account_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_operations_subject_profile_id_not_nil CHECK (subject_profile_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_operations_space_id_not_nil CHECK (space_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_operations_source_voice_room_id_not_nil CHECK (source_voice_room_id IS NULL OR source_voice_room_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_operations_destination_voice_room_id_not_nil CHECK (destination_voice_room_id IS NULL OR destination_voice_room_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_operations_source_room_id_not_nil CHECK (source_room_id IS NULL OR source_room_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_operations_destination_room_id_not_nil CHECK (destination_room_id IS NULL OR destination_room_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_operations_source_media_epoch_not_nil CHECK (source_media_epoch IS NULL OR source_media_epoch <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_operations_destination_media_epoch_not_nil CHECK (destination_media_epoch IS NULL OR destination_media_epoch <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_operations_lease_owner_not_nil CHECK (lease_owner IS NULL OR lease_owner <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_operations_receipt_source_voice_room_id_not_nil CHECK (receipt_source_voice_room_id IS NULL OR receipt_source_voice_room_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_operations_receipt_destination_voice_room_id_not_nil CHECK (receipt_destination_voice_room_id IS NULL OR receipt_destination_voice_room_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_operations_receipt_room_id_not_nil CHECK (receipt_room_id IS NULL OR receipt_room_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_operations_receipt_media_epoch_not_nil CHECK (receipt_media_epoch IS NULL OR receipt_media_epoch <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_operations_space_access_epoch_check CHECK (space_access_epoch IS NULL OR space_access_epoch > 0),
    CONSTRAINT voice_lifecycle_operations_subject_role_policy_epoch_check CHECK (subject_role_policy_epoch IS NULL OR subject_role_policy_epoch > 0),
    CONSTRAINT voice_lifecycle_operations_actor_source_role_policy_epoch_check CHECK (actor_source_role_policy_epoch IS NULL OR actor_source_role_policy_epoch > 0),
    CONSTRAINT voice_lifecycle_operations_actor_destination_role_policy_epoch_check CHECK (actor_destination_role_policy_epoch IS NULL OR actor_destination_role_policy_epoch > 0),
    CONSTRAINT voice_lifecycle_operations_receipt_space_access_epoch_check CHECK (receipt_space_access_epoch IS NULL OR receipt_space_access_epoch > 0),
    CONSTRAINT voice_lifecycle_operations_receipt_role_policy_epoch_check CHECK (receipt_role_policy_epoch IS NULL OR receipt_role_policy_epoch > 0),
    CONSTRAINT voice_lifecycle_operations_receipt_source_roster_version_check CHECK (receipt_source_roster_version IS NULL OR receipt_source_roster_version >= 0),
    CONSTRAINT voice_lifecycle_operations_receipt_destination_roster_version_check CHECK (receipt_destination_roster_version IS NULL OR receipt_destination_roster_version >= 0),
    CONSTRAINT voice_lifecycle_operations_fingerprint_check CHECK (octet_length(fingerprint) = 32),
    CONSTRAINT voice_lifecycle_operations_binding_hash_check CHECK (octet_length(binding_hash) = 32),
    CONSTRAINT voice_lifecycle_operations_authorization_digest_check CHECK (authorization_digest IS NULL OR octet_length(authorization_digest) = 32),
    CONSTRAINT voice_lifecycle_operations_redis_owner_token_check CHECK (octet_length(redis_owner_token) = 32),
    CONSTRAINT voice_lifecycle_operations_receipt_authorization_digest_check CHECK (receipt_authorization_digest IS NULL OR octet_length(receipt_authorization_digest) = 32),
    CONSTRAINT voice_lifecycle_operations_receipt_hash_check CHECK (receipt_hash IS NULL OR octet_length(receipt_hash) = 32),
    CONSTRAINT voice_lifecycle_operations_schema_version_check CHECK (schema_version > 0),
    CONSTRAINT voice_lifecycle_operations_method_check CHECK (method IN ('join', 'leave', 'self_move', 'moderator_move')),
    CONSTRAINT voice_lifecycle_operations_state_check CHECK (state IN ('decided', 'completed', 'quarantined')),
    CONSTRAINT voice_lifecycle_operations_lease_fence_check CHECK (lease_fence >= 0),
    CONSTRAINT voice_lifecycle_operations_lease_pair_check CHECK ((lease_owner IS NULL) = (lease_until IS NULL)),
    CONSTRAINT voice_lifecycle_operations_terminal_lease_check CHECK (
        state = 'decided' OR (lease_owner IS NULL AND lease_until IS NULL)
    ),
    CONSTRAINT voice_lifecycle_operations_quarantine_shape_check CHECK (
        (state = 'quarantined'
            AND quarantine_class IS NOT NULL
            AND btrim(quarantine_class) <> ''
            AND (quarantine_detail IS NULL OR btrim(quarantine_detail) <> '')
            AND quarantined_at IS NOT NULL)
        OR
        (state <> 'quarantined'
            AND quarantine_class IS NULL
            AND quarantine_detail IS NULL
            AND quarantined_at IS NULL)
    ),
    CONSTRAINT voice_lifecycle_operations_receipt_outcome_check CHECK (
        receipt_outcome IS NULL OR receipt_outcome IN ('joined', 'left', 'moved', 'no_op')
    ),
    CONSTRAINT voice_lifecycle_operations_completion_shape_check CHECK (
        (state = 'completed'
            AND receipt_outcome IS NOT NULL
            AND receipt_bytes IS NOT NULL
            AND octet_length(receipt_bytes) > 0
            AND receipt_hash IS NOT NULL
            AND completed_at IS NOT NULL
            AND replay_until IS NOT NULL)
        OR
        (state <> 'completed'
            AND receipt_outcome IS NULL
            AND receipt_source_voice_room_id IS NULL
            AND receipt_destination_voice_room_id IS NULL
            AND receipt_room_id IS NULL
            AND receipt_source_roster_version IS NULL
            AND receipt_destination_roster_version IS NULL
            AND receipt_media_epoch IS NULL
            AND receipt_space_access_epoch IS NULL
            AND receipt_role_policy_epoch IS NULL
            AND receipt_authorization_digest IS NULL
            AND receipt_bytes IS NULL
            AND receipt_hash IS NULL
            AND completed_at IS NULL
            AND replay_until IS NULL)
    ),
    CONSTRAINT voice_lifecycle_operations_replay_window_check CHECK (
        state <> 'completed' OR replay_until >= completed_at + interval '24 hours'
    ),
    CONSTRAINT voice_lifecycle_operations_binding_shape_check CHECK (
        (method = 'join'
            AND source_voice_room_id IS NULL AND source_room_id IS NULL AND source_media_epoch IS NULL
            AND destination_voice_room_id IS NOT NULL AND destination_room_id IS NOT NULL AND destination_media_epoch IS NOT NULL
            AND space_access_epoch IS NOT NULL AND subject_role_policy_epoch IS NOT NULL
            AND authorization_digest IS NOT NULL
            AND can_join IS NOT NULL AND can_publish_audio IS NOT NULL AND can_publish_video IS NOT NULL
            AND can_publish_screen_share IS NOT NULL AND can_subscribe IS NOT NULL
            AND can_mute_others IS NOT NULL AND can_deafen_others IS NOT NULL
            AND can_move_others IS NOT NULL AND can_use_ptt IS NOT NULL AND priority_speaker IS NOT NULL
            AND actor_source_role_policy_epoch IS NULL AND actor_destination_role_policy_epoch IS NULL
            AND actor_can_move_source IS NULL AND actor_can_move_destination IS NULL)
        OR
        (method = 'leave'
            AND source_voice_room_id IS NOT NULL AND destination_voice_room_id IS NULL
            AND destination_room_id IS NULL AND destination_media_epoch IS NULL
            AND (
                (state = 'completed' AND receipt_outcome = 'no_op'
                    AND source_room_id IS NULL AND source_media_epoch IS NULL)
                OR
                ((state IN ('decided', 'quarantined')
                    OR (state = 'completed' AND receipt_outcome = 'left'))
                    AND source_room_id IS NOT NULL AND source_media_epoch IS NOT NULL)
            )
            AND space_access_epoch IS NULL AND subject_role_policy_epoch IS NULL
            AND authorization_digest IS NULL
            AND can_join IS NULL AND can_publish_audio IS NULL AND can_publish_video IS NULL
            AND can_publish_screen_share IS NULL AND can_subscribe IS NULL
            AND can_mute_others IS NULL AND can_deafen_others IS NULL
            AND can_move_others IS NULL AND can_use_ptt IS NULL AND priority_speaker IS NULL
            AND actor_source_role_policy_epoch IS NULL AND actor_destination_role_policy_epoch IS NULL
            AND actor_can_move_source IS NULL AND actor_can_move_destination IS NULL)
        OR
        (method = 'self_move'
            AND source_voice_room_id IS NOT NULL AND destination_voice_room_id IS NOT NULL
            AND source_voice_room_id <> destination_voice_room_id
            AND source_room_id IS NOT NULL AND destination_room_id IS NOT NULL AND source_room_id <> destination_room_id
            AND source_media_epoch IS NOT NULL AND destination_media_epoch IS NOT NULL
            AND space_access_epoch IS NOT NULL AND subject_role_policy_epoch IS NOT NULL
            AND authorization_digest IS NOT NULL
            AND can_join IS NOT NULL AND can_publish_audio IS NOT NULL AND can_publish_video IS NOT NULL
            AND can_publish_screen_share IS NOT NULL AND can_subscribe IS NOT NULL
            AND can_mute_others IS NOT NULL AND can_deafen_others IS NOT NULL
            AND can_move_others IS NOT NULL AND can_use_ptt IS NOT NULL AND priority_speaker IS NOT NULL
            AND actor_source_role_policy_epoch IS NULL AND actor_destination_role_policy_epoch IS NULL
            AND actor_can_move_source IS NULL AND actor_can_move_destination IS NULL)
        OR
        (method = 'moderator_move'
            AND source_voice_room_id IS NOT NULL AND destination_voice_room_id IS NOT NULL
            AND source_voice_room_id <> destination_voice_room_id
            AND source_room_id IS NOT NULL AND destination_room_id IS NOT NULL AND source_room_id <> destination_room_id
            AND source_media_epoch IS NOT NULL AND destination_media_epoch IS NOT NULL
            AND space_access_epoch IS NOT NULL AND subject_role_policy_epoch IS NOT NULL
            AND authorization_digest IS NOT NULL
            AND can_join IS NOT NULL AND can_publish_audio IS NOT NULL AND can_publish_video IS NOT NULL
            AND can_publish_screen_share IS NOT NULL AND can_subscribe IS NOT NULL
            AND can_mute_others IS NOT NULL AND can_deafen_others IS NOT NULL
            AND can_move_others IS NOT NULL AND can_use_ptt IS NOT NULL AND priority_speaker IS NOT NULL
            AND actor_source_role_policy_epoch IS NOT NULL AND actor_destination_role_policy_epoch IS NOT NULL
            AND actor_can_move_source IS NOT NULL AND actor_can_move_destination IS NOT NULL)
    ),
    CONSTRAINT voice_lifecycle_operations_receipt_shape_check CHECK (
        state <> 'completed' OR
        (method = 'join' AND receipt_outcome IN ('joined', 'no_op')
            AND receipt_source_voice_room_id IS NULL
            AND receipt_destination_voice_room_id IS NOT NULL
            AND receipt_room_id IS NOT NULL
            AND receipt_source_roster_version IS NULL
            AND receipt_destination_roster_version IS NOT NULL
            AND receipt_media_epoch IS NOT NULL
            AND receipt_space_access_epoch IS NOT NULL
            AND receipt_role_policy_epoch IS NOT NULL
            AND receipt_authorization_digest IS NOT NULL) OR
        (method = 'leave' AND receipt_outcome = 'left'
            AND receipt_source_voice_room_id IS NOT NULL
            AND receipt_destination_voice_room_id IS NULL
            AND receipt_room_id IS NOT NULL
            AND receipt_source_roster_version IS NOT NULL
            AND receipt_destination_roster_version IS NULL
            AND receipt_media_epoch IS NULL
            AND receipt_space_access_epoch IS NULL
            AND receipt_role_policy_epoch IS NULL
            AND receipt_authorization_digest IS NULL) OR
        (method IN ('self_move', 'moderator_move') AND receipt_outcome = 'moved'
            AND receipt_source_voice_room_id IS NOT NULL
            AND receipt_destination_voice_room_id IS NOT NULL
            AND receipt_source_voice_room_id <> receipt_destination_voice_room_id
            AND receipt_room_id IS NOT NULL
            AND receipt_source_roster_version IS NOT NULL
            AND receipt_destination_roster_version IS NOT NULL
            AND receipt_media_epoch IS NOT NULL
            AND receipt_space_access_epoch IS NOT NULL
            AND receipt_role_policy_epoch IS NOT NULL
            AND receipt_authorization_digest IS NOT NULL) OR
        (method = 'leave' AND receipt_outcome = 'no_op'
            AND receipt_source_voice_room_id IS NULL
            AND receipt_destination_voice_room_id IS NULL
            AND receipt_room_id IS NULL
            AND receipt_source_roster_version IS NULL
            AND receipt_destination_roster_version IS NULL
            AND receipt_media_epoch IS NULL
            AND receipt_space_access_epoch IS NULL
            AND receipt_role_policy_epoch IS NULL
            AND receipt_authorization_digest IS NULL)
    ),
    CONSTRAINT voice_lifecycle_operations_source_room_fk FOREIGN KEY (source_room_id)
        REFERENCES voice_room_instances (room_id) ON DELETE RESTRICT,
    CONSTRAINT voice_lifecycle_operations_destination_room_fk FOREIGN KEY (destination_room_id)
        REFERENCES voice_room_instances (room_id) ON DELETE RESTRICT
);

CREATE UNIQUE INDEX voice_lifecycle_one_nonterminal_subject
    ON voice_lifecycle_operations (subject_profile_id)
    WHERE completed_at IS NULL;
CREATE INDEX voice_lifecycle_operations_recovery_idx
    ON voice_lifecycle_operations (state, lease_until, decided_at, actor_profile_id, operation_id);
CREATE INDEX voice_lifecycle_operations_subject_completion_idx
    ON voice_lifecycle_operations (subject_profile_id, completed_at DESC);

CREATE TABLE voice_lifecycle_effects (
    effect_id UUID PRIMARY KEY,
    actor_profile_id UUID NOT NULL,
    operation_id UUID NOT NULL,
    ordinal SMALLINT NOT NULL,
    kind TEXT NOT NULL,
    schema_version SMALLINT NOT NULL,
    target_voice_room_id UUID NOT NULL,
    target_livekit_room_name TEXT NOT NULL,
    target_profile_id UUID NULL,
    target_participant_identity TEXT NULL,
    target_media_epoch UUID NULL,
    request_bytes BYTEA NOT NULL,
    request_digest BYTEA NOT NULL,
    state TEXT NOT NULL,
    attempt_count BIGINT NOT NULL DEFAULT 0,
    last_error_class TEXT NULL,
    last_error_at TIMESTAMPTZ NULL,
    next_attempt_at TIMESTAMPTZ NOT NULL,
    lease_owner UUID NULL,
    lease_until TIMESTAMPTZ NULL,
    lease_fence BIGINT NOT NULL DEFAULT 0,
    applied_at TIMESTAMPTZ NULL,
    quarantine_class TEXT NULL,
    quarantine_detail TEXT NULL,
    quarantined_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT voice_lifecycle_effects_effect_id_not_nil CHECK (effect_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_effects_actor_profile_id_not_nil CHECK (actor_profile_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_effects_operation_id_not_nil CHECK (operation_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_effects_target_voice_room_id_not_nil CHECK (target_voice_room_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_effects_target_profile_id_not_nil CHECK (target_profile_id IS NULL OR target_profile_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_effects_target_media_epoch_not_nil CHECK (target_media_epoch IS NULL OR target_media_epoch <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_effects_lease_owner_not_nil CHECK (lease_owner IS NULL OR lease_owner <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_lifecycle_effects_request_digest_check CHECK (octet_length(request_digest) = 32),
    CONSTRAINT voice_lifecycle_effects_ordinal_check CHECK (ordinal >= 0),
    CONSTRAINT voice_lifecycle_effects_operation_ordinal_unique UNIQUE (actor_profile_id, operation_id, ordinal),
    CONSTRAINT voice_lifecycle_effects_kind_check CHECK (kind IN ('livekit_ensure_room', 'livekit_eject_participant')),
    CONSTRAINT voice_lifecycle_effects_schema_version_check CHECK (schema_version > 0),
    CONSTRAINT voice_lifecycle_effects_target_livekit_room_check CHECK (btrim(target_livekit_room_name) <> ''),
    CONSTRAINT voice_lifecycle_effects_target_shape_check CHECK (
        (kind = 'livekit_ensure_room'
            AND target_profile_id IS NULL
            AND target_participant_identity IS NULL
            AND target_media_epoch IS NULL)
        OR
        (kind = 'livekit_eject_participant'
            AND target_profile_id IS NOT NULL
            AND target_participant_identity IS NOT NULL
            AND target_media_epoch IS NOT NULL
            AND target_participant_identity =
                'profile:' || target_profile_id::TEXT || ':media:' || target_media_epoch::TEXT)
    ),
    CONSTRAINT voice_lifecycle_effects_state_check CHECK (state IN ('ready', 'applied', 'quarantined')),
    CONSTRAINT voice_lifecycle_effects_attempt_count_check CHECK (attempt_count >= 0),
    CONSTRAINT voice_lifecycle_effects_last_error_pair_check CHECK (
        (last_error_class IS NULL AND last_error_at IS NULL)
        OR (last_error_class IS NOT NULL AND btrim(last_error_class) <> '' AND last_error_at IS NOT NULL)
    ),
    CONSTRAINT voice_lifecycle_effects_lease_pair_check CHECK ((lease_owner IS NULL) = (lease_until IS NULL)),
    CONSTRAINT voice_lifecycle_effects_lease_fence_check CHECK (lease_fence >= 0),
    CONSTRAINT voice_lifecycle_effects_applied_shape_check CHECK ((state = 'applied') = (applied_at IS NOT NULL)),
    CONSTRAINT voice_lifecycle_effects_terminal_lease_check CHECK (
        state = 'ready' OR (lease_owner IS NULL AND lease_until IS NULL)
    ),
    CONSTRAINT voice_lifecycle_effects_quarantine_shape_check CHECK (
        (state = 'quarantined'
            AND quarantine_class IS NOT NULL
            AND btrim(quarantine_class) <> ''
            AND (quarantine_detail IS NULL OR btrim(quarantine_detail) <> '')
            AND quarantined_at IS NOT NULL)
        OR
        (state <> 'quarantined'
            AND quarantine_class IS NULL
            AND quarantine_detail IS NULL
            AND quarantined_at IS NULL)
    ),
    CONSTRAINT voice_lifecycle_effects_operation_fk FOREIGN KEY (actor_profile_id, operation_id)
        REFERENCES voice_lifecycle_operations (actor_profile_id, operation_id) ON DELETE RESTRICT
);

CREATE TABLE voice_media_epoch_denials (
    media_epoch UUID PRIMARY KEY,
    profile_id UUID NOT NULL,
    room_id UUID NOT NULL,
    livekit_room_name TEXT NOT NULL,
    participant_identity TEXT NOT NULL,
    grant_expires_at TIMESTAMPTZ NOT NULL,
    accepted_clock_skew INTERVAL NOT NULL,
    deny_until TIMESTAMPTZ NOT NULL,
    reason TEXT NOT NULL,
    operation_actor_profile_id UUID NULL,
    operation_id UUID NULL,
    absence_observed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT voice_media_epoch_denials_media_epoch_not_nil CHECK (media_epoch <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_media_epoch_denials_profile_id_not_nil CHECK (profile_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_media_epoch_denials_room_id_not_nil CHECK (room_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_media_epoch_denials_operation_actor_profile_id_not_nil CHECK (operation_actor_profile_id IS NULL OR operation_actor_profile_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_media_epoch_denials_operation_id_not_nil CHECK (operation_id IS NULL OR operation_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_media_epoch_denials_livekit_room_check CHECK (btrim(livekit_room_name) <> ''),
    CONSTRAINT voice_media_epoch_denials_participant_identity_check CHECK (
        participant_identity = 'profile:' || profile_id::TEXT || ':media:' || media_epoch::TEXT
    ),
    CONSTRAINT voice_media_epoch_denials_clock_skew_check CHECK (accepted_clock_skew >= interval '0'),
    CONSTRAINT voice_media_epoch_denials_deadline_check CHECK (
        deny_until >= grant_expires_at + accepted_clock_skew
    ),
    CONSTRAINT voice_media_epoch_denials_reason_check CHECK (btrim(reason) <> ''),
    CONSTRAINT voice_media_epoch_denials_room_fk FOREIGN KEY (room_id)
        REFERENCES voice_room_instances (room_id) ON DELETE RESTRICT,
    CONSTRAINT voice_media_epoch_denials_operation_fk FOREIGN KEY (operation_actor_profile_id, operation_id)
        REFERENCES voice_lifecycle_operations (actor_profile_id, operation_id) MATCH FULL ON DELETE RESTRICT
);

CREATE INDEX voice_media_epoch_denials_cleanup_idx
    ON voice_media_epoch_denials (deny_until, absence_observed_at);
CREATE INDEX voice_media_epoch_denials_watcher_idx
    ON voice_media_epoch_denials (room_id, participant_identity);

CREATE TABLE voice_event_outbox (
    event_id UUID PRIMARY KEY,
    actor_profile_id UUID NOT NULL,
    operation_id UUID NOT NULL,
    ordinal SMALLINT NOT NULL,
    subject TEXT NOT NULL,
    schema_version SMALLINT NOT NULL,
    payload_bytes BYTEA NOT NULL,
    payload_hash BYTEA NOT NULL,
    subject_profile_id UUID NOT NULL,
    room_id UUID NOT NULL,
    voice_room_id UUID NOT NULL,
    space_id UUID NOT NULL,
    roster_version BIGINT NOT NULL,
    state TEXT NOT NULL,
    attempt_count BIGINT NOT NULL DEFAULT 0,
    last_error_class TEXT NULL,
    last_error_at TIMESTAMPTZ NULL,
    next_attempt_at TIMESTAMPTZ NOT NULL,
    lease_owner UUID NULL,
    lease_until TIMESTAMPTZ NULL,
    lease_fence BIGINT NOT NULL DEFAULT 0,
    delivered_at TIMESTAMPTZ NULL,
    quarantine_class TEXT NULL,
    quarantine_detail TEXT NULL,
    quarantined_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT voice_event_outbox_event_id_not_nil CHECK (event_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_event_outbox_actor_profile_id_not_nil CHECK (actor_profile_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_event_outbox_operation_id_not_nil CHECK (operation_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_event_outbox_subject_profile_id_not_nil CHECK (subject_profile_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_event_outbox_room_id_not_nil CHECK (room_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_event_outbox_voice_room_id_not_nil CHECK (voice_room_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_event_outbox_space_id_not_nil CHECK (space_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_event_outbox_lease_owner_not_nil CHECK (lease_owner IS NULL OR lease_owner <> '00000000-0000-0000-0000-000000000000'::UUID),
    CONSTRAINT voice_event_outbox_roster_version_check CHECK (roster_version >= 0),
    CONSTRAINT voice_event_outbox_payload_hash_check CHECK (octet_length(payload_hash) = 32),
    CONSTRAINT voice_event_outbox_ordinal_check CHECK (ordinal >= 0),
    CONSTRAINT voice_event_outbox_operation_ordinal_unique UNIQUE (actor_profile_id, operation_id, ordinal),
    CONSTRAINT voice_event_outbox_subject_check CHECK (
        subject LIKE 'voice.%' AND char_length(subject) > char_length('voice.') AND btrim(subject) = subject
    ),
    CONSTRAINT voice_event_outbox_schema_version_check CHECK (schema_version > 0),
    CONSTRAINT voice_event_outbox_state_check CHECK (state IN ('ready', 'delivered', 'quarantined')),
    CONSTRAINT voice_event_outbox_attempt_count_check CHECK (attempt_count >= 0),
    CONSTRAINT voice_event_outbox_last_error_pair_check CHECK (
        (last_error_class IS NULL AND last_error_at IS NULL)
        OR (last_error_class IS NOT NULL AND btrim(last_error_class) <> '' AND last_error_at IS NOT NULL)
    ),
    CONSTRAINT voice_event_outbox_lease_pair_check CHECK ((lease_owner IS NULL) = (lease_until IS NULL)),
    CONSTRAINT voice_event_outbox_lease_fence_check CHECK (lease_fence >= 0),
    CONSTRAINT voice_event_outbox_delivery_shape_check CHECK ((state = 'delivered') = (delivered_at IS NOT NULL)),
    CONSTRAINT voice_event_outbox_terminal_lease_check CHECK (
        state = 'ready' OR (lease_owner IS NULL AND lease_until IS NULL)
    ),
    CONSTRAINT voice_event_outbox_quarantine_shape_check CHECK (
        (state = 'quarantined'
            AND quarantine_class IS NOT NULL
            AND btrim(quarantine_class) <> ''
            AND (quarantine_detail IS NULL OR btrim(quarantine_detail) <> '')
            AND quarantined_at IS NOT NULL)
        OR
        (state <> 'quarantined'
            AND quarantine_class IS NULL
            AND quarantine_detail IS NULL
            AND quarantined_at IS NULL)
    ),
    CONSTRAINT voice_event_outbox_operation_fk FOREIGN KEY (actor_profile_id, operation_id)
        REFERENCES voice_lifecycle_operations (actor_profile_id, operation_id) ON DELETE RESTRICT,
    CONSTRAINT voice_event_outbox_room_fk FOREIGN KEY (room_id)
        REFERENCES voice_room_instances (room_id) ON DELETE RESTRICT
);

CREATE FUNCTION voice_room_instance_immutable_guard_fn()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF ROW(OLD.room_id, OLD.space_id, OLD.voice_room_id, OLD.livekit_room_name)
        IS DISTINCT FROM
       ROW(NEW.room_id, NEW.space_id, NEW.voice_room_id, NEW.livekit_room_name) THEN
        RAISE EXCEPTION 'voice room instance identity is immutable';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER voice_room_instance_immutable_guard
BEFORE UPDATE ON voice_room_instances
FOR EACH ROW EXECUTE FUNCTION voice_room_instance_immutable_guard_fn();

CREATE FUNCTION voice_room_membership_generation_guard_fn()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF NEW.profile_id IS DISTINCT FROM OLD.profile_id THEN
        RAISE EXCEPTION 'voice membership profile is immutable';
    END IF;

    IF NEW.updated_at < OLD.updated_at THEN
        RAISE EXCEPTION 'voice membership updated_at cannot decrease';
    END IF;

    IF NEW.media_epoch IS NOT DISTINCT FROM OLD.media_epoch THEN
        IF ROW(
            NEW.room_id,
            NEW.space_access_epoch,
            NEW.role_policy_epoch,
            NEW.authorization_digest,
            NEW.can_join,
            NEW.can_publish_audio,
            NEW.can_publish_video,
            NEW.can_publish_screen_share,
            NEW.can_subscribe,
            NEW.can_mute_others,
            NEW.can_deafen_others,
            NEW.can_move_others,
            NEW.can_use_ptt,
            NEW.priority_speaker,
            NEW.joined_at
        ) IS DISTINCT FROM ROW(
            OLD.room_id,
            OLD.space_access_epoch,
            OLD.role_policy_epoch,
            OLD.authorization_digest,
            OLD.can_join,
            OLD.can_publish_audio,
            OLD.can_publish_video,
            OLD.can_publish_screen_share,
            OLD.can_subscribe,
            OLD.can_mute_others,
            OLD.can_deafen_others,
            OLD.can_move_others,
            OLD.can_use_ptt,
            OLD.priority_speaker,
            OLD.joined_at
        ) THEN
            RAISE EXCEPTION 'voice membership generation fields require a new media epoch';
        END IF;

        IF OLD.latest_grant_expires_at IS NOT NULL
           AND (
               NEW.latest_grant_expires_at IS NULL
               OR NEW.latest_grant_expires_at < OLD.latest_grant_expires_at
           ) THEN
            RAISE EXCEPTION 'voice membership grant expiry cannot decrease within a media epoch';
        END IF;
    ELSIF NEW.latest_grant_expires_at IS NOT NULL THEN
        RAISE EXCEPTION 'new voice membership media epoch must start without a grant expiry';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER voice_room_membership_generation_guard
BEFORE UPDATE ON voice_room_memberships
FOR EACH ROW EXECUTE FUNCTION voice_room_membership_generation_guard_fn();

CREATE FUNCTION voice_lifecycle_operation_immutable_guard_fn()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF ROW(
        OLD.actor_profile_id, OLD.operation_id, OLD.schema_version, OLD.method,
        OLD.fingerprint, OLD.binding_bytes, OLD.binding_hash, OLD.actor_account_id,
        OLD.subject_profile_id, OLD.space_id, OLD.source_voice_room_id,
        OLD.destination_voice_room_id, OLD.source_room_id, OLD.destination_room_id,
        OLD.source_media_epoch, OLD.destination_media_epoch, OLD.space_access_epoch,
        OLD.subject_role_policy_epoch, OLD.actor_source_role_policy_epoch,
        OLD.actor_destination_role_policy_epoch, OLD.authorization_digest,
        OLD.can_join, OLD.can_publish_audio, OLD.can_publish_video,
        OLD.can_publish_screen_share, OLD.can_subscribe, OLD.can_mute_others,
        OLD.can_deafen_others, OLD.can_move_others, OLD.can_use_ptt,
        OLD.priority_speaker, OLD.actor_can_move_source,
        OLD.actor_can_move_destination, OLD.decided_at, OLD.created_at,
        OLD.redis_owner_token
    ) IS DISTINCT FROM ROW(
        NEW.actor_profile_id, NEW.operation_id, NEW.schema_version, NEW.method,
        NEW.fingerprint, NEW.binding_bytes, NEW.binding_hash, NEW.actor_account_id,
        NEW.subject_profile_id, NEW.space_id, NEW.source_voice_room_id,
        NEW.destination_voice_room_id, NEW.source_room_id, NEW.destination_room_id,
        NEW.source_media_epoch, NEW.destination_media_epoch, NEW.space_access_epoch,
        NEW.subject_role_policy_epoch, NEW.actor_source_role_policy_epoch,
        NEW.actor_destination_role_policy_epoch, NEW.authorization_digest,
        NEW.can_join, NEW.can_publish_audio, NEW.can_publish_video,
        NEW.can_publish_screen_share, NEW.can_subscribe, NEW.can_mute_others,
        NEW.can_deafen_others, NEW.can_move_others, NEW.can_use_ptt,
        NEW.priority_speaker, NEW.actor_can_move_source,
        NEW.actor_can_move_destination, NEW.decided_at, NEW.created_at,
        NEW.redis_owner_token
    ) THEN
        RAISE EXCEPTION 'voice lifecycle operation decision is immutable';
    END IF;

    IF OLD.state = 'completed' AND ROW(
        OLD.receipt_outcome, OLD.receipt_source_voice_room_id,
        OLD.receipt_destination_voice_room_id, OLD.receipt_room_id,
        OLD.receipt_source_roster_version, OLD.receipt_destination_roster_version,
        OLD.receipt_media_epoch, OLD.receipt_space_access_epoch,
        OLD.receipt_role_policy_epoch, OLD.receipt_authorization_digest,
        OLD.receipt_bytes, OLD.receipt_hash, OLD.completed_at, OLD.replay_until
    ) IS DISTINCT FROM ROW(
        NEW.receipt_outcome, NEW.receipt_source_voice_room_id,
        NEW.receipt_destination_voice_room_id, NEW.receipt_room_id,
        NEW.receipt_source_roster_version, NEW.receipt_destination_roster_version,
        NEW.receipt_media_epoch, NEW.receipt_space_access_epoch,
        NEW.receipt_role_policy_epoch, NEW.receipt_authorization_digest,
        NEW.receipt_bytes, NEW.receipt_hash, NEW.completed_at, NEW.replay_until
    ) THEN
        RAISE EXCEPTION 'completed voice lifecycle receipt is immutable';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER voice_lifecycle_operation_immutable_guard
BEFORE UPDATE ON voice_lifecycle_operations
FOR EACH ROW EXECUTE FUNCTION voice_lifecycle_operation_immutable_guard_fn();

CREATE FUNCTION voice_lifecycle_operation_transition_guard_fn()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.state = NEW.state THEN
        IF OLD.state <> 'decided' THEN
            RAISE EXCEPTION 'terminal voice lifecycle operation cannot be updated';
        END IF;
        RETURN NEW;
    END IF;
    IF OLD.state = 'decided' AND NEW.state IN ('completed', 'quarantined') THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'invalid voice lifecycle operation transition from % to %', OLD.state, NEW.state;
END;
$$;

CREATE TRIGGER voice_lifecycle_operation_transition_guard
BEFORE UPDATE ON voice_lifecycle_operations
FOR EACH ROW EXECUTE FUNCTION voice_lifecycle_operation_transition_guard_fn();

CREATE FUNCTION voice_lifecycle_effect_immutable_guard_fn()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF ROW(
        OLD.effect_id, OLD.actor_profile_id, OLD.operation_id, OLD.ordinal,
        OLD.kind, OLD.schema_version, OLD.target_voice_room_id,
        OLD.target_livekit_room_name, OLD.target_profile_id,
        OLD.target_participant_identity, OLD.target_media_epoch,
        OLD.request_bytes, OLD.request_digest, OLD.created_at
    ) IS DISTINCT FROM ROW(
        NEW.effect_id, NEW.actor_profile_id, NEW.operation_id, NEW.ordinal,
        NEW.kind, NEW.schema_version, NEW.target_voice_room_id,
        NEW.target_livekit_room_name, NEW.target_profile_id,
        NEW.target_participant_identity, NEW.target_media_epoch,
        NEW.request_bytes, NEW.request_digest, NEW.created_at
    ) THEN
        RAISE EXCEPTION 'voice lifecycle effect instruction is immutable';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER voice_lifecycle_effect_immutable_guard
BEFORE UPDATE ON voice_lifecycle_effects
FOR EACH ROW EXECUTE FUNCTION voice_lifecycle_effect_immutable_guard_fn();

CREATE FUNCTION voice_lifecycle_effect_transition_guard_fn()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.state = NEW.state THEN
        IF OLD.state <> 'ready' THEN
            RAISE EXCEPTION 'terminal voice lifecycle effect cannot be updated';
        END IF;
        RETURN NEW;
    END IF;
    IF OLD.state = 'ready' AND NEW.state IN ('applied', 'quarantined') THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'invalid voice lifecycle effect transition from % to %', OLD.state, NEW.state;
END;
$$;

CREATE TRIGGER voice_lifecycle_effect_transition_guard
BEFORE UPDATE ON voice_lifecycle_effects
FOR EACH ROW EXECUTE FUNCTION voice_lifecycle_effect_transition_guard_fn();

CREATE FUNCTION voice_media_epoch_denial_update_guard_fn()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF ROW(
        OLD.media_epoch, OLD.profile_id, OLD.room_id, OLD.livekit_room_name,
        OLD.participant_identity, OLD.grant_expires_at, OLD.accepted_clock_skew,
        OLD.reason, OLD.operation_actor_profile_id, OLD.operation_id, OLD.created_at
    ) IS DISTINCT FROM ROW(
        NEW.media_epoch, NEW.profile_id, NEW.room_id, NEW.livekit_room_name,
        NEW.participant_identity, NEW.grant_expires_at, NEW.accepted_clock_skew,
        NEW.reason, NEW.operation_actor_profile_id, NEW.operation_id, NEW.created_at
    ) THEN
        RAISE EXCEPTION 'voice media epoch denial identity is immutable';
    END IF;
    IF NEW.deny_until < OLD.deny_until THEN
        RAISE EXCEPTION 'voice media epoch denial deadline cannot decrease';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER voice_media_epoch_denial_update_guard
BEFORE UPDATE ON voice_media_epoch_denials
FOR EACH ROW EXECUTE FUNCTION voice_media_epoch_denial_update_guard_fn();

CREATE FUNCTION voice_media_epoch_denial_delete_guard_fn()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.absence_observed_at IS NULL OR clock_timestamp() < OLD.deny_until THEN
        RAISE EXCEPTION 'voice media epoch denial is not safe to delete';
    END IF;
    RETURN OLD;
END;
$$;

CREATE TRIGGER voice_media_epoch_denial_delete_guard
BEFORE DELETE ON voice_media_epoch_denials
FOR EACH ROW EXECUTE FUNCTION voice_media_epoch_denial_delete_guard_fn();

CREATE FUNCTION voice_event_outbox_immutable_guard_fn()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF ROW(
        OLD.event_id, OLD.actor_profile_id, OLD.operation_id, OLD.ordinal,
        OLD.subject, OLD.schema_version, OLD.payload_bytes, OLD.payload_hash,
        OLD.subject_profile_id, OLD.room_id, OLD.voice_room_id, OLD.space_id,
        OLD.roster_version, OLD.created_at
    ) IS DISTINCT FROM ROW(
        NEW.event_id, NEW.actor_profile_id, NEW.operation_id, NEW.ordinal,
        NEW.subject, NEW.schema_version, NEW.payload_bytes, NEW.payload_hash,
        NEW.subject_profile_id, NEW.room_id, NEW.voice_room_id, NEW.space_id,
        NEW.roster_version, NEW.created_at
    ) THEN
        RAISE EXCEPTION 'voice event outbox payload is immutable';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER voice_event_outbox_immutable_guard
BEFORE UPDATE ON voice_event_outbox
FOR EACH ROW EXECUTE FUNCTION voice_event_outbox_immutable_guard_fn();

CREATE FUNCTION voice_event_outbox_transition_guard_fn()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.state = NEW.state THEN
        IF OLD.state <> 'ready' THEN
            RAISE EXCEPTION 'terminal voice event outbox row cannot be updated';
        END IF;
        RETURN NEW;
    END IF;
    IF OLD.state = 'ready' AND NEW.state IN ('delivered', 'quarantined') THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'invalid voice event outbox transition from % to %', OLD.state, NEW.state;
END;
$$;

CREATE TRIGGER voice_event_outbox_transition_guard
BEFORE UPDATE ON voice_event_outbox
FOR EACH ROW EXECUTE FUNCTION voice_event_outbox_transition_guard_fn();

COMMIT;
