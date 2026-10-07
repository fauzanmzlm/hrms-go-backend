package domain

import (
	"errors"
	"testing"
)

func TestApprovalStatusCanTransitionTo(t *testing.T) {
	tests := []struct {
		from, to ApprovalStatus
		want     bool
	}{
		{StatusPending, StatusApproved, true},
		{StatusPending, StatusRejected, true},
		{StatusPending, StatusPending, false},
		{StatusApproved, StatusRejected, false},
		{StatusApproved, StatusPending, false},
		{StatusRejected, StatusApproved, false},
		{StatusRejected, StatusPending, false},
		{StatusPending, ApprovalStatus("UNKNOWN"), false},
		{ApprovalStatus(""), StatusApproved, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			if got := tt.from.CanTransitionTo(tt.to); got != tt.want {
				t.Errorf("CanTransitionTo = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApprovalStatusIsValid(t *testing.T) {
	for _, s := range []ApprovalStatus{StatusPending, StatusApproved, StatusRejected} {
		if !s.IsValid() {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range []ApprovalStatus{"", "pending", "CANCELLED"} {
		if s.IsValid() {
			t.Errorf("%q should be invalid", s)
		}
	}
}

func TestReviewDecisionValidate(t *testing.T) {
	tests := []struct {
		name      string
		d         ReviewDecision
		wantField string
	}{
		{"approve without remarks", ReviewDecision{ReviewerID: 1, Status: StatusApproved}, ""},
		{"reject with remarks", ReviewDecision{ReviewerID: 1, Status: StatusRejected, Remarks: "incomplete"}, ""},
		{"reject without remarks", ReviewDecision{ReviewerID: 1, Status: StatusRejected, Remarks: "  "}, "remarks"},
		{"missing reviewer", ReviewDecision{Status: StatusApproved}, "reviewer_id"},
		{"pending status", ReviewDecision{ReviewerID: 1, Status: StatusPending}, "status"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.d.Validate()
			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("want *ValidationError, got %v", err)
			}
			if _, ok := ve.Fields[tt.wantField]; !ok {
				t.Errorf("want error on field %q, got %v", tt.wantField, ve.Fields)
			}
			if !errors.Is(err, ErrInvalidInput) {
				t.Error("ValidationError should match ErrInvalidInput")
			}
		})
	}
}
