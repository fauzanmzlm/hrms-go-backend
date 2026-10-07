package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	httpdelivery "github.com/fauzanmzlm/hrms-go-backend/internal/delivery/http"
	"github.com/fauzanmzlm/hrms-go-backend/internal/delivery/http/handler"
	"github.com/fauzanmzlm/hrms-go-backend/internal/domain"
	"github.com/fauzanmzlm/hrms-go-backend/pkg/jwt"
)

const testSecret = "test-secret-that-is-at-least-32-bytes-long"

type stubGrades struct {
	domain.GradeUsecase
	getErr error
}

func (s stubGrades) GetByID(context.Context, int64) (*domain.Grade, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return &domain.Grade{ID: 1, Code: "N41"}, nil
}

type stubPromotions struct {
	domain.PromotionUsecase
	submitted *domain.SubmitPromotionInput
}

func (s *stubPromotions) Submit(_ context.Context, in domain.SubmitPromotionInput) (*domain.PromotionApplication, error) {
	s.submitted = &in
	return &domain.PromotionApplication{ID: 7, EmployeeID: in.EmployeeID, Status: domain.StatusPending}, nil
}

type stubPinger struct{ err error }

func (p stubPinger) PingContext(context.Context) error { return p.err }

type testServer struct {
	t          *testing.T
	handler    http.Handler
	tokens     *jwt.Manager
	promotions *stubPromotions
}

func newTestServer(t *testing.T, grades domain.GradeUsecase) *testServer {
	t.Helper()
	tokens, err := jwt.NewManager(testSecret, "hrms-test", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	promotions := &stubPromotions{}
	h := httpdelivery.NewRouter(httpdelivery.RouterDeps{
		Logger: log,
		Tokens: tokens,
		DB:     stubPinger{},
		Handlers: httpdelivery.Handlers{
			Auth:          handler.NewAuthHandler(nil, log),
			Grade:         handler.NewGradeHandler(grades, log),
			Employee:      handler.NewEmployeeHandler(nil, log),
			ServiceRecord: handler.NewServiceRecordHandler(nil, log),
			Promotion:     handler.NewPromotionHandler(promotions, log),
			Disciplinary:  handler.NewDisciplinaryHandler(nil, log),
		},
	})
	return &testServer{t: t, handler: h, tokens: tokens, promotions: promotions}
}

func (s *testServer) token(userID int64, role domain.Role, employeeID *int64) string {
	s.t.Helper()
	tok, _, err := s.tokens.Issue(userID, string(role), employeeID)
	if err != nil {
		s.t.Fatal(err)
	}
	return tok
}

type errorResponse struct {
	Error struct {
		Code      string            `json:"code"`
		Message   string            `json:"message"`
		Fields    map[string]string `json:"fields"`
		RequestID string            `json:"request_id"`
	} `json:"error"`
}

func (s *testServer) do(method, path, token, body string) (*httptest.ResponseRecorder, errorResponse) {
	s.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)

	var er errorResponse
	if rec.Code >= 400 {
		if err := json.Unmarshal(rec.Body.Bytes(), &er); err != nil {
			s.t.Fatalf("error body is not JSON: %q", rec.Body.String())
		}
	}
	return rec, er
}

func TestAuthenticationAndRoles(t *testing.T) {
	s := newTestServer(t, stubGrades{})
	empID := int64(10)

	tests := []struct {
		name, method, path, token, body string
		wantStatus                      int
		wantCode                        string
	}{
		{"missing token", "GET", "/api/v1/grades/1", "", "", 401, "UNAUTHORIZED"},
		{"garbage token", "GET", "/api/v1/grades/1", "not-a-jwt", "", 401, "UNAUTHORIZED"},
		{"employee reads grade", "GET", "/api/v1/grades/1", s.token(1, domain.RoleEmployee, &empID), "", 200, ""},
		{"employee creates grade", "POST", "/api/v1/grades", s.token(1, domain.RoleEmployee, &empID), `{}`, 403, "FORBIDDEN"},
		{"employee lists employees", "GET", "/api/v1/employees", s.token(1, domain.RoleEmployee, &empID), "", 403, "FORBIDDEN"},
		{"employee reads other employee", "GET", "/api/v1/employees/99", s.token(1, domain.RoleEmployee, &empID), "", 403, "FORBIDDEN"},
		{"approver submits promotion", "POST", "/api/v1/promotions", s.token(2, domain.RoleApprover, nil), `{}`, 403, "FORBIDDEN"},
		{"unknown route", "GET", "/api/v1/nope", s.token(1, domain.RoleAdmin, nil), "", 404, "NOT_FOUND"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, er := s.do(tt.method, tt.path, tt.token, tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if er.Error.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", er.Error.Code, tt.wantCode)
			}
			if tt.wantStatus >= 400 && er.Error.RequestID == "" {
				t.Error("error response should carry a request_id")
			}
		})
	}
}

func TestExpiredTokenRejected(t *testing.T) {
	s := newTestServer(t, stubGrades{})
	short, _ := jwt.NewManager(testSecret, "hrms-test", time.Nanosecond)
	tok, _, _ := short.Issue(1, string(domain.RoleAdmin), nil)
	time.Sleep(time.Millisecond)
	if rec, _ := s.do("GET", "/api/v1/grades/1", tok, ""); rec.Code != 401 {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestEmployeeSubmitsOwnPromotion(t *testing.T) {
	s := newTestServer(t, stubGrades{})
	empID := int64(10)
	tok := s.token(5, domain.RoleEmployee, &empID)

	rec, _ := s.do("POST", "/api/v1/promotions", tok, `{"proposed_grade_id": 44, "justification": "x"}`)
	if rec.Code != 201 {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if s.promotions.submitted == nil || s.promotions.submitted.EmployeeID != empID {
		t.Errorf("submitted = %+v, want employee %d", s.promotions.submitted, empID)
	}

	rec, er := s.do("POST", "/api/v1/promotions", tok, `{"employee_id": 99, "proposed_grade_id": 44, "justification": "x"}`)
	if rec.Code != 403 || er.Error.Code != "FORBIDDEN" {
		t.Errorf("applying for someone else: status %d code %q", rec.Code, er.Error.Code)
	}
}

func TestErrorMapping(t *testing.T) {
	validation := domain.NewValidationError()
	validation.Add("code", "is required")

	tests := []struct {
		name        string
		err         error
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{"validation", validation, 422, "VALIDATION_FAILED", "validation failed"},
		{"not found", domain.ErrNotFound, 404, "NOT_FOUND", "resource not found"},
		{"conflict", domain.ErrConflict, 409, "CONFLICT", "resource already exists"},
		{"status transition", domain.ErrInvalidStatusTransition, 409, "INVALID_STATUS_TRANSITION", "invalid status transition"},
		{"rule violation", domain.ErrRuleViolation, 422, "RULE_VIOLATION", "business rule violation"},
		{"forbidden", domain.ErrForbidden, 403, "FORBIDDEN", "forbidden"},
		{"internal", errors.New("dial tcp 10.0.0.5:3306: connection refused"), 500, "INTERNAL_ERROR", "internal server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestServer(t, stubGrades{getErr: tt.err})
			rec, er := s.do("GET", "/api/v1/grades/1", s.token(1, domain.RoleAdmin, nil), "")
			if rec.Code != tt.wantStatus || er.Error.Code != tt.wantCode || er.Error.Message != tt.wantMessage {
				t.Errorf("got %d %q %q, want %d %q %q",
					rec.Code, er.Error.Code, er.Error.Message, tt.wantStatus, tt.wantCode, tt.wantMessage)
			}
		})
	}

	s := newTestServer(t, stubGrades{getErr: validation})
	_, er := s.do("GET", "/api/v1/grades/1", s.token(1, domain.RoleAdmin, nil), "")
	if er.Error.Fields["code"] != "is required" {
		t.Errorf("fields = %v", er.Error.Fields)
	}
}

func TestBadRequests(t *testing.T) {
	s := newTestServer(t, stubGrades{})
	admin := s.token(1, domain.RoleAdmin, nil)

	for name, tc := range map[string]struct{ method, path, body string }{
		"non-numeric id": {"GET", "/api/v1/grades/abc", ""},
		"malformed json": {"POST", "/api/v1/promotions", `{"proposed_grade_id":`},
		"unknown field":  {"POST", "/api/v1/promotions", `{"salary": 1}`},
		"empty body":     {"POST", "/api/v1/promotions", ``},
	} {
		t.Run(name, func(t *testing.T) {
			rec, er := s.do(tc.method, tc.path, admin, tc.body)
			if rec.Code != 400 || er.Error.Code != "BAD_REQUEST" {
				t.Errorf("status %d code %q, want 400 BAD_REQUEST", rec.Code, er.Error.Code)
			}
		})
	}
}

func TestRequestID(t *testing.T) {
	s := newTestServer(t, stubGrades{})

	rec, er := s.do("GET", "/api/v1/grades/1", "", "")
	id := rec.Header().Get("X-Request-Id")
	if len(id) != 32 || strings.Contains(id, "/") {
		t.Errorf("generated request id = %q, want 32 hex chars without hostname", id)
	}
	if er.Error.RequestID != id {
		t.Errorf("body request_id %q != header %q", er.Error.RequestID, id)
	}

	for incoming, wantReuse := range map[string]bool{
		"trace-abc_123.x":           true,
		"bad id with spaces":        false,
		strings.Repeat("a", 65):     false,
		"<script>alert(1)</script>": false,
	} {
		req := httptest.NewRequest("GET", "/healthz", nil)
		req.Header.Set("X-Request-Id", incoming)
		rec := httptest.NewRecorder()
		s.handler.ServeHTTP(rec, req)
		if got := rec.Header().Get("X-Request-Id"); (got == incoming) != wantReuse {
			t.Errorf("incoming %q: response id %q, reuse=%v want %v", incoming, got, got == incoming, wantReuse)
		}
	}
}

func TestHealthEndpoints(t *testing.T) {
	s := newTestServer(t, stubGrades{})
	if rec, _ := s.do("GET", "/healthz", "", ""); rec.Code != 200 {
		t.Errorf("healthz = %d", rec.Code)
	}
	if rec, _ := s.do("GET", "/readyz", "", ""); rec.Code != 200 {
		t.Errorf("readyz = %d", rec.Code)
	}
}
