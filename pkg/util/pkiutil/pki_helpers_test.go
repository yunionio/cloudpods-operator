package pkiutil

import (
	"crypto/x509"
	"testing"
	"time"
)

func TestNeedsRenewal(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		notAfter time.Time
		want     bool
	}{
		{
			name:     "nil cert",
			notAfter: time.Time{},
			want:     true,
		},
		{
			name:     "already expired",
			notAfter: now.Add(-time.Hour),
			want:     true,
		},
		{
			name:     "within renew window",
			notAfter: now.Add(CertificateRenewBefore / 2),
			want:     true,
		},
		{
			name:     "still far from expiry",
			notAfter: now.Add(CertificateValidity),
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cert *x509.Certificate
			if tt.name != "nil cert" {
				cert = &x509.Certificate{NotAfter: tt.notAfter}
			}
			if got := NeedsRenewal(cert); got != tt.want {
				t.Fatalf("NeedsRenewal() = %v, want %v", got, tt.want)
			}
		})
	}
}
