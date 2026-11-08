-- Demo seed data for development and testing environments

-- 1. Teachers
INSERT INTO teachers (code, name, phone, subject, rate_cents, active, created_at)
VALUES
    ('tch_seed1', '陈静', '13800000001', '高中数学', 20000, TRUE, CURRENT_TIMESTAMP - INTERVAL '6 month'),
    ('tch_seed2', '李磊', '13800000002', '初中物理', 18000, TRUE, CURRENT_TIMESTAMP - INTERVAL '5 month'),
    ('tch_seed3', '王芳', '13800000003', '少儿英语', 15000, TRUE, CURRENT_TIMESTAMP - INTERVAL '4 month')
ON CONFLICT (code) DO NOTHING;

-- 2. Classrooms
INSERT INTO classrooms (code, name, capacity, location, note)
VALUES
    ('rom_seed1', '文远楼 201', 30, '东校区二楼', '配备多媒体投影'),
    ('rom_seed2', '博雅楼 102', 15, '西校区一楼', '小班 VIP 教室')
ON CONFLICT (code) DO NOTHING;

-- 3. Courses
INSERT INTO courses (code, name, subject, teacher_id, total_lessons, price_cents, minutes, active)
SELECT 'crs_seed1', '初二数学同步培优', '初中数学', id, 24, 480000, 90, TRUE
FROM teachers WHERE code = 'tch_seed1'
ON CONFLICT (code) DO NOTHING;

INSERT INTO courses (code, name, subject, teacher_id, total_lessons, price_cents, minutes, active)
SELECT 'crs_seed2', '少儿英语自然拼读', '少儿英语', id, 18, 360000, 60, TRUE
FROM teachers WHERE code = 'tch_seed3'
ON CONFLICT (code) DO NOTHING;

-- 4. Students
INSERT INTO students (code, name, guardian, phone, balance_cents, note, active, created_at)
VALUES
    ('stu_seed1', '王小明', '王女士', '13900000001', 480000, '周三、周五上课', TRUE, CURRENT_TIMESTAMP - INTERVAL '3 month'),
    ('stu_seed2', '李思雨', '李先生', '13900000002', 360000, '需接送', TRUE, CURRENT_TIMESTAMP - INTERVAL '2 month'),
    ('stu_seed3', '张浩然', '张先生', '13900000003', 200000, '周末走读', TRUE, CURRENT_TIMESTAMP - INTERVAL '1 month')
ON CONFLICT (code) DO NOTHING;

-- 5. Ledger
INSERT INTO ledger (code, student_id, kind, amount_cents, balance_cents, note, at)
SELECT 'ldg_seed1', id, 'topup', 480000, 480000, '报名初二数学同步', CURRENT_TIMESTAMP - INTERVAL '3 month'
FROM students WHERE code = 'stu_seed1'
ON CONFLICT (code) DO NOTHING;

INSERT INTO ledger (code, student_id, kind, amount_cents, balance_cents, note, at)
SELECT 'ldg_seed2', id, 'topup', 360000, 360000, '报名少儿英语启蒙', CURRENT_TIMESTAMP - INTERVAL '2 month'
FROM students WHERE code = 'stu_seed2'
ON CONFLICT (code) DO NOTHING;

INSERT INTO ledger (code, student_id, kind, amount_cents, balance_cents, note, at)
SELECT 'ldg_seed3', id, 'topup', 200000, 200000, '预存充值', CURRENT_TIMESTAMP - INTERVAL '1 month'
FROM students WHERE code = 'stu_seed3'
ON CONFLICT (code) DO NOTHING;

-- 6. Lessons
INSERT INTO lessons (code, course_id, teacher_id, classroom_id, start_at, minutes, status)
SELECT 'les_seed1', c.id, t.id, r.id, CURRENT_TIMESTAMP + INTERVAL '1 day', 90, 'planned'
FROM courses c, teachers t, classrooms r
WHERE c.code = 'crs_seed1' AND t.code = 'tch_seed1' AND r.code = 'rom_seed1'
ON CONFLICT (code) DO NOTHING;

INSERT INTO lessons (code, course_id, teacher_id, classroom_id, start_at, minutes, status)
SELECT 'les_seed2', c.id, t.id, r.id, CURRENT_TIMESTAMP + INTERVAL '2 day', 60, 'planned'
FROM courses c, teachers t, classrooms r
WHERE c.code = 'crs_seed2' AND t.code = 'tch_seed3' AND r.code = 'rom_seed2'
ON CONFLICT (code) DO NOTHING;

INSERT INTO lessons (code, course_id, teacher_id, classroom_id, start_at, minutes, status)
SELECT 'les_seed3', c.id, t.id, r.id, CURRENT_TIMESTAMP - INTERVAL '3 day', 90, 'done'
FROM courses c, teachers t, classrooms r
WHERE c.code = 'crs_seed1' AND t.code = 'tch_seed1' AND r.code = 'rom_seed1'
ON CONFLICT (code) DO NOTHING;
