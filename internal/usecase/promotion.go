package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
)

type PromotionConfig struct {
	// DisciplinaryBlockMonths is how long an approved disciplinary penalty
	// blocks promotion. Zero disables the check.
	DisciplinaryBlockMonths int
}

type PromotionUsecase struct {
	tx           domain.Transactor
	employees    domain.EmployeeRepository
	grades       domain.GradeRepository
	promotions   domain.PromotionRepository
	disciplinary domain.DisciplinaryRepository
	users        domain.UserRepository
	history      serviceHistory
	cfg          PromotionConfig
	now          Clock
}

var _ domain.PromotionUsecase = (*PromotionUsecase)(nil)

func NewPromotionUsecase(
	tx domain.Transactor,
	employees domain.EmployeeRepository,
	grades domain.GradeRepository,
	records domain.ServiceRecordRepository,
	promotions domain.PromotionRepository,
	disciplinary domain.DisciplinaryRepository,
	users domain.UserRepository,
	cfg PromotionConfig,
	now Clock,
) *PromotionUsecase {
	return &PromotionUsecase{
		tx:           tx,
		employees:    employees,
		grades:       grades,
		promotions:   promotions,
		disciplinary: disciplinary,
		users:        users,
		history:      serviceHistory{employees: employees, records: records},
		cfg:          cfg,
		now:          now,
	}
}

func (u *PromotionUsecase) Submit(ctx context.Context, in domain.SubmitPromotionInput) (*domain.PromotionApplication, error) {
	in.Justification = strings.TrimSpace(in.Justification)
	v := domain.NewValidationError()
	if in.EmployeeID <= 0 {
		v.Add("employee_id", "is required")
	}
	if in.ProposedGradeID <= 0 {
		v.Add("proposed_grade_id", "is required")
	}
	if in.Justification == "" {
		v.Add("justification", "is required")
	}
	if err := v.Err(); err != nil {
		return nil, err
	}

	emp, err := u.employees.GetByID(ctx, in.EmployeeID)
	if err != nil {
		return nil, fieldNotFound(err, "employee_id")
	}
	if !emp.IsActive() {
		return nil, ruleViolation("only ACTIVE employees can apply for promotion (employee is %s)", emp.ServiceStatus)
	}
	proposed, err := u.grades.GetByID(ctx, in.ProposedGradeID)
	if err != nil {
		return nil, fieldNotFound(err, "proposed_grade_id")
	}
	current, err := u.grades.GetByID(ctx, emp.CurrentGradeID)
	if err != nil {
		return nil, err
	}
	if !proposed.IsHigherThan(current) {
		return nil, validationErr("proposed_grade_id",
			fmt.Sprintf("must be higher than the current grade %s", current.Code))
	}

	pending, err := u.promotions.HasPendingByEmployee(ctx, emp.ID)
	if err != nil {
		return nil, err
	}
	if pending {
		return nil, fmt.Errorf("%w: employee already has a pending promotion application", domain.ErrConflict)
	}
	now := u.now()
	if err := u.checkDisciplinaryBlock(ctx, emp.ID, now); err != nil {
		return nil, err
	}

	p := &domain.PromotionApplication{
		EmployeeID:      emp.ID,
		CurrentGradeID:  emp.CurrentGradeID,
		ProposedGradeID: proposed.ID,
		Justification:   in.Justification,
		Status:          domain.StatusPending,
		SubmittedAt:     now,
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if err := u.promotions.Create(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (u *PromotionUsecase) Review(ctx context.Context, id int64, d domain.ReviewDecision, effectiveDate time.Time) (*domain.PromotionApplication, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}

	var result *domain.PromotionApplication
	err := u.tx.WithinTx(ctx, func(ctx context.Context) error {
		app, err := u.promotions.GetByID(ctx, id)
		if err != nil {
			return err
		}
		reviewer, err := loadReviewer(ctx, u.users, d.ReviewerID)
		if err != nil {
			return err
		}
		if isSameEmployee(reviewer, app.EmployeeID) {
			return fmt.Errorf("%w: you cannot review your own promotion application", domain.ErrForbidden)
		}

		now := u.now()
		if d.Status == domain.StatusApproved {
			if err := app.Approve(d.ReviewerID, d.Remarks, dateOnly(effectiveDate), now); err != nil {
				return err
			}
			if err := u.checkDisciplinaryBlock(ctx, app.EmployeeID, now); err != nil {
				return err
			}
		} else if err := app.Reject(d.ReviewerID, d.Remarks, now); err != nil {
			return err
		}

		if err := u.promotions.UpdateReview(ctx, app); err != nil {
			return err
		}
		if app.Status == domain.StatusApproved {
			if err := u.applyPromotion(ctx, app); err != nil {
				return err
			}
		}
		result = app
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// applyPromotion moves the employee to the proposed grade by opening a
// PROMOTION service record at the effective date.
func (u *PromotionUsecase) applyPromotion(ctx context.Context, app *domain.PromotionApplication) error {
	emp, err := u.employees.GetByID(ctx, app.EmployeeID)
	if err != nil {
		return err
	}
	if !emp.IsActive() {
		return ruleViolation("employee is %s and cannot be promoted", emp.ServiceStatus)
	}
	if emp.CurrentGradeID != app.CurrentGradeID {
		return ruleViolation("employee's grade has changed since the application was submitted")
	}

	record := &domain.ServiceRecord{
		EmployeeID:      emp.ID,
		GradeID:         app.ProposedGradeID,
		Position:        emp.Position,
		Department:      emp.Department,
		AppointmentType: domain.AppointmentPromotion,
		EffectiveFrom:   *app.EffectiveDate,
		Remarks:         fmt.Sprintf("Promotion application #%d", app.ID),
	}
	return u.history.appendCurrent(ctx, emp, record, "effective_date")
}

func (u *PromotionUsecase) checkDisciplinaryBlock(ctx context.Context, employeeID int64, now time.Time) error {
	if u.cfg.DisciplinaryBlockMonths <= 0 {
		return nil
	}
	blocked, err := u.disciplinary.HasApprovedSince(ctx, employeeID, now.AddDate(0, -u.cfg.DisciplinaryBlockMonths, 0))
	if err != nil {
		return err
	}
	if blocked {
		return ruleViolation("employee has an approved disciplinary penalty within the last %d months",
			u.cfg.DisciplinaryBlockMonths)
	}
	return nil
}

func (u *PromotionUsecase) Get(ctx context.Context, id int64) (*domain.PromotionApplication, error) {
	return u.promotions.GetByID(ctx, id)
}

func (u *PromotionUsecase) List(ctx context.Context, f domain.PromotionFilter) (domain.Page[domain.PromotionApplication], error) {
	f.Normalize()
	items, total, err := u.promotions.List(ctx, f)
	if err != nil {
		return domain.Page[domain.PromotionApplication]{}, err
	}
	return domain.NewPage(items, total, f.Pagination), nil
}
