package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"news_panel/internal/model"
	"news_panel/internal/triage"
)

// IngestItem reserves the idempotency key and commits pending state before evaluation.
// Suspended source feeds retain upstream feed associations without enabling polling.
func (s *Store) IngestItem(ctx context.Context, input triage.Input) (id int64, created bool, err error) {
	raw, err := json.Marshal(input.RawMeta)
	if err != nil {
		return 0, false, err
	}
	if input.RawMeta == nil {
		raw = []byte("{}")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, err
	}
	defer tx.Rollback()
	// Encode the source ID into a private URI namespace; never fetch this URI.
	link := "urn:news_panel:source:" + input.SourceID
	if _, err = tx.ExecContext(ctx, `INSERT INTO feeds (name, link, suspended) VALUES (?, ?, 1) ON CONFLICT(link) DO NOTHING`, input.SourceID, link); err != nil {
		return 0, false, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO items
        (feed_id, guid, title, link, content, pub_date, source_id, external_id, source_type, author, raw_meta, published_at)
        VALUES ((SELECT id FROM feeds WHERE link = ?), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(source_id, external_id) WHERE source_id != '' AND external_id != '' DO NOTHING`,
		link, input.ExternalID, input.Title, input.URL, input.Content, input.PublishedAt.Unix(), input.SourceID, input.ExternalID, input.SourceType, input.Author, string(raw), input.PublishedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, false, err
	}
	if n > 0 {
		id, err = result.LastInsertId()
	} else {
		err = tx.QueryRowContext(ctx, `SELECT id FROM items WHERE source_id=? AND external_id=?`, input.SourceID, input.ExternalID).Scan(&id)
	}
	if err != nil {
		return 0, false, err
	}
	if err = tx.Commit(); err != nil {
		return 0, false, err
	}
	return id, n > 0, nil
}

func (s *Store) UpdateTriage(ctx context.Context, id int64, r triage.Result) error {
	if err := r.Validate(); err != nil {
		return err
	}
	if r.AITags == nil {
		r.AITags = []string{}
	}
	tags, err := json.Marshal(r.AITags)
	if err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE items SET domain=?, ai_score=?, ai_tags=?, ai_summary=?, priority=?, triage_status=? WHERE id=?`, r.Domain, r.AIScore, string(tags), r.AISummary, r.Priority, r.TriageStatus, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		return fmt.Errorf("%w: item", ErrNotFound)
	}
	return err
}

type PluginHeartbeat struct {
	SourceID      string  `json:"source_id"`
	PluginVersion string  `json:"plugin_version"`
	Status        string  `json:"status"`
	FetchedCount  int64   `json:"fetched_count"`
	ErrorMessage  *string `json:"error_message"`
	Timestamp     string  `json:"timestamp"`
	UpdatedAt     string  `json:"updated_at"`
}

func (s *Store) UpsertHeartbeat(ctx context.Context, h PluginHeartbeat) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO plugin_heartbeats
        (source_id, plugin_version, status, fetched_count, error_message, timestamp, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(source_id) DO UPDATE SET plugin_version=excluded.plugin_version,
        status=excluded.status, fetched_count=excluded.fetched_count, error_message=excluded.error_message,
        timestamp=excluded.timestamp, updated_at=excluded.updated_at`,
		h.SourceID, h.PluginVersion, h.Status, h.FetchedCount, h.ErrorMessage, h.Timestamp, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) ListHeartbeats(ctx context.Context) ([]PluginHeartbeat, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT source_id, plugin_version, status, fetched_count, error_message, timestamp, updated_at FROM plugin_heartbeats ORDER BY source_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []PluginHeartbeat{}
	for rows.Next() {
		var h PluginHeartbeat
		if err := rows.Scan(&h.SourceID, &h.PluginVersion, &h.Status, &h.FetchedCount, &h.ErrorMessage, &h.Timestamp, &h.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, h)
	}
	return result, rows.Err()
}

const itemColumns = `items.id, items.feed_id, items.guid, items.title, items.link, items.content, items.pub_date, items.unread, items.created_at,
    items.source_id, items.external_id, items.source_type, items.published_at, items.author, items.raw_meta, items.domain, items.ai_score, items.ai_tags, items.ai_summary, items.priority, items.triage_status`

func scanItem(row interface{ Scan(...any) error }) (*model.Item, error) {
	i := &model.Item{}
	var unread int
	var tags, raw string
	err := row.Scan(&i.ID, &i.FeedID, &i.GUID, &i.Title, &i.Link, &i.Content, &i.PubDate, &unread, &i.CreatedAt,
		&i.SourceID, &i.ExternalID, &i.SourceType, &i.PublishedAt, &i.Author, &raw, &i.Domain, &i.AIScore, &tags, &i.AISummary, &i.Priority, &i.TriageStatus)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal([]byte(tags), &i.AITags); err != nil {
		return nil, fmt.Errorf("decode item tags: %w", err)
	}
	i.RawMeta = json.RawMessage(raw)
	i.Unread = intToBool(unread)
	return i, nil
}

func appendTriageFilters(query string, args []any, p ListItemsParams) (string, []any) {
	return appendTriageFiltersAlias(query, args, p, "items")
}

func appendTriageFiltersAlias(query string, args []any, p ListItemsParams, alias string) (string, []any) {
	for _, f := range []struct{ name, value string }{{"triage_status", p.TriageStatus}, {"domain", p.Domain}, {"priority", p.Priority}} {
		if f.value != "" {
			query += " AND " + alias + "." + f.name + " = :" + f.name
			args = append(args, sql.Named(f.name, f.value))
		}
	}
	if p.MinScore != nil {
		query += " AND " + alias + ".ai_score >= :min_score"
		args = append(args, sql.Named("min_score", *p.MinScore))
	}
	return query, args
}
