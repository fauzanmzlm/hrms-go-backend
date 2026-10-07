package handler

import (
	"log/slog"
	"net/http"

	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
	"github.com/fauzanmzlm/hrms-go-backend/pkg/response"
)

type AuthHandler struct {
	auth domain.AuthUsecase
	log  *slog.Logger
}

func NewAuthHandler(auth domain.AuthUsecase, log *slog.Logger) *AuthHandler {
	return &AuthHandler{auth: auth, log: log}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	res, err := h.auth.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.Data(w, http.StatusOK, res)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	u, err := h.auth.GetUser(r.Context(), identity(r).UserID)
	if err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.Data(w, http.StatusOK, u)
}

type createUserRequest struct {
	Username   string      `json:"username"`
	Password   string      `json:"password"`
	Role       domain.Role `json:"role"`
	EmployeeID *int64      `json:"employee_id"`
}

func (h *AuthHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	u := &domain.User{Username: req.Username, Role: req.Role, EmployeeID: req.EmployeeID}
	if err := h.auth.CreateUser(r.Context(), u, req.Password); err != nil {
		writeError(w, r, h.log, err)
		return
	}
	response.Data(w, http.StatusCreated, u)
}
