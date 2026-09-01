CREATE TABLE folders (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  profile_id UUID NOT NULL,
  name TEXT NOT NULL,
  folder_type VARCHAR(16) NOT NULL CHECK (folder_type IN ('system', 'custom')),
  filter_config_json JSONB NOT NULL DEFAULT '{}',
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX folders_profile_id_idx ON folders (profile_id, sort_order);
