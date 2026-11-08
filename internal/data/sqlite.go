package data

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"mygo-app/internal/data/crypto"
	"mygo-app/internal/domain"

	"github.com/egoist/mygo/plugins/sqlite"
)

// schema is applied statement by statement at every open; ALTER-free
// migrations go here as the app grows, or a numbered migration table.
var schema = []string{
	`CREATE TABLE IF NOT EXISTS teachers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		phone TEXT NOT NULL DEFAULT '',
		subject TEXT NOT NULL DEFAULT '',
		rate_cents INTEGER NOT NULL DEFAULT 0,
		active INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS students (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		guardian TEXT NOT NULL DEFAULT '',
		phone TEXT NOT NULL DEFAULT '',
		balance_cents INTEGER NOT NULL DEFAULT 0,
		note TEXT NOT NULL DEFAULT '',
		active INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS classrooms (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		capacity INTEGER NOT NULL DEFAULT 0,
		location TEXT NOT NULL DEFAULT '',
		note TEXT NOT NULL DEFAULT '')`,
	`CREATE TABLE IF NOT EXISTS courses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		subject TEXT NOT NULL DEFAULT '',
		teacher_id INTEGER NOT NULL DEFAULT 0,
		total_lessons INTEGER NOT NULL DEFAULT 0,
		price_cents INTEGER NOT NULL DEFAULT 0,
		minutes INTEGER NOT NULL DEFAULT 0,
		active INTEGER NOT NULL DEFAULT 1)`,
	`CREATE TABLE IF NOT EXISTS lessons (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code TEXT NOT NULL UNIQUE,
		course_id INTEGER NOT NULL DEFAULT 0,
		teacher_id INTEGER NOT NULL DEFAULT 0,
		classroom_id INTEGER NOT NULL DEFAULT 0,
		start_at TEXT NOT NULL,
		minutes INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'planned')`,
	`CREATE TABLE IF NOT EXISTS ledger (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code TEXT NOT NULL UNIQUE,
		student_id INTEGER NOT NULL REFERENCES students(id) ON DELETE CASCADE,
		kind TEXT NOT NULL,
		amount_cents INTEGER NOT NULL,
		balance_cents INTEGER NOT NULL,
		note TEXT NOT NULL DEFAULT '',
		at TEXT NOT NULL)`,
	`CREATE INDEX IF NOT EXISTS ledger_student ON ledger (student_id, at)`,
	`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code TEXT NOT NULL UNIQUE,
		username TEXT NOT NULL UNIQUE,
		display_name TEXT NOT NULL DEFAULT '',
		role TEXT NOT NULL DEFAULT '',
		salt TEXT NOT NULL,
		hash TEXT NOT NULL,
		created_at TEXT NOT NULL)`,
	`CREATE TABLE IF NOT EXISTS settings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key TEXT NOT NULL UNIQUE,
		value TEXT NOT NULL)`,
}

type sqliteRepo struct {
	db   *sqlite.DB
	path string
	mu   sync.Mutex
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
	rev, err := count("SELECT COALESCE(SUM(amount_cents), 0) FROM ledger WHERE kind = 'topup'")
	if err != nil {
		return st, err
	}
	st.Revenue = rev

	out, err := count("SELECT COALESCE(SUM(balance_cents), 0) FROM students WHERE active = 1")
	if err != nil {
		return st, err
	}
	st.Outstanding = out
	return st, nil
}

func (r *sqliteRepo) seed(ctx context.Context) error {
	rows, err := r.db.Query(ctx, "SELECT COUNT(*) FROM teachers")
	if err != nil {
		return err
	}
	if len(rows.Values) > 0 && colInt(rows.Values[0][0]) > 0 {
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

// ─── students ───────────────────────────────────────────────────────────

type studentStore struct{ r *sqliteRepo }

const studentCols = `id, code, name, guardian, phone, balance_cents, note, active, created_at`

func (s studentStore) List(ctx context.Context, q domain.Query) ([]domain.Student, error) {
	sql := `SELECT ` + studentCols + ` FROM students`
	var args []any
	if q.Search != "" {
		sql += ` WHERE name LIKE ?1 OR phone LIKE ?1 OR guardian LIKE ?1 OR code LIKE ?1`
		args = append(args, "%"+q.Search+"%")
	}
	sql += ` ORDER BY id DESC`
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

func (s studentStore) Get(ctx context.Context, idOrCode string) (domain.Student, error) {
	var (
		rows sqlite.Rows
		err  error
	)
	if num, e := strconv.ParseInt(idOrCode, 10, 64); e == nil && num > 0 {
		rows, err = s.r.db.Query(ctx, `SELECT `+studentCols+` FROM students WHERE id = ?1 OR code = ?2`, num, idOrCode)
	} else {
		rows, err = s.r.db.Query(ctx, `SELECT `+studentCols+` FROM students WHERE code = ?1`, idOrCode)
	}
	if err != nil {
		return domain.Student{}, err
	}
	if len(rows.Values) == 0 {
		return domain.Student{}, domain.ErrNotFound
	}
	return scanStudent(rows.Values[0]), nil
}

func (s studentStore) Save(ctx context.Context, v *domain.Student) error {
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
		res, err := s.r.db.Execute(ctx, `
			INSERT INTO students (code, name, guardian, phone, balance_cents, note, active, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			v.Code, v.Name, v.Guardian, v.Phone, v.Balance, v.Note, b2i(v.Active), v.CreatedAt.Format(time.RFC3339))
		if err != nil {
			return err
		}
		v.ID = res.LastInsertID
		return nil
	}

	_, err := s.r.db.Execute(ctx, `
		UPDATE students SET
			name = ?, guardian = ?, phone = ?, note = ?, active = ?
		WHERE id = ?`,
		v.Name, v.Guardian, v.Phone, v.Note, b2i(v.Active), v.ID)
	return err
}

func (s studentStore) Delete(ctx context.Context, idOrCode string) error {
	if num, e := strconv.ParseInt(idOrCode, 10, 64); e == nil && num > 0 {
		_, err := s.r.db.Execute(ctx, `DELETE FROM students WHERE id = ?1 OR code = ?2`, num, idOrCode)
		return err
	}
	_, err := s.r.db.Execute(ctx, `DELETE FROM students WHERE code = ?1`, idOrCode)
	return err
}

func scanStudent(v []any) domain.Student {
	return domain.Student{
		ID:        colInt(at(v, 0)),
		Code:      colString(at(v, 1)),
		Name:      colString(at(v, 2)),
		Guardian:  colString(at(v, 3)),
		Phone:     colString(at(v, 4)),
		Balance:   colInt(at(v, 5)),
		Note:      colString(at(v, 6)),
		Active:    colBool(at(v, 7)),
		CreatedAt: colTime(at(v, 8)),
	}
}

// ─── teachers ───────────────────────────────────────────────────────────

type teacherStore struct{ r *sqliteRepo }

const teacherCols = `id, code, name, phone, subject, rate_cents, active, created_at`

func (s teacherStore) List(ctx context.Context, q domain.Query) ([]domain.Teacher, error) {
	sql := `SELECT ` + teacherCols + ` FROM teachers`
	var args []any
	if q.Search != "" {
		sql += ` WHERE name LIKE ?1 OR phone LIKE ?1 OR subject LIKE ?1 OR code LIKE ?1`
		args = append(args, "%"+q.Search+"%")
	}
	sql += ` ORDER BY id DESC`
	if q.Limit > 0 {
		sql += ` LIMIT ` + strconv.Itoa(q.Limit)
	}
	rows, err := s.r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Teacher, 0, len(rows.Values))
	for _, v := range rows.Values {
		out = append(out, scanTeacher(v))
	}
	return out, nil
}

func (s teacherStore) Get(ctx context.Context, idOrCode string) (domain.Teacher, error) {
	var (
		rows sqlite.Rows
		err  error
	)
	if num, e := strconv.ParseInt(idOrCode, 10, 64); e == nil && num > 0 {
		rows, err = s.r.db.Query(ctx, `SELECT `+teacherCols+` FROM teachers WHERE id = ?1 OR code = ?2`, num, idOrCode)
	} else {
		rows, err = s.r.db.Query(ctx, `SELECT `+teacherCols+` FROM teachers WHERE code = ?1`, idOrCode)
	}
	if err != nil {
		return domain.Teacher{}, err
	}
	if len(rows.Values) == 0 {
		return domain.Teacher{}, domain.ErrNotFound
	}
	return scanTeacher(rows.Values[0]), nil
}

func (s teacherStore) Save(ctx context.Context, v *domain.Teacher) error {
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
		res, err := s.r.db.Execute(ctx, `
			INSERT INTO teachers (code, name, phone, subject, rate_cents, active, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			v.Code, v.Name, v.Phone, v.Subject, v.RateCents, b2i(v.Active), v.CreatedAt.Format(time.RFC3339))
		if err != nil {
			return err
		}
		v.ID = res.LastInsertID
		return nil
	}

	_, err := s.r.db.Execute(ctx, `
		UPDATE teachers SET
			name = ?, phone = ?, subject = ?, rate_cents = ?, active = ?
		WHERE id = ?`,
		v.Name, v.Phone, v.Subject, v.RateCents, b2i(v.Active), v.ID)
	return err
}

func (s teacherStore) Delete(ctx context.Context, idOrCode string) error {
	if num, e := strconv.ParseInt(idOrCode, 10, 64); e == nil && num > 0 {
		_, err := s.r.db.Execute(ctx, `DELETE FROM teachers WHERE id = ?1 OR code = ?2`, num, idOrCode)
		return err
	}
	_, err := s.r.db.Execute(ctx, `DELETE FROM teachers WHERE code = ?1`, idOrCode)
	return err
}

func scanTeacher(v []any) domain.Teacher {
	return domain.Teacher{
		ID:        colInt(at(v, 0)),
		Code:      colString(at(v, 1)),
		Name:      colString(at(v, 2)),
		Phone:     colString(at(v, 3)),
		Subject:   colString(at(v, 4)),
		RateCents: colInt(at(v, 5)),
		Active:    colBool(at(v, 6)),
		CreatedAt: colTime(at(v, 7)),
	}
}

// ─── classrooms ─────────────────────────────────────────────────────────

type classroomStore struct{ r *sqliteRepo }

const classroomCols = `id, code, name, capacity, location, note`

func (s classroomStore) List(ctx context.Context, q domain.Query) ([]domain.Classroom, error) {
	sql := `SELECT ` + classroomCols + ` FROM classrooms`
	var args []any
	if q.Search != "" {
		sql += ` WHERE name LIKE ?1 OR location LIKE ?1 OR code LIKE ?1`
		args = append(args, "%"+q.Search+"%")
	}
	sql += ` ORDER BY id ASC`
	if q.Limit > 0 {
		sql += ` LIMIT ` + strconv.Itoa(q.Limit)
	}
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

func (s classroomStore) Get(ctx context.Context, idOrCode string) (domain.Classroom, error) {
	var (
		rows sqlite.Rows
		err  error
	)
	if num, e := strconv.ParseInt(idOrCode, 10, 64); e == nil && num > 0 {
		rows, err = s.r.db.Query(ctx, `SELECT `+classroomCols+` FROM classrooms WHERE id = ?1 OR code = ?2`, num, idOrCode)
	} else {
		rows, err = s.r.db.Query(ctx, `SELECT `+classroomCols+` FROM classrooms WHERE code = ?1`, idOrCode)
	}
	if err != nil {
		return domain.Classroom{}, err
	}
	if len(rows.Values) == 0 {
		return domain.Classroom{}, domain.ErrNotFound
	}
	return scanClassroom(rows.Values[0]), nil
}

func (s classroomStore) Save(ctx context.Context, v *domain.Classroom) error {
	if strings.TrimSpace(v.Name) == "" {
		return errors.New("教室名称不能为空")
	}
	if v.Code == "" {
		v.Code = domain.NewID("rom")
	}

	if v.ID == 0 {
		res, err := s.r.db.Execute(ctx, `
			INSERT INTO classrooms (code, name, capacity, location, note)
			VALUES (?, ?, ?, ?, ?)`,
			v.Code, v.Name, v.Capacity, v.Location, v.Note)
		if err != nil {
			return err
		}
		v.ID = res.LastInsertID
		return nil
	}

	_, err := s.r.db.Execute(ctx, `
		UPDATE classrooms SET
			name = ?, capacity = ?, location = ?, note = ?
		WHERE id = ?`,
		v.Name, v.Capacity, v.Location, v.Note, v.ID)
	return err
}

func (s classroomStore) Delete(ctx context.Context, idOrCode string) error {
	if num, e := strconv.ParseInt(idOrCode, 10, 64); e == nil && num > 0 {
		_, err := s.r.db.Execute(ctx, `DELETE FROM classrooms WHERE id = ?1 OR code = ?2`, num, idOrCode)
		return err
	}
	_, err := s.r.db.Execute(ctx, `DELETE FROM classrooms WHERE code = ?1`, idOrCode)
	return err
}

func scanClassroom(v []any) domain.Classroom {
	return domain.Classroom{
		ID:       colInt(at(v, 0)),
		Code:     colString(at(v, 1)),
		Name:     colString(at(v, 2)),
		Capacity: int(colInt(at(v, 3))),
		Location: colString(at(v, 4)),
		Note:     colString(at(v, 5)),
	}
}

// ─── courses ────────────────────────────────────────────────────────────

type courseStore struct{ r *sqliteRepo }

const courseCols = `id, code, name, subject, teacher_id, total_lessons, price_cents, minutes, active`

func (s courseStore) List(ctx context.Context, q domain.Query) ([]domain.Course, error) {
	sql := `SELECT ` + courseCols + ` FROM courses`
	var args []any
	if q.Search != "" {
		sql += ` WHERE name LIKE ?1 OR subject LIKE ?1 OR code LIKE ?1`
		args = append(args, "%"+q.Search+"%")
	}
	sql += ` ORDER BY id DESC`
	if q.Limit > 0 {
		sql += ` LIMIT ` + strconv.Itoa(q.Limit)
	}
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

func (s courseStore) Get(ctx context.Context, idOrCode string) (domain.Course, error) {
	var (
		rows sqlite.Rows
		err  error
	)
	if num, e := strconv.ParseInt(idOrCode, 10, 64); e == nil && num > 0 {
		rows, err = s.r.db.Query(ctx, `SELECT `+courseCols+` FROM courses WHERE id = ?1 OR code = ?2`, num, idOrCode)
	} else {
		rows, err = s.r.db.Query(ctx, `SELECT `+courseCols+` FROM courses WHERE code = ?1`, idOrCode)
	}
	if err != nil {
		return domain.Course{}, err
	}
	if len(rows.Values) == 0 {
		return domain.Course{}, domain.ErrNotFound
	}
	return scanCourse(rows.Values[0]), nil
}

func (s courseStore) Save(ctx context.Context, v *domain.Course) error {
	if strings.TrimSpace(v.Name) == "" {
		return errors.New("课程名称不能为空")
	}
	if v.Code == "" {
		v.Code = domain.NewID("crs")
	}

	if v.ID == 0 {
		res, err := s.r.db.Execute(ctx, `
			INSERT INTO courses (code, name, subject, teacher_id, total_lessons, price_cents, minutes, active)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			v.Code, v.Name, v.Subject, v.TeacherID, v.TotalLessons, v.PriceCents, v.Minutes, b2i(v.Active))
		if err != nil {
			return err
		}
		v.ID = res.LastInsertID
		return nil
	}

	_, err := s.r.db.Execute(ctx, `
		UPDATE courses SET
			name = ?, subject = ?, teacher_id = ?, total_lessons = ?, price_cents = ?, minutes = ?, active = ?
		WHERE id = ?`,
		v.Name, v.Subject, v.TeacherID, v.TotalLessons, v.PriceCents, v.Minutes, b2i(v.Active), v.ID)
	return err
}

func (s courseStore) Delete(ctx context.Context, idOrCode string) error {
	if num, e := strconv.ParseInt(idOrCode, 10, 64); e == nil && num > 0 {
		_, err := s.r.db.Execute(ctx, `DELETE FROM courses WHERE id = ?1 OR code = ?2`, num, idOrCode)
		return err
	}
	_, err := s.r.db.Execute(ctx, `DELETE FROM courses WHERE code = ?1`, idOrCode)
	return err
}

func scanCourse(v []any) domain.Course {
	return domain.Course{
		ID:           colInt(at(v, 0)),
		Code:         colString(at(v, 1)),
		Name:         colString(at(v, 2)),
		Subject:      colString(at(v, 3)),
		TeacherID:    colInt(at(v, 4)),
		TotalLessons: int(colInt(at(v, 5))),
		PriceCents:   colInt(at(v, 6)),
		Minutes:      int(colInt(at(v, 7))),
		Active:       colBool(at(v, 8)),
	}
}

// ─── lessons ────────────────────────────────────────────────────────────

type lessonStore struct{ r *sqliteRepo }

const lessonCols = `id, code, course_id, teacher_id, classroom_id, start_at, minutes, status`

func (s lessonStore) List(ctx context.Context, q domain.Query) ([]domain.Lesson, error) {
	sql := `SELECT ` + lessonCols + ` FROM lessons`
	var args []any
	if q.Search != "" {
		sql += ` WHERE status LIKE ?1 OR code LIKE ?1`
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

func (s lessonStore) Get(ctx context.Context, idOrCode string) (domain.Lesson, error) {
	var (
		rows sqlite.Rows
		err  error
	)
	if num, e := strconv.ParseInt(idOrCode, 10, 64); e == nil && num > 0 {
		rows, err = s.r.db.Query(ctx, `SELECT `+lessonCols+` FROM lessons WHERE id = ?1 OR code = ?2`, num, idOrCode)
	} else {
		rows, err = s.r.db.Query(ctx, `SELECT `+lessonCols+` FROM lessons WHERE code = ?1`, idOrCode)
	}
	if err != nil {
		return domain.Lesson{}, err
	}
	if len(rows.Values) == 0 {
		return domain.Lesson{}, domain.ErrNotFound
	}
	return scanLesson(rows.Values[0]), nil
}

func (s lessonStore) Save(ctx context.Context, v *domain.Lesson) error {
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
		res, err := s.r.db.Execute(ctx, `
			INSERT INTO lessons (code, course_id, teacher_id, classroom_id, start_at, minutes, status)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			v.Code, v.CourseID, v.TeacherID, v.ClassroomID, v.StartAt.Format(time.RFC3339), v.Minutes, v.Status)
		if err != nil {
			return err
		}
		v.ID = res.LastInsertID
		return nil
	}

	_, err := s.r.db.Execute(ctx, `
		UPDATE lessons SET
			course_id = ?, teacher_id = ?, classroom_id = ?, start_at = ?, minutes = ?, status = ?
		WHERE id = ?`,
		v.CourseID, v.TeacherID, v.ClassroomID, v.StartAt.Format(time.RFC3339), v.Minutes, v.Status, v.ID)
	return err
}

func (s lessonStore) Delete(ctx context.Context, idOrCode string) error {
	if num, e := strconv.ParseInt(idOrCode, 10, 64); e == nil && num > 0 {
		_, err := s.r.db.Execute(ctx, `DELETE FROM lessons WHERE id = ?1 OR code = ?2`, num, idOrCode)
		return err
	}
	_, err := s.r.db.Execute(ctx, `DELETE FROM lessons WHERE code = ?1`, idOrCode)
	return err
}

func scanLesson(v []any) domain.Lesson {
	return domain.Lesson{
		ID:          colInt(at(v, 0)),
		Code:        colString(at(v, 1)),
		CourseID:    colInt(at(v, 2)),
		TeacherID:   colInt(at(v, 3)),
		ClassroomID: colInt(at(v, 4)),
		StartAt:     colTime(at(v, 5)),
		Minutes:     int(colInt(at(v, 6))),
		Status:      colString(at(v, 7)),
	}
}

// ─── ledger ─────────────────────────────────────────────────────────────

type ledgerStore struct{ r *sqliteRepo }

const ledgerCols = `id, code, student_id, kind, amount_cents, balance_cents, note, at`

func (l ledgerStore) resolveStudentID(ctx context.Context, studentID any) (int64, error) {
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
		rows, err := l.r.db.Query(ctx, "SELECT id FROM students WHERE code = ?", v)
		if err != nil {
			return 0, err
		}
		if len(rows.Values) == 0 {
			return 0, domain.ErrNotFound
		}
		return colInt(rows.Values[0][0]), nil
	}
	return 0, domain.ErrNotFound
}

func (l ledgerStore) List(ctx context.Context, studentID any, limit int) ([]domain.LedgerEntry, error) {
	sql := `SELECT ` + ledgerCols + ` FROM ledger`
	var args []any
	if studentID != nil && studentID != "" && studentID != int64(0) {
		stuID, err := l.resolveStudentID(ctx, studentID)
		if err == nil {
			sql += ` WHERE student_id = ?`
			args = append(args, stuID)
		}
	}
	sql += ` ORDER BY at DESC`
	if limit > 0 {
		sql += ` LIMIT ` + strconv.Itoa(limit)
	}
	rows, err := l.r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	out := make([]domain.LedgerEntry, 0, len(rows.Values))
	for _, v := range rows.Values {
		out = append(out, scanLedger(v))
	}
	return out, nil
}

func (l ledgerStore) TopUp(ctx context.Context, studentID any, amountCents int64, note string) (domain.LedgerEntry, error) {
	if amountCents <= 0 {
		return domain.LedgerEntry{}, errors.New("充值金额必须大于 0")
	}
	return l.apply(ctx, studentID, domain.KindTopUp, amountCents, note)
}

func (l ledgerStore) Consume(ctx context.Context, studentID any, amountCents int64, note string) (domain.LedgerEntry, error) {
	if amountCents <= 0 {
		return domain.LedgerEntry{}, errors.New("扣费金额必须大于 0")
	}
	return l.apply(ctx, studentID, domain.KindConsume, -amountCents, note)
}

func (l ledgerStore) apply(ctx context.Context, studentID any, kind string, delta int64, note string) (domain.LedgerEntry, error) {
	l.r.mu.Lock()
	defer l.r.mu.Unlock()

	stuID, err := l.resolveStudentID(ctx, studentID)
	if err != nil {
		return domain.LedgerEntry{}, fmt.Errorf("student %v: %w", studentID, domain.ErrNotFound)
	}

	rows, err := l.r.db.Query(ctx, `SELECT balance_cents FROM students WHERE id = ?`, stuID)
	if err != nil {
		return domain.LedgerEntry{}, err
	}
	if len(rows.Values) == 0 {
		return domain.LedgerEntry{}, fmt.Errorf("student %v: %w", studentID, domain.ErrNotFound)
	}
	current := colInt(rows.Values[0][0])
	next := current + delta
	if next < 0 {
		return domain.LedgerEntry{}, errors.New("余额不足，无法消课")
	}

	if _, err := l.r.db.Execute(ctx, `UPDATE students SET balance_cents = ? WHERE id = ?`, next, stuID); err != nil {
		return domain.LedgerEntry{}, err
	}
	entry := domain.LedgerEntry{
		Code:      domain.NewID("ldg"),
		StudentID: stuID,
		Kind:      kind,
		Amount:    delta,
		Balance:   next,
		Note:      note,
		At:        time.Now(),
	}
	res, err := l.r.db.Execute(ctx, `
		INSERT INTO ledger (code, student_id, kind, amount_cents, balance_cents, note, at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		entry.Code, entry.StudentID, entry.Kind, entry.Amount, entry.Balance, entry.Note, entry.At.Format(time.RFC3339))
	if err != nil {
		return domain.LedgerEntry{}, err
	}
	entry.ID = res.LastInsertID
	return entry, nil
}

func scanLedger(v []any) domain.LedgerEntry {
	return domain.LedgerEntry{
		ID:        colInt(at(v, 0)),
		Code:      colString(at(v, 1)),
		StudentID: colInt(at(v, 2)),
		Kind:      colString(at(v, 3)),
		Amount:    colInt(at(v, 4)),
		Balance:   colInt(at(v, 5)),
		Note:      colString(at(v, 6)),
		At:        colTime(at(v, 7)),
	}
}

// ─── auth & users ───────────────────────────────────────────────────────

func (r *sqliteRepo) ensureAdmin(ctx context.Context) error {
	rows, err := r.db.Query(ctx, `SELECT COUNT(*) FROM users WHERE username = 'admin'`)
	if err != nil {
		return err
	}
	if len(rows.Values) > 0 && colInt(rows.Values[0][0]) > 0 {
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
	_, err = r.db.Execute(ctx, `
		INSERT INTO users (code, username, display_name, role, salt, hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(username) DO NOTHING`,
		admin.Code, admin.Username, admin.DisplayName, admin.Role, salt, hash, admin.CreatedAt.Format(time.RFC3339))
	return err
}

type authStore struct{ r *sqliteRepo }

func (a authStore) Login(ctx context.Context, username, password string) (domain.User, error) {
	username = strings.TrimSpace(username)
	rows, err := a.r.db.Query(ctx, `SELECT id, code, username, display_name, role, salt, hash, created_at FROM users WHERE username = ?`, username)
	if err != nil {
		return domain.User{}, err
	}
	if len(rows.Values) == 0 {
		return domain.User{}, domain.ErrBadCredentials
	}
	row := rows.Values[0]
	salt := colString(at(row, 5))
	hash := colString(at(row, 6))
	if !crypto.CheckPassword(password, salt, hash) {
		return domain.User{}, domain.ErrBadCredentials
	}
	return domain.User{
		ID:          colInt(at(row, 0)),
		Code:        colString(at(row, 1)),
		Username:    colString(at(row, 2)),
		DisplayName: colString(at(row, 3)),
		Role:        colString(at(row, 4)),
		CreatedAt:   colTime(at(row, 7)),
	}, nil
}

func (a authStore) ResetPassword(ctx context.Context, username, recoveryKey, newPassword string) error {
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

	rows, err := a.r.db.Query(ctx, `SELECT id FROM users WHERE username = ?`, username)
	if err != nil {
		return err
	}
	if len(rows.Values) == 0 {
		return domain.ErrUserNotFound
	}
	userID := colInt(rows.Values[0][0])

	salt, hash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return err
	}
	_, err = a.r.db.Execute(ctx, `UPDATE users SET salt = ?, hash = ? WHERE id = ?`, salt, hash, userID)
	return err
}

func (a authStore) Register(ctx context.Context, username, password, displayName string) (domain.User, error) {
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

type userStore struct{ r *sqliteRepo }

const userCols = `id, code, username, display_name, role, created_at`

func (u userStore) List(ctx context.Context) ([]domain.User, error) {
	rows, err := u.r.db.Query(ctx, `SELECT `+userCols+` FROM users ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	out := make([]domain.User, 0, len(rows.Values))
	for _, v := range rows.Values {
		out = append(out, domain.User{
			ID:          colInt(at(v, 0)),
			Code:        colString(at(v, 1)),
			Username:    colString(at(v, 2)),
			DisplayName: colString(at(v, 3)),
			Role:        colString(at(v, 4)),
			CreatedAt:   colTime(at(v, 5)),
		})
	}
	return out, nil
}

func (u userStore) Create(ctx context.Context, v *domain.User, password string) error {
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
	res, err := u.r.db.Execute(ctx, `
		INSERT INTO users (code, username, display_name, role, salt, hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		v.Code, v.Username, v.DisplayName, v.Role, salt, hash, v.CreatedAt.Format(time.RFC3339))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "unique") {
			return errors.New("用户名已存在")
		}
		return err
	}
	v.ID = res.LastInsertID
	return nil
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

func (u userStore) resolveUserID(ctx context.Context, id any) (int64, error) {
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
		rows, err := u.r.db.Query(ctx, "SELECT id FROM users WHERE code = ? OR username = ?", v, v)
		if err != nil {
			return 0, err
		}
		if len(rows.Values) == 0 {
			return 0, domain.ErrNotFound
		}
		return colInt(rows.Values[0][0]), nil
	}
	return 0, domain.ErrNotFound
}

func (u userStore) SetPassword(ctx context.Context, id any, password string) error {
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
	_, err = u.r.db.Execute(ctx, `UPDATE users SET salt = ?, hash = ? WHERE id = ?`, salt, hash, userID)
	return err
}

func (u userStore) Delete(ctx context.Context, id any) error {
	userID, err := u.resolveUserID(ctx, id)
	if err != nil {
		return err
	}
	if err := u.keepOneAdmin(ctx, userID); err != nil {
		return err
	}
	_, err = u.r.db.Execute(ctx, `DELETE FROM users WHERE id = ?`, userID)
	return err
}

func (u userStore) keepOneAdmin(ctx context.Context, id int64) error {
	rows, err := u.r.db.Query(ctx, `SELECT COUNT(*) FROM users WHERE role = 'admin' AND id <> ?`, id)
	if err != nil {
		return err
	}
	if len(rows.Values) > 0 && colInt(rows.Values[0][0]) > 0 {
		return nil
	}
	rows, err = u.r.db.Query(ctx, `SELECT role FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if len(rows.Values) > 0 && colString(rows.Values[0][0]) == string(domain.RoleAdmin) {
		return errors.New("至少要保留一个管理员")
	}
	return nil
}

// ─── settings ───────────────────────────────────────────────────────────

type settingsStore struct{ r *sqliteRepo }

func (s settingsStore) Get(ctx context.Context, key string) (string, error) {
	rows, err := s.r.db.Query(ctx, `SELECT value FROM settings WHERE key = ?`, key)
	if err != nil {
		return "", err
	}
	if len(rows.Values) == 0 {
		return "", nil
	}
	return colString(rows.Values[0][0]), nil
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
