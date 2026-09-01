// Package httpapi contains the router, middleware and handlers that expose the
// store over a JSON REST API.
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Error codes returned in the error envelope.
const (
	codeValidation    = "validation_error"
	codeNotFound      = "not_found"
	codeConflict      = "conflict"
	codeUnprocessable = "unprocessable"
	codeInternal      = "internal_error"
	codeBadRequest    = "bad_request"
)

// envelope is the success shape: {"data": ...}.
type envelope struct {
	Data any `json:"data"`
}

// errorEnvelope is the failure shape: {"error": {"code": ..., "message": ...}}.
type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// apiError carries the status code and error code a handler wants to return.
type apiError struct {
	Status  int
	Code    string
	Message string
}

func (e apiError) Error() string { return e.Message }

// validationError reports a malformed or missing field (400).
func validationError(format string, args ...any) apiError {
	return apiError{Status: http.StatusBadRequest, Code: codeValidation, Message: fmt.Sprintf(format, args...)}
}

// unprocessableError reports a well-formed request the domain rejects (422).
func unprocessableError(format string, args ...any) apiError {
	return apiError{Status: http.StatusUnprocessableEntity, Code: codeUnprocessable, Message: fmt.Sprintf(format, args...)}
}

// notFoundError reports a missing resource (404).
func notFoundError(what string) apiError {
	return apiError{Status: http.StatusNotFound, Code: codeNotFound, Message: what + " not found"}
}

// conflictError reports a uniqueness violation (409).
func conflictError(message string) apiError {
	return apiError{Status: http.StatusConflict, Code: codeConflict, Message: message}
}

// writeJSON serialises v inside the success envelope.
func writeJSON(w http.ResponseWriter, status int, data any) {
	body, err := json.Marshal(envelope{Data: data})
	if err != nil {
		// Marshalling a domain type should never fail; if it does the client
		// gets a plain 500 rather than a half-written body.
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"internal_error","message":"failed to encode response"}}`))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// writeError renders err as an error envelope, logging anything unexpected.
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	var apiErr apiError
	if !errors.As(err, &apiErr) {
		apiErr = apiError{Status: http.StatusInternalServerError, Code: codeInternal, Message: "something went wrong"}
		loggerFrom(r.Context()).Error("unhandled error", "error", err, "path", r.URL.Path, "method", r.Method)
	}

	body, marshalErr := json.Marshal(errorEnvelope{Error: errorBody{Code: apiErr.Code, Message: apiErr.Message}})
	if marshalErr != nil {
		body = []byte(`{"error":{"code":"internal_error","message":"something went wrong"}}`)
		apiErr.Status = http.StatusInternalServerError
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(apiErr.Status)
	_, _ = w.Write(body)
}

// maxBodyBytes caps request bodies; TipTap documents are small.
const maxBodyBytes = 1 << 20 // 1 MiB

// decodeJSON reads a JSON request body into dst, rejecting unknown fields and
// trailing content so typos in a client surface as 400s rather than silence.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	if ct := r.Header.Get("Content-Type"); ct != "" {
		if mediaType := strings.TrimSpace(strings.Split(ct, ";")[0]); mediaType != "application/json" {
			return apiError{
				Status:  http.StatusUnsupportedMediaType,
				Code:    codeBadRequest,
				Message: "Content-Type must be application/json",
			}
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return apiError{
				Status:  http.StatusRequestEntityTooLarge,
				Code:    codeValidation,
				Message: fmt.Sprintf("request body must not exceed %d bytes", maxBodyBytes),
			}
		}
		if errors.Is(err, io.EOF) {
			return validationError("request body must not be empty")
		}
		return validationError("request body is not valid JSON: %v", err)
	}
	if dec.More() {
		return validationError("request body must contain a single JSON object")
	}
	return nil
}
