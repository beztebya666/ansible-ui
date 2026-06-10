package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/nikiv/ansible-ui/internal/model"
)

// AWS Secrets Manager client, signed with Signature V4 by hand (no SDK — keeps
// the dependency surface small, matching the hand-rolled Vault client).

func awsHost(b *model.SecretBackend) string {
	if b.Address != "" {
		h := strings.TrimPrefix(strings.TrimPrefix(b.Address, "https://"), "http://")
		return strings.TrimRight(h, "/")
	}
	return "secretsmanager." + b.Region + ".amazonaws.com"
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// awsCall signs and performs a Secrets Manager JSON action (e.g. GetSecretValue).
func (s *Server) awsCall(ctx context.Context, b *model.SecretBackend, action string, payload map[string]any) (int, []byte, error) {
	if b.Region == "" {
		return 0, nil, fmt.Errorf("region is required")
	}
	host := awsHost(b)
	body, _ := json.Marshal(payload)
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	const service = "secretsmanager"
	target := "secretsmanager." + action

	// Canonical request.
	hashedPayload := sha256Hex(body)
	canonicalHeaders := "content-type:application/x-amz-json-1.1\n" +
		"host:" + host + "\n" +
		"x-amz-date:" + amzDate + "\n" +
		"x-amz-target:" + target + "\n"
	signedHeaders := "content-type;host;x-amz-date;x-amz-target"
	canonicalRequest := "POST\n/\n\n" + canonicalHeaders + "\n" + signedHeaders + "\n" + hashedPayload

	// String to sign.
	scope := dateStamp + "/" + b.Region + "/" + service + "/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + sha256Hex([]byte(canonicalRequest))

	// Signing key + signature.
	kDate := hmacSHA256([]byte("AWS4"+b.Token), dateStamp)
	kRegion := hmacSHA256(kDate, b.Region)
	kService := hmacSHA256(kRegion, service)
	kSigning := hmacSHA256(kService, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	authz := "AWS4-HMAC-SHA256 Credential=" + b.AccessKeyID + "/" + scope +
		", SignedHeaders=" + signedHeaders + ", Signature=" + signature

	scheme := "https://"
	if strings.HasPrefix(b.Address, "http://") {
		scheme = "http://"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, scheme+host+"/", bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/x-amz-json-1.1")
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Target", target)
	req.Header.Set("Authorization", authz)

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, rb, nil
}

// awsError extracts AWS's {"__type","message"} into a readable error.
func awsError(code int, body []byte) error {
	var e struct {
		Type    string `json:"__type"`
		Message string `json:"message"`
		Msg     string `json:"Message"`
	}
	_ = json.Unmarshal(body, &e)
	t := e.Type
	if i := strings.LastIndex(t, "#"); i >= 0 {
		t = t[i+1:]
	}
	msg := e.Message
	if msg == "" {
		msg = e.Msg
	}
	if t != "" {
		if msg != "" {
			return fmt.Errorf("%s: %s", t, msg)
		}
		return fmt.Errorf("%s", t)
	}
	return fmt.Errorf("AWS returned HTTP %d", code)
}

// testAWS validates credentials by listing secrets (cheap, read-only).
func (s *Server) testAWS(ctx context.Context, b *model.SecretBackend) (string, error) {
	if b.Region == "" || b.AccessKeyID == "" {
		return "", fmt.Errorf("region and access key id are required")
	}
	code, body, err := s.awsCall(ctx, b, "ListSecrets", map[string]any{"MaxResults": 1})
	if err != nil {
		return "", fmt.Errorf("cannot reach AWS Secrets Manager: %w", err)
	}
	if code != http.StatusOK {
		return "", awsError(code, body)
	}
	var lr struct {
		SecretList []struct {
			Name string `json:"Name"`
		} `json:"SecretList"`
	}
	_ = json.Unmarshal(body, &lr)
	return fmt.Sprintf("authenticated in %s", b.Region), nil
}

// awsSecretRead fetches a secret value and, when field is set, picks one key
// from a JSON SecretString.
func (s *Server) awsSecretRead(ctx context.Context, b *model.SecretBackend, path, field string) (string, error) {
	code, body, err := s.awsCall(ctx, b, "GetSecretValue", map[string]any{"SecretId": path})
	if err != nil {
		return "", fmt.Errorf("cannot reach AWS Secrets Manager: %w", err)
	}
	if code != http.StatusOK {
		return "", awsError(code, body)
	}
	var r struct {
		SecretString string `json:"SecretString"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", fmt.Errorf("unexpected AWS response: %w", err)
	}
	if field == "" {
		return r.SecretString, nil
	}
	var kv map[string]any
	if err := json.Unmarshal([]byte(r.SecretString), &kv); err != nil {
		return "", fmt.Errorf("secret %q is not JSON; cannot select field %q", path, field)
	}
	v, ok := kv[field]
	if !ok {
		return "", fmt.Errorf("field %q not present in secret %q", field, path)
	}
	return fmt.Sprintf("%v", v), nil
}
