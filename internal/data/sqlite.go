package data

import (
	"context"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"mygo-app/internal/domain"

	"github.com/egoist/mygo/plugins/sqlite"
)

// schema is applied statement by statement at every open; ALTER-free
// migrations go here as the app grows, or a numbered migration table.
var schema = []string{
	`CREATE TABLE IF NOT EXISTS teachers (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		phone TEXT NOT NULL DEFAULT '',
		subject TEXT NOT NULL DEFAULT '',
		rate_cents INTEGER NOT NULL DEFAULT 0,
		active INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS students (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		guardian TEXT NOT NULL DEFAULT '',
		phone TEXT NOT NULL DEFAULT '',
		balance_cents INTEGER NOT NULL DEFAULT 0,
		note TEXT NOT NULL DEFAULT '',
		active INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS classrooms (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		capacity INTEGER NOT NULL DEFAULT 0,
		location TEXT NOT NULL DEFAULT '',
		note TEXT NOT NULL DEFAULT '')`,
	`CREATE TABLE IF NOT EXISTS courses (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		subject TEXT NOT NULL DEFAULT '',
		teacher_id TEXT NOT NULL DEFAULT '',
		total_lessons INTEGER NOT NULL DEFAULT 0,
		price_cents INTEGER NOT NULL DEFAULT 0,
		minutes INTEGER NOT NULL DEFAULT 0,
		active INTEGER NOT NULL DEFAULT 1)`,
	`CREATE TABLE IF NOT EXISTS lessons (
		id TEXT PRIMARY KEY,
		course_id TEXT NOT NULL DEFAULT '',
		teacher_id TEXT NOT NULL DEFAULT '',
		classroom_id TEXT NOT NULL DEFAULT '',
		start_at TEXT NOT NULL,
		minutes INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'planned')`,
	`CREATE TABLE IF NOT EXISTS ledger (
		id TEXT PRIMARY KEY,
		student_id TEXT NOT NULL,
		kind TEXT NOT NULL,
		amount_cents INTEGER NOT NULL,
		balance_cents INTEGER NOT NULL,
		note TEXT NOT NULL DEFAULT '',
		at TEXT NOT NULL)`,
	`CREATE INDEX IF NOT EXISTS ledger_student ON ledger (student_id, at)`,
	`CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		username TEXT NOT NULL UNIQUE,
		display_name TEXT NOT NULL DEFAULT '',
		role TEXT NOT NULL DEFAULT '',
		salt TEXT NOT NULL,
		hash TEXT NOT NULL,
		created_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL)`,
}

// sqliteRepo is the local store: one file, no server, no cgo.
type sqliteRepo struct {
	db   *sqlite.DB
	path string
	// mu serializes the read-modify-write of a balance, which the
	// plugin's connection does not make atomic on its own.
	mu sync.Mutex
}

func openSQLite(ctx context.Context, path string) (domain.Repo, error) {
	db, err := sqlite.Open(ctx, path, sqlite.OpenOptions{})
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	r := &sqliteRepo{db: db, path: path}
	for _, stmt := range schema {
		if _, err := db.Execute(ctx, stmt); err != nil {
			db.Close()
			return nil, fmt.Errorf("migrate: %w", err)
		}
	}
	if err := r.ensureAdmin(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("admin account: %w", err)
	}
	if err := r.seed(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("seed: %w", err)
	}
	return r, nil
}

func (r *sqliteRepo) Close() error   { return r.db.Close() }
func (r *sqliteRepo) Source() string { return "SQLite · " + r.path }

func (r *sqliteRepo) Auth() domain.Auth                         { return authStore{r} }
func (r *sqliteRepo) Settings() domain.Settings                 { return settingsStore{r} }
func (r *sqliteRepo) Users() domain.UserStore                   { return userStore{r} }
func (r *sqliteRepo) Students() domain.Store[domain.Student]     { return studentStore{r} }
func (r *sqliteRepo) Teachers() domain.Store[domain.Teacher]     { return teacherStore{r} }
func (r *sqliteRepo) Classrooms() domain.Store[domain.Classroom] { return classroomStore{r} }
func (r *sqliteRepo) Courses() domain.Store[domain.Course]       { return courseStore{r} }
func (r *sqliteRepo) Lessons() domain.Store[domain.Lesson]       { return lessonStore{r} }
func (r *sqliteRepo) Ledger() domain.Ledger                      { return ledgerStore{r} }

func (r *sqliteRepo) Stats(ctx context.Context) (domain.Stats, error) {
	var st domain.Stats
	count := func(sql string) (int64, error) {
		rows, err := r.db.Query(ctx, sql)
		if err != nil {
			return 0, err
		}
		if len(rows.Values) == 0 || len(rows.Values[0]) == 0 {
			return 0, nil
		}
		return colInt(rows.Values[0][0]), nil
	}
	for _, q := range []struct {
		sql string
		dst *int
	}{
		{"SELECT COUNT(*) FROM students WHERE active = 1", &st.Students},
		{"SELECT COUNT(*) FROM teachers WHERE active = 1", &st.Teachers},
		{"SELECT COUNT(*) FROM courses WHERE active = 1", &st.Courses},
		{"SELECT COUNT(*) FROM lessons WHERE status = 'planned'", &st.Lessons},
	} {
		n, err := count(q.sql)
		if err != nil {
			return st, err
		}
		*q.dst = int(n)
	}
	var err error
	if st.Revenue, err = count("SELECT COALESCE(SUM(amount_cents), 0) FROM ledger WHERE kind = 'topup'"); err != nil {
		return st, err
	}
	if st.Outstanding, err = count("SELECT COALESCE(SUM(balance_cents), 0) FROM students"); err != nil {
		return st, err
	}
	return st, nil
}

// seed fills an empty database with one of each, so that a first run has
// something to show instead of empty tables.
func (r *sqliteRepo) seed(ctx context.Context) error {
	rows, err := r.db.Query(ctx, "SELECT COUNT(*) FROM students")
	if err != nil {
		return err
	}
	if len(rows.Values) == 1 && colInt(rows.Values[0][0]) > 0 {
		return nil
	}
	now := time.Now()
	teachers := []domain.Teacher{
		{ID: "tch_seed1", Name: "陈老师", Phone: "13800000001", Subject: "数学", RateCents: 18000, Active: true, CreatedAt: now.AddDate(0, -6, 0)},
		{ID: "tch_seed2", Name: "林老师", Phone: "13800000002", Subject: "英语", RateCents: 16000, Active: true, CreatedAt: now.AddDate(0, -4, 0)},
	}
	for i := range teachers {
		if err := r.Teachers().Save(ctx, &teachers[i]); err != nil {
			return err
		}
	}
	rooms := []domain.Classroom{
		{ID: "rom_seed1", Name: "A101", Capacity: 12, Location: "一楼东侧"},
		{ID: "rom_seed2", Name: "B203", Capacity: 8, Location: "二楼"},
	}
	for i := range rooms {
		if err := r.Classrooms().Save(ctx, &rooms[i]); err != nil {
			return err
		}
	}
	courses := []domain.Course{
		{ID: "crs_seed1", Name: "初二数学同步", Subject: "数学", TeacherID: "tch_seed1", TotalLessons: 24, PriceCents: 480000, Minutes: 90, Active: true},
		{ID: "crs_seed2", Name: "少儿英语启蒙", Subject: "英语", TeacherID: "tch_seed2", TotalLessons: 20, PriceCents: 360000, Minutes: 60, Active: true},
	}
	for i := range courses {
		if err := r.Courses().Save(ctx, &courses[i]); err != nil {
			return err
		}
	}
	students := []domain.Student{
		{ID: "stu_seed1", Name: "王小明", Guardian: "王女士", Phone: "13900000001", Note: "周三、周五上课", Active: true, CreatedAt: now.AddDate(0, -3, 0)},
		{ID: "stu_seed2", Name: "李思雨", Guardian: "李先生", Phone: "13900000002", Note: "需接送", Active: true, CreatedAt: now.AddDate(0, -2, 0)},
		{ID: "stu_seed3", Name: "张浩然", Guardian: "张先生", Phone: "13900000003", Active: true, CreatedAt: now.AddDate(0, -1, 0)},
	}
	for i := range students {
		if err := r.Students().Save(ctx, &students[i]); err != nil {
			return err
		}
	}
	// Two top-ups through the ledger, so that balance and history agree.
	for _, p := range []struct {
		id     string
		amount int64
		note   string
	}{
		{"stu_seed1", 480000, "报名初二数学同步"},
		{"stu_seed2", 360000, "报名少儿英语启蒙"},
		{"stu_seed3", 200000, "预存"},
	} {
		if _, err := r.Ledger().TopUp(ctx, p.id, p.amount, p.note); err != nil {
			return err
		}
	}
	lessons := []domain.Lesson{
		{ID: "les_seed1", CourseID: "crs_seed1", TeacherID: "tch_seed1", ClassroomID: "rom_seed1", StartAt: now.Add(24 * time.Hour), Minutes: 90, Status: "planned"},
		{ID: "les_seed2", CourseID: "crs_seed2", TeacherID: "tch_seed2", ClassroomID: "rom_seed2", StartAt: now.Add(48 * time.Hour), Minutes: 60, Status: "planned"},
		{ID: "les_seed3", CourseID: "crs_seed1", TeacherID: "tch_seed1", ClassroomID: "rom_seed1", StartAt: now.Add(-72 * time.Hour), Minutes: 90, Status: "done"},
	}
	for i := range lessons {
		if err := r.Lessons().Save(ctx, &lessons[i]); err != nil {
			return err
		}
	}
	return nil
}

// ─── students ───────────────────────────────────────────────────────────

type studentStore struct{ r *sqliteRepo }

const studentCols = `id, name, guardian, phone, balance_cents, note, active, created_at`

func (s studentStore) List(ctx context.Context, q domain.Query) ([]domain.Student, error) {
	sql := `SELECT ` + studentCols + ` FROM students`
	var args []any
	if q.Search != "" {
		sql += ` WHERE name LIKE ?1 OR phone LIKE ?1 OR guardian LIKE ?1`
		args = append(args, "%"+q.Search+"%")
	}
	sql += ` ORDER BY created_at DESC`
	if q.Limit > 0 {
		sql += ` LIMIT ` + strconv.Itoa(q.Limit)
	}
	rows, err := s.r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Student, 0, len(rows.Values))
	for _, v := range rows.Values {
		out = append(out, scanStudent(v))
	}
	return out, nil
}

func (s studentStore) Get(ctx context.Context, id string) (domain.Student, error) {
	rows, err := s.r.db.Query(ctx, `SELECT `+studentCols+` FROM students WHERE id = ?`, id)
	if err != nil {
		return domain.Student{}, err
	}
	if len(rows.Values) == 0 {
		return domain.Student{}, domain.ErrNotFound
	}
	return scanStudent(rows.Values[0]), nil
}

// Save writes the profile only: a balance moves through the ledger, so that
// every cent has a history entry.
func (s studentStore) Save(ctx context.Context, v *domain.Student) error {
	if v.ID == "" {
		v.ID = domain.NewID("stu")
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}
	_, err := s.r.db.Execute(ctx, `
		INSERT INTO students (id, name, guardian, phone, balance_cents, note, active, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			guardian = excluded.guardian,
			phone = excluded.phone,
			note = excluded.note,
			active = excluded.active`,
		v.ID, v.Name, v.Guardian, v.Phone, v.Balance, v.Note, b2i(v.Active), v.CreatedAt.Format(time.RFC3339))
	return err
}

func (s studentStore) Delete(ctx context.Context, id string) error {
	_, err := s.r.db.Execute(ctx, `DELETE FROM students WHERE id = ?`, id)
	return err
}

func scanStudent(v []any) domain.Student {
	return domain.Student{
		ID:        colString(at(v, 0)),
		Name:      colString(at(v, 1)),
		Guardian:  colString(at(v, 2)),
		Phone:     colString(at(v, 3)),
		Balance:   colInt(at(v, 4)),
		Note:      colString(at(v, 5)),
		Active:    colBool(at(v, 6)),
		CreatedAt: colTime(at(v, 7)),
	}
}

// ─── teachers ───────────────────────────────────────────────────────────

type teacherStore struct{ r *sqliteRepo }

const teacherCols = `id, name, phone, subject, rate_cents, active, created_at`

func (s teacherStore) List(ctx context.Context, q domain.Query) ([]domain.Teacher, error) {
	sql := `SELECT ` + teacherCols + ` FROM teachers`
	var args []any
	if q.Search != "" {
		sql += ` WHERE name LIKE ?1 OR phone LIKE ?1 OR subject LIKE ?1`
		args = append(args, "%"+q.Search+"%")
	}
	sql += ` ORDER BY created_at DESC`
	if q.Limit > 0 {
		sql += ` LIMIT ` + strconv.Itoa(q.Limit)
	}
	rows, err := s.r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Teacher, 0, len(rows.Values))
	for _, v := range rows.Values {
		out = append(out, domain.Teacher{
			ID:        colString(at(v, 0)),
			Name:      colString(at(v, 1)),
			Phone:     colString(at(v, 2)),
			Subject:   colString(at(v, 3)),
			RateCents: colInt(at(v, 4)),
			Active:    colBool(at(v, 5)),
			CreatedAt: colTime(at(v, 6)),
		})
	}
	return out, nil
}

func (s teacherStore) Get(ctx context.Context, id string) (domain.Teacher, error) {
	rows, err := s.r.db.Query(ctx, `SELECT `+teacherCols+` FROM teachers WHERE id = ?`, id)
	if err != nil {
		return domain.Teacher{}, err
	}
	if len(rows.Values) == 0 {
		return domain.Teacher{}, domain.ErrNotFound
	}
	v := rows.Values[0]
	return domain.Teacher{
		ID: colString(at(v, 0)), Name: colString(at(v, 1)), Phone: colString(at(v, 2)),
		Subject: colString(at(v, 3)), RateCents: colInt(at(v, 4)), Active: colBool(at(v, 5)),
		CreatedAt: colTime(at(v, 6)),
	}, nil
}

func (s teacherStore) Save(ctx context.Context, v *domain.Teacher) error {
	if v.ID == "" {
		v.ID = domain.NewID("tch")
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}
	_, err := s.r.db.Execute(ctx, `
		INSERT INTO teachers (id, name, phone, subject, rate_cents, active, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name, phone = excluded.phone, subject = excluded.subject,
			rate_cents = excluded.rate_cents, active = excluded.active`,
		v.ID, v.Name, v.Phone, v.Subject, v.RateCents, b2i(v.Active), v.CreatedAt.Format(time.RFC3339))
	return err
}

func (s teacherStore) Delete(ctx context.Context, id string) error {
	_, err := s.r.db.Execute(ctx, `DELETE FROM teachers WHERE id = ?`, id)
	return err
}

// ─── classrooms ─────────────────────────────────────────────────────────

type classroomStore struct{ r *sqliteRepo }

func (s classroomStore) List(ctx context.Context, q domain.Query) ([]domain.Classroom, error) {
	sql := `SELECT id, name, capacity, location, note FROM classrooms`
	var args []any
	if q.Search != "" {
		sql += ` WHERE name LIKE ?1 OR location LIKE ?1`
		args = append(args, "%"+q.Search+"%")
	}
	sql += ` ORDER BY name`
	rows, err := s.r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Classroom, 0, len(rows.Values))
	for _, v := range rows.Values {
		out = append(out, scanClassroom(v))
	}
	return out, nil
}

func (s classroomStore) Get(ctx context.Context, id string) (domain.Classroom, error) {
	rows, err := s.r.db.Query(ctx, `SELECT id, name, capacity, location, note FROM classrooms WHERE id = ?`, id)
	if err != nil {
		return domain.Classroom{}, err
	}
	if len(rows.Values) == 0 {
		return domain.Classroom{}, domain.ErrNotFound
	}
	return scanClassroom(rows.Values[0]), nil
}

func (s classroomStore) Save(ctx context.Context, v *domain.Classroom) error {
	if v.ID == "" {
		v.ID = domain.NewID("rom")
	}
	_, err := s.r.db.Execute(ctx, `
		INSERT INTO classrooms (id, name, capacity, location, note) VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name, capacity = excluded.capacity,
			location = excluded.location, note = excluded.note`,
		v.ID, v.Name, v.Capacity, v.Location, v.Note)
	return err
}

func (s classroomStore) Delete(ctx context.Context, id string) error {
	_, err := s.r.db.Execute(ctx, `DELETE FROM classrooms WHERE id = ?`, id)
	return err
}

func scanClassroom(v []any) domain.Classroom {
	return domain.Classroom{
		ID: colString(at(v, 0)), Name: colString(at(v, 1)),
		Capacity: int(colInt(at(v, 2))), Location: colString(at(v, 3)), Note: colString(at(v, 4)),
	}
}

// ─── courses ────────────────────────────────────────────────────────────

type courseStore struct{ r *sqliteRepo }

const courseCols = `id, name, subject, teacher_id, total_lessons, price_cents, minutes, active`

func (s courseStore) List(ctx context.Context, q domain.Query) ([]domain.Course, error) {
	sql := `SELECT ` + courseCols + ` FROM courses`
	var args []any
	if q.Search != "" {
		sql += ` WHERE name LIKE ?1 OR subject LIKE ?1`
		args = append(args, "%"+q.Search+"%")
	}
	sql += ` ORDER BY name`
	rows, err := s.r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Course, 0, len(rows.Values))
	for _, v := range rows.Values {
		out = append(out, scanCourse(v))
	}
	return out, nil
}

func (s courseStore) Get(ctx context.Context, id string) (domain.Course, error) {
	rows, err := s.r.db.Query(ctx, `SELECT `+courseCols+` FROM courses WHERE id = ?`, id)
	if err != nil {
		return domain.Course{}, err
	}
	if len(rows.Values) == 0 {
		return domain.Course{}, domain.ErrNotFound
	}
	return scanCourse(rows.Values[0]), nil
}

func (s courseStore) Save(ctx context.Context, v *domain.Course) error {
	if v.ID == "" {
		v.ID = domain.NewID("crs")
	}
	_, err := s.r.db.Execute(ctx, `
		INSERT INTO courses (id, name, subject, teacher_id, total_lessons, price_cents, minutes, active)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name, subject = excluded.subject, teacher_id = excluded.teacher_id,
			total_lessons = excluded.total_lessons, price_cents = excluded.price_cents,
			minutes = excluded.minutes, active = excluded.active`,
		v.ID, v.Name, v.Subject, v.TeacherID, v.TotalLessons, v.PriceCents, v.Minutes, b2i(v.Active))
	return err
}

func (s courseStore) Delete(ctx context.Context, id string) error {
	_, err := s.r.db.Execute(ctx, `DELETE FROM courses WHERE id = ?`, id)
	return err
}

func scanCourse(v []any) domain.Course {
	return domain.Course{
		ID: colString(at(v, 0)), Name: colString(at(v, 1)), Subject: colString(at(v, 2)),
		TeacherID: colString(at(v, 3)), TotalLessons: int(colInt(at(v, 4))),
		PriceCents: colInt(at(v, 5)), Minutes: int(colInt(at(v, 6))), Active: colBool(at(v, 7)),
	}
}

// ─── lessons ────────────────────────────────────────────────────────────

type lessonStore struct{ r *sqliteRepo }

const lessonCols = `id, course_id, teacher_id, classroom_id, start_at, minutes, status`

func (s lessonStore) List(ctx context.Context, q domain.Query) ([]domain.Lesson, error) {
	sql := `SELECT ` + lessonCols + ` FROM lessons`
	var args []any
	if q.Search != "" {
		sql += ` WHERE status LIKE ?1`
		args = append(args, "%"+q.Search+"%")
	}
	sql += ` ORDER BY start_at DESC`
	if q.Limit > 0 {
		sql += ` LIMIT ` + strconv.Itoa(q.Limit)
	}
	rows, err := s.r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Lesson, 0, len(rows.Values))
	for _, v := range rows.Values {
		out = append(out, scanLesson(v))
	}
	return out, nil
}

func (s lessonStore) Get(ctx context.Context, id string) (domain.Lesson, error) {
	rows, err := s.r.db.Query(ctx, `SELECT `+lessonCols+` FROM lessons WHERE id = ?`, id)
	if err != nil {
		return domain.Lesson{}, err
	}
	if len(rows.Values) == 0 {
		return domain.Lesson{}, domain.ErrNotFound
	}
	return scanLesson(rows.Values[0]), nil
}

func (s lessonStore) Save(ctx context.Context, v *domain.Lesson) error {
	if v.ID == "" {
		v.ID = domain.NewID("les")
	}
	if v.Status == "" {
		v.Status = "planned"
	}
	_, err := s.r.db.Execute(ctx, `
		INSERT INTO lessons (id, course_id, teacher_id, classroom_id, start_at, minutes, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			course_id = excluded.course_id, teacher_id = excluded.teacher_id,
			classroom_id = excluded.classroom_id, start_at = excluded.start_at,
			minutes = excluded.minutes, status = excluded.status`,
		v.ID, v.CourseID, v.TeacherID, v.ClassroomID, v.StartAt.Format(time.RFC3339), v.Minutes, v.Status)
	return err
}

func (s lessonStore) Delete(ctx context.Context, id string) error {
	_, err := s.r.db.Execute(ctx, `DELETE FROM lessons WHERE id = ?`, id)
	return err
}

func scanLesson(v []any) domain.Lesson {
	return domain.Lesson{
		ID: colString(at(v, 0)), CourseID: colString(at(v, 1)), TeacherID: colString(at(v, 2)),
		ClassroomID: colString(at(v, 3)), StartAt: colTime(at(v, 4)),
		Minutes: int(colInt(at(v, 5))), Status: colString(at(v, 6)),
	}
}

// ─── ledger ─────────────────────────────────────────────────────────────

type ledgerStore struct{ r *sqliteRepo }

func (l ledgerStore) List(ctx context.Context, studentID string, limit int) ([]domain.LedgerEntry, error) {
	sql := `SELECT id, student_id, kind, amount_cents, balance_cents, note, at FROM ledger`
	var args []any
	if studentID != "" {
		sql += ` WHERE student_id = ?1`
		args = append(args, studentID)
	}
	sql += ` ORDER BY at DESC, id DESC`
	if limit > 0 {
		sql += ` LIMIT ` + strconv.Itoa(limit)
	}
	rows, err := l.r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	out := make([]domain.LedgerEntry, 0, len(rows.Values))
	for _, v := range rows.Values {
		out = append(out, domain.LedgerEntry{
			ID: colString(at(v, 0)), StudentID: colString(at(v, 1)), Kind: colString(at(v, 2)),
			Amount: colInt(at(v, 3)), Balance: colInt(at(v, 4)), Note: colString(at(v, 5)),
			At: colTime(at(v, 6)),
		})
	}
	return out, nil
}

func (l ledgerStore) TopUp(ctx context.Context, studentID string, amountCents int64, note string) (domain.LedgerEntry, error) {
	if amountCents <= 0 {
		return domain.LedgerEntry{}, errors.New("充值金额必须大于 0")
	}
	return l.move(ctx, studentID, amountCents, domain.KindTopUp, note)
}

func (l ledgerStore) Consume(ctx context.Context, studentID string, amountCents int64, note string) (domain.LedgerEntry, error) {
	if amountCents <= 0 {
		return domain.LedgerEntry{}, errors.New("消课金额必须大于 0")
	}
	return l.move(ctx, studentID, -amountCents, domain.KindConsume, note)
}

// move reads the balance, applies delta and appends the entry, one step at
// a time under mu, so that two quick clicks cannot both read the old value.
func (l ledgerStore) move(ctx context.Context, studentID string, delta int64, kind, note string) (domain.LedgerEntry, error) {
	l.r.mu.Lock()
	defer l.r.mu.Unlock()

	rows, err := l.r.db.Query(ctx, `SELECT balance_cents FROM students WHERE id = ?`, studentID)
	if err != nil {
		return domain.LedgerEntry{}, err
	}
	if len(rows.Values) == 0 {
		return domain.LedgerEntry{}, domain.ErrNotFound
	}
	next := colInt(rows.Values[0][0]) + delta
	if next < 0 {
		return domain.LedgerEntry{}, errors.New("余额不足，无法消课")
	}
	e := domain.LedgerEntry{
		ID: domain.NewID("led"), StudentID: studentID, Kind: kind,
		Amount: delta, Balance: next, Note: note, At: time.Now(),
	}
	if _, err := l.r.db.Execute(ctx, `UPDATE students SET balance_cents = ? WHERE id = ?`, next, studentID); err != nil {
		return domain.LedgerEntry{}, err
	}
	if _, err := l.r.db.Execute(ctx,
		`INSERT INTO ledger (id, student_id, kind, amount_cents, balance_cents, note, at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.StudentID, e.Kind, e.Amount, e.Balance, e.Note, e.At.Format(time.RFC3339)); err != nil {
		return domain.LedgerEntry{}, err
	}
	return e, nil
}

// ─── users ──────────────────────────────────────────────────────────────

// The account a fresh database starts with, so that the app opens on a
// login it can pass. Change the password before shipping.
const (
	DefaultUsername = "admin"
	DefaultPassword = "admin123"
)

// pbkdf2Iterations is high enough to make a guess slow, low enough that a
// login stays instant.
const pbkdf2Iterations = 120_000

type authStore struct{ r *sqliteRepo }

// ensureAdmin adds the default account to a database that has none, and
// leaves an existing one alone: a changed password survives a restart.
func (r *sqliteRepo) ensureAdmin(ctx context.Context) error {
	rows, err := r.db.Query(ctx, `SELECT COUNT(*) FROM users`)
	if err != nil {
		return err
	}
	if len(rows.Values) == 1 && colInt(rows.Values[0][0]) > 0 {
		return nil
	}
	salt, hash, err := hashPassword(DefaultPassword)
	if err != nil {
		return err
	}
	_, err = r.db.Execute(ctx, `
		INSERT INTO users (id, username, display_name, role, salt, hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		domain.NewID("usr"), DefaultUsername, "管理员", "admin", salt, hash,
		time.Now().Format(time.RFC3339))
	return err
}

func (a authStore) Login(ctx context.Context, username, password string) (domain.User, error) {
	rows, err := a.r.db.Query(ctx, `
		SELECT id, username, display_name, role, salt, hash, created_at
		FROM users WHERE username = ?`, strings.TrimSpace(username))
	if err != nil {
		return domain.User{}, err
	}
	if len(rows.Values) == 0 {
		return domain.User{}, domain.ErrBadCredentials
	}
	v := rows.Values[0]
	if !checkPassword(password, colString(at(v, 4)), colString(at(v, 5))) {
		return domain.User{}, domain.ErrBadCredentials
	}
	return domain.User{
		ID:          colString(at(v, 0)),
		Username:    colString(at(v, 1)),
		DisplayName: colString(at(v, 2)),
		Role:        colString(at(v, 3)),
		CreatedAt:   colTime(at(v, 6)),
	}, nil
}

// hashPassword salts and stretches the password with PBKDF2-SHA256, from
// the standard library: no dependency, and no plain text on disk.
func hashPassword(password string) (saltHex, hashHex string, err error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", "", err
	}
	sum, err := pbkdf2.Key(sha256.New, password, salt, pbkdf2Iterations, 32)
	if err != nil {
		return "", "", err
	}
	return hex.EncodeToString(salt), hex.EncodeToString(sum), nil
}

// checkPassword derives the hash with the stored salt and compares in
// constant time, so that a wrong password takes as long as a right one.
func checkPassword(password, saltHex, hashHex string) bool {
	salt, err := hex.DecodeString(saltHex)
	if err != nil {
		return false
	}
	want, err := hex.DecodeString(hashHex)
	if err != nil || len(want) == 0 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, pbkdf2Iterations, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

// ─── accounts ───────────────────────────────────────────────────────────

// userStore manages the accounts an admin hands out. It refuses the two
// ways a database could end up with nobody able to administer it: dropping
// the last admin, and demoting it.
type userStore struct{ r *sqliteRepo }

func (u userStore) List(ctx context.Context) ([]domain.User, error) {
	rows, err := u.r.db.Query(ctx, `
		SELECT id, username, display_name, role, created_at FROM users ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	out := make([]domain.User, 0, len(rows.Values))
	for _, v := range rows.Values {
		out = append(out, domain.User{
			ID:          colString(at(v, 0)),
			Username:    colString(at(v, 1)),
			DisplayName: colString(at(v, 2)),
			Role:        colString(at(v, 3)),
			CreatedAt:   colTime(at(v, 4)),
		})
	}
	return out, nil
}

func (u userStore) Create(ctx context.Context, v *domain.User, password string) error {
	v.Username = strings.TrimSpace(v.Username)
	if v.Username == "" {
		return errors.New("用户名不能为空")
	}
	if len([]rune(password)) < 6 {
		return errors.New("密码至少 6 位")
	}
	if v.Role == "" {
		v.Role = string(domain.RoleStaff)
	}
	salt, hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	if v.ID == "" {
		v.ID = domain.NewID("usr")
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}
	_, err = u.r.db.Execute(ctx, `
		INSERT INTO users (id, username, display_name, role, salt, hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		v.ID, v.Username, v.DisplayName, v.Role, salt, hash, v.CreatedAt.Format(time.RFC3339))
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return errors.New("用户名已存在")
	}
	return err
}

func (u userStore) Update(ctx context.Context, v domain.User) error {
	if v.Role != string(domain.RoleAdmin) {
		if err := u.keepOneAdmin(ctx, v.ID); err != nil {
			return err
		}
	}
	_, err := u.r.db.Execute(ctx, `
		UPDATE users SET display_name = ?, role = ? WHERE id = ?`,
		v.DisplayName, v.Role, v.ID)
	return err
}

func (u userStore) SetPassword(ctx context.Context, id, password string) error {
	if len([]rune(password)) < 6 {
		return errors.New("密码至少 6 位")
	}
	salt, hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	_, err = u.r.db.Execute(ctx, `UPDATE users SET salt = ?, hash = ? WHERE id = ?`, salt, hash, id)
	return err
}

func (u userStore) Delete(ctx context.Context, id string) error {
	if err := u.keepOneAdmin(ctx, id); err != nil {
		return err
	}
	_, err := u.r.db.Execute(ctx, `DELETE FROM users WHERE id = ?`, id)
	return err
}

// keepOneAdmin refuses to leave the database without an admin: it asks
// whether id is the last one.
func (u userStore) keepOneAdmin(ctx context.Context, id string) error {
	rows, err := u.r.db.Query(ctx, `SELECT COUNT(*) FROM users WHERE role = 'admin' AND id <> ?`, id)
	if err != nil {
		return err
	}
	if colInt(at(rows.Values[0], 0)) > 0 {
		return nil
	}
	cur, err := u.r.db.Query(ctx, `SELECT role FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if len(cur.Values) == 1 && colString(at(cur.Values[0], 0)) == string(domain.RoleAdmin) {
		return errors.New("至少要保留一个管理员")
	}
	return nil
}

// ─── settings ───────────────────────────────────────────────────────────

// settingsStore keeps the app's preferences as key/value pairs: the same
// file as the data, so a backup carries both.
type settingsStore struct{ r *sqliteRepo }

func (s settingsStore) Get(ctx context.Context, key string) (string, error) {
	rows, err := s.r.db.Query(ctx, `SELECT value FROM settings WHERE key = ?`, key)
	if err != nil {
		return "", err
	}
	if len(rows.Values) == 0 {
		return "", nil
	}
	return colString(at(rows.Values[0], 0)), nil
}

func (s settingsStore) Set(ctx context.Context, key, value string) error {
	_, err := s.r.db.Execute(ctx, `
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// ─── scanning helpers ───────────────────────────────────────────────────

func at(v []any, i int) any {
	if i < len(v) {
		return v[i]
	}
	return nil
}

func colString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return string(x)
	case nil:
		return ""
	default:
		return fmt.Sprint(x)
	}
}

func colInt(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		return n
	}
	return 0
}

func colBool(v any) bool { return colInt(v) != 0 }

func colTime(v any) time.Time {
	s := colString(v)
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
