package replayauth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func CreateUploadTicket(secret, jobId string, expiresAt time.Time) (string, error) {
	if strings.TrimSpace(secret) == "" {
		return "", errors.New("replay analyzer shared secret is not configured")
	}
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%s.%d", jobId, expiresAt.Unix())))
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + signature, nil
}

func ValidateUploadTicket(secret, ticket, expectedJobId string, now time.Time) error {
	parts := strings.Split(ticket, ".")
	if len(parts) != 2 || strings.TrimSpace(secret) == "" {
		return errors.New("invalid replay upload ticket")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(parts[0]))
	expectedSignature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !hmac.Equal(mac.Sum(nil), expectedSignature) {
		return errors.New("invalid replay upload ticket signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return errors.New("invalid replay upload ticket payload")
	}
	payloadParts := strings.Split(string(payload), ".")
	if len(payloadParts) != 2 || payloadParts[0] != expectedJobId {
		return errors.New("replay upload ticket does not match job")
	}
	expiresAt, err := strconv.ParseInt(payloadParts[1], 10, 64)
	if err != nil || now.Unix() > expiresAt {
		return errors.New("replay upload ticket expired")
	}
	return nil
}
