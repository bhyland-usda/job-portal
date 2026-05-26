CREATE TABLE IF NOT EXISTS badges (
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(100) NOT NULL UNIQUE,
    description TEXT         NOT NULL,
    icon        VARCHAR(50)  NOT NULL DEFAULT 'star'
);

CREATE TABLE IF NOT EXISTS user_badges (
    user_id   UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    badge_id  UUID        NOT NULL REFERENCES badges(id) ON DELETE CASCADE,
    earned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, badge_id)
);

INSERT INTO badges (id, name, description, icon) VALUES
    (gen_random_uuid(), 'First Post',        'Created your first post',         'post'),
    (gen_random_uuid(), 'Connector',         'Made 10 connections',             'link'),
    (gen_random_uuid(), 'Profile Complete',  'Completed your profile',          'check'),
    (gen_random_uuid(), 'Detail Completed',  'Completed a detail assignment',   'briefcase'),
    (gen_random_uuid(), 'Mentor',            'Became a mentor',                 'graduation'),
    (gen_random_uuid(), 'Team Player',       'Received 5 kudos',               'heart');
