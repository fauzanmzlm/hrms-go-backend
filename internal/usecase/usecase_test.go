package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
)

var (
	ctx      = context.Background()
	fixedNow = time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
)

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

// fixture is a small HR dataset: two grades, one active employee on N41 with
// a current service record, an HR officer and an approver.
type fixture struct {
	s            *store
	n41, n44     *domain.Grade
	emp          *domain.Employee
	hr, approver *domain.User
	promotions   *PromotionUsecase
	disciplinary *DisciplinaryUsecase
	records      *ServiceRecordUsecase
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	s := newStore()
	f := &fixture{s: s}
	grades, employees, records := gradeRepo{s}, employeeRepo{s}, recordRepo{s}
	users := userRepo{s}

	f.n41 = &domain.Grade{Code: "N41", Scheme: "N", Level: 41, Title: "Pegawai Tadbir"}
	f.n44 = &domain.Grade{Code: "N44", Scheme: "N", Level: 44, Title: "Pegawai Tadbir Kanan"}
	must(t, grades.Create(ctx, f.n41))
	must(t, grades.Create(ctx, f.n44))

	f.emp = &domain.Employee{
		EmployeeNo: "E001", ICNumber: "900101145678", FullName: "Aminah binti Ali",
		Email: "aminah@agency.gov.my", DateOfBirth: date(1990, 1, 1), Gender: domain.GenderFemale,
		CurrentGradeID: f.n41.ID, Position: "Pegawai Tadbir", Department: "Pentadbiran",
		ServiceStatus: domain.ServiceStatusActive, DateJoined: date(2015, 3, 1),
	}
	must(t, employees.Create(ctx, f.emp))
	must(t, records.Create(ctx, &domain.ServiceRecord{
		EmployeeID: f.emp.ID, GradeID: f.n41.ID, Position: f.emp.Position, Department: f.emp.Department,
		AppointmentType: domain.AppointmentPermanent, EffectiveFrom: date(2015, 3, 1),
	}))

	f.hr = &domain.User{Username: "hr", Role: domain.RoleHROfficer, IsActive: true}
	f.approver = &domain.User{Username: "approver", Role: domain.RoleApprover, IsActive: true}
	must(t, users.Create(ctx, f.hr))
	must(t, users.Create(ctx, f.approver))

	clock := func() time.Time { return fixedNow }
	f.promotions = NewPromotionUsecase(passthroughTx{}, employees, grades, records,
		promotionRepo{s}, disciplinaryRepo{s}, users, PromotionConfig{DisciplinaryBlockMonths: 12}, clock)
	f.disciplinary = NewDisciplinaryUsecase(passthroughTx{}, employees, disciplinaryRepo{s}, users, clock)
	f.records = NewServiceRecordUsecase(passthroughTx{}, employees, grades, records)
	return f
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) submit(t *testing.T) *domain.PromotionApplication {
	t.Helper()
	app, err := f.promotions.Submit(ctx, domain.SubmitPromotionInput{
		EmployeeID: f.emp.ID, ProposedGradeID: f.n44.ID, Justification: "Cemerlang 3 tahun berturut-turut",
	})
	must(t, err)
	return app
}

func TestPromotionApproveUpdatesGradeAndServiceHistory(t *testing.T) {
	f := newFixture(t)
	app := f.submit(t)
	if app.Status != domain.StatusPending || app.CurrentGradeID != f.n41.ID {
		t.Fatalf("submitted app = %+v", app)
	}

	effective := date(2026, 11, 1)
	got, err := f.promotions.Review(ctx, app.ID,
		domain.ReviewDecision{ReviewerID: f.approver.ID, Status: domain.StatusApproved, Remarks: "Disokong"}, effective)
	must(t, err)
	if got.Status != domain.StatusApproved || !got.EffectiveDate.Equal(effective) {
		t.Fatalf("reviewed app = %+v", got)
	}

	emp, _ := employeeRepo{f.s}.GetByID(ctx, f.emp.ID)
	if emp.CurrentGradeID != f.n44.ID {
		t.Errorf("employee grade = %d, want N44 (%d)", emp.CurrentGradeID, f.n44.ID)
	}

	history, _ := recordRepo{f.s}.ListByEmployee(ctx, f.emp.ID)
	if len(history) != 2 {
		t.Fatalf("want 2 service records, got %d", len(history))
	}
	for _, r := range history {
		switch r.AppointmentType {
		case domain.AppointmentPermanent:
			if r.EffectiveTo == nil || !r.EffectiveTo.Equal(date(2026, 10, 31)) {
				t.Errorf("previous record should close on 2026-10-31, got %v", r.EffectiveTo)
			}
		case domain.AppointmentPromotion:
			if !r.IsCurrent() || r.GradeID != f.n44.ID || !r.EffectiveFrom.Equal(effective) {
				t.Errorf("promotion record = %+v", r)
			}
		}
	}

	_, err = f.promotions.Review(ctx, app.ID,
		domain.ReviewDecision{ReviewerID: f.approver.ID, Status: domain.StatusRejected, Remarks: "late"}, time.Time{})
	if !errors.Is(err, domain.ErrInvalidStatusTransition) {
		t.Errorf("second review err = %v, want ErrInvalidStatusTransition", err)
	}
}

func TestPromotionSubmitRules(t *testing.T) {
	t.Run("proposed grade not higher", func(t *testing.T) {
		f := newFixture(t)
		_, err := f.promotions.Submit(ctx, domain.SubmitPromotionInput{
			EmployeeID: f.emp.ID, ProposedGradeID: f.n41.ID, Justification: "x",
		})
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("err = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("already pending", func(t *testing.T) {
		f := newFixture(t)
		f.submit(t)
		_, err := f.promotions.Submit(ctx, domain.SubmitPromotionInput{
			EmployeeID: f.emp.ID, ProposedGradeID: f.n44.ID, Justification: "again",
		})
		if !errors.Is(err, domain.ErrConflict) {
			t.Errorf("err = %v, want ErrConflict", err)
		}
	})

	t.Run("recent disciplinary penalty", func(t *testing.T) {
		f := newFixture(t)
		reviewed := fixedNow.AddDate(0, -3, 0)
		f.s.disciplinary[999] = domain.DisciplinaryRecord{
			ID: 999, EmployeeID: f.emp.ID, Status: domain.StatusApproved, ReviewedAt: &reviewed,
		}
		_, err := f.promotions.Submit(ctx, domain.SubmitPromotionInput{
			EmployeeID: f.emp.ID, ProposedGradeID: f.n44.ID, Justification: "x",
		})
		if !errors.Is(err, domain.ErrRuleViolation) {
			t.Errorf("err = %v, want ErrRuleViolation", err)
		}
	})

	t.Run("old disciplinary penalty does not block", func(t *testing.T) {
		f := newFixture(t)
		reviewed := fixedNow.AddDate(-2, 0, 0)
		f.s.disciplinary[999] = domain.DisciplinaryRecord{
			ID: 999, EmployeeID: f.emp.ID, Status: domain.StatusApproved, ReviewedAt: &reviewed,
		}
		f.submit(t)
	})

	t.Run("inactive employee", func(t *testing.T) {
		f := newFixture(t)
		e := f.s.employees[f.emp.ID]
		e.ServiceStatus = domain.ServiceStatusSuspended
		f.s.employees[f.emp.ID] = e
		_, err := f.promotions.Submit(ctx, domain.SubmitPromotionInput{
			EmployeeID: f.emp.ID, ProposedGradeID: f.n44.ID, Justification: "x",
		})
		if !errors.Is(err, domain.ErrRuleViolation) {
			t.Errorf("err = %v, want ErrRuleViolation", err)
		}
	})
}

func TestPromotionReviewGuards(t *testing.T) {
	t.Run("cannot review own application", func(t *testing.T) {
		f := newFixture(t)
		app := f.submit(t)
		self := &domain.User{Username: "self", Role: domain.RoleApprover, EmployeeID: &f.emp.ID, IsActive: true}
		must(t, userRepo{f.s}.Create(ctx, self))

		_, err := f.promotions.Review(ctx, app.ID,
			domain.ReviewDecision{ReviewerID: self.ID, Status: domain.StatusApproved}, date(2026, 11, 1))
		if !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("err = %v, want ErrForbidden", err)
		}
	})

	t.Run("approval requires effective date", func(t *testing.T) {
		f := newFixture(t)
		app := f.submit(t)
		_, err := f.promotions.Review(ctx, app.ID,
			domain.ReviewDecision{ReviewerID: f.approver.ID, Status: domain.StatusApproved}, time.Time{})
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("err = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("effective date before current record", func(t *testing.T) {
		f := newFixture(t)
		app := f.submit(t)
		_, err := f.promotions.Review(ctx, app.ID,
			domain.ReviewDecision{ReviewerID: f.approver.ID, Status: domain.StatusApproved}, date(2014, 1, 1))
		var ve *domain.ValidationError
		if !errors.As(err, &ve) || ve.Fields["effective_date"] == "" {
			t.Errorf("err = %v, want validation error on effective_date", err)
		}
	})

	t.Run("reject leaves grade unchanged", func(t *testing.T) {
		f := newFixture(t)
		app := f.submit(t)
		_, err := f.promotions.Review(ctx, app.ID,
			domain.ReviewDecision{ReviewerID: f.approver.ID, Status: domain.StatusRejected, Remarks: "Belum layak"}, time.Time{})
		must(t, err)
		if emp := f.s.employees[f.emp.ID]; emp.CurrentGradeID != f.n41.ID {
			t.Errorf("grade changed on rejection")
		}
	})
}

func (f *fixture) openCase(t *testing.T, penalty domain.PenaltyType) *domain.DisciplinaryRecord {
	t.Helper()
	d := &domain.DisciplinaryRecord{
		EmployeeID: f.emp.ID, CaseNo: "TT/2026/001", Category: domain.OffenseSerious,
		Description: "Tidak hadir bertugas tanpa cuti", IncidentDate: date(2026, 9, 1),
		Penalty: penalty, ReportedBy: f.hr.ID,
	}
	must(t, f.disciplinary.Create(ctx, d))
	return d
}

func TestDisciplinaryReview(t *testing.T) {
	t.Run("reporter cannot review", func(t *testing.T) {
		f := newFixture(t)
		d := f.openCase(t, domain.PenaltyWarning)
		_, err := f.disciplinary.Review(ctx, d.ID, domain.ReviewDecision{ReviewerID: f.hr.ID, Status: domain.StatusApproved})
		if !errors.Is(err, domain.ErrForbidden) {
			t.Errorf("err = %v, want ErrForbidden", err)
		}
	})

	t.Run("approved dismissal terminates employee", func(t *testing.T) {
		f := newFixture(t)
		d := f.openCase(t, domain.PenaltyDismissal)
		got, err := f.disciplinary.Review(ctx, d.ID, domain.ReviewDecision{ReviewerID: f.approver.ID, Status: domain.StatusApproved})
		must(t, err)
		if got.Status != domain.StatusApproved {
			t.Errorf("status = %s", got.Status)
		}
		if emp := f.s.employees[f.emp.ID]; emp.ServiceStatus != domain.ServiceStatusTerminated {
			t.Errorf("service status = %s, want TERMINATED", emp.ServiceStatus)
		}
	})

	t.Run("future incident date rejected", func(t *testing.T) {
		f := newFixture(t)
		err := f.disciplinary.Create(ctx, &domain.DisciplinaryRecord{
			EmployeeID: f.emp.ID, CaseNo: "TT/2026/002", Category: domain.OffenseMinor,
			Description: "x", IncidentDate: fixedNow.AddDate(0, 0, 1), Penalty: domain.PenaltyWarning, ReportedBy: f.hr.ID,
		})
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("err = %v, want ErrInvalidInput", err)
		}
	})
}

func TestServiceRecordCreateCurrentClosesPrevious(t *testing.T) {
	f := newFixture(t)
	r := &domain.ServiceRecord{
		EmployeeID: f.emp.ID, GradeID: f.n41.ID, Position: "Ketua Unit", Department: "Kewangan",
		AppointmentType: domain.AppointmentSecondment, EffectiveFrom: date(2024, 1, 15),
	}
	must(t, f.records.Create(ctx, r))

	emp := f.s.employees[f.emp.ID]
	if emp.Department != "Kewangan" || emp.Position != "Ketua Unit" {
		t.Errorf("employee not synced: %+v", emp)
	}
	current, _ := recordRepo{f.s}.GetCurrentByEmployee(ctx, f.emp.ID)
	if current.ID != r.ID {
		t.Errorf("current record = %d, want %d", current.ID, r.ID)
	}

	err := f.records.Create(ctx, &domain.ServiceRecord{
		EmployeeID: f.emp.ID, GradeID: f.n41.ID, Position: "x", Department: "y",
		AppointmentType: domain.AppointmentPermanent, EffectiveFrom: date(2023, 1, 1),
	})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("backdated current record err = %v, want ErrInvalidInput", err)
	}
}

func TestEmployeeUpdateKeepsStatusAndGrade(t *testing.T) {
	f := newFixture(t)
	e := f.s.employees[f.emp.ID]
	e.ServiceStatus = domain.ServiceStatusTerminated
	f.s.employees[f.emp.ID] = e

	uc := NewEmployeeUsecase(employeeRepo{f.s}, gradeRepo{f.s})
	upd := e
	upd.ServiceStatus = ""
	upd.CurrentGradeID = f.n44.ID
	upd.Phone = "012-3456789"
	must(t, uc.Update(ctx, &upd))

	got := f.s.employees[f.emp.ID]
	if got.ServiceStatus != domain.ServiceStatusTerminated {
		t.Errorf("status = %s, want TERMINATED kept", got.ServiceStatus)
	}
	if got.CurrentGradeID != f.n41.ID {
		t.Errorf("grade changed through Update")
	}
	if got.Phone != "012-3456789" {
		t.Errorf("phone not updated")
	}
}

func TestAuthLogin(t *testing.T) {
	s := newStore()
	auth, err := NewAuthUsecase(userRepo{s}, employeeRepo{s}, fakeHasher{}, fakeTokens{})
	must(t, err)
	must(t, auth.CreateUser(ctx, &domain.User{Username: "admin", Role: domain.RoleAdmin}, "s3cret-pass"))

	res, err := auth.Login(ctx, " admin ", "s3cret-pass")
	must(t, err)
	if res.AccessToken == "" || res.TokenType != "Bearer" || res.User.Username != "admin" {
		t.Errorf("login result = %+v", res)
	}

	for name, creds := range map[string][2]string{
		"wrong password": {"admin", "nope-nope"},
		"unknown user":   {"ghost", "s3cret-pass"},
	} {
		if _, err := auth.Login(ctx, creds[0], creds[1]); !errors.Is(err, domain.ErrUnauthorized) {
			t.Errorf("%s: err = %v, want ErrUnauthorized", name, err)
		}
	}

	if err := auth.CreateUser(ctx, &domain.User{Username: "short", Role: domain.RoleHROfficer}, "123"); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("short password err = %v, want ErrInvalidInput", err)
	}
	err = auth.CreateUser(ctx, &domain.User{Username: "orphan", Role: domain.RoleEmployee}, "s3cret-pass")
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || ve.Fields["employee_id"] == "" {
		t.Errorf("EMPLOYEE without employee_id err = %v, want validation error on employee_id", err)
	}
}
