package domain

import (
	"fmt"
	"strings"
)

// ApprovalStatus is the workflow state shared by promotion applications and
// disciplinary records.
type ApprovalStatus string

const (
	StatusPending  ApprovalStatus = "PENDING"
	StatusApproved ApprovalStatus = "APPROVED"
	StatusRejected ApprovalStatus = "REJECTED"
)

func (s ApprovalStatus) IsValid() bool {
	switch s {
	case StatusPending, StatusApproved, StatusRejected:
		return true
	}
	return false
}

// IsFinal reports whether no further transitions are possible.
func (s ApprovalStatus) IsFinal() bool {
	return s == StatusApproved || s == StatusRejected
}

// CanTransitionTo reports whether moving from s to next is allowed.
// Only PENDING -> APPROVED and PENDING -> REJECTED are permitted.
func (s ApprovalStatus) CanTransitionTo(next ApprovalStatus) bool {
	return s == StatusPending && next.IsFinal()
}

// ReviewDecision is a reviewer's verdict on a pending workflow item.
type ReviewDecision struct {
	ReviewerID int64          `json:"reviewer_id"`
	Status     ApprovalStatus `json:"status"`
	Remarks    string         `json:"remarks"`
}

func (d ReviewDecision) Validate() error {
	v := reviewValidation(d.ReviewerID, d.Status, d.Remarks)
	if !d.Status.IsFinal() {
		v.Add("status", "must be APPROVED or REJECTED")
	}
	return v.Err()
}

func checkTransition(entity string, from, to ApprovalStatus) error {
	if !from.CanTransitionTo(to) {
		return fmt.Errorf("%w: %s is %s and cannot be %s", ErrInvalidStatusTransition, entity, from, to)
	}
	return nil
}

func reviewValidation(reviewerID int64, next ApprovalStatus, remarks string) *ValidationError {
	v := NewValidationError()
	if reviewerID <= 0 {
		v.Add("reviewer_id", "is required")
	}
	if next == StatusRejected && strings.TrimSpace(remarks) == "" {
		v.Add("remarks", "is required when rejecting")
	}
	return v
}
