package database

import (
	"context"
	"fmt"
	"strings"

	"tsuruyomi/internal/models"
)

// GetSourceList returns all configured sources from the database.
func GetSourceList(ctx context.Context) ([]models.SourceDataClass, error) {
	if DB == nil {
		return []models.SourceDataClass{}, nil
	}

	query := `
		SELECT s.id, s.name, s.lang, COALESCE(e.icon_url, ''), COALESCE(e.pkg_name, ''), COALESCE(s.content_warning, e.content_warning, 0)
		FROM suwayomi.source s
		LEFT JOIN suwayomi.extension e ON s.extension = e.id
		ORDER BY s.name ASC
	`

	rows, err := DB.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying sources: %w", err)
	}
	defer rows.Close()

	list := []models.SourceDataClass{}
	for rows.Next() {
		var src models.SourceDataClass
		var idVal int64
		var iconURL, pkgName string
		var cwInt int
		err := rows.Scan(&idVal, &src.Name, &src.Lang, &iconURL, &pkgName, &cwInt)
		if err != nil {
			return nil, fmt.Errorf("scanning source row: %w", err)
		}
		src.ID = fmt.Sprintf("%d", idVal)
		if src.Lang != "" && src.Lang != "all" {
			src.DisplayName = fmt.Sprintf("%s (%s)", src.Name, strings.ToUpper(src.Lang))
		} else {
			src.DisplayName = src.Name
		}
		src.SupportsLatest = true
		src.IconURL = iconURL
		src.PkgName = pkgName
		switch cwInt {
		case 1:
			src.ContentWarning = "SUGGESTIVE"
			src.IsNsfw = false
		case 2:
			src.ContentWarning = "NSFW"
			src.IsNsfw = true
		default:
			src.ContentWarning = "SAFE"
			src.IsNsfw = false
		}
		list = append(list, src)
	}

	return list, nil
}

// GetSourceByID retrieves a single source details by its ID.
func GetSourceByID(ctx context.Context, id int64) (*models.SourceDataClass, error) {
	if DB == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	query := `
		SELECT s.id, s.name, s.lang, COALESCE(e.icon_url, ''), COALESCE(e.pkg_name, ''), COALESCE(s.content_warning, e.content_warning, 0)
		FROM suwayomi.source s
		LEFT JOIN suwayomi.extension e ON s.extension = e.id
		WHERE s.id = $1
		LIMIT 1
	`

	var src models.SourceDataClass
	var idVal int64
	var iconURL, pkgName string
	var cwInt int
	err := DB.QueryRow(ctx, query, id).Scan(&idVal, &src.Name, &src.Lang, &iconURL, &pkgName, &cwInt)
	if err != nil {
		return nil, err
	}

	src.ID = fmt.Sprintf("%d", idVal)
	if src.Lang != "" && src.Lang != "all" {
		src.DisplayName = fmt.Sprintf("%s (%s)", src.Name, strings.ToUpper(src.Lang))
	} else {
		src.DisplayName = src.Name
	}
	src.SupportsLatest = true
	src.IconURL = iconURL
	src.PkgName = pkgName
	switch cwInt {
	case 1:
		src.ContentWarning = "SUGGESTIVE"
		src.IsNsfw = false
	case 2:
		src.ContentWarning = "NSFW"
		src.IsNsfw = true
	default:
		src.ContentWarning = "SAFE"
		src.IsNsfw = false
	}

	return &src, nil
}
