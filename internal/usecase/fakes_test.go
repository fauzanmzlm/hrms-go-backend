package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
)

// store is an in-memory database shared by the fake repositories. Values are
// copied in and out so tests observe only what was persisted.
type store struct {
	mu           sync.Mutex
	nextID       int64
	grades       map[int64]domain.Grade
	employees    map[int64]domain.Employee
	records      map[int64]domain.ServiceRecord
	promotions   map[int64]domain.PromotionApplication
	disciplinary map[int64]domain.DisciplinaryRecord
	users        map[int64]domain.User
}

func newStore() *store {
	return &store{
		grades:       map[int64]domain.Grade{},
		employees:    map[int64]domain.Employee{},
		records:      map[int64]domain.ServiceRecord{},
		promotions:   map[int64]domain.PromotionApplication{},
		disciplinary: map[int64]domain.DisciplinaryRecord{},
		users:        map[int64]domain.User{},
	}
}

func (s *store) id() int64 {
	s.nextID++
	return s.nextID
}

func get[T any](s *store, m map[int64]T, id int64) (*T, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := m[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return &v, nil
}

func update[T any](s *store, m map[int64]T, id int64, v T) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := m[id]; !ok {
		return domain.ErrNotFound
	}
	m[id] = v
	return nil
}

func remove[T any](s *store, m map[int64]T, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := m[id]; !ok {
		return domain.ErrNotFound
	}
	delete(m, id)
	return nil
}

type passthroughTx struct{}

func (passthroughTx) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type gradeRepo struct{ s *store }

func (r gradeRepo) Create(_ context.Context, g *domain.Grade) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, x := range r.s.grades {
		if x.Code == g.Code {
			return domain.ErrConflict
		}
	}
	g.ID = r.s.id()
	r.s.grades[g.ID] = *g
	return nil
}
func (r gradeRepo) GetByID(_ context.Context, id int64) (*domain.Grade, error) {
	return get(r.s, r.s.grades, id)
}
func (r gradeRepo) GetByCode(_ context.Context, code string) (*domain.Grade, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, g := range r.s.grades {
		if g.Code == code {
			return &g, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (r gradeRepo) List(context.Context) ([]domain.Grade, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []domain.Grade
	for _, g := range r.s.grades {
		out = append(out, g)
	}
	return out, nil
}
func (r gradeRepo) Update(_ context.Context, g *domain.Grade) error {
	return update(r.s, r.s.grades, g.ID, *g)
}
func (r gradeRepo) Delete(_ context.Context, id int64) error { return remove(r.s, r.s.grades, id) }

type employeeRepo struct{ s *store }

func (r employeeRepo) Create(_ context.Context, e *domain.Employee) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	e.ID = r.s.id()
	r.s.employees[e.ID] = *e
	return nil
}
func (r employeeRepo) GetByID(_ context.Context, id int64) (*domain.Employee, error) {
	return get(r.s, r.s.employees, id)
}
func (r employeeRepo) find(match func(domain.Employee) bool) (*domain.Employee, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, e := range r.s.employees {
		if match(e) {
			return &e, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (r employeeRepo) GetByEmployeeNo(_ context.Context, no string) (*domain.Employee, error) {
	return r.find(func(e domain.Employee) bool { return e.EmployeeNo == no })
}
func (r employeeRepo) GetByICNumber(_ context.Context, ic string) (*domain.Employee, error) {
	return r.find(func(e domain.Employee) bool { return e.ICNumber == ic })
}
func (r employeeRepo) List(context.Context, domain.EmployeeFilter) ([]domain.Employee, int64, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []domain.Employee
	for _, e := range r.s.employees {
		out = append(out, e)
	}
	return out, int64(len(out)), nil
}
func (r employeeRepo) Update(_ context.Context, e *domain.Employee) error {
	return update(r.s, r.s.employees, e.ID, *e)
}
func (r employeeRepo) UpdateGrade(_ context.Context, id, gradeID int64) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	e, ok := r.s.employees[id]
	if !ok {
		return domain.ErrNotFound
	}
	e.CurrentGradeID = gradeID
	r.s.employees[id] = e
	return nil
}
func (r employeeRepo) Delete(_ context.Context, id int64) error {
	return remove(r.s, r.s.employees, id)
}

type recordRepo struct{ s *store }

func (r recordRepo) Create(_ context.Context, rec *domain.ServiceRecord) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if rec.IsCurrent() {
		for _, x := range r.s.records {
			if x.EmployeeID == rec.EmployeeID && x.IsCurrent() {
				return domain.ErrConflict
			}
		}
	}
	rec.ID = r.s.id()
	r.s.records[rec.ID] = *rec
	return nil
}
func (r recordRepo) GetByID(_ context.Context, id int64) (*domain.ServiceRecord, error) {
	return get(r.s, r.s.records, id)
}
func (r recordRepo) ListByEmployee(_ context.Context, employeeID int64) ([]domain.ServiceRecord, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []domain.ServiceRecord
	for _, x := range r.s.records {
		if x.EmployeeID == employeeID {
			out = append(out, x)
		}
	}
	return out, nil
}
func (r recordRepo) GetCurrentByEmployee(_ context.Context, employeeID int64) (*domain.ServiceRecord, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, x := range r.s.records {
		if x.EmployeeID == employeeID && x.IsCurrent() {
			return &x, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (r recordRepo) CloseCurrent(_ context.Context, employeeID int64, end time.Time) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for id, x := range r.s.records {
		if x.EmployeeID == employeeID && x.IsCurrent() {
			x.EffectiveTo = &end
			r.s.records[id] = x
		}
	}
	return nil
}
func (r recordRepo) Update(_ context.Context, rec *domain.ServiceRecord) error {
	return update(r.s, r.s.records, rec.ID, *rec)
}
func (r recordRepo) Delete(_ context.Context, id int64) error { return remove(r.s, r.s.records, id) }

type promotionRepo struct{ s *store }

func (r promotionRepo) Create(_ context.Context, p *domain.PromotionApplication) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	p.ID = r.s.id()
	r.s.promotions[p.ID] = *p
	return nil
}
func (r promotionRepo) GetByID(_ context.Context, id int64) (*domain.PromotionApplication, error) {
	return get(r.s, r.s.promotions, id)
}
func (r promotionRepo) List(context.Context, domain.PromotionFilter) ([]domain.PromotionApplication, int64, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []domain.PromotionApplication
	for _, p := range r.s.promotions {
		out = append(out, p)
	}
	return out, int64(len(out)), nil
}
func (r promotionRepo) HasPendingByEmployee(_ context.Context, employeeID int64) (bool, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, p := range r.s.promotions {
		if p.EmployeeID == employeeID && p.Status == domain.StatusPending {
			return true, nil
		}
	}
	return false, nil
}
func (r promotionRepo) UpdateReview(_ context.Context, p *domain.PromotionApplication) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if stored, ok := r.s.promotions[p.ID]; !ok || stored.Status != domain.StatusPending {
		return domain.ErrInvalidStatusTransition
	}
	r.s.promotions[p.ID] = *p
	return nil
}

type disciplinaryRepo struct{ s *store }

func (r disciplinaryRepo) Create(_ context.Context, d *domain.DisciplinaryRecord) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	d.ID = r.s.id()
	r.s.disciplinary[d.ID] = *d
	return nil
}
func (r disciplinaryRepo) GetByID(_ context.Context, id int64) (*domain.DisciplinaryRecord, error) {
	return get(r.s, r.s.disciplinary, id)
}
func (r disciplinaryRepo) List(context.Context, domain.DisciplinaryFilter) ([]domain.DisciplinaryRecord, int64, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	var out []domain.DisciplinaryRecord
	for _, d := range r.s.disciplinary {
		out = append(out, d)
	}
	return out, int64(len(out)), nil
}
func (r disciplinaryRepo) UpdateReview(_ context.Context, d *domain.DisciplinaryRecord) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	if stored, ok := r.s.disciplinary[d.ID]; !ok || stored.Status != domain.StatusPending {
		return domain.ErrInvalidStatusTransition
	}
	r.s.disciplinary[d.ID] = *d
	return nil
}
func (r disciplinaryRepo) HasApprovedSince(_ context.Context, employeeID int64, since time.Time) (bool, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, d := range r.s.disciplinary {
		if d.EmployeeID == employeeID && d.Status == domain.StatusApproved && !d.ReviewedAt.Before(since) {
			return true, nil
		}
	}
	return false, nil
}

type userRepo struct{ s *store }

func (r userRepo) Create(_ context.Context, u *domain.User) error {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, x := range r.s.users {
		if x.Username == u.Username {
			return domain.ErrConflict
		}
	}
	u.ID = r.s.id()
	r.s.users[u.ID] = *u
	return nil
}
func (r userRepo) GetByID(_ context.Context, id int64) (*domain.User, error) {
	return get(r.s, r.s.users, id)
}
func (r userRepo) GetByUsername(_ context.Context, username string) (*domain.User, error) {
	r.s.mu.Lock()
	defer r.s.mu.Unlock()
	for _, u := range r.s.users {
		if u.Username == username {
			return &u, nil
		}
	}
	return nil, domain.ErrNotFound
}

type fakeHasher struct{}

func (fakeHasher) Hash(p string) (string, error) { return "hash:" + p, nil }
func (fakeHasher) Compare(h, p string) error {
	if h != "hash:"+p {
		return errors.New("mismatch")
	}
	return nil
}

type fakeTokens struct{}

func (fakeTokens) Issue(userID int64, role string, _ *int64) (string, time.Time, error) {
	return fmt.Sprintf("token-%d-%s", userID, strings.ToLower(role)), time.Unix(0, 0), nil
}

var (
	_ domain.GradeRepository         = gradeRepo{}
	_ domain.EmployeeRepository      = employeeRepo{}
	_ domain.ServiceRecordRepository = recordRepo{}
	_ domain.PromotionRepository     = promotionRepo{}
	_ domain.DisciplinaryRepository  = disciplinaryRepo{}
	_ domain.UserRepository          = userRepo{}
)
