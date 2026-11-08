-- +goose Up
-- SQL in this section is executed when the migration is applied.

CREATE TABLE IF NOT EXISTS teachers (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    phone VARCHAR(64) NOT NULL DEFAULT '',
    subject VARCHAR(128) NOT NULL DEFAULT '',
    rate_cents BIGINT NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS students (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    guardian VARCHAR(128) NOT NULL DEFAULT '',
    phone VARCHAR(64) NOT NULL DEFAULT '',
    balance_cents BIGINT NOT NULL DEFAULT 0,
    note TEXT NOT NULL DEFAULT '',
    active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS classrooms (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    capacity INTEGER NOT NULL DEFAULT 0,
    location VARCHAR(255) NOT NULL DEFAULT '',
    note TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS courses (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    subject VARCHAR(128) NOT NULL DEFAULT '',
    teacher_id VARCHAR(64) NOT NULL DEFAULT '',
    total_lessons INTEGER NOT NULL DEFAULT 0,
    price_cents BIGINT NOT NULL DEFAULT 0,
    minutes INTEGER NOT NULL DEFAULT 0,
    active BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS lessons (
    id VARCHAR(64) PRIMARY KEY,
    course_id VARCHAR(64) NOT NULL DEFAULT '',
    teacher_id VARCHAR(64) NOT NULL DEFAULT '',
    classroom_id VARCHAR(64) NOT NULL DEFAULT '',
    start_at TIMESTAMPTZ NOT NULL,
    minutes INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'planned'
);

CREATE TABLE IF NOT EXISTS ledger (
    id VARCHAR(64) PRIMARY KEY,
    student_id VARCHAR(64) NOT NULL REFERENCES students(id) ON DELETE CASCADE,
    kind VARCHAR(32) NOT NULL,
    amount_cents BIGINT NOT NULL,
    balance_cents BIGINT NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(64) PRIMARY KEY,
    username VARCHAR(64) NOT NULL UNIQUE,
    display_name VARCHAR(128) NOT NULL DEFAULT '',
    role VARCHAR(32) NOT NULL DEFAULT 'staff',
    salt VARCHAR(128) NOT NULL,
    hash VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS settings (
    key VARCHAR(128) PRIMARY KEY,
    value TEXT NOT NULL
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_teachers_phone ON teachers(phone);
CREATE INDEX IF NOT EXISTS idx_students_phone ON students(phone);
CREATE INDEX IF NOT EXISTS idx_courses_teacher ON courses(teacher_id);
CREATE INDEX IF NOT EXISTS idx_lessons_course ON lessons(course_id);
CREATE INDEX IF NOT EXISTS idx_lessons_teacher ON lessons(teacher_id);
CREATE INDEX IF NOT EXISTS idx_lessons_start_at ON lessons(start_at);
CREATE INDEX IF NOT EXISTS idx_ledger_student_at ON ledger(student_id, at);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);

-- +goose Down
-- SQL in this section is executed when the migration is rolled back.

DROP TABLE IF EXISTS settings;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS ledger;
DROP TABLE IF EXISTS lessons;
DROP TABLE IF EXISTS courses;
DROP TABLE IF EXISTS classrooms;
DROP TABLE IF EXISTS students;
DROP TABLE IF EXISTS teachers;
