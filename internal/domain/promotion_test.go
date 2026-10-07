package domain

import (
	"errors"
	"testing"
	"time"
)

func pendingPromotion() *PromotionApplication {
	return &PromotionApplication{
		ID:              1,
		EmployeeID:      10,
		CurrentGradeID:  41,
		ProposedGradeID: 44,
		Justification:   "Excellent performance for 3 consecutive years",
		Status:          StatusPending,
	}
}

func TestPromotionApprove(t *testing.T) {
	now := time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)
	effective := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

	p := pendingPromotion()
	if err := p.Approve(99, " Recommended ", effective, now); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if p.Status != StatusApproved {
		t.Errorf("Status = %s, want APPROVED", p.Status)
	}
	if p.ReviewedBy == nil || *p.ReviewedBy != 99 {
		t.Errorf("ReviewedBy = %v, want 99", p.ReviewedBy)
	}
	if p.ReviewedAt == nil || !p.ReviewedAt.Equal(now) {
		t.Errorf("ReviewedAt = %v, want %v", p.ReviewedAt, now)
	}
	if p.EffectiveDate == nil || !p.EffectiveDate.Equal(effective) {
		t.Errorf("EffectiveDate = %v, want %v", p.EffectiveDate, effective)
	}
	if p.ReviewRemarks != "Recommended" {
		t.Errorf("ReviewRemarks = %q, want trimmed", p.ReviewRemarks)
	}
}

func TestPromotionReject(t *testing.T) {
	now := time.Now()
	p := pendingPromotion()
	if err := p.Reject(99, "Does not meet minimum service period", now); err != nil {
		t.Fatalf("Reject: %v", err)
	}
	if p.Status != StatusRejected {
		t.Errorf("Status = %s, want REJECTED", p.Status)
	}
	if p.EffectiveDate != nil {
		t.Error("EffectiveDate should stay nil on rejection")
	}
}

func TestPromotionReviewErrors(t *testing.T) {
	now := time.Now()
	effective := now.AddDate(0, 1, 0)

	tests := []struct {
		name    string
		status  ApprovalStatus
		review  func(p *PromotionApplication) error
		wantErr error
	}{
		{
			name:    "approve already approved",
			status:  StatusApproved,
			review:  func(p *PromotionApplication) error { return p.Approve(99, "", effective, now) },
			wantErr: ErrInvalidStatusTransition,
		},
		{
			name:    "reject already approved",
			status:  StatusApproved,
			review:  func(p *PromotionApplication) error { return p.Reject(99, "late", now) },
			wantErr: ErrInvalidStatusTransition,
		},
		{
			name:    "approve already rejected",
			status:  StatusRejected,
			review:  func(p *PromotionApplication) error { return p.Approve(99, "", effective, now) },
			wantErr: ErrInvalidStatusTransition,
		},
		{
			name:    "approve without effective date",
			status:  StatusPending,
			review:  func(p *PromotionApplication) error { return p.Approve(99, "", time.Time{}, now) },
			wantErr: ErrInvalidInput,
		},
		{
			name:    "approve without reviewer",
			status:  StatusPending,
			review:  func(p *PromotionApplication) error { return p.Approve(0, "", effective, now) },
			wantErr: ErrInvalidInput,
		},
		{
			name:    "reject without remarks",
			status:  StatusPending,
			review:  func(p *PromotionApplication) error { return p.Reject(99, "", now) },
			wantErr: ErrInvalidInput,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := pendingPromotion()
			p.Status = tt.status
			before := *p

			err := tt.review(p)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if p.Status != before.Status || p.ReviewedBy != nil || p.ReviewedAt != nil || p.EffectiveDate != nil {
				t.Error("application must not change when review fails")
			}
		})
	}
}

func TestPromotionValidate(t *testing.T) {
	p := pendingPromotion()
	if err := p.Validate(); err != nil {
		t.Fatalf("valid application: %v", err)
	}

	p.ProposedGradeID = p.CurrentGradeID
	p.Justification = ""
	var ve *ValidationError
	if !errors.As(p.Validate(), &ve) {
		t.Fatal("want *ValidationError")
	}
	for _, field := range []string{"proposed_grade_id", "justification"} {
		if _, ok := ve.Fields[field]; !ok {
			t.Errorf("want error on %q, got %v", field, ve.Fields)
		}
	}
}

func TestDisciplinaryPenaltyAllowedFor(t *testing.T) {
	d := &DisciplinaryRecord{
		EmployeeID:   10,
		CaseNo:       "TT/2026/001",
		Category:     OffenseMinor,
		Description:  "Absent without leave",
		IncidentDate: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Penalty:      PenaltyDismissal,
		Status:       StatusPending,
		ReportedBy:   5,
	}
	var ve *ValidationError
	if !errors.As(d.Validate(), &ve) || ve.Fields["penalty"] == "" {
		t.Fatal("dismissal must be rejected for a MINOR case")
	}

	d.Category = OffenseSerious
	if err := d.Validate(); err != nil {
		t.Fatalf("dismissal for a SERIOUS case: %v", err)
	}
}

func TestIsValidICNumber(t *testing.T) {
	tests := map[string]bool{
		"900101145678":   true,
		"001231011234":   true,
		"900101-14-5678": false,
		"90010114567":    false,
		"901301145678":   false,
		"9001011456ab":   false,
	}
	for ic, want := range tests {
		if got := IsValidICNumber(ic); got != want {
			t.Errorf("IsValidICNumber(%q) = %v, want %v", ic, got, want)
		}
	}
	if got := NormalizeICNumber("900101-14-5678"); got != "900101145678" {
		t.Errorf("NormalizeICNumber = %q", got)
	}
}
