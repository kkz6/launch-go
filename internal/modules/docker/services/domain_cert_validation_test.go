package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kkz6/launch-go/internal/modules/docker/models"
	"github.com/kkz6/launch-go/internal/pkg/certificatecheck"
	"github.com/kkz6/launch-go/internal/pkg/i18n"
)

// strptr is a tiny helper for the pointer-y request fields.
func strptr(s string) *string { return &s }

// TestValidateStoredCert pins the cross-field rule that replaced the
// broken `len=26` struct tag. The bug: the UI sends
// stored_certificate_id="" for letsencrypt domains, and validator's
// omitempty doesn't skip a non-nil empty-string pointer, so `len=26`
// 422'd every such request (e.g. just editing the container port).
func TestValidateStoredCert(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		storedID *string
		wantErr  bool
	}{
		{"letsencrypt + empty string (the reported bug)", "letsencrypt", strptr(""), false},
		{"letsencrypt + nil", "letsencrypt", nil, false},
		{"empty provider (update not touching it) + empty id", "", strptr(""), false},
		{"letsencrypt ignores a junk id", "letsencrypt", strptr("nope"), false},
		{"stored + valid 26-char ULID", "stored", strptr("01HJXVHGRGTQRX4P0G3Y8R6CK7"), false},
		{"stored + nil id is rejected", "stored", nil, true},
		{"stored + empty id is rejected", "stored", strptr(""), true},
		{"stored + short id is rejected", "stored", strptr("abc"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateStoredCert(tc.provider, tc.storedID)
			if tc.wantErr && err == nil {
				t.Fatalf("expected a validation error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

type fakeDomainCertificateChecker struct {
	result certificatecheck.Result
	hosts  []string
}

func (f *fakeDomainCertificateChecker) Check(_ context.Context, host string) certificatecheck.Result {
	f.hosts = append(f.hosts, host)
	return f.result
}

func TestCheckDomainCertificateReflectsHTTPSConfiguration(t *testing.T) {
	checker := &fakeDomainCertificateChecker{result: certificatecheck.Result{
		Host:   "app.example.com",
		Status: certificatecheck.StatusValid,
		Valid:  true,
	}}
	service := &DomainService{certificateChecker: checker}

	result := service.checkDomainCertificate(context.Background(), &models.ApplicationDomain{
		Host:  "app.example.com",
		HTTPS: true,
	})
	assert.True(t, result.Valid)
	assert.Equal(t, []string{"app.example.com"}, checker.hosts)

	result = service.checkDomainCertificate(context.Background(), &models.ApplicationDomain{
		Host:  "plain.example.com",
		HTTPS: false,
	})
	assert.Equal(t, certificatecheck.StatusNotIssued, result.Status)
	assert.Contains(t, result.Message, "disabled")
	assert.Equal(t, []string{"app.example.com"}, checker.hosts)
}

func TestCheckDomainCertificateLocalizesJapaneseMessages(t *testing.T) {
	ctx := i18n.WithLocale(context.Background(), i18n.LocaleJapanese)
	checker := &fakeDomainCertificateChecker{result: certificatecheck.Result{
		Host:    "app.example.com",
		Status:  certificatecheck.StatusValid,
		Reason:  certificatecheck.ReasonValid,
		Valid:   true,
		Message: "A valid certificate is being served for app.example.com.",
	}}
	service := &DomainService{certificateChecker: checker}

	result := service.checkDomainCertificate(ctx, &models.ApplicationDomain{
		Host: "app.example.com", HTTPS: true,
	})
	assert.Equal(t, certificatecheck.ReasonValid, result.Reason)
	assert.Contains(t, result.Message, "有効な証明書")

	result = service.checkDomainCertificate(ctx, &models.ApplicationDomain{
		Host: "plain.example.com", HTTPS: false,
	})
	assert.Equal(t, certificatecheck.ReasonHTTPSDisabled, result.Reason)
	assert.Equal(t, "このドメインではHTTPSが無効です。", result.Message)
}
