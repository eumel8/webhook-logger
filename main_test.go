package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

func captureWebhook(t *testing.T, body string, authorization ...string) (int, string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = stdout
		r.Close()
		w.Close()
	})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	if len(authorization) > 0 {
		request.Header.Set("Authorization", authorization[0])
	}
	handleWebhook(response, request)
	os.Stdout = stdout
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return response.Code, string(output)
}

func TestWebhookBearerAuthentication(t *testing.T) {
	previousToken := bearerToken
	bearerToken = "test-webhook-token"
	t.Cleanup(func() { bearerToken = previousToken })
	for _, header := range []string{"", "Bearer wrong-token", "Basic test-webhook-token", "Bearer test-webhook-token-extra"} {
		t.Run(header, func(t *testing.T) {
			code, output := captureWebhook(t, `{"alerts":[{"status":"firing"}]}`, header)
			if code != http.StatusUnauthorized || output != "" {
				t.Fatalf("status = %d, output = %q; want 401 and no output", code, output)
			}
		})
	}
	code, output := captureWebhook(t, `{"alerts":[{"status":"firing"}]}`, "Bearer test-webhook-token")
	if code != http.StatusOK || output != "{\"status\":\"firing\"}\n" {
		t.Fatalf("status = %d, output = %q; want 200 and alert JSON", code, output)
	}
}

func TestWebhookIndividualAlerts(t *testing.T) {
	body := `{
		"status": "firing",
		"commonLabels": {"alertname": "GroupName"},
		"alerts": [
			{"status":"firing","labels":{"alertname":"First","namespace":"one"},"annotations":{"description":"line one\nline two"},"startsAt":"2026-09-30T06:45:42.462Z","endsAt":"0001-01-01T00:00:00Z","generatorURL":"/graph","fingerprint":"first","extra":9007199254740993},
			{"status":"resolved","labels":{"alertname":"Second","namespace":"two"},"annotations":{"summary":"Recovered"},"startsAt":"2026-09-29T06:45:42.462Z","endsAt":"2026-09-30T06:45:42.462Z","generatorURL":"/graph","fingerprint":"second"}
		]
	}`
	code, output := captureWebhook(t, body)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	if len(lines) != 2 || !strings.HasSuffix(output, "\n") {
		t.Fatalf("want two newline-terminated records, got %q", output)
	}
	var expected struct {
		Alerts []map[string]json.RawMessage `json:"alerts"`
	}
	if err := json.Unmarshal([]byte(body), &expected); err != nil {
		t.Fatal(err)
	}
	for i, line := range lines {
		var actual map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &actual); err != nil {
			t.Fatalf("record %d is not JSON: %v", i, err)
		}
		if !reflect.DeepEqual(actual, expected.Alerts[i]) {
			t.Errorf("record %d = %s, want original alert fields", i, line)
		}
	}
}

func TestWebhookInvalidPayloads(t *testing.T) {
	for _, body := range []string{
		`{`, `null`, `{}`, `{"alerts":null}`, `{"alerts":{}}`,
		`{"alerts":[42]}`, `{"alerts":[{"status":"firing"},null]}`,
	} {
		t.Run(body, func(t *testing.T) {
			code, output := captureWebhook(t, body)
			if code != http.StatusBadRequest || output != "" {
				t.Fatalf("status = %d, output = %q; want 400 and no output", code, output)
			}
		})
	}
}

func TestWebhookEmptyAlerts(t *testing.T) {
	code, output := captureWebhook(t, `{"alerts":[]}`)
	if code != http.StatusOK || output != "" {
		t.Fatalf("status = %d, output = %q; want 200 and no output", code, output)
	}
}
