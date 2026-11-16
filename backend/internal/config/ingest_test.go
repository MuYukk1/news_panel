package config

import "testing"

func TestIngestTokenFromEnvironment(t *testing.T) {
	t.Setenv("FUSION_ALLOW_EMPTY_PASSWORD", "true")
	for _, token := range []string{"", "plugin-secret"} {
		t.Setenv("INGEST_TOKEN", token)
		cfg, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if cfg.IngestToken != token {
			t.Fatal("INGEST_TOKEN was not loaded exactly")
		}
	}
}
