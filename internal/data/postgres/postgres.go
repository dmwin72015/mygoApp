package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"mygo-app/internal/data/crypto"
	"mygo-app/internal/domain"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Repo is the PostgreSQL implementation of domain.Repo.
type Repo struct {
	db  *sql.DB
	dsn string
}

// Open connects to PostgreSQL and verifies the schema via migrations.
func Open(ctx context.Context, dsn string) (domain.Repo, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres %s: %w", SanitizeDSN(dsn), err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres %s: %w", SanitizeDSN(dsn), err)
	}

	if err := Migrate(ctx, db, "up"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate postgres: %w", err)
	}

	r := &Repo{db: db, dsn: dsn}

	if err := r.ensureAdmin(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("admin account: %w", err)
	}

	if err := r.seed(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("seed: %w", err)
	}

	return r, nil
}

func (r *Repo) Close() error {
	return r.db.Close()
}

func (r *Repo) Source() string {
	return "PostgreSQL · " + SanitizeDSN(r.dsn)
}

func (r *Repo) Auth() domain.Auth                         { return pgAuthStore{r} }
func (r *Repo) Settings() domain.Settings                 { return pgSettingsStore{r} }
func (r *Repo) Users() domain.UserStore                   { return pgUserStore{r} }
func (r *Repo) Students() domain.Store[domain.Student]     { return pgStudentStore{r} }
func (r *Repo) Teachers() domain.Store[domain.Teacher]     { return pgTeacherStore{r} }
func (r *Repo) Classrooms() domain.Store[domain.Classroom] { return pgClassroomStore{r} }
func (r *Repo) Courses() domain.Store[domain.Course]       { return pgCourseStore{r} }
func (r *Repo) Lessons() domain.Store[domain.Lesson]       { return pgLessonStore{r} }
func (r *Repo) Ledger() domain.Ledger                      { return pgLedgerStore{r} }

func (r *Repo) Stats(ctx context.Context) (domain.Stats, error) {
	var st domain.Stats
	count := func(query string) (int64, error) {
		var n int64
		err := r.db.QueryRowContext(ctx, query).Scan(&n)
		if err != nil {
			return 0, err
		}
		return n, nil
	}

	var err error
	var n int64
	if n, err = count("SELECT COUNT(*) FROM students WHERE active = TRUE"); err != nil {
		return st, err
	}
	st.Students = int(n)

	if n, err = count("SELECT COUNT(*) FROM teachers WHERE active = TRUE"); err != nil {
		return st, err
	}
	st.Teachers = int(n)

	if n, err = count("SELECT COUNT(*) FROM courses WHERE active = TRUE"); err != nil {
		return st, err
	}
	st.Courses = int(n)

	if n, err = count("SELECT COUNT(*) FROM lessons WHERE status = 'planned'"); err != nil {
		return st, err
	}
	st.Lessons = int(n)

	if st.Revenue, err = count("SELECT COALESCE(SUM(amount_cents), 0) FROM ledger WHERE kind = 'topup'"); err != nil {
		return st, err
	}
	if st.Outstanding, err = count("SELECT COALESCE(SUM(balance_cents), 0) FROM students WHERE active = TRUE"); err != nil {
		return st, err
	}

	return st, nil
}

func (r *Repo) ensureAdmin(ctx context.Context) error {
	var count int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE username = 'admin'").Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	salt, hash, err := crypto.HashPassword("admin123")
	if err != nil {
		return err
	}

	admin := domain.User{
		ID:          "usr_admin",
		Username:    "admin",
		DisplayName: "系统管理员",
		Role:        string(domain.RoleAdmin),
		CreatedAt:   time.Now(),
	}

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO users (id, username, display_name, role, salt, hash, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (username) DO NOTHING`,
		admin.ID, admin.Username, admin.DisplayName, admin.Role, salt, hash, admin.CreatedAt,
	)
	return err
}

func (r *Repo) seed(ctx context.Context) error {
	var count int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM students").Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
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
		{ID: "rom_seed1", Name: "A101", Capacity: 12, Location: "一楼东侧", Note: "多媒体教室"},
		{ID: "rom_seed2", Name: "B203", Capacity: 8, Location: "二楼", Note: "小班教室"},
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
		{ID: "stu_seed3", Name: "张浩然", Guardian: "张先生", Phone: "13900000003", Note: "周末走读", Active: true, CreatedAt: now.AddDate(0, -1, 0)},
	}
	for i := range students {
		if err := r.Students().Save(ctx, &students[i]); err != nil {
			return err
		}
	}

	for _, p := range []struct {
		id     string
		amount int64
		note   string
	}{
		{"stu_seed1", 480000, "报名初二数学同步"},
		{"stu_seed2", 360000, "报名少儿英语启蒙"},
		{"stu_seed3", 200000, "预存充值"},
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

// ─── Students ──────────────────────────────────────────────────────────

type pgStudentStore struct{ r *Repo }

func (s pgStudentStore) List(ctx context.Context, q domain.Query) ([]domain.Student, error) {
	var (
		query strings.Builder
		args  []any
	)
	query.WriteString("SELECT id, name, guardian, phone, balance_cents, note, active, created_at FROM students")
	if q.Search != "" {
		like := "%" + q.Search + "%"
		args = append(args, like)
		query.WriteString(fmt.Sprintf(" WHERE (name ILIKE $%d OR phone ILIKE $%d OR guardian ILIKE $%d)", len(args), len(args), len(args)))
	}
	query.WriteString(" ORDER BY created_at DESC")
	if q.Limit > 0 {
		args = append(args, q.Limit)
		query.WriteString(fmt.Sprintf(" LIMIT $%d", len(args)))
	}

	rows, err := s.r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Student
	for rows.Next() {
		var item domain.Student
		if err := rows.Scan(&item.ID, &item.Name, &item.Guardian, &item.Phone, &item.Balance, &item.Note, &item.Active, &item.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func (s pgStudentStore) Get(ctx context.Context, id string) (domain.Student, error) {
	var item domain.Student
	err := s.r.db.QueryRowContext(ctx,
		"SELECT id, name, guardian, phone, balance_cents, note, active, created_at FROM students WHERE id = $1",
		id,
	).Scan(&item.ID, &item.Name, &item.Guardian, &item.Phone, &item.Balance, &item.Note, &item.Active, &item.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Student{}, domain.ErrNotFound
	}
	return item, err
}

func (s pgStudentStore) Save(ctx context.Context, v *domain.Student) error {
	if strings.TrimSpace(v.Name) == "" {
		return errors.New("姓名不能为空")
	}
	if v.ID == "" {
		v.ID = domain.NewID("stu")
		if v.CreatedAt.IsZero() {
			v.CreatedAt = time.Now()
		}
		_, err := s.r.db.ExecContext(ctx,
			`INSERT INTO students (id, name, guardian, phone, balance_cents, note, active, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			v.ID, v.Name, v.Guardian, v.Phone, v.Balance, v.Note, v.Active, v.CreatedAt,
		)
		return err
	}

	res, err := s.r.db.ExecContext(ctx,
		`UPDATE students
		 SET name = $1, guardian = $2, phone = $3, balance_cents = $4, note = $5, active = $6
		 WHERE id = $7`,
		v.Name, v.Guardian, v.Phone, v.Balance, v.Note, v.Active, v.ID,
	)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (s pgStudentStore) Delete(ctx context.Context, id string) error {
	res, err := s.r.db.ExecContext(ctx, "DELETE FROM students WHERE id = $1", id)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}

// ─── Teachers ──────────────────────────────────────────────────────────

type pgTeacherStore struct{ r *Repo }

func (t pgTeacherStore) List(ctx context.Context, q domain.Query) ([]domain.Teacher, error) {
	var (
		query strings.Builder
		args  []any
	)
	query.WriteString("SELECT id, name, phone, subject, rate_cents, active, created_at FROM teachers")
	if q.Search != "" {
		like := "%" + q.Search + "%"
		args = append(args, like)
		query.WriteString(fmt.Sprintf(" WHERE (name ILIKE $%d OR phone ILIKE $%d OR subject ILIKE $%d)", len(args), len(args), len(args)))
	}
	query.WriteString(" ORDER BY created_at DESC")
	if q.Limit > 0 {
		args = append(args, q.Limit)
		query.WriteString(fmt.Sprintf(" LIMIT $%d", len(args)))
	}

	rows, err := t.r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Teacher
	for rows.Next() {
		var item domain.Teacher
		if err := rows.Scan(&item.ID, &item.Name, &item.Phone, &item.Subject, &item.RateCents, &item.Active, &item.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func (t pgTeacherStore) Get(ctx context.Context, id string) (domain.Teacher, error) {
	var item domain.Teacher
	err := t.r.db.QueryRowContext(ctx,
		"SELECT id, name, phone, subject, rate_cents, active, created_at FROM teachers WHERE id = $1",
		id,
	).Scan(&item.ID, &item.Name, &item.Phone, &item.Subject, &item.RateCents, &item.Active, &item.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Teacher{}, domain.ErrNotFound
	}
	return item, err
}

func (t pgTeacherStore) Save(ctx context.Context, v *domain.Teacher) error {
	if strings.TrimSpace(v.Name) == "" {
		return errors.New("姓名不能为空")
	}
	if v.ID == "" {
		v.ID = domain.NewID("tch")
		if v.CreatedAt.IsZero() {
			v.CreatedAt = time.Now()
		}
		_, err := t.r.db.ExecContext(ctx,
			`INSERT INTO teachers (id, name, phone, subject, rate_cents, active, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			v.ID, v.Name, v.Phone, v.Subject, v.RateCents, v.Active, v.CreatedAt,
		)
		return err
	}

	res, err := t.r.db.ExecContext(ctx,
		`UPDATE teachers
		 SET name = $1, phone = $2, subject = $3, rate_cents = $4, active = $5
		 WHERE id = $6`,
		v.Name, v.Phone, v.Subject, v.RateCents, v.Active, v.ID,
	)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (t pgTeacherStore) Delete(ctx context.Context, id string) error {
	res, err := t.r.db.ExecContext(ctx, "DELETE FROM teachers WHERE id = $1", id)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}

// ─── Classrooms ────────────────────────────────────────────────────────

type pgClassroomStore struct{ r *Repo }

func (c pgClassroomStore) List(ctx context.Context, q domain.Query) ([]domain.Classroom, error) {
	var (
		query strings.Builder
		args  []any
	)
	query.WriteString("SELECT id, name, capacity, location, note FROM classrooms")
	if q.Search != "" {
		like := "%" + q.Search + "%"
		args = append(args, like)
		query.WriteString(fmt.Sprintf(" WHERE (name ILIKE $%d OR location ILIKE $%d)", len(args), len(args)))
	}
	query.WriteString(" ORDER BY name ASC")
	if q.Limit > 0 {
		args = append(args, q.Limit)
		query.WriteString(fmt.Sprintf(" LIMIT $%d", len(args)))
	}

	rows, err := c.r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Classroom
	for rows.Next() {
		var item domain.Classroom
		if err := rows.Scan(&item.ID, &item.Name, &item.Capacity, &item.Location, &item.Note); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func (c pgClassroomStore) Get(ctx context.Context, id string) (domain.Classroom, error) {
	var item domain.Classroom
	err := c.r.db.QueryRowContext(ctx,
		"SELECT id, name, capacity, location, note FROM classrooms WHERE id = $1",
		id,
	).Scan(&item.ID, &item.Name, &item.Capacity, &item.Location, &item.Note)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Classroom{}, domain.ErrNotFound
	}
	return item, err
}

func (c pgClassroomStore) Save(ctx context.Context, v *domain.Classroom) error {
	if strings.TrimSpace(v.Name) == "" {
		return errors.New("教室名称不能为空")
	}
	if v.ID == "" {
		v.ID = domain.NewID("rom")
		_, err := c.r.db.ExecContext(ctx,
			`INSERT INTO classrooms (id, name, capacity, location, note)
			 VALUES ($1, $2, $3, $4, $5)`,
			v.ID, v.Name, v.Capacity, v.Location, v.Note,
		)
		return err
	}

	res, err := c.r.db.ExecContext(ctx,
		`UPDATE classrooms
		 SET name = $1, capacity = $2, location = $3, note = $4
		 WHERE id = $5`,
		v.Name, v.Capacity, v.Location, v.Note, v.ID,
	)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (c pgClassroomStore) Delete(ctx context.Context, id string) error {
	res, err := c.r.db.ExecContext(ctx, "DELETE FROM classrooms WHERE id = $1", id)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}

// ─── Courses ───────────────────────────────────────────────────────────

type pgCourseStore struct{ r *Repo }

func (s pgCourseStore) List(ctx context.Context, q domain.Query) ([]domain.Course, error) {
	var (
		query strings.Builder
		args  []any
	)
	query.WriteString("SELECT id, name, subject, teacher_id, total_lessons, price_cents, minutes, active FROM courses")
	if q.Search != "" {
		like := "%" + q.Search + "%"
		args = append(args, like)
		query.WriteString(fmt.Sprintf(" WHERE (name ILIKE $%d OR subject ILIKE $%d)", len(args), len(args)))
	}
	query.WriteString(" ORDER BY id DESC")
	if q.Limit > 0 {
		args = append(args, q.Limit)
		query.WriteString(fmt.Sprintf(" LIMIT $%d", len(args)))
	}

	rows, err := s.r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Course
	for rows.Next() {
		var item domain.Course
		if err := rows.Scan(&item.ID, &item.Name, &item.Subject, &item.TeacherID, &item.TotalLessons, &item.PriceCents, &item.Minutes, &item.Active); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func (s pgCourseStore) Get(ctx context.Context, id string) (domain.Course, error) {
	var item domain.Course
	err := s.r.db.QueryRowContext(ctx,
		"SELECT id, name, subject, teacher_id, total_lessons, price_cents, minutes, active FROM courses WHERE id = $1",
		id,
	).Scan(&item.ID, &item.Name, &item.Subject, &item.TeacherID, &item.TotalLessons, &item.PriceCents, &item.Minutes, &item.Active)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Course{}, domain.ErrNotFound
	}
	return item, err
}

func (s pgCourseStore) Save(ctx context.Context, v *domain.Course) error {
	if strings.TrimSpace(v.Name) == "" {
		return errors.New("课程名称不能为空")
	}
	if v.ID == "" {
		v.ID = domain.NewID("crs")
		_, err := s.r.db.ExecContext(ctx,
			`INSERT INTO courses (id, name, subject, teacher_id, total_lessons, price_cents, minutes, active)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			v.ID, v.Name, v.Subject, v.TeacherID, v.TotalLessons, v.PriceCents, v.Minutes, v.Active,
		)
		return err
	}

	res, err := s.r.db.ExecContext(ctx,
		`UPDATE courses
		 SET name = $1, subject = $2, teacher_id = $3, total_lessons = $4, price_cents = $5, minutes = $6, active = $7
		 WHERE id = $8`,
		v.Name, v.Subject, v.TeacherID, v.TotalLessons, v.PriceCents, v.Minutes, v.Active, v.ID,
	)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (s pgCourseStore) Delete(ctx context.Context, id string) error {
	res, err := s.r.db.ExecContext(ctx, "DELETE FROM courses WHERE id = $1", id)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}

// ─── Lessons ───────────────────────────────────────────────────────────

type pgLessonStore struct{ r *Repo }

func (s pgLessonStore) List(ctx context.Context, q domain.Query) ([]domain.Lesson, error) {
	var (
		query strings.Builder
		args  []any
	)
	query.WriteString("SELECT id, course_id, teacher_id, classroom_id, start_at, minutes, status FROM lessons")
	if q.Search != "" {
		like := "%" + q.Search + "%"
		args = append(args, like)
		query.WriteString(fmt.Sprintf(" WHERE (status ILIKE $%d OR course_id ILIKE $%d)", len(args), len(args)))
	}
	query.WriteString(" ORDER BY start_at DESC")
	if q.Limit > 0 {
		args = append(args, q.Limit)
		query.WriteString(fmt.Sprintf(" LIMIT $%d", len(args)))
	}

	rows, err := s.r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Lesson
	for rows.Next() {
		var item domain.Lesson
		if err := rows.Scan(&item.ID, &item.CourseID, &item.TeacherID, &item.ClassroomID, &item.StartAt, &item.Minutes, &item.Status); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func (s pgLessonStore) Get(ctx context.Context, id string) (domain.Lesson, error) {
	var item domain.Lesson
	err := s.r.db.QueryRowContext(ctx,
		"SELECT id, course_id, teacher_id, classroom_id, start_at, minutes, status FROM lessons WHERE id = $1",
		id,
	).Scan(&item.ID, &item.CourseID, &item.TeacherID, &item.ClassroomID, &item.StartAt, &item.Minutes, &item.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Lesson{}, domain.ErrNotFound
	}
	return item, err
}

func (s pgLessonStore) Save(ctx context.Context, v *domain.Lesson) error {
	if v.StartAt.IsZero() {
		return errors.New("上课时间不能为空")
	}
	if v.Status == "" {
		v.Status = "planned"
	}
	if v.ID == "" {
		v.ID = domain.NewID("les")
		_, err := s.r.db.ExecContext(ctx,
			`INSERT INTO lessons (id, course_id, teacher_id, classroom_id, start_at, minutes, status)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			v.ID, v.CourseID, v.TeacherID, v.ClassroomID, v.StartAt, v.Minutes, v.Status,
		)
		return err
	}

	res, err := s.r.db.ExecContext(ctx,
		`UPDATE lessons
		 SET course_id = $1, teacher_id = $2, classroom_id = $3, start_at = $4, minutes = $5, status = $6
		 WHERE id = $7`,
		v.CourseID, v.TeacherID, v.ClassroomID, v.StartAt, v.Minutes, v.Status, v.ID,
	)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (s pgLessonStore) Delete(ctx context.Context, id string) error {
	res, err := s.r.db.ExecContext(ctx, "DELETE FROM lessons WHERE id = $1", id)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}

// ─── Ledger ────────────────────────────────────────────────────────────

type pgLedgerStore struct{ r *Repo }

func (l pgLedgerStore) List(ctx context.Context, studentID string, limit int) ([]domain.LedgerEntry, error) {
	var (
		query strings.Builder
		args  []any
	)
	query.WriteString("SELECT id, student_id, kind, amount_cents, balance_cents, note, at FROM ledger WHERE student_id = $1 ORDER BY at DESC")
	args = append(args, studentID)
	if limit > 0 {
		args = append(args, limit)
		query.WriteString(fmt.Sprintf(" LIMIT $%d", len(args)))
	}

	rows, err := l.r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.LedgerEntry
	for rows.Next() {
		var item domain.LedgerEntry
		if err := rows.Scan(&item.ID, &item.StudentID, &item.Kind, &item.Amount, &item.Balance, &item.Note, &item.At); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func (l pgLedgerStore) TopUp(ctx context.Context, studentID string, amountCents int64, note string) (domain.LedgerEntry, error) {
	if amountCents <= 0 {
		return domain.LedgerEntry{}, errors.New("充值金额必须大于 0")
	}
	return l.apply(ctx, studentID, domain.KindTopUp, amountCents, note)
}

func (l pgLedgerStore) Consume(ctx context.Context, studentID string, amountCents int64, note string) (domain.LedgerEntry, error) {
	if amountCents <= 0 {
		return domain.LedgerEntry{}, errors.New("扣减金额必须大于 0")
	}
	return l.apply(ctx, studentID, domain.KindConsume, -amountCents, note)
}

func (l pgLedgerStore) apply(ctx context.Context, studentID, kind string, delta int64, note string) (domain.LedgerEntry, error) {
	tx, err := l.r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return domain.LedgerEntry{}, err
	}
	defer tx.Rollback()

	var currentBalance int64
	err = tx.QueryRowContext(ctx, "SELECT balance_cents FROM students WHERE id = $1 FOR UPDATE", studentID).Scan(&currentBalance)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.LedgerEntry{}, fmt.Errorf("student %s: %w", studentID, domain.ErrNotFound)
	}
	if err != nil {
		return domain.LedgerEntry{}, err
	}

	newBalance := currentBalance + delta

	_, err = tx.ExecContext(ctx, "UPDATE students SET balance_cents = $1 WHERE id = $2", newBalance, studentID)
	if err != nil {
		return domain.LedgerEntry{}, err
	}

	entry := domain.LedgerEntry{
		ID:        domain.NewID("ldg"),
		StudentID: studentID,
		Kind:      kind,
		Amount:    delta,
		Balance:   newBalance,
		Note:      note,
		At:        time.Now(),
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO ledger (id, student_id, kind, amount_cents, balance_cents, note, at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		entry.ID, entry.StudentID, entry.Kind, entry.Amount, entry.Balance, entry.Note, entry.At,
	)
	if err != nil {
		return domain.LedgerEntry{}, err
	}

	if err := tx.Commit(); err != nil {
		return domain.LedgerEntry{}, err
	}
	return entry, nil
}

// ─── Users & Auth ──────────────────────────────────────────────────────

type pgAuthStore struct{ r *Repo }

func (a pgAuthStore) Login(ctx context.Context, username, password string) (domain.User, error) {
	var (
		u          domain.User
		salt, hash string
	)
	err := a.r.db.QueryRowContext(ctx,
		"SELECT id, username, display_name, role, salt, hash, created_at FROM users WHERE username = $1",
		username,
	).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Role, &salt, &hash, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, domain.ErrBadCredentials
	}
	if err != nil {
		return domain.User{}, err
	}

	if !crypto.CheckPassword(password, salt, hash) {
		return domain.User{}, domain.ErrBadCredentials
	}
	return u, nil
}

type pgUserStore struct{ r *Repo }

func (u pgUserStore) List(ctx context.Context) ([]domain.User, error) {
	rows, err := u.r.db.QueryContext(ctx,
		"SELECT id, username, display_name, role, created_at FROM users ORDER BY created_at ASC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.User
	for rows.Next() {
		var item domain.User
		if err := rows.Scan(&item.ID, &item.Username, &item.DisplayName, &item.Role, &item.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func (u pgUserStore) Create(ctx context.Context, v *domain.User, password string) error {
	if strings.TrimSpace(v.Username) == "" {
		return errors.New("用户名不能为空")
	}
	if len([]rune(password)) < 6 {
		return errors.New("密码至少 6 位")
	}
	salt, hash, err := crypto.HashPassword(password)
	if err != nil {
		return err
	}
	if v.ID == "" {
		v.ID = domain.NewID("usr")
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}

	_, err = u.r.db.ExecContext(ctx,
		`INSERT INTO users (id, username, display_name, role, salt, hash, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		v.ID, v.Username, v.DisplayName, v.Role, salt, hash, v.CreatedAt,
	)
	if err != nil && (strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate")) {
		return errors.New("用户名已存在")
	}
	return err
}

func (u pgUserStore) Update(ctx context.Context, v domain.User) error {
	if v.Role != string(domain.RoleAdmin) {
		if err := u.keepOneAdmin(ctx, v.ID); err != nil {
			return err
		}
	}
	res, err := u.r.db.ExecContext(ctx,
		"UPDATE users SET display_name = $1, role = $2 WHERE id = $3",
		v.DisplayName, v.Role, v.ID,
	)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (u pgUserStore) SetPassword(ctx context.Context, id, password string) error {
	if len([]rune(password)) < 6 {
		return errors.New("密码至少 6 位")
	}
	salt, hash, err := crypto.HashPassword(password)
	if err != nil {
		return err
	}
	res, err := u.r.db.ExecContext(ctx, "UPDATE users SET salt = $1, hash = $2 WHERE id = $3", salt, hash, id)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (u pgUserStore) Delete(ctx context.Context, id string) error {
	if err := u.keepOneAdmin(ctx, id); err != nil {
		return err
	}
	res, err := u.r.db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", id)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (u pgUserStore) keepOneAdmin(ctx context.Context, id string) error {
	var remainingAdminCount int64
	err := u.r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE role = 'admin' AND id <> $1",
		id,
	).Scan(&remainingAdminCount)
	if err != nil {
		return err
	}
	if remainingAdminCount > 0 {
		return nil
	}

	var currentRole string
	err = u.r.db.QueryRowContext(ctx, "SELECT role FROM users WHERE id = $1", id).Scan(&currentRole)
	if err != nil {
		return err
	}
	if currentRole == string(domain.RoleAdmin) {
		return errors.New("至少要保留一个管理员")
	}
	return nil
}

// ─── Settings ──────────────────────────────────────────────────────────

type pgSettingsStore struct{ r *Repo }

func (s pgSettingsStore) Get(ctx context.Context, key string) (string, error) {
	var val string
	err := s.r.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = $1", key).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return val, err
}

func (s pgSettingsStore) Set(ctx context.Context, key, value string) error {
	_, err := s.r.db.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		key, value,
	)
	return err
}

// SanitizeDSN masks password from database connection string for logs.
func SanitizeDSN(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return u.Redacted()
}
