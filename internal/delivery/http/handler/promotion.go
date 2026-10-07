package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
	"github.com/fauzanmzlm/hrms-go-backend/pkg/response"
)

type PromotionHandler struct {
	promotions domain.PromotionUsecase
	log        *slog.Logger
}

func NewPromotionHandler(promotions domain.PromotionUsecase, log *slog.Logger) *PromotionHandler {
	return &PromotionHandler{promotions: promotions, log: log}
}

type submitPromotionRequest struct {
	EmployeeID      int64  `json:"employee_id"`
	ProposedGradeID int64  `json:"proposed_grade_id"`
	Justification   string `json:"justification"`
}

type reviewRequest struct {
	Status        domain.ApprovalStatus `json:"status"`
	Remarks       string                `json:"remarks"`
	EffectiveDate *Date                 `json:"effective_date"`
}

// Submit lets HR submit for any employee. An EMPLOYEE-role caller may only
// apply for themselves; employee_id defaults to their own record.
func (h *PromotionHandler) Submit(w http.ResponseWriter, r *http.Request) {
	var req submitPromotionRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	own, restricted, err := ownEmployeeID(identity(r))
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	if restricted {
		if req.EmployeeID != 0 && req.EmployeeID != own {
			writeError(w, r, h.log, fmt.Errorf("%w: you can only apply for your own promotion", domain.ErrForbidden))
			return
		}
		req.EmployeeID = own
	}

	app, err := h.promotions.Submit(r.Context(), domain.SubmitPromotionInput{
		EmployeeID:      req.EmployeeID,
		ProposedGradeID: req.ProposedGradeID,
		Justification:   req.Justification,
	})
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.Data(w, http.StatusCreated, app)
}

func (h *PromotionHandler) List(w http.ResponseWriter, r *http.Request) {
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
	own, restricted, err := ownEmployeeID(identity(r))
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	if restricted {
		employeeID = own
	}

	page, err := h.promotions.List(r.Context(), domain.PromotionFilter{
		EmployeeID: employeeID, Status: status, Pagination: p,
	})
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.Data(w, http.StatusOK, page)
}

func (h *PromotionHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	app, err := h.promotions.Get(r.Context(), id)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	if err := ensureCanView(identity(r), app.EmployeeID); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.Data(w, http.StatusOK, app)
}

func (h *PromotionHandler) Review(w http.ResponseWriter, r *http.Request) {
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
	var effective time.Time
	if t := req.EffectiveDate.ptr(); t != nil {
		effective = *t
	}

	app, err := h.promotions.Review(r.Context(), id, domain.ReviewDecision{
		ReviewerID: identity(r).UserID,
		Status:     req.Status,
		Remarks:    req.Remarks,
	}, effective)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.Data(w, http.StatusOK, app)
}
