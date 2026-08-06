package replayauth

import (
	"testing"
	"time"
)

func TestUploadTicket(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	ticket, err := CreateUploadTicket("shared-secret", "job-1", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateUploadTicket("shared-secret", ticket, "job-1", now); err != nil {
		t.Fatalf("valid ticket rejected: %v", err)
	}
	if err := ValidateUploadTicket("shared-secret", ticket, "job-2", now); err == nil {
		t.Fatal("ticket for another job was accepted")
	}
	if err := ValidateUploadTicket("shared-secret", ticket, "job-1", now.Add(2*time.Minute)); err == nil {
		t.Fatal("expired ticket was accepted")
	}
}
