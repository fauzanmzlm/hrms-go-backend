package handler

import (
	"log/slog"
	"net/http"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
	"github.com/fauzanmzlm/hrms-go-backend/pkg/response"
)

type DisciplinaryHandler struct {
	records domain.DisciplinaryUsecase
	log     *slog.Logger
}

func NewDisciplinaryHandler(records domain.DisciplinaryUsecase, log *slog.Logger) *DisciplinaryHandler {
	return &DisciplinaryHandler{records: records, log: log}
}

type disciplinaryRequest struct {
	EmployeeID   int64                  `json:"employee_id"`
	CaseNo       string                 `json:"case_no"`
	Category     domain.OffenseCategory `json:"category"`
	Description  string                 `json:"description"`
	IncidentDate Date                   `json:"incident_date"`
	Penalty      domain.PenaltyType     `json:"penalty"`
}

// Create opens a case reported by the caller.
func (h *DisciplinaryHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req disciplinaryRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	d := &domain.DisciplinaryRecord{
		EmployeeID:   req.EmployeeID,
		CaseNo:       req.CaseNo,
		Category:     req.Category,
		Description:  req.Description,
		IncidentDate: req.IncidentDate.Time,
		Penalty:      req.Penalty,
		ReportedBy:   identity(r).UserID,
	}
	if err := h.records.Create(r.Context(), d); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.Data(w, http.StatusCreated, d)
}

func (h *DisciplinaryHandler) List(w http.ResponseWriter, r *http.Request) {
	p, err := pagination(r)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	status, err := queryStatus(r)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	employeeID, err := queryInt64(r, "employee_id")
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	category := domain.OffenseCategory(r.URL.Query().Get("category"))
	if category != "" && !category.IsValid() {
		writeError(w, r, h.log, badRequest("query parameter \"category\" must be MINOR or SERIOUS"))
		return
	}
	own, restricted, err := ownEmployeeID(identity(r))
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	if restricted {
		employeeID = own
	}

	page, err := h.records.List(r.Context(), domain.DisciplinaryFilter{
		EmployeeID: employeeID, Status: status, Category: category, Pagination: p,
	})
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.Data(w, http.StatusOK, page)
}

func (h *DisciplinaryHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	d, err := h.records.Get(r.Context(), id)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	if err := ensureCanView(identity(r), d.EmployeeID); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.Data(w, http.StatusOK, d)
}

func (h *DisciplinaryHandler) Review(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	var req reviewRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	if req.EffectiveDate != nil {
		writeError(w, r, h.log, badRequest("effective_date does not apply to disciplinary reviews"))
		return
	}

	d, err := h.records.Review(r.Context(), id, domain.ReviewDecision{
		ReviewerID: identity(r).UserID,
		Status:     req.Status,
		Remarks:    req.Remarks,
	})
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.Data(w, http.StatusOK, d)
}
