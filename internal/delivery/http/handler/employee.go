package handler

import (
	"log/slog"
	"net/http"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
	"github.com/fauzanmzlm/hrms-go-backend/pkg/response"
)

type EmployeeHandler struct {
	employees domain.EmployeeUsecase
	log       *slog.Logger
}

func NewEmployeeHandler(employees domain.EmployeeUsecase, log *slog.Logger) *EmployeeHandler {
	return &EmployeeHandler{employees: employees, log: log}
}

type employeeRequest struct {
	EmployeeNo     string               `json:"employee_no"`
	ICNumber       string               `json:"ic_number"`
	FullName       string               `json:"full_name"`
	Email          string               `json:"email"`
	Phone          string               `json:"phone"`
	DateOfBirth    Date                 `json:"date_of_birth"`
	Gender         domain.Gender        `json:"gender"`
	CurrentGradeID int64                `json:"current_grade_id"`
	Position       string               `json:"position"`
	Department     string               `json:"department"`
	ServiceStatus  domain.ServiceStatus `json:"service_status"`
	DateJoined     Date                 `json:"date_joined"`
	ConfirmedAt    *Date                `json:"confirmed_at"`
}

func (req employeeRequest) toDomain() *domain.Employee {
	return &domain.Employee{
		EmployeeNo:     req.EmployeeNo,
		ICNumber:       req.ICNumber,
		FullName:       req.FullName,
		Email:          req.Email,
		Phone:          req.Phone,
		DateOfBirth:    req.DateOfBirth.Time,
		Gender:         req.Gender,
		CurrentGradeID: req.CurrentGradeID,
		Position:       req.Position,
		Department:     req.Department,
		ServiceStatus:  req.ServiceStatus,
		DateJoined:     req.DateJoined.Time,
		ConfirmedAt:    req.ConfirmedAt.ptr(),
	}
}

func (h *EmployeeHandler) List(w http.ResponseWriter, r *http.Request) {
	p, err := pagination(r)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	gradeID, err := queryInt64(r, "grade_id")
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	q := r.URL.Query()
	status := domain.ServiceStatus(q.Get("status"))
	if status != "" && !status.IsValid() {
		writeError(w, r, h.log, badRequest("query parameter \"status\" is not a valid service status"))
		return
	}

	page, err := h.employees.List(r.Context(), domain.EmployeeFilter{
		Search:     q.Get("search"),
		Department: q.Get("department"),
		GradeID:    gradeID,
		Status:     status,
		Pagination: p,
	})
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.Data(w, http.StatusOK, page)
}

func (h *EmployeeHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	if err := ensureCanView(identity(r), id); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	e, err := h.employees.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.Data(w, http.StatusOK, e)
}

func (h *EmployeeHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req employeeRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	e := req.toDomain()
	if err := h.employees.Create(r.Context(), e); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.Data(w, http.StatusCreated, e)
}

func (h *EmployeeHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	var req employeeRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	e := req.toDomain()
	e.ID = id
	if err := h.employees.Update(r.Context(), e); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.Data(w, http.StatusOK, e)
}

func (h *EmployeeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	if err := h.employees.Delete(r.Context(), id); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.NoContent(w)
}
