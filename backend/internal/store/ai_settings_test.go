package store

import (
	"news_panel/internal/model"
	"os"
	"runtime"
	"testing"
)

func TestAISettingsPersistence(t *testing.T) {
	db, path := setupTestDB(t)
	if err := db.SecureAIFiles(); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		for _, suffix := range []string{"", "-wal", "-shm"} {
			info, err := os.Stat(path + suffix)
			if os.IsNotExist(err) && suffix != "" {
				continue
			}
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatalf("incorrect SQLite permissions for %q", suffix)
			}
		}
	}
	no := false
	if err := db.UpdateAISettings(model.AISettings{DecisionEnabled: &no, DecisionAPIKey: strPtr("fixture")}, false); err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateAISettings(model.AISettings{DecisionAPIKey: strPtr(""), Topics: strPtr("updates")}, false); err != nil {
		t.Fatal(err)
	}
	closeStore(t, db)
	db, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	defer closeStore(t, db)
	row, err := db.GetAISettings()
	if err != nil || row.Topics == nil || *row.Topics != "updates" || row.DecisionEnabled == nil || *row.DecisionEnabled || row.DecisionAPIKey == nil || *row.DecisionAPIKey != "fixture" {
		t.Fatal("partial settings did not survive reopen")
	}
	if err = db.UpdateAISettings(model.AISettings{}, true); err != nil {
		t.Fatal(err)
	}
	row, err = db.GetAISettings()
	if err != nil || row.DecisionAPIKey != nil || row.Topics == nil {
		t.Fatal("key clear changed other overrides")
	}
	if err = db.ResetAISettings(); err != nil {
		t.Fatal(err)
	}
	row, err = db.GetAISettings()
	if err != nil || row != (model.AISettings{}) {
		t.Fatal("reset left overrides")
	}
}
