package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"mygo-app/internal/data/crypto"
	"mygo-app/internal/domain"

	_ "github.com/go-sql-driver/mysql"
)

// Repo is the MySQL implementation of domain.Repo.
type Repo struct {
	db  *sql.DB
	dsn string
}

// NormalizeDSN ensures MySQL DSN contains required connection options like parseTime=true.
func NormalizeDSN(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if !strings.Contains(dsn, "parseTime=") {
		if strings.Contains(dsn, "?") {
			dsn += "&parseTime=true"
		} else {
			dsn += "?parseTime=true"
		}
	}
	if !strings.Contains(dsn, "charset=") {
		dsn += "&charset=utf8mb4"
	}
	return dsn
}

// SanitizeDSN masks sensitive credentials for logging.
func SanitizeDSN(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Scheme != "" && u.User != nil {
		return u.Redacted()
	}
	atIdx := strings.LastIndex(raw, "@")
	if atIdx != -1 {
		userInfo := raw[:atIdx]
		colonIdx := strings.Index(userInfo, ":")
		if colonIdx != -1 {
			return userInfo[:colonIdx] + ":xxxxx@" + raw[atIdx+1:]
		}
	}
	return raw
}

// Open connects to MySQL, runs migrations, ensures admin, seeds initial data, and returns domain.Repo.
func Open(ctx context.Context, dsn string) (domain.Repo, error) {
	normDSN := NormalizeDSN(dsn)
	db, err := sql.Open("mysql", normDSN)
	if err != nil {
		return nil, fmt.Errorf("open mysql %s: %w", SanitizeDSN(dsn), err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping mysql %s: %w", SanitizeDSN(dsn), err)
	}

	if err := Migrate(ctx, db, "up"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate mysql: %w", err)
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
	return "MySQL · " + SanitizeDSN(r.dsn)
}

func (r *Repo) Auth() domain.Auth                         { return mysqlAuthStore{r} }
func (r *Repo) Settings() domain.Settings                 { return mysqlSettingsStore{r} }
func (r *Repo) Users() domain.UserStore                   { return mysqlUserStore{r} }
func (r *Repo) Students() domain.Store[domain.Student]     { return mysqlStudentStore{r} }
func (r *Repo) Teachers() domain.Store[domain.Teacher]     { return mysqlTeacherStore{r} }
func (r *Repo) Classrooms() domain.Store[domain.Classroom] { return mysqlClassroomStore{r} }
func (r *Repo) Courses() domain.Store[domain.Course]       { return mysqlCourseStore{r} }
func (r *Repo) Lessons() domain.Store[domain.Lesson]       { return mysqlLessonStore{r} }
func (r *Repo) Ledger() domain.Ledger                      { return mysqlLedgerStore{r} }

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
	if n, err = count("SELECT COUNT(*) FROM students WHERE active = 1"); err != nil {
		return st, err
	}
	st.Students = int(n)

	if n, err = count("SELECT COUNT(*) FROM teachers WHERE active = 1"); err != nil {
		return st, err
	}
	st.Teachers = int(n)

	if n, err = count("SELECT COUNT(*) FROM courses WHERE active = 1"); err != nil {
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
	if st.Outstanding, err = count("SELECT COALESCE(SUM(balance_cents), 0) FROM students WHERE active = 1"); err != nil {
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
		Code:        "usr_admin",
		Username:    "admin",
		DisplayName: "系统管理员",
		Role:        string(domain.RoleAdmin),
		CreatedAt:   time.Now(),
	}

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO users (code, username, display_name, role, salt, hash, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE id=id`,
		admin.Code, admin.Username, admin.DisplayName, admin.Role, salt, hash, admin.CreatedAt,
	)
	return err
}

func (r *Repo) seed(ctx context.Context) error {
	var count int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM teachers").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	now := time.Now()
	tch1 := domain.Teacher{Code: "tch_seed1", Name: "陈静", Phone: "13800000001", Subject: "高中数学", RateCents: 20000, Active: true, CreatedAt: now.AddDate(0, -6, 0)}
	tch2 := domain.Teacher{Code: "tch_seed2", Name: "李磊", Phone: "13800000002", Subject: "初中物理", RateCents: 18000, Active: true, CreatedAt: now.AddDate(0, -5, 0)}
	tch3 := domain.Teacher{Code: "tch_seed3", Name: "王芳", Phone: "13800000003", Subject: "少儿英语", RateCents: 15000, Active: true, CreatedAt: now.AddDate(0, -4, 0)}
	for _, t := range []*domain.Teacher{&tch1, &tch2, &tch3} {
		if err := r.Teachers().Save(ctx, t); err != nil {
			return err
		}
	}

	rom1 := domain.Classroom{Code: "rom_seed1", Name: "文远楼 201", Capacity: 30, Location: "东校区二楼", Note: "配备多媒体投影"}
	rom2 := domain.Classroom{Code: "rom_seed2", Name: "博雅楼 102", Capacity: 15, Location: "西校区一楼", Note: "小班 VIP 教室"}
	for _, cr := range []*domain.Classroom{&rom1, &rom2} {
		if err := r.Classrooms().Save(ctx, cr); err != nil {
			return err
		}
	}

	crs1 := domain.Course{Code: "crs_seed1", Name: "初二数学同步培优", Subject: "初中数学", TeacherID: tch1.ID, TotalLessons: 24, PriceCents: 480000, Minutes: 90, Active: true}
	crs2 := domain.Course{Code: "crs_seed2", Name: "少儿英语自然拼读", Subject: "少儿英语", TeacherID: tch3.ID, TotalLessons: 18, PriceCents: 360000, Minutes: 60, Active: true}
	for _, c := range []*domain.Course{&crs1, &crs2} {
		if err := r.Courses().Save(ctx, c); err != nil {
			return err
		}
	}

	stu1 := domain.Student{Code: "stu_seed1", Name: "王小明", Guardian: "王女士", Phone: "13900000001", Note: "周三、周五上课", Active: true, CreatedAt: now.AddDate(0, -3, 0)}
	stu2 := domain.Student{Code: "stu_seed2", Name: "李思雨", Guardian: "李先生", Phone: "13900000002", Note: "需接送", Active: true, CreatedAt: now.AddDate(0, -2, 0)}
	stu3 := domain.Student{Code: "stu_seed3", Name: "张浩然", Guardian: "张先生", Phone: "13900000003", Note: "周末走读", Active: true, CreatedAt: now.AddDate(0, -1, 0)}
	for _, s := range []*domain.Student{&stu1, &stu2, &stu3} {
		if err := r.Students().Save(ctx, s); err != nil {
			return err
		}
	}

	if _, err := r.Ledger().TopUp(ctx, stu1.ID, 480000, "报名初二数学同步"); err != nil {
		return err
	}
	if _, err := r.Ledger().TopUp(ctx, stu2.ID, 360000, "报名少儿英语启蒙"); err != nil {
		return err
	}
	if _, err := r.Ledger().TopUp(ctx, stu3.ID, 200000, "预存充值"); err != nil {
		return err
	}

	les1 := domain.Lesson{Code: "les_seed1", CourseID: crs1.ID, TeacherID: tch1.ID, ClassroomID: rom1.ID, StartAt: now.Add(24 * time.Hour), Minutes: 90, Status: "planned"}
	les2 := domain.Lesson{Code: "les_seed2", CourseID: crs2.ID, TeacherID: tch3.ID, ClassroomID: rom2.ID, StartAt: now.Add(48 * time.Hour), Minutes: 60, Status: "planned"}
	les3 := domain.Lesson{Code: "les_seed3", CourseID: crs1.ID, TeacherID: tch1.ID, ClassroomID: rom1.ID, StartAt: now.Add(-72 * time.Hour), Minutes: 90, Status: "done"}
	for _, l := range []*domain.Lesson{&les1, &les2, &les3} {
		if err := r.Lessons().Save(ctx, l); err != nil {
			return err
		}
	}

	return nil
}

// ─── Students ──────────────────────────────────────────────────────────

type mysqlStudentStore struct{ r *Repo }

func (s mysqlStudentStore) List(ctx context.Context, q domain.Query) ([]domain.Student, error) {
	var (
		query strings.Builder
		args  []any
	)
	query.WriteString("SELECT id, code, name, guardian, phone, balance_cents, note, active, created_at FROM students")
	if q.Search != "" {
		like := "%" + q.Search + "%"
		args = append(args, like, like, like, like)
		query.WriteString(" WHERE (name LIKE ? OR phone LIKE ? OR guardian LIKE ? OR code LIKE ?)")
	}
	query.WriteString(" ORDER BY id DESC")
	if q.Limit > 0 {
		args = append(args, q.Limit)
		query.WriteString(" LIMIT ?")
	}

	rows, err := s.r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Student
	for rows.Next() {
		var item domain.Student
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &item.Guardian, &item.Phone, &item.Balance, &item.Note, &item.Active, &item.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func (s mysqlStudentStore) Get(ctx context.Context, idOrCode string) (domain.Student, error) {
	var (
		item  domain.Student
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = "SELECT id, code, name, guardian, phone, balance_cents, note, active, created_at FROM students WHERE id = ? OR code = ?"
		args = []any{numID, idOrCode}
	} else {
		query = "SELECT id, code, name, guardian, phone, balance_cents, note, active, created_at FROM students WHERE code = ?"
		args = []any{idOrCode}
	}

	err := s.r.db.QueryRowContext(ctx, query, args...).Scan(
		&item.ID, &item.Code, &item.Name, &item.Guardian, &item.Phone, &item.Balance, &item.Note, &item.Active, &item.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Student{}, domain.ErrNotFound
	}
	return item, err
}

func (s mysqlStudentStore) Save(ctx context.Context, v *domain.Student) error {
	if strings.TrimSpace(v.Name) == "" {
		return errors.New("姓名不能为空")
	}
	if v.Code == "" {
		v.Code = domain.NewID("stu")
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}

	if v.ID == 0 {
		res, err := s.r.db.ExecContext(ctx,
			`INSERT INTO students (code, name, guardian, phone, balance_cents, note, active, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			v.Code, v.Name, v.Guardian, v.Phone, v.Balance, v.Note, v.Active, v.CreatedAt,
		)
		if err != nil {
			return err
		}
		newID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		v.ID = newID
		return nil
	}

	res, err := s.r.db.ExecContext(ctx,
		`UPDATE students SET
			name = ?, guardian = ?, phone = ?, balance_cents = ?, note = ?, active = ?
		 WHERE id = ?`,
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

func (s mysqlStudentStore) Delete(ctx context.Context, idOrCode string) error {
	var (
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = "DELETE FROM students WHERE id = ? OR code = ?"
		args = []any{numID, idOrCode}
	} else {
		query = "DELETE FROM students WHERE code = ?"
		args = []any{idOrCode}
	}

	res, err := s.r.db.ExecContext(ctx, query, args...)
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

type mysqlTeacherStore struct{ r *Repo }

func (t mysqlTeacherStore) List(ctx context.Context, q domain.Query) ([]domain.Teacher, error) {
	var (
		query strings.Builder
		args  []any
	)
	query.WriteString("SELECT id, code, name, phone, subject, rate_cents, active, created_at FROM teachers")
	if q.Search != "" {
		like := "%" + q.Search + "%"
		args = append(args, like, like, like, like)
		query.WriteString(" WHERE (name LIKE ? OR phone LIKE ? OR subject LIKE ? OR code LIKE ?)")
	}
	query.WriteString(" ORDER BY id DESC")
	if q.Limit > 0 {
		args = append(args, q.Limit)
		query.WriteString(" LIMIT ?")
	}

	rows, err := t.r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Teacher
	for rows.Next() {
		var item domain.Teacher
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &item.Phone, &item.Subject, &item.RateCents, &item.Active, &item.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func (t mysqlTeacherStore) Get(ctx context.Context, idOrCode string) (domain.Teacher, error) {
	var (
		item  domain.Teacher
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = "SELECT id, code, name, phone, subject, rate_cents, active, created_at FROM teachers WHERE id = ? OR code = ?"
		args = []any{numID, idOrCode}
	} else {
		query = "SELECT id, code, name, phone, subject, rate_cents, active, created_at FROM teachers WHERE code = ?"
		args = []any{idOrCode}
	}

	err := t.r.db.QueryRowContext(ctx, query, args...).Scan(
		&item.ID, &item.Code, &item.Name, &item.Phone, &item.Subject, &item.RateCents, &item.Active, &item.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Teacher{}, domain.ErrNotFound
	}
	return item, err
}

func (t mysqlTeacherStore) Save(ctx context.Context, v *domain.Teacher) error {
	if strings.TrimSpace(v.Name) == "" {
		return errors.New("姓名不能为空")
	}
	if v.Code == "" {
		v.Code = domain.NewID("tch")
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}

	if v.ID == 0 {
		res, err := t.r.db.ExecContext(ctx,
			`INSERT INTO teachers (code, name, phone, subject, rate_cents, active, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			v.Code, v.Name, v.Phone, v.Subject, v.RateCents, v.Active, v.CreatedAt,
		)
		if err != nil {
			return err
		}
		newID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		v.ID = newID
		return nil
	}

	res, err := t.r.db.ExecContext(ctx,
		`UPDATE teachers SET
			name = ?, phone = ?, subject = ?, rate_cents = ?, active = ?
		 WHERE id = ?`,
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

func (t mysqlTeacherStore) Delete(ctx context.Context, idOrCode string) error {
	var (
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = "DELETE FROM teachers WHERE id = ? OR code = ?"
		args = []any{numID, idOrCode}
	} else {
		query = "DELETE FROM teachers WHERE code = ?"
		args = []any{idOrCode}
	}

	res, err := t.r.db.ExecContext(ctx, query, args...)
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

type mysqlClassroomStore struct{ r *Repo }

func (c mysqlClassroomStore) List(ctx context.Context, q domain.Query) ([]domain.Classroom, error) {
	var (
		query strings.Builder
		args  []any
	)
	query.WriteString("SELECT id, code, name, capacity, location, note FROM classrooms")
	if q.Search != "" {
		like := "%" + q.Search + "%"
		args = append(args, like, like, like)
		query.WriteString(" WHERE (name LIKE ? OR location LIKE ? OR code LIKE ?)")
	}
	query.WriteString(" ORDER BY id ASC")
	if q.Limit > 0 {
		args = append(args, q.Limit)
		query.WriteString(" LIMIT ?")
	}

	rows, err := c.r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Classroom
	for rows.Next() {
		var item domain.Classroom
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &item.Capacity, &item.Location, &item.Note); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func (c mysqlClassroomStore) Get(ctx context.Context, idOrCode string) (domain.Classroom, error) {
	var (
		item  domain.Classroom
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = "SELECT id, code, name, capacity, location, note FROM classrooms WHERE id = ? OR code = ?"
		args = []any{numID, idOrCode}
	} else {
		query = "SELECT id, code, name, capacity, location, note FROM classrooms WHERE code = ?"
		args = []any{idOrCode}
	}

	err := c.r.db.QueryRowContext(ctx, query, args...).Scan(
		&item.ID, &item.Code, &item.Name, &item.Capacity, &item.Location, &item.Note,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Classroom{}, domain.ErrNotFound
	}
	return item, err
}

func (c mysqlClassroomStore) Save(ctx context.Context, v *domain.Classroom) error {
	if strings.TrimSpace(v.Name) == "" {
		return errors.New("教室名称不能为空")
	}
	if v.Code == "" {
		v.Code = domain.NewID("rom")
	}

	if v.ID == 0 {
		res, err := c.r.db.ExecContext(ctx,
			`INSERT INTO classrooms (code, name, capacity, location, note)
			 VALUES (?, ?, ?, ?, ?)`,
			v.Code, v.Name, v.Capacity, v.Location, v.Note,
		)
		if err != nil {
			return err
		}
		newID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		v.ID = newID
		return nil
	}

	res, err := c.r.db.ExecContext(ctx,
		`UPDATE classrooms SET
			name = ?, capacity = ?, location = ?, note = ?
		 WHERE id = ?`,
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

func (c mysqlClassroomStore) Delete(ctx context.Context, idOrCode string) error {
	var (
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = "DELETE FROM classrooms WHERE id = ? OR code = ?"
		args = []any{numID, idOrCode}
	} else {
		query = "DELETE FROM classrooms WHERE code = ?"
		args = []any{idOrCode}
	}

	res, err := c.r.db.ExecContext(ctx, query, args...)
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

type mysqlCourseStore struct{ r *Repo }

func (s mysqlCourseStore) List(ctx context.Context, q domain.Query) ([]domain.Course, error) {
	var (
		query strings.Builder
		args  []any
	)
	query.WriteString("SELECT id, code, name, subject, teacher_id, total_lessons, price_cents, minutes, active FROM courses")
	if q.Search != "" {
		like := "%" + q.Search + "%"
		args = append(args, like, like, like)
		query.WriteString(" WHERE (name LIKE ? OR subject LIKE ? OR code LIKE ?)")
	}
	query.WriteString(" ORDER BY id DESC")
	if q.Limit > 0 {
		args = append(args, q.Limit)
		query.WriteString(" LIMIT ?")
	}

	rows, err := s.r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Course
	for rows.Next() {
		var item domain.Course
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &item.Subject, &item.TeacherID, &item.TotalLessons, &item.PriceCents, &item.Minutes, &item.Active); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func (s mysqlCourseStore) Get(ctx context.Context, idOrCode string) (domain.Course, error) {
	var (
		item  domain.Course
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = "SELECT id, code, name, subject, teacher_id, total_lessons, price_cents, minutes, active FROM courses WHERE id = ? OR code = ?"
		args = []any{numID, idOrCode}
	} else {
		query = "SELECT id, code, name, subject, teacher_id, total_lessons, price_cents, minutes, active FROM courses WHERE code = ?"
		args = []any{idOrCode}
	}

	err := s.r.db.QueryRowContext(ctx, query, args...).Scan(
		&item.ID, &item.Code, &item.Name, &item.Subject, &item.TeacherID, &item.TotalLessons, &item.PriceCents, &item.Minutes, &item.Active,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Course{}, domain.ErrNotFound
	}
	return item, err
}

func (s mysqlCourseStore) Save(ctx context.Context, v *domain.Course) error {
	if strings.TrimSpace(v.Name) == "" {
		return errors.New("课程名称不能为空")
	}
	if v.Code == "" {
		v.Code = domain.NewID("crs")
	}

	if v.ID == 0 {
		res, err := s.r.db.ExecContext(ctx,
			`INSERT INTO courses (code, name, subject, teacher_id, total_lessons, price_cents, minutes, active)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			v.Code, v.Name, v.Subject, v.TeacherID, v.TotalLessons, v.PriceCents, v.Minutes, v.Active,
		)
		if err != nil {
			return err
		}
		newID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		v.ID = newID
		return nil
	}

	res, err := s.r.db.ExecContext(ctx,
		`UPDATE courses SET
			name = ?, subject = ?, teacher_id = ?, total_lessons = ?, price_cents = ?, minutes = ?, active = ?
		 WHERE id = ?`,
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

func (s mysqlCourseStore) Delete(ctx context.Context, idOrCode string) error {
	var (
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = "DELETE FROM courses WHERE id = ? OR code = ?"
		args = []any{numID, idOrCode}
	} else {
		query = "DELETE FROM courses WHERE code = ?"
		args = []any{idOrCode}
	}

	res, err := s.r.db.ExecContext(ctx, query, args...)
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

type mysqlLessonStore struct{ r *Repo }

func (s mysqlLessonStore) List(ctx context.Context, q domain.Query) ([]domain.Lesson, error) {
	var (
		query strings.Builder
		args  []any
	)
	query.WriteString("SELECT id, code, course_id, teacher_id, classroom_id, start_at, minutes, status FROM lessons")
	if q.Search != "" {
		like := "%" + q.Search + "%"
		args = append(args, like, like)
		query.WriteString(" WHERE (status LIKE ? OR code LIKE ?)")
	}
	query.WriteString(" ORDER BY start_at DESC")
	if q.Limit > 0 {
		args = append(args, q.Limit)
		query.WriteString(" LIMIT ?")
	}

	rows, err := s.r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Lesson
	for rows.Next() {
		var item domain.Lesson
		if err := rows.Scan(&item.ID, &item.Code, &item.CourseID, &item.TeacherID, &item.ClassroomID, &item.StartAt, &item.Minutes, &item.Status); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func (s mysqlLessonStore) Get(ctx context.Context, idOrCode string) (domain.Lesson, error) {
	var (
		item  domain.Lesson
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = "SELECT id, code, course_id, teacher_id, classroom_id, start_at, minutes, status FROM lessons WHERE id = ? OR code = ?"
		args = []any{numID, idOrCode}
	} else {
		query = "SELECT id, code, course_id, teacher_id, classroom_id, start_at, minutes, status FROM lessons WHERE code = ?"
		args = []any{idOrCode}
	}

	err := s.r.db.QueryRowContext(ctx, query, args...).Scan(
		&item.ID, &item.Code, &item.CourseID, &item.TeacherID, &item.ClassroomID, &item.StartAt, &item.Minutes, &item.Status,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Lesson{}, domain.ErrNotFound
	}
	return item, err
}

func (s mysqlLessonStore) Save(ctx context.Context, v *domain.Lesson) error {
	if v.Code == "" {
		v.Code = domain.NewID("les")
	}
	if v.StartAt.IsZero() {
		v.StartAt = time.Now()
	}
	if v.Status == "" {
		v.Status = "planned"
	}

	if v.ID == 0 {
		res, err := s.r.db.ExecContext(ctx,
			`INSERT INTO lessons (code, course_id, teacher_id, classroom_id, start_at, minutes, status)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			v.Code, v.CourseID, v.TeacherID, v.ClassroomID, v.StartAt, v.Minutes, v.Status,
		)
		if err != nil {
			return err
		}
		newID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		v.ID = newID
		return nil
	}

	res, err := s.r.db.ExecContext(ctx,
		`UPDATE lessons SET
			course_id = ?, teacher_id = ?, classroom_id = ?, start_at = ?, minutes = ?, status = ?
		 WHERE id = ?`,
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

func (s mysqlLessonStore) Delete(ctx context.Context, idOrCode string) error {
	var (
		query string
		args  []any
	)
	if numID, err := strconv.ParseInt(idOrCode, 10, 64); err == nil && numID > 0 {
		query = "DELETE FROM lessons WHERE id = ? OR code = ?"
		args = []any{numID, idOrCode}
	} else {
		query = "DELETE FROM lessons WHERE code = ?"
		args = []any{idOrCode}
	}

	res, err := s.r.db.ExecContext(ctx, query, args...)
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

type mysqlLedgerStore struct{ r *Repo }

func (l mysqlLedgerStore) resolveStudentID(ctx context.Context, q interface{ QueryRowContext(context.Context, string, ...any) *sql.Row }, studentID any) (int64, error) {
	switch v := studentID.(type) {
	case int64:
		if v > 0 {
			return v, nil
		}
	case int:
		if v > 0 {
			return int64(v), nil
		}
	case string:
		if num, err := strconv.ParseInt(v, 10, 64); err == nil && num > 0 {
			return num, nil
		}
		var realID int64
		err := q.QueryRowContext(ctx, "SELECT id FROM students WHERE code = ?", v).Scan(&realID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return 0, domain.ErrNotFound
			}
			return 0, err
		}
		return realID, nil
	}
	return 0, domain.ErrNotFound
}

func (l mysqlLedgerStore) List(ctx context.Context, studentID any, limit int) ([]domain.LedgerEntry, error) {
	var (
		query strings.Builder
		args  []any
	)
	query.WriteString("SELECT id, code, student_id, kind, amount_cents, balance_cents, note, at FROM ledger")
	if studentID != nil && studentID != "" && studentID != int64(0) {
		stuID, err := l.resolveStudentID(ctx, l.r.db, studentID)
		if err == nil {
			args = append(args, stuID)
			query.WriteString(" WHERE student_id = ?")
		}
	}
	query.WriteString(" ORDER BY at DESC")
	if limit > 0 {
		args = append(args, limit)
		query.WriteString(" LIMIT ?")
	}

	rows, err := l.r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.LedgerEntry
	for rows.Next() {
		var item domain.LedgerEntry
		if err := rows.Scan(&item.ID, &item.Code, &item.StudentID, &item.Kind, &item.Amount, &item.Balance, &item.Note, &item.At); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func (l mysqlLedgerStore) TopUp(ctx context.Context, studentID any, amountCents int64, note string) (domain.LedgerEntry, error) {
	if amountCents <= 0 {
		return domain.LedgerEntry{}, errors.New("充值金额必须大于 0")
	}
	return l.apply(ctx, studentID, "topup", amountCents, note)
}

func (l mysqlLedgerStore) Consume(ctx context.Context, studentID any, amountCents int64, note string) (domain.LedgerEntry, error) {
	if amountCents <= 0 {
		return domain.LedgerEntry{}, errors.New("扣费金额必须大于 0")
	}
	return l.apply(ctx, studentID, "consume", -amountCents, note)
}

func (l mysqlLedgerStore) apply(ctx context.Context, studentID any, kind string, delta int64, note string) (domain.LedgerEntry, error) {
	tx, err := l.r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return domain.LedgerEntry{}, err
	}
	defer tx.Rollback()

	stuID, err := l.resolveStudentID(ctx, tx, studentID)
	if err != nil {
		return domain.LedgerEntry{}, fmt.Errorf("student %v: %w", studentID, domain.ErrNotFound)
	}

	var currentBalance int64
	err = tx.QueryRowContext(ctx, "SELECT balance_cents FROM students WHERE id = ? FOR UPDATE", stuID).Scan(&currentBalance)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.LedgerEntry{}, fmt.Errorf("student %v: %w", studentID, domain.ErrNotFound)
	}
	if err != nil {
		return domain.LedgerEntry{}, err
	}

	newBalance := currentBalance + delta
	if newBalance < 0 {
		return domain.LedgerEntry{}, errors.New("余额不足，无法消课")
	}

	_, err = tx.ExecContext(ctx, "UPDATE students SET balance_cents = ? WHERE id = ?", newBalance, stuID)
	if err != nil {
		return domain.LedgerEntry{}, err
	}

	entry := domain.LedgerEntry{
		Code:      domain.NewID("ldg"),
		StudentID: stuID,
		Kind:      kind,
		Amount:    delta,
		Balance:   newBalance,
		Note:      note,
		At:        time.Now(),
	}

	res, err := tx.ExecContext(ctx,
		`INSERT INTO ledger (code, student_id, kind, amount_cents, balance_cents, note, at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		entry.Code, entry.StudentID, entry.Kind, entry.Amount, entry.Balance, entry.Note, entry.At,
	)
	if err != nil {
		return domain.LedgerEntry{}, err
	}
	newID, err := res.LastInsertId()
	if err != nil {
		return domain.LedgerEntry{}, err
	}
	entry.ID = newID

	if err := tx.Commit(); err != nil {
		return domain.LedgerEntry{}, err
	}
	return entry, nil
}

// ─── Auth ──────────────────────────────────────────────────────────────

type mysqlAuthStore struct{ r *Repo }

func (a mysqlAuthStore) Login(ctx context.Context, username, password string) (domain.User, error) {
	username = strings.TrimSpace(username)
	var (
		u          domain.User
		salt, hash string
	)
	err := a.r.db.QueryRowContext(ctx,
		"SELECT id, code, username, display_name, role, salt, hash, created_at FROM users WHERE username = ?",
		username,
	).Scan(&u.ID, &u.Code, &u.Username, &u.DisplayName, &u.Role, &salt, &hash, &u.CreatedAt)
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

func (a mysqlAuthStore) ResetPassword(ctx context.Context, username, recoveryKey, newPassword string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("请输入用户名")
	}
	recoveryKey = strings.TrimSpace(recoveryKey)
	if recoveryKey == "" {
		return domain.ErrInvalidRecoveryKey
	}
	newPassword = strings.TrimSpace(newPassword)
	if len(newPassword) < 6 {
		return errors.New("新密码长度不能少于 6 位")
	}

	expectedKey := domain.DefaultRecoveryKey
	savedKey, err := a.r.Settings().Get(ctx, domain.SettingsKeyRecoveryKey)
	if err == nil && savedKey != "" {
		expectedKey = savedKey
	}
	if recoveryKey != expectedKey {
		return domain.ErrInvalidRecoveryKey
	}

	var userID int64
	err = a.r.db.QueryRowContext(ctx, "SELECT id FROM users WHERE username = ?", username).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ErrUserNotFound
	}
	if err != nil {
		return err
	}

	salt, hash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return err
	}

	_, err = a.r.db.ExecContext(ctx,
		"UPDATE users SET salt = ?, hash = ? WHERE id = ?",
		salt, hash, userID,
	)
	return err
}

func (a mysqlAuthStore) Register(ctx context.Context, username, password, displayName string) (domain.User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return domain.User{}, errors.New("用户名不能为空")
	}
	if len([]rune(password)) < 6 {
		return domain.User{}, errors.New("密码至少 6 位")
	}
	displayName = strings.TrimSpace(displayName)
	if displayName == "" {
		displayName = username
	}
	user := domain.User{
		Username:    username,
		DisplayName: displayName,
		Role:        string(domain.RoleStaff),
	}
	if err := a.r.Users().Create(ctx, &user, password); err != nil {
		return domain.User{}, err
	}
	return user, nil
}

// ─── Users ─────────────────────────────────────────────────────────────

type mysqlUserStore struct{ r *Repo }

func (u mysqlUserStore) List(ctx context.Context) ([]domain.User, error) {
	rows, err := u.r.db.QueryContext(ctx,
		"SELECT id, code, username, display_name, role, created_at FROM users ORDER BY id ASC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.User
	for rows.Next() {
		var item domain.User
		if err := rows.Scan(&item.ID, &item.Code, &item.Username, &item.DisplayName, &item.Role, &item.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, item)
	}
	return list, rows.Err()
}

func (u mysqlUserStore) Create(ctx context.Context, v *domain.User, password string) error {
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
	if v.Code == "" {
		v.Code = domain.NewID("usr")
	}
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now()
	}

	res, err := u.r.db.ExecContext(ctx,
		`INSERT INTO users (code, username, display_name, role, salt, hash, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		v.Code, v.Username, v.DisplayName, v.Role, salt, hash, v.CreatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") || strings.Contains(err.Error(), "1062") {
			return errors.New("用户名已存在")
		}
		return err
	}
	newID, err := res.LastInsertId()
	if err != nil {
		return err
	}
	v.ID = newID
	return nil
}

func (u mysqlUserStore) Update(ctx context.Context, v domain.User) error {
	if v.Role != string(domain.RoleAdmin) {
		if err := u.keepOneAdmin(ctx, v.ID); err != nil {
			return err
		}
	}
	res, err := u.r.db.ExecContext(ctx,
		"UPDATE users SET display_name = ?, role = ? WHERE id = ?",
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

func (u mysqlUserStore) resolveUserID(ctx context.Context, id any) (int64, error) {
	switch v := id.(type) {
	case int64:
		if v > 0 {
			return v, nil
		}
	case int:
		if v > 0 {
			return int64(v), nil
		}
	case string:
		if num, err := strconv.ParseInt(v, 10, 64); err == nil && num > 0 {
			return num, nil
		}
		var realID int64
		err := u.r.db.QueryRowContext(ctx, "SELECT id FROM users WHERE code = ? OR username = ?", v, v).Scan(&realID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return 0, domain.ErrNotFound
			}
			return 0, err
		}
		return realID, nil
	}
	return 0, domain.ErrNotFound
}

func (u mysqlUserStore) SetPassword(ctx context.Context, id any, password string) error {
	if len([]rune(password)) < 6 {
		return errors.New("密码至少 6 位")
	}
	userID, err := u.resolveUserID(ctx, id)
	if err != nil {
		return err
	}

	salt, hash, err := crypto.HashPassword(password)
	if err != nil {
		return err
	}
	res, err := u.r.db.ExecContext(ctx, "UPDATE users SET salt = ?, hash = ? WHERE id = ?", salt, hash, userID)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (u mysqlUserStore) Delete(ctx context.Context, id any) error {
	userID, err := u.resolveUserID(ctx, id)
	if err != nil {
		return err
	}
	if err := u.keepOneAdmin(ctx, userID); err != nil {
		return err
	}
	res, err := u.r.db.ExecContext(ctx, "DELETE FROM users WHERE id = ?", userID)
	if err != nil {
		return err
	}
	aff, err := res.RowsAffected()
	if err == nil && aff == 0 {
		return domain.ErrNotFound
	}
	return err
}

func (u mysqlUserStore) keepOneAdmin(ctx context.Context, id int64) error {
	var remainingAdminCount int64
	err := u.r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM users WHERE role = 'admin' AND id <> ?",
		id,
	).Scan(&remainingAdminCount)
	if err != nil {
		return err
	}
	if remainingAdminCount > 0 {
		return nil
	}

	var currentRole string
	err = u.r.db.QueryRowContext(ctx, "SELECT role FROM users WHERE id = ?", id).Scan(&currentRole)
	if err != nil {
		return err
	}
	if currentRole == string(domain.RoleAdmin) {
		return errors.New("至少要保留一个管理员")
	}
	return nil
}

// ─── Settings ──────────────────────────────────────────────────────────

type mysqlSettingsStore struct{ r *Repo }

func (s mysqlSettingsStore) Get(ctx context.Context, key string) (string, error) {
	var val string
	err := s.r.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE `key` = ?", key).Scan(&val)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return val, err
}

func (s mysqlSettingsStore) Set(ctx context.Context, key, value string) error {
	_, err := s.r.db.ExecContext(ctx,
		`INSERT INTO settings (`+"`key`"+`, value) VALUES (?, ?)
		 ON DUPLICATE KEY UPDATE value = VALUES(value)`,
		key, value,
	)
	return err
}
