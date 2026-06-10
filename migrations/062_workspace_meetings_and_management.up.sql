ALTER TABLE workspaces
    ADD COLUMN meeting_frequency TEXT,
    ADD COLUMN primary_audience TEXT,
    ADD COLUMN how_to_join TEXT;

ALTER TABLE workspace_members
    ADD COLUMN role TEXT NOT NULL DEFAULT 'member';

UPDATE workspace_members wm
SET role = 'owner'
FROM workspaces ws
WHERE wm.workspace_id = ws.id
  AND wm.user_id = ws.created_by;

ALTER TABLE workspace_members
    ADD CONSTRAINT workspace_members_role_chk
    CHECK (role IN ('owner', 'moderator', 'member'));

CREATE TABLE workspace_meetings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    description TEXT,
    meeting_at TIMESTAMPTZ NOT NULL,
    location TEXT,
    join_url TEXT,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_workspace_meetings_workspace_time ON workspace_meetings(workspace_id, meeting_at ASC);
