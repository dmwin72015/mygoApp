package rdb

import (
	"context"

	"mygo-app/internal/domain"

	"github.com/jmoiron/sqlx"
)

// Repo is a unified relational database repository implementation of domain.Repo.
type Repo struct {
	db         *sqlx.DB
	dialect    *Dialect
	source     string
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

// NewRepo creates and initializes a unified relational Repo without injecting mock data.
func NewRepo(ctx context.Context, db *sqlx.DB, driverName string, source string) (*Repo, error) {
	dialect := NewDialect(driverName)
	txm := NewTxManager(db)

	students := NewStudentStore(db, dialect)
	teachers := NewTeacherStore(db, dialect)
	classrooms := NewClassroomStore(db, dialect)
	courses := NewCourseStore(db, dialect)
	lessons := NewLessonStore(db, dialect)
	ledger := NewLedgerStore(db, dialect, txm, students)
	users := NewUserStore(db, dialect)
	settings := NewSettingsStore(db, dialect)
	auth := NewAuthStore(db, dialect, users, settings)

	r := &Repo{
		db:         db,
		dialect:    dialect,
		source:     source,
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

	return r, nil
}

func (r *Repo) Close() error {
	return r.db.Close()
}

func (r *Repo) Source() string {
	return r.source
}

func (r *Repo) DB() *sqlx.DB                              { return r.db }
func (r *Repo) Dialect() *Dialect                         { return r.dialect }
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
