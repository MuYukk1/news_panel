package store

import (
	"database/sql"
	"errors"
	"news_panel/internal/model"
	"os"
)

// SecureAIFiles restricts the database and any existing WAL sidecars before serving settings.
func (s *Store) SecureAIFiles() error {
	var seq int
	var name, path string
	if err := s.db.QueryRow("PRAGMA database_list").Scan(&seq, &name, &path); err != nil {
		return err
	}
	if path == "" {
		return nil
	} // In-memory databases have no backing file.
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		err := os.Chmod(path+suffix, 0600)
		if err != nil && !(suffix != "" && os.IsNotExist(err)) {
			return err
		}
	}
	return nil
}

const aiColumns = `decision_enabled,
 decision_provider,
 decision_base_url,
 decision_api_key,
 decision_account_id,
 decision_auth_header,
 decision_model,
 decision_json_format,
 decision_timeout_seconds,
 decision_l3_priorities,
 decision_low_action,
 decision_fallback_mode,
 topics,
 rss_max_items_per_pull`

func (s *Store) GetAISettings() (model.AISettings, error) {
	var row model.AISettings
	err := s.db.QueryRow("SELECT "+aiColumns+" FROM ai_settings WHERE id=1").Scan(
		&row.DecisionEnabled,
		&row.DecisionProvider,
		&row.DecisionBaseURL,
		&row.DecisionAPIKey,
		&row.DecisionAccountID,
		&row.DecisionAuthHeader,
		&row.DecisionModel,
		&row.DecisionJSONFormat,
		&row.DecisionTimeoutSeconds,
		&row.DecisionL3Priorities,
		&row.DecisionLowAction,
		&row.DecisionFallbackMode,
		&row.Topics,
		&row.RSSMaxItemsPerPull,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return model.AISettings{}, nil
	}
	return row, err
}

// UpdateAISettings applies only the supplied fields in one SQLite statement.
func (s *Store) UpdateAISettings(p model.AISettings, clearKey bool) error {
	if clearKey {
		p.DecisionAPIKey = nil
	}
	if p.DecisionAPIKey != nil && *p.DecisionAPIKey == "" {
		p.DecisionAPIKey = nil
	}
	_, err := s.db.Exec("INSERT INTO ai_settings (id,"+aiColumns+`) VALUES (1,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
 ON CONFLICT(id) DO UPDATE SET
 decision_enabled=COALESCE(excluded.decision_enabled,decision_enabled),
 decision_provider=COALESCE(excluded.decision_provider,decision_provider),
 decision_base_url=COALESCE(excluded.decision_base_url,decision_base_url),
 decision_api_key=CASE WHEN ? THEN NULL ELSE COALESCE(excluded.decision_api_key,decision_api_key) END,
 decision_account_id=COALESCE(excluded.decision_account_id,decision_account_id),
 decision_auth_header=COALESCE(excluded.decision_auth_header,decision_auth_header),
 decision_model=COALESCE(excluded.decision_model,decision_model),
 decision_json_format=COALESCE(excluded.decision_json_format,decision_json_format),
 decision_timeout_seconds=COALESCE(excluded.decision_timeout_seconds,decision_timeout_seconds),
 decision_l3_priorities=COALESCE(excluded.decision_l3_priorities,decision_l3_priorities),
 decision_low_action=COALESCE(excluded.decision_low_action,decision_low_action),
 decision_fallback_mode=COALESCE(excluded.decision_fallback_mode,decision_fallback_mode),
 topics=COALESCE(excluded.topics,topics),
 rss_max_items_per_pull=COALESCE(excluded.rss_max_items_per_pull,rss_max_items_per_pull),
 updated_at=unixepoch()`,
		p.DecisionEnabled,
		p.DecisionProvider,
		p.DecisionBaseURL,
		p.DecisionAPIKey,
		p.DecisionAccountID,
		p.DecisionAuthHeader,
		p.DecisionModel,
		p.DecisionJSONFormat,
		p.DecisionTimeoutSeconds,
		p.DecisionL3Priorities,
		p.DecisionLowAction,
		p.DecisionFallbackMode,
		p.Topics,
		p.RSSMaxItemsPerPull,
		clearKey)
	return err
}
func (s *Store) ResetAISettings() error {
	_, err := s.db.Exec("DELETE FROM ai_settings WHERE id=1")
	return err
}
