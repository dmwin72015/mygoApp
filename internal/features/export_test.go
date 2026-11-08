package features

// Exported helpers for headless integration tests in package features_test.

func (t *Teachers) NewTeacherForTest(name, subject, phone string, rateCents int64) {
	t.startNew()
	t.draft.Name = name
	t.draft.Subject = subject
	t.draft.Phone = phone
	t.draft.RateCents = rateCents
	t.save()
}

func (cr *Classrooms) NewClassroomForTest(name string, capacity int, location string) {
	cr.startNew()
	cr.draft.Name = name
	cr.draft.Capacity = capacity
	cr.draft.Location = location
	cr.save()
}

func (c *Courses) NewCourseForTest(name, subject string, totalLessons, minutes int, priceCents int64) {
	c.startNew()
	c.draft.Name = name
	c.draft.Subject = subject
	c.draft.TotalLessons = totalLessons
	c.draft.Minutes = minutes
	c.draft.PriceCents = priceCents
	c.save()
}

func (p *Lessons) NewLessonForTest(courseName, teacherName, roomName, startAtStr string, minutes int, status string) {
	p.startNew()
	p.selectedCourse = courseName
	p.selectedTeacher = teacherName
	p.selectedRoom = roomName
	p.startAtStr = startAtStr
	p.draft.Minutes = minutes
	p.selectedStatus = status
	p.save()
}

func (s *Settings) ChangePasswordForTest(oldPwd, newPwd, confirmPwd string) {
	s.oldPassword = oldPwd
	s.newPassword = newPwd
	s.confirmPwd = confirmPwd
	s.changePassword()
}

func (t *Teachers) ReloadForTest() { t.reload() }
func (cr *Classrooms) ReloadForTest() { cr.reload() }
func (c *Courses) ReloadForTest() { c.reload() }
func (p *Lessons) ReloadForTest() { p.reload() }

