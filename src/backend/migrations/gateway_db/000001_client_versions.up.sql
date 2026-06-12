CREATE TABLE IF NOT EXISTS client_versions (
    platform          VARCHAR(20) PRIMARY KEY,
    min_supported     VARCHAR(20) NOT NULL,
    latest_version    VARCHAR(20) NOT NULL,
    update_url        TEXT        NOT NULL,
    release_notes     TEXT,
    shorebird_patch   INT,
    updated_at        TIMESTAMPTZ DEFAULT NOW()
);

INSERT INTO client_versions (platform, min_supported, latest_version, update_url, release_notes)
VALUES (
    'windows',
    '1.0.0',
    '1.0.0',
    'https://updates.voice.example/windows/appcast.xml',
    'Initial Windows desktop release'
)
ON CONFLICT (platform) DO NOTHING;
