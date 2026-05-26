CREATE TABLE onboarding_steps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255) NOT NULL,
    description TEXT,
    sort_order INT NOT NULL DEFAULT 0
);

CREATE TABLE user_onboarding (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    step_id UUID NOT NULL REFERENCES onboarding_steps(id) ON DELETE CASCADE,
    completed_at TIMESTAMPTZ,
    PRIMARY KEY (user_id, step_id)
);

INSERT INTO onboarding_steps (id, title, description, sort_order) VALUES
    (gen_random_uuid(), 'Complete Your Profile', 'Add your name, headline, location, and about section', 1),
    (gen_random_uuid(), 'Add Your Skills', 'List your professional skills to get matched with opportunities', 2),
    (gen_random_uuid(), 'Upload a Photo', 'Add a profile photo so colleagues can recognize you', 3),
    (gen_random_uuid(), 'Add Experience', 'Share your work history', 4),
    (gen_random_uuid(), 'Add Education', 'Add your educational background', 5),
    (gen_random_uuid(), 'Make Your First Connection', 'Find and connect with a colleague', 6),
    (gen_random_uuid(), 'Find a Mentor', 'Browse available mentors in your field', 7);
