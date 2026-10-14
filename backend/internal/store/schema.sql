CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL PRIMARY KEY,
    nickname      TEXT NOT NULL,
    token         TEXT NOT NULL UNIQUE,
    device_fp     TEXT NOT NULL DEFAULT '',
    current_round      INT NOT NULL DEFAULT 0,
    current_difficulty TEXT NOT NULL DEFAULT '全部',
    abandoned          BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE users ADD COLUMN IF NOT EXISTS current_round INT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS current_difficulty TEXT NOT NULL DEFAULT '全部';
ALTER TABLE users ADD COLUMN IF NOT EXISTS abandoned BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS questions (
    id         BIGSERIAL PRIMARY KEY,
    content    TEXT NOT NULL,
    options    JSONB NOT NULL,
    answer     INT NOT NULL,
    score      INT NOT NULL DEFAULT 10,
    sort       INT NOT NULL DEFAULT 0,
    active     BOOLEAN NOT NULL DEFAULT TRUE,
    difficulty TEXT NOT NULL DEFAULT '简单'
);
ALTER TABLE questions ADD COLUMN IF NOT EXISTS difficulty TEXT NOT NULL DEFAULT '简单';

CREATE TABLE IF NOT EXISTS attempts (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id),
    question_id BIGINT NOT NULL REFERENCES questions(id),
    selected    INT,
    correct     BOOLEAN NOT NULL,
    score       INT NOT NULL DEFAULT 0,
    request_id  TEXT NOT NULL UNIQUE,
    round_no    INT NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE attempts ADD COLUMN IF NOT EXISTS round_no INT NOT NULL DEFAULT 1;
CREATE INDEX IF NOT EXISTS idx_attempts_user_round ON attempts(user_id, round_no);
-- 迁移：历史库回填 current_round（必须排在 attempts 建表之后）
UPDATE users u SET current_round = (SELECT COALESCE(MAX(round_no), 0) FROM attempts a WHERE a.user_id = u.id);
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
    id         INT PRIMARY KEY,
    state      TEXT NOT NULL DEFAULT 'stop',
    difficulty TEXT NOT NULL DEFAULT '全部'
);
ALTER TABLE quiz_state ADD COLUMN IF NOT EXISTS difficulty TEXT NOT NULL DEFAULT '全部';
INSERT INTO quiz_state (id, state) VALUES (1, 'stop') ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS backgrounds (
    id         BIGSERIAL PRIMARY KEY,
    file_path  TEXT NOT NULL UNIQUE,
    active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS footer_settings (
    id         INT PRIMARY KEY,
    owner      TEXT NOT NULL DEFAULT '',
    license    TEXT NOT NULL DEFAULT '',
    icp        TEXT NOT NULL DEFAULT '',
    icp_icon   TEXT NOT NULL DEFAULT '',
    police     TEXT NOT NULL DEFAULT '',
    police_icon TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE footer_settings ADD COLUMN IF NOT EXISTS icp_icon TEXT NOT NULL DEFAULT '';
ALTER TABLE footer_settings ADD COLUMN IF NOT EXISTS police TEXT NOT NULL DEFAULT '';
ALTER TABLE footer_settings ADD COLUMN IF NOT EXISTS police_icon TEXT NOT NULL DEFAULT '';
