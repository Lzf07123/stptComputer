CREATE TABLE IF NOT EXISTS users (
    id         BIGSERIAL PRIMARY KEY,
    nickname   TEXT NOT NULL,
    token      TEXT NOT NULL UNIQUE,
    device_fp  TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS questions (
    id      BIGSERIAL PRIMARY KEY,
    content TEXT NOT NULL,
    options JSONB NOT NULL,
    answer  INT NOT NULL,
    score   INT NOT NULL DEFAULT 10,
    sort    INT NOT NULL DEFAULT 0,
    active  BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS attempts (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id),
    question_id BIGINT NOT NULL REFERENCES questions(id),
    selected    INT,
    correct     BOOLEAN NOT NULL,
    score       INT NOT NULL DEFAULT 0,
    request_id  TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_attempts_user ON attempts(user_id);

CREATE TABLE IF NOT EXISTS qrcodes (
    id         BIGSERIAL PRIMARY KEY,
    image_url  TEXT NOT NULL,
    active     BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS announcements (
    id         BIGSERIAL PRIMARY KEY,
    content    TEXT NOT NULL,
    type       TEXT NOT NULL DEFAULT 'info',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS quiz_state (
    id    INT PRIMARY KEY,
    state TEXT NOT NULL DEFAULT 'stop'
);
INSERT INTO quiz_state (id, state) VALUES (1, 'stop') ON CONFLICT (id) DO NOTHING;
