package handler

import (
	"log/slog"
	"net/http"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
	"github.com/fauzanmzlm/hrms-go-backend/pkg/response"
)

type GradeHandler struct {
	grades domain.GradeUsecase
	log    *slog.Logger
}

func NewGradeHandler(grades domain.GradeUsecase, log *slog.Logger) *GradeHandler {
	return &GradeHandler{grades: grades, log: log}
}

type gradeRequest struct {
	Code   string `json:"code"`
	Scheme string `json:"scheme"`
	Level  int    `json:"level"`
	Title  string `json:"title"`
}

func (req gradeRequest) toDomain() *domain.Grade {
	return &domain.Grade{Code: req.Code, Scheme: req.Scheme, Level: req.Level, Title: req.Title}
}

func (h *GradeHandler) List(w http.ResponseWriter, r *http.Request) {
	grades, err := h.grades.List(r.Context())
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.Data(w, http.StatusOK, grades)
}

func (h *GradeHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	g, err := h.grades.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.Data(w, http.StatusOK, g)
}

func (h *GradeHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req gradeRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	g := req.toDomain()
	if err := h.grades.Create(r.Context(), g); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.Data(w, http.StatusCreated, g)
}

func (h *GradeHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	var req gradeRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	g := req.toDomain()
	g.ID = id
	if err := h.grades.Update(r.Context(), g); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.Data(w, http.StatusOK, g)
}

func (h *GradeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	if err := h.grades.Delete(r.Context(), id); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.NoContent(w)
}
