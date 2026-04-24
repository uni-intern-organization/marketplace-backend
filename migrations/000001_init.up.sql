-- Roles enum
CREATE TYPE user_role AS ENUM ('student', 'recruiter', 'admin');

-- Users (login/email stored in plain for lookup; sensitive data encrypted at app layer)
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role user_role NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_role ON users(role);

-- Encrypted profile data (AES256 decrypted in app)
-- Student profile
CREATE TABLE student_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    full_name_enc BYTEA,
    phone_enc BYTEA,
    bio_enc BYTEA,
    resume_object_key VARCHAR(512),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id)
);

CREATE INDEX idx_student_profiles_user_id ON student_profiles(user_id);

-- Recruiter/Company profile
CREATE TABLE recruiter_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    company_name_enc BYTEA,
    full_name_enc BYTEA,
    phone_enc BYTEA,
    company_logo_object_key VARCHAR(512),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id)
);

CREATE INDEX idx_recruiter_profiles_user_id ON recruiter_profiles(user_id);

-- Invitations: recruiter invites student
CREATE TABLE invitations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recruiter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    student_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message_enc BYTEA,
    status VARCHAR(32) NOT NULL DEFAULT 'pending', -- pending, accepted, declined
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(recruiter_id, student_id)
);

CREATE INDEX idx_invitations_recruiter ON invitations(recruiter_id);
CREATE INDEX idx_invitations_student ON invitations(student_id);
CREATE INDEX idx_invitations_status ON invitations(status);

-- Applications: student submits application (e.g. to a vacancy or to recruiter)
CREATE TABLE applications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    recruiter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    invitation_id UUID REFERENCES invitations(id) ON DELETE SET NULL,
    cover_letter_enc BYTEA,
    status VARCHAR(32) NOT NULL DEFAULT 'submitted', -- submitted, viewed, accepted, rejected
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_applications_student ON applications(student_id);
CREATE INDEX idx_applications_recruiter ON applications(recruiter_id);
CREATE INDEX idx_applications_status ON applications(status);

-- Optional: vacancies (recruiter creates, student can apply)
CREATE TABLE vacancies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recruiter_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title_enc BYTEA,
    description_enc BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_vacancies_recruiter ON vacancies(recruiter_id);
