package handler

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"news_panel/internal/store"
	"news_panel/internal/triage"
)

// MaxPluginPayload is the bounded, uncompressed request size for plugin writes.
const MaxPluginPayload int64 = 2 << 20

func (h *Handler) ingestAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := ""
		if h.config != nil {
			token = h.config.IngestToken
		}
		headers := c.Request.Header.Values("Authorization")
		valid := false
		if len(headers) == 1 {
			scheme, supplied, found := strings.Cut(headers[0], " ")
			valid = found && strings.EqualFold(scheme, "Bearer") && validBearerToken(supplied) &&
				strings.TrimSpace(token) != "" && subtle.ConstantTimeCompare([]byte(supplied), []byte(token)) == 1
		}
		if !valid {
			c.Header("WWW-Authenticate", "Bearer")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}
		c.Next()
	}
}

func validBearerToken(token string) bool {
	if token == "" {
		return false
	}
	padding := false
	for i, c := range []byte(token) {
		if c == '=' {
			if i == 0 {
				return false
			}
			padding = true
			continue
		}
		if padding || !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || strings.ContainsRune("-._~+/", rune(c))) {
			return false
		}
	}
	return true
}

// Strictly decode a single object and reject unknown properties and null string fields.
func decodePluginObject(data []byte, dst any, nullable ...string) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if fields == nil {
		return errors.New("expected object")
	}
	allowed := make(map[string]bool)
	shape := reflect.TypeOf(dst).Elem()
	for i := 0; i < shape.NumField(); i++ {
		allowed[shape.Field(i).Tag.Get("json")] = true
	}
	for key, value := range fields {
		if !allowed[key] {
			return errors.New("unknown property")
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) && !oneOf(key, nullable...) {
			return errors.New("null property")
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

func readPluginBody(c *gin.Context, dst any, nullable ...string) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxPluginPayload)
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "payload exceeds 2 MiB"})
		} else {
			badRequestError(c, "invalid body")
		}
		return false
	}
	if err := decodePluginObject(data, dst, nullable...); err != nil {
		badRequestError(c, "invalid JSON object")
		return false
	}
	return true
}

func oneOf(s string, values ...string) bool {
	for _, value := range values {
		if s == value {
			return true
		}
	}
	return false
}

func validPluginString(s string, max int) bool {
	return strings.TrimSpace(s) != "" && utf8.RuneCountInString(s) <= max
}

func pluginTime(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return t, err
	}
	_, offset := t.Zone()
	if offset != 0 {
		return t, errors.New("timestamp must use UTC")
	}
	return t.UTC(), nil
}

type ingestRequest struct {
	SourceID   string            `json:"source_id"`
	SourceType string            `json:"source_type"`
	Items      []json.RawMessage `json:"items"`
}

type ingestItemRequest struct {
	ExternalID  string         `json:"external_id"`
	URL         string         `json:"url"`
	Title       string         `json:"title"`
	Content     string         `json:"content"`
	Author      string         `json:"author"`
	PublishedAt string         `json:"published_at"`
	RawMeta     map[string]any `json:"raw_meta"`
}

type ingestError struct {
	ExternalID string `json:"external_id"`
	Reason     string `json:"reason"`
}

type ingestResponse struct {
	Accepted     int           `json:"accepted"`
	Deduplicated int           `json:"deduplicated"`
	Rejected     int           `json:"rejected"`
	Errors       []ingestError `json:"errors"`
}

func normalizeIngestItem(req ingestRequest, raw json.RawMessage) (triage.Input, error) {
	var item ingestItemRequest
	input := triage.Input{SourceID: req.SourceID, SourceType: req.SourceType}
	// Retain a valid ID for diagnostics even when another field is malformed.
	var identity struct {
		ExternalID string `json:"external_id"`
	}
	_ = json.Unmarshal(raw, &identity)
	input.ExternalID = identity.ExternalID
	if err := decodePluginObject(raw, &item); err != nil {
		return input, errors.New("invalid item object or fields")
	}
	if !validPluginString(item.ExternalID, 256) {
		return input, errors.New("external_id must contain 1..256 characters")
	}
	parsedURL, err := url.ParseRequestURI(item.URL)
	if err != nil || !parsedURL.IsAbs() {
		return input, errors.New("url must be an absolute URI")
	}
	if strings.TrimSpace(item.Content) == "" {
		return input, errors.New("content is required")
	}
	publishedAt, err := pluginTime(item.PublishedAt)
	if err != nil {
		return input, errors.New("published_at must be RFC3339 UTC")
	}
	input.URL, input.Title, input.Content, input.Author = item.URL, item.Title, item.Content, item.Author
	input.PublishedAt, input.RawMeta = publishedAt, item.RawMeta
	return input, nil
}

func (h *Handler) ingest(c *gin.Context) {
	var req ingestRequest
	if !readPluginBody(c, &req) {
		return
	}
	if !validPluginString(req.SourceID, 128) || !validPluginString(req.SourceType, 64) || len(req.Items) < 1 || len(req.Items) > 100 {
		badRequestError(c, "source_id, source_type and 1..100 items are required")
		return
	}
	response := ingestResponse{Errors: []ingestError{}}
	for _, raw := range req.Items {
		input, err := normalizeIngestItem(req, raw)
		if err != nil {
			response.Rejected++
			response.Errors = append(response.Errors, ingestError{ExternalID: input.ExternalID, Reason: err.Error()})
			continue
		}
		id, created, err := h.store.IngestItem(c.Request.Context(), input)
		if err != nil {
			internalError(c, err, "ingest item")
			return
		}
		if !created {
			response.Deduplicated++
			saved, err := h.store.GetItem(id)
			if err != nil {
				internalError(c, err, "read pending item")
				return
			}
			if saved.TriageStatus != "pending" {
				continue
			}
			// Recover an interrupted evaluation using the original immutable envelope.
			publishedAt, err := pluginTime(saved.PublishedAt)
			if err != nil {
				internalError(c, err, "read item publication time")
				return
			}
			input = triage.Input{SourceID: saved.SourceID, SourceType: saved.SourceType, ExternalID: saved.ExternalID,
				URL: saved.Link, Title: saved.Title, Content: saved.Content, Author: saved.Author, PublishedAt: publishedAt}
			if err := json.Unmarshal(saved.RawMeta, &input.RawMeta); err != nil {
				internalError(c, err, "read item metadata")
				return
			}
		}
		result, err := h.evaluator.Evaluate(c.Request.Context(), input)
		if err == nil {
			err = result.Validate()
		}
		if err != nil {
			// A stored item remains accepted; failed processing is visible for audit.
			slog.Warn("triage evaluation failed", "item_id", id)
			result = triage.Result{Domain: "other", AITags: []string{}, Priority: "normal", TriageStatus: "failed"}
		}
		if err := h.store.UpdateTriage(c.Request.Context(), id, result); err != nil {
			internalError(c, err, "save triage")
			return
		}
		if created {
			response.Accepted++
		}
	}
	c.JSON(http.StatusOK, response)
}

type heartbeatRequest struct {
	SourceID      string  `json:"source_id"`
	PluginVersion string  `json:"plugin_version"`
	Status        string  `json:"status"`
	FetchedCount  *int64  `json:"fetched_count"`
	ErrorMessage  *string `json:"error_message"`
	Timestamp     string  `json:"timestamp"`
}

func (h *Handler) heartbeat(c *gin.Context) {
	var req heartbeatRequest
	if !readPluginBody(c, &req, "error_message") {
		return
	}
	timestamp, err := pluginTime(req.Timestamp)
	if !validPluginString(req.SourceID, 128) || !validPluginString(req.PluginVersion, 64) ||
		!oneOf(req.Status, "ok", "degraded", "error") || req.FetchedCount == nil || *req.FetchedCount < 0 || err != nil {
		badRequestError(c, "invalid heartbeat fields")
		return
	}
	hb := store.PluginHeartbeat{SourceID: req.SourceID, PluginVersion: req.PluginVersion, Status: req.Status,
		FetchedCount: *req.FetchedCount, ErrorMessage: req.ErrorMessage, Timestamp: timestamp.Format(time.RFC3339Nano)}
	if err := h.store.UpsertHeartbeat(c.Request.Context(), hb); err != nil {
		internalError(c, err, "save heartbeat")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "server_time": time.Now().UTC().Format(time.RFC3339Nano)})
}

func (h *Handler) listHeartbeats(c *gin.Context) {
	heartbeats, err := h.store.ListHeartbeats(c.Request.Context())
	if err != nil {
		internalError(c, err, "list heartbeats")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": heartbeats, "total": len(heartbeats)})
}
