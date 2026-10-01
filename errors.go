package casebook

import (
	"encoding/json/v2"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const maxRawError = 512

type APIError struct {
	code    int
	message string
}

func (e *APIError) Error() string {
	return strconv.Itoa(e.code) + ": " + e.message
}

func (e *APIError) IsRetryable() bool {
	return e.code == http.StatusTooManyRequests || e.code >= http.StatusInternalServerError
}

type apiErrorDto struct {
	Details []string `json:"details"`
}

func unmarshalError(code int, body io.ReadCloser) *APIError {
	return &APIError{
		code:    code,
		message: errorMessage(body),
	}
}

func errorMessage(body io.ReadCloser) string {
	if body == nil {
		return ""
	}
	defer body.Close()

	raw, _ := io.ReadAll(body)

	var dto apiErrorDto
	if err := json.Unmarshal(raw, &dto); err == nil {
		return strings.Join(dto.Details, ", ")
	}

	msg := strings.TrimSpace(string(raw))
	if len(msg) > maxRawError {
		msg = msg[:maxRawError]
	}
	return msg
}
