package database

import (
	"context"
	"fmt"
	"log"
	"strings"

	"tsuruyomi/internal/models"
)

// DefaultExtensionStores contains the out-of-the-box extension stores.
var DefaultExtensionStores = []models.ExtensionStore{
	{
		Name:           "Tsuruyomi Native Go Extensions",
		IndexURL:       "https://raw.githubusercontent.com/devil1210/tsuruyomi-extensions/main/index.json",
		BadgeLabel:     "TSURU",
		SigningKey:     "tsuruyomi",
		ContactWebsite: "https://github.com/devil1210/tsuruyomi-extensions",
		IsLegacy:       false,
		TotalCount:     6,
	},
	{
		Name:           "Keiyoushi",
		IndexURL:       "https://raw.githubusercontent.com/keiyoushi/extensions/repo/index.pb",
		BadgeLabel:     "KEI",
		SigningKey:     "9add655a78e96c4ec7a53ef89dccb557cb5d767489fac5e785d671a5a75d4da2",
		ContactWebsite: "https://keiyoushi.github.io",
		ContactDiscord: "https://discord.gg/3FbCpdKbdY",
		IsLegacy:       false,
		TotalCount:     1377,
	},
}

// EnsureDefaultExtensionStores seeds the extensionstore table if it is currently empty.
func EnsureDefaultExtensionStores(ctx context.Context) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	var count int
	err := DB.QueryRow(ctx, "SELECT COUNT(*) FROM suwayomi.extensionstore").Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		log.Println("[DATABASE] Seeding default extension repositories...")
		for _, s := range DefaultExtensionStores {
			query := `
				INSERT INTO suwayomi.extensionstore 
				(index_url, name, badge_label, signing_key, contact_website, contact_discord, is_legacy, extension_list_url)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				ON CONFLICT (index_url) DO NOTHING
			`
			_, err := DB.Exec(ctx, query, s.IndexURL, s.Name, s.BadgeLabel, s.SigningKey, s.ContactWebsite, s.ContactDiscord, s.IsLegacy, s.ExtensionListURL)
			if err != nil {
				log.Printf("[DATABASE] Warning inserting default extension store %s: %v\n", s.Name, err)
			}
		}
	}

	return nil
}

// GetExtensionStores retrieves all configured extension stores ordered by ID.
func GetExtensionStores(ctx context.Context) ([]models.ExtensionStore, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Make sure defaults are present if completely empty
	_ = EnsureDefaultExtensionStores(ctx)

	rows, err := DB.Query(ctx, `
		SELECT id, index_url, name, badge_label, signing_key, contact_website, 
		       COALESCE(contact_discord, ''), is_legacy, COALESCE(extension_list_url, '')
		FROM suwayomi.extensionstore
		ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stores []models.ExtensionStore
	for rows.Next() {
		var s models.ExtensionStore
		err := rows.Scan(
			&s.ID, &s.IndexURL, &s.Name, &s.BadgeLabel, &s.SigningKey,
			&s.ContactWebsite, &s.ContactDiscord, &s.IsLegacy, &s.ExtensionListURL,
		)
		if err == nil {
			if strings.Contains(s.IndexURL, "devil1210") || strings.Contains(s.IndexURL, "tsuruyomi") {
				s.TotalCount = 6
			} else if strings.Contains(s.IndexURL, "keiyoushi") {
				s.TotalCount = 1377
			} else {
				s.TotalCount = 1
			}
			stores = append(stores, s)
		}
	}

	return stores, nil
}

// AddExtensionStore adds or updates a repository by its index URL.
func AddExtensionStore(ctx context.Context, store models.ExtensionStore) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	cleanURL := strings.TrimSpace(store.IndexURL)
	if cleanURL == "" {
		return fmt.Errorf("indexUrl cannot be empty")
	}

	if store.Name == "" {
		store.Name = "Custom Extension Repo"
		if strings.Contains(cleanURL, "keiyoushi") {
			store.Name = "Keiyoushi"
		} else if strings.Contains(cleanURL, "tsuruyomi") {
			store.Name = "Tsuruyomi Native Go Extensions"
		}
	}
	if store.BadgeLabel == "" {
		store.BadgeLabel = "REPO"
	}

	query := `
		INSERT INTO suwayomi.extensionstore 
		(index_url, name, badge_label, signing_key, contact_website, contact_discord, is_legacy, extension_list_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (index_url) DO UPDATE 
		SET name = EXCLUDED.name, badge_label = EXCLUDED.badge_label
	`
	_, err := DB.Exec(ctx, query, cleanURL, store.Name, store.BadgeLabel, store.SigningKey, store.ContactWebsite, store.ContactDiscord, store.IsLegacy, store.ExtensionListURL)
	return err
}

// RemoveExtensionStore removes an extension repository by index URL.
func RemoveExtensionStore(ctx context.Context, indexURL string) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	cleanURL := strings.TrimSpace(indexURL)
	if cleanURL == "" {
		return fmt.Errorf("indexUrl cannot be empty")
	}

	log.Printf("[DATABASE] Deleting extension store with index_url: %s\n", cleanURL)
	_, err := DB.Exec(ctx, "DELETE FROM suwayomi.extensionstore WHERE index_url = $1", cleanURL)
	return err
}

// SetExtensionStores updates the stores table to match a list of index URLs.
func SetExtensionStores(ctx context.Context, urls []string) error {
	if DB == nil {
		return fmt.Errorf("database not initialized")
	}

	// 1. Delete stores not in urls
	if len(urls) == 0 {
		_, err := DB.Exec(ctx, "DELETE FROM suwayomi.extensionstore")
		return err
	}

	// Keep existing stores that match URLs, remove the rest
	current, err := GetExtensionStores(ctx)
	if err != nil {
		return err
	}

	urlMap := make(map[string]bool)
	for _, u := range urls {
		uClean := strings.TrimSpace(u)
		if uClean != "" {
			urlMap[uClean] = true
		}
	}

	for _, s := range current {
		if !urlMap[s.IndexURL] {
			_ = RemoveExtensionStore(ctx, s.IndexURL)
		}
	}

	// Add any new URLs
	for u := range urlMap {
		_ = AddExtensionStore(ctx, models.ExtensionStore{
			IndexURL: u,
		})
	}

	return nil
}
