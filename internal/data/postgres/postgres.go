package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"mygo-app/internal/data/crypto"
	"mygo-app/internal/domain"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

// Repo is the PostgreSQL implementation of domain.Repo.
// It acts as the composition root that coordinates connection lifecycle,
// database migrations, and assembles fully decoupled sub-stores.
type Repo struct {
	db         *sqlx.DB
	rawDB      *sql.DB
	dsn        string
	txm        TxManager
	students   *StudentStore
	teachers   *TeacherStore
	classrooms *ClassroomStore
	courses    *CourseStore
	lessons    *LessonStore
	ledger     *LedgerStore
	users      *UserStore
	auth       *AuthStore
	settings   *SettingsStore
}

// Open connects to PostgreSQL, runs schema migrations, and wires together decoupled stores.
func Open(ctx context.Context, dsn string) (domain.Repo, error) {
	rawDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres %s: %w", SanitizeDSN(dsn), err)
	}

	rawDB.SetMaxOpenConns(25)
	rawDB.SetMaxIdleConns(10)
	rawDB.SetConnMaxLifetime(5 * time.Minute)

	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	if err := rawDB.PingContext(pingCtx); err != nil {
		_ = rawDB.Close()
		return nil, fmt.Errorf("ping postgres %s: %w", SanitizeDSN(dsn), err)
	}

	if err := Migrate(ctx, rawDB, "up"); err != nil {
		_ = rawDB.Close()
		return nil, fmt.Errorf("migrate postgres: %w", err)
	}

	db := sqlx.NewDb(rawDB, "postgres")

	// Wire decoupled components
	txm := NewTxManager(db)
	students := NewStudentStore(db)
	teachers := NewTeacherStore(db)
	classrooms := NewClassroomStore(db)
	courses := NewCourseStore(db)
	lessons := NewLessonStore(db)
	ledger := NewLedgerStore(db, txm, students)
	users := NewUserStore(db)
	settings := NewSettingsStore(db)
	auth := NewAuthStore(db, users, settings)

	r := &Repo{
		db:         db,
		rawDB:      rawDB,
		dsn:        dsn,
		txm:        txm,
		students:   students,
		teachers:   teachers,
		classrooms: classrooms,
		courses:    courses,
		lessons:    lessons,
		ledger:     ledger,
		users:      users,
		auth:       auth,
		settings:   settings,
	}

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

func (r *Repo) TxManager() TxManager                      { return r.txm }
func (r *Repo) Auth() domain.Auth                         { return r.auth }
func (r *Repo) Settings() domain.Settings                 { return r.settings }
func (r *Repo) Users() domain.UserStore                   { return r.users }
func (r *Repo) Students() domain.Store[domain.Student]     { return r.students }
func (r *Repo) Teachers() domain.Store[domain.Teacher]     { return r.teachers }
func (r *Repo) Classrooms() domain.Store[domain.Classroom] { return r.classrooms }
func (r *Repo) Courses() domain.Store[domain.Course]       { return r.courses }
func (r *Repo) Lessons() domain.Store[domain.Lesson]       { return r.lessons }
func (r *Repo) Ledger() domain.Ledger                      { return r.ledger }

func (r *Repo) Stats(ctx context.Context) (domain.Stats, error) {
	var st domain.Stats
	count := func(query string) (int64, error) {
		var n int64
		err := r.db.GetContext(ctx, &n, query)
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
	err := r.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM users WHERE username = 'admin'")
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
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (username) DO NOTHING`,
		admin.Code, admin.Username, admin.DisplayName, admin.Role, salt, hash, admin.CreatedAt,
	)
	return err
}

func (r *Repo) seed(ctx context.Context) error {
	var count int64
	if err := r.db.GetContext(ctx, &count, "SELECT COUNT(*) FROM teachers"); err != nil {
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

// SanitizeDSN masks password from database connection string for logs.
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
