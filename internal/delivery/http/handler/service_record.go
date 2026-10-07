package handler

import (
	"log/slog"
	"net/http"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
	"github.com/fauzanmzlm/hrms-go-backend/pkg/response"
)

type ServiceRecordHandler struct {
	records domain.ServiceRecordUsecase
	log     *slog.Logger
}

func NewServiceRecordHandler(records domain.ServiceRecordUsecase, log *slog.Logger) *ServiceRecordHandler {
	return &ServiceRecordHandler{records: records, log: log}
}

type serviceRecordRequest struct {
	GradeID         int64                  `json:"grade_id"`
	Position        string                 `json:"position"`
	Department      string                 `json:"department"`
	AppointmentType domain.AppointmentType `json:"appointment_type"`
	EffectiveFrom   Date                   `json:"effective_from"`
	EffectiveTo     *Date                  `json:"effective_to"`
	Remarks         string                 `json:"remarks"`
}

func (req serviceRecordRequest) toDomain() *domain.ServiceRecord {
	return &domain.ServiceRecord{
		GradeID:         req.GradeID,
		Position:        req.Position,
		Department:      req.Department,
		AppointmentType: req.AppointmentType,
		EffectiveFrom:   req.EffectiveFrom.Time,
		EffectiveTo:     req.EffectiveTo.ptr(),
		Remarks:         req.Remarks,
	}
}

// ListByEmployee serves GET /employees/{id}/service-records.
func (h *ServiceRecordHandler) ListByEmployee(w http.ResponseWriter, r *http.Request) {
	employeeID, err := pathID(r, "id")
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	if err := ensureCanView(identity(r), employeeID); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	records, err := h.records.ListByEmployee(r.Context(), employeeID)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.Data(w, http.StatusOK, records)
}

// Create serves POST /employees/{id}/service-records.
func (h *ServiceRecordHandler) Create(w http.ResponseWriter, r *http.Request) {
	employeeID, err := pathID(r, "id")
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	var req serviceRecordRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	rec := req.toDomain()
	rec.EmployeeID = employeeID
	if err := h.records.Create(r.Context(), rec); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.Data(w, http.StatusCreated, rec)
}

func (h *ServiceRecordHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	rec, err := h.records.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	if err := ensureCanView(identity(r), rec.EmployeeID); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.Data(w, http.StatusOK, rec)
}

func (h *ServiceRecordHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	var req serviceRecordRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	rec := req.toDomain()
	rec.ID = id
	if err := h.records.Update(r.Context(), rec); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.Data(w, http.StatusOK, rec)
}

func (h *ServiceRecordHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	if err := h.records.Delete(r.Context(), id); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.NoContent(w)
}
