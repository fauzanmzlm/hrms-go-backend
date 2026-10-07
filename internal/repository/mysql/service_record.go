package mysql

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
)

const serviceRecordColumns = `id, employee_id, grade_id, position, department, appointment_type,
	effective_from, effective_to, remarks, created_at, updated_at`

var serviceRecordKeys = uniqueKeys{
	"uq_service_records_current": "employee already has a current service record",
}

type ServiceRecordRepository struct {
	base
}

var _ domain.ServiceRecordRepository = (*ServiceRecordRepository)(nil)

func NewServiceRecordRepository(db *sqlx.DB) *ServiceRecordRepository {
	return &ServiceRecordRepository{base{db: db}}
}

func (r *ServiceRecordRepository) Create(ctx context.Context, s *domain.ServiceRecord) error {
	ts := now()
	id, err := r.insert(ctx, "service record create", serviceRecordKeys,
		`INSERT INTO service_records (employee_id, grade_id, position, department, appointment_type,
			effective_from, effective_to, remarks, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.EmployeeID, s.GradeID, s.Position, s.Department, s.AppointmentType,
		s.EffectiveFrom, s.EffectiveTo, s.Remarks, ts, ts)
	if err != nil {
		return err
	}
	s.ID, s.CreatedAt, s.UpdatedAt = id, ts, ts
	return nil
}

func (r *ServiceRecordRepository) GetByID(ctx context.Context, id int64) (*domain.ServiceRecord, error) {
	var s domain.ServiceRecord
	if err := r.get(ctx, "service record get", &s,
		"SELECT "+serviceRecordColumns+" FROM service_records WHERE id = ?", id); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *ServiceRecordRepository) ListByEmployee(ctx context.Context, employeeID int64) ([]domain.ServiceRecord, error) {
	var records []domain.ServiceRecord
	err := r.selectAll(ctx, "service record list", &records,
		"SELECT "+serviceRecordColumns+" FROM service_records WHERE employee_id = ? ORDER BY effective_from DESC, id DESC",
		employeeID)
	return records, err
}

func (r *ServiceRecordRepository) GetCurrentByEmployee(ctx context.Context, employeeID int64) (*domain.ServiceRecord, error) {
	var s domain.ServiceRecord
	if err := r.get(ctx, "service record get current", &s,
		"SELECT "+serviceRecordColumns+" FROM service_records WHERE employee_id = ? AND effective_to IS NULL",
		employeeID); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *ServiceRecordRepository) CloseCurrent(ctx context.Context, employeeID int64, endDate time.Time) error {
	_, err := r.exec(ctx, "service record close current", nil,
		"UPDATE service_records SET effective_to = ?, updated_at = ? WHERE employee_id = ? AND effective_to IS NULL",
		endDate, now(), employeeID)
	return err
}

func (r *ServiceRecordRepository) Update(ctx context.Context, s *domain.ServiceRecord) error {
	ts := now()
	err := r.execOne(ctx, "service record update", serviceRecordKeys,
		`UPDATE service_records SET grade_id = ?, position = ?, department = ?, appointment_type = ?,
			effective_from = ?, effective_to = ?, remarks = ?, updated_at = ?
		 WHERE id = ?`,
		s.GradeID, s.Position, s.Department, s.AppointmentType,
		s.EffectiveFrom, s.EffectiveTo, s.Remarks, ts, s.ID)
	if err != nil {
		return err
	}
	s.UpdatedAt = ts
	return nil
}

func (r *ServiceRecordRepository) Delete(ctx context.Context, id int64) error {
	return r.execOne(ctx, "service record delete", nil, "DELETE FROM service_records WHERE id = ?", id)
}
