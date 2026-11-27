package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"news_panel/internal/model"
)

// ListItemsParams specifies filtering and pagination for item queries.
//
// Pointer fields (FeedID, GroupID, Unread) are optional filters - nil means "no filter".
// BeforePubDate/BeforeID form an optional cursor: when both are non-nil, only items
// ordered before that (pub_date, id) position are returned (nil = first page).
// OrderBy accepts "pub_date" (default) or "created_at".
// Limit = 0 means no limit.
type ListItemsParams struct {
	TriageStatus  string
	Domain        string
	Priority      string
	MinScore      *int
	Scored        *bool
	FeedID        *int64
	GroupID       *int64
	Unread        *bool
	Limit         int
	BeforePubDate *int64
	BeforeID      *int64
	OrderBy       string // "pub_date" or "created_at"
}

func (s *Store) ListItems(params ListItemsParams) ([]*model.Item, error) {
	query := `
		SELECT ` + itemColumns + `
		FROM items
	`
	args := []any{}

	// Join feeds table if filtering by GroupID
	if params.GroupID != nil {
		query += ` INNER JOIN feeds ON items.feed_id = feeds.id`
	}

	query += ` WHERE 1=1`
	query, args = appendTriageFilters(query, args, params)

	if params.FeedID != nil {
		query += ` AND items.feed_id = :feed_id`
		args = append(args, sql.Named("feed_id", *params.FeedID))
	}
	if params.GroupID != nil {
		query += ` AND feeds.group_id = :group_id`
		args = append(args, sql.Named("group_id", *params.GroupID))
	}
	if params.Unread != nil {
		query += ` AND items.unread = :unread`
		args = append(args, sql.Named("unread", boolToInt(*params.Unread)))
	}

	// Cursor pagination: skip items at or before the cursor position, matching
	// the ORDER BY (pub_date DESC, id DESC) tie-break semantics.
	if params.BeforePubDate != nil && params.BeforeID != nil {
		query += ` AND (items.pub_date < :before_pub_date OR (items.pub_date = :before_pub_date AND items.id < :before_id))`
		args = append(args, sql.Named("before_pub_date", *params.BeforePubDate), sql.Named("before_id", *params.BeforeID))
	}

	// ORDER BY cannot use named parameters, validated via allowlist instead
	orderBy := "items.pub_date DESC, items.id DESC"
	if params.OrderBy == "created_at" {
		orderBy = "items.created_at DESC, items.id DESC"
	}
	query += ` ORDER BY ` + orderBy

	if params.Limit > 0 {
		query += ` LIMIT :limit`
		args = append(args, sql.Named("limit", params.Limit))
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []*model.Item{}
	for rows.Next() {
		i, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

func (s *Store) GetItem(id int64) (*model.Item, error) {
	i, err := scanItem(s.db.QueryRow(`
		SELECT `+itemColumns+`
		FROM items
		WHERE id = :id
	`, sql.Named("id", id)))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: item", ErrNotFound)
		}
		return nil, fmt.Errorf("get item: %w", err)
	}

	return i, nil
}

func (s *Store) CreateItem(feedID int64, guid, title, link, content string, pubDate int64) (*model.Item, error) {
	result, err := s.db.Exec(`
		INSERT INTO items (feed_id, guid, title, link, content, pub_date, triage_status)
		VALUES (:feed_id, :guid, :title, :link, :content, :pub_date, 'kept')
	`, sql.Named("feed_id", feedID), sql.Named("guid", guid), sql.Named("title", title),
		sql.Named("link", link), sql.Named("content", content), sql.Named("pub_date", pubDate))
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return s.GetItem(id)
}

type BatchCreateItemInput struct {
	GUID    string
	Title   string
	Link    string
	Content string
	PubDate int64
}

// BatchCreateItemsIgnore inserts items in one transaction and ignores duplicates by (feed_id, guid).
// Returns the number of newly inserted rows.
func (s *Store) BatchCreateItemsIgnore(feedID int64, inputs []BatchCreateItemInput) (int, error) {
	n, _, err := s.batchCreateRSS(feedID, inputs, 0)
	return n, err
}

type CreatedItem struct {
	ID int64
	BatchCreateItemInput
}

// BatchCreateItemsForTriage returns only rows inserted by this transaction.
func (s *Store) BatchCreateItemsForTriage(feedID int64, inputs []BatchCreateItemInput) ([]CreatedItem, error) {
	_, items, err := s.BatchCreateItemsWithTriageLimit(feedID, inputs, 0)
	return items, err
}

// BatchCreateItemsWithTriageLimit keeps overflow unscored in the same transaction.
// The caller supplies newest-first inputs. A zero limit means unlimited.
func (s *Store) BatchCreateItemsWithTriageLimit(feedID int64, inputs []BatchCreateItemInput, limit int) (int, []CreatedItem, error) {
	if limit == 0 {
		limit = len(inputs)
	}
	return s.batchCreateRSS(feedID, inputs, limit)
}

func (s *Store) batchCreateRSS(feedID int64, inputs []BatchCreateItemInput, pendingLimit int) (int, []CreatedItem, error) {
	if len(inputs) == 0 {
		return 0, nil, nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, nil, err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO items(feed_id,guid,title,link,content,pub_date,triage_status)
 VALUES(?,?,?,?,?,?,?) ON CONFLICT(feed_id,guid) DO NOTHING`)
	if err != nil {
		return 0, nil, err
	}
	defer stmt.Close()
	var pending []CreatedItem
	created := 0
	for _, in := range inputs {
		status := "kept"
		if len(pending) < pendingLimit {
			status = "pending"
		}
		res, err := stmt.Exec(feedID, in.GUID, in.Title, in.Link, in.Content, in.PubDate, status)
		if err != nil {
			return 0, nil, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return 0, nil, err
		}
		if n == 0 {
			continue
		}
		created++
		if status == "pending" {
			id, err := res.LastInsertId()
			if err != nil {
				return 0, nil, err
			}
			pending = append(pending, CreatedItem{ID: id, BatchCreateItemInput: in})
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, nil, err
	}
	return created, pending, nil
}

// ClaimUnscoredItemsForTriage claims up to limit newest unscored items for a feed
// and marks them pending in one transaction so a backfill run can evaluate them.
func (s *Store) ClaimUnscoredItemsForTriage(feedID int64, limit int) ([]CreatedItem, error) {
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.Query(`
		SELECT id, guid, title, link, content, pub_date
		FROM items
		WHERE feed_id = ? AND triage_status = 'kept' AND ai_score IS NULL
		ORDER BY pub_date DESC, id DESC
		LIMIT ?
	`, feedID, limit)
	if err != nil {
		return nil, err
	}
	var claimed []CreatedItem
	for rows.Next() {
		var item CreatedItem
		if err := rows.Scan(&item.ID, &item.GUID, &item.Title, &item.Link, &item.Content, &item.PubDate); err != nil {
			rows.Close()
			return nil, err
		}
		claimed = append(claimed, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	if len(claimed) == 0 {
		return nil, nil
	}

	upd, err := tx.Prepare(`UPDATE items SET triage_status = 'pending' WHERE id = ?`)
	if err != nil {
		return nil, err
	}
	defer upd.Close()
	for _, item := range claimed {
		if _, err := upd.Exec(item.ID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return claimed, nil
}

func (s *Store) UpdateItemUnread(id int64, unread bool) error {
	result, err := s.db.Exec(`UPDATE items SET unread = :unread WHERE id = :id`,
		sql.Named("unread", boolToInt(unread)), sql.Named("id", id))
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("%w: item", ErrNotFound)
	}
	return nil
}

// BatchUpdateItemsUnread marks multiple items as read/unread.
// IDs are chunked to keep SQL statements bounded and avoid oversized IN clauses.
func (s *Store) BatchUpdateItemsUnread(ids []int64, unread bool) error {
	if len(ids) == 0 {
		return nil
	}

	const chunkSize = 500
	for start := 0; start < len(ids); start += chunkSize {
		end := min(start+chunkSize, len(ids))

		if err := s.batchUpdateItemsUnreadChunk(ids[start:end], unread); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) batchUpdateItemsUnreadChunk(ids []int64, unread bool) error {
	if len(ids) == 0 {
		return nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids)+1)
	args = append(args, sql.Named("unread", boolToInt(unread)))
	for i, id := range ids {
		paramName := fmt.Sprintf("id%d", i)
		placeholders[i] = ":" + paramName
		args = append(args, sql.Named(paramName, id))
	}

	query := fmt.Sprintf(`UPDATE items SET unread = :unread WHERE id IN (%s)`, strings.Join(placeholders, ","))
	_, err := s.db.Exec(query, args...)
	return err
}

// MarkAllAsRead marks items as read. If feedID is nil, marks ALL items across all feeds.
// If feedID is non-nil, only marks items from that specific feed.
func (s *Store) MarkAllAsRead(feedID *int64) error {
	if feedID != nil {
		_, err := s.db.Exec(`UPDATE items SET unread = 0 WHERE feed_id = :feed_id`, sql.Named("feed_id", *feedID))
		return err
	}
	_, err := s.db.Exec(`UPDATE items SET unread = 0`)
	return err
}

func (s *Store) MarkGroupAsRead(groupID int64) error {
	_, err := s.db.Exec(`
		UPDATE items
		SET unread = 0
		WHERE feed_id IN (
			SELECT id
			FROM feeds
			WHERE group_id = :group_id
		)
	`, sql.Named("group_id", groupID))
	return err
}

func (s *Store) MarkFeedAsReadBefore(feedID, before int64) error {
	_, err := s.db.Exec(`
		UPDATE items
		SET unread = 0
		WHERE feed_id = :feed_id
		  AND (CASE WHEN pub_date > 0 THEN pub_date ELSE created_at END) <= :before
	`, sql.Named("feed_id", feedID), sql.Named("before", before))
	return err
}

func (s *Store) MarkGroupAsReadBefore(groupID, before int64) error {
	_, err := s.db.Exec(`
		UPDATE items
		SET unread = 0
		WHERE feed_id IN (
			SELECT id
			FROM feeds
			WHERE group_id = :group_id
		)
		  AND (CASE WHEN pub_date > 0 THEN pub_date ELSE created_at END) <= :before
	`, sql.Named("group_id", groupID), sql.Named("before", before))
	return err
}

func (s *Store) MarkAllAsReadBefore(before int64) error {
	_, err := s.db.Exec(`
		UPDATE items
		SET unread = 0
		WHERE (CASE WHEN pub_date > 0 THEN pub_date ELSE created_at END) <= :before
	`, sql.Named("before", before))
	return err
}

func (s *Store) ListUnreadItemIDs() ([]int64, error) {
	rows, err := s.db.Query(`
		SELECT id
		FROM items
		WHERE unread = 1
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, rows.Err()
}

type ListFeverItemsParams struct {
	WithIDs []int64
	SinceID *int64
	MaxID   *int64
	Limit   int
	SortAsc bool
}

func (s *Store) ListFeverItems(params ListFeverItemsParams) ([]*model.Item, error) {
	query := `
		SELECT ` + itemColumns + `
		FROM items
		WHERE 1=1
	`
	args := []any{}

	if len(params.WithIDs) > 0 {
		placeholders := make([]string, len(params.WithIDs))
		for i, id := range params.WithIDs {
			name := fmt.Sprintf("with_id_%d", i)
			placeholders[i] = ":" + name
			args = append(args, sql.Named(name, id))
		}
		query += fmt.Sprintf(" AND id IN (%s)", strings.Join(placeholders, ","))
	}

	if params.SinceID != nil {
		query += ` AND id > :since_id`
		args = append(args, sql.Named("since_id", *params.SinceID))
	}

	if params.MaxID != nil {
		query += ` AND id <= :max_id`
		args = append(args, sql.Named("max_id", *params.MaxID))
	}

	orderBy := "DESC"
	if params.SortAsc {
		orderBy = "ASC"
	}
	query += ` ORDER BY id ` + orderBy

	if params.Limit > 0 {
		query += ` LIMIT :limit`
		args = append(args, sql.Named("limit", params.Limit))
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []*model.Item{}
	for rows.Next() {
		i, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, i)
	}

	return items, rows.Err()
}

func (s *Store) ItemExists(feedID int64, guid string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM items WHERE feed_id = :feed_id AND guid = :guid)`,
		sql.Named("feed_id", feedID), sql.Named("guid", guid)).Scan(&exists)
	return exists, err
}

type SearchItemResult struct {
	ID      int64  `json:"id"`
	FeedID  int64  `json:"feed_id"`
	Title   string `json:"title"`
	PubDate int64  `json:"pub_date"`
}

func (s *Store) SearchItems(query string, limit int) ([]*SearchItemResult, error) {
	ftsQuery := buildFTSQuery(query)
	if ftsQuery == "" {
		return s.searchItemsLike(query, limit)
	}

	rows, err := s.db.Query(`
		SELECT i.id, i.feed_id, i.title, i.pub_date
		FROM items_fts
		INNER JOIN items i ON i.id = items_fts.rowid
		WHERE items_fts MATCH :query
		ORDER BY i.pub_date DESC, i.id DESC
		LIMIT :limit
	`, sql.Named("query", ftsQuery), sql.Named("limit", limit))
	if err != nil {
		return s.searchItemsLike(query, limit)
	}
	defer rows.Close()

	items := []*SearchItemResult{}
	for rows.Next() {
		i := &SearchItemResult{}
		if err := rows.Scan(&i.ID, &i.FeedID, &i.Title, &i.PubDate); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

func buildFTSQuery(query string) string {
	parts := strings.Fields(strings.TrimSpace(query))
	if len(parts) == 0 {
		return ""
	}

	terms := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		part = strings.ReplaceAll(part, `"`, `""`)
		terms = append(terms, `"`+part+`"*`)
	}

	return strings.Join(terms, " AND ")
}

func (s *Store) searchItemsLike(query string, limit int) ([]*SearchItemResult, error) {
	rows, err := s.db.Query(`
		SELECT id, feed_id, title, pub_date
		FROM items
		WHERE title LIKE :query OR content LIKE :query
		ORDER BY pub_date DESC, id DESC
		LIMIT :limit
	`, sql.Named("query", "%"+query+"%"), sql.Named("limit", limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []*SearchItemResult{}
	for rows.Next() {
		i := &SearchItemResult{}
		if err := rows.Scan(&i.ID, &i.FeedID, &i.Title, &i.PubDate); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

// CountItems returns the total count of items matching the filter criteria.
func (s *Store) CountItems(params ListItemsParams) (int, error) {
	query := `SELECT COUNT(*) FROM items`
	args := []any{}

	if params.GroupID != nil {
		query += ` INNER JOIN feeds ON items.feed_id = feeds.id`
	}

	query += ` WHERE 1=1`
	query, args = appendTriageFilters(query, args, params)

	if params.FeedID != nil {
		query += ` AND items.feed_id = :feed_id`
		args = append(args, sql.Named("feed_id", *params.FeedID))
	}
	if params.GroupID != nil {
		query += ` AND feeds.group_id = :group_id`
		args = append(args, sql.Named("group_id", *params.GroupID))
	}
	if params.Unread != nil {
		query += ` AND items.unread = :unread`
		args = append(args, sql.Named("unread", boolToInt(*params.Unread)))
	}

	var count int
	err := s.db.QueryRow(query, args...).Scan(&count)
	return count, err
}
