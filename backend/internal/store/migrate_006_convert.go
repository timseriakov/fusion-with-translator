package store

import (
	"database/sql"
	"log/slog"
	"regexp"

	"github.com/0x2E/fusion/internal/pkg/md"
)

// htmlTagPattern matches likely HTML tags (<div>, <p>, <span>, etc.).
// Used to detect rows that still contain HTML and need conversion.
var htmlTagPattern = regexp.MustCompile(`<[a-zA-Z][a-zA-Z0-9]*[\s>/]`)

// convertHTMLToMarkdown converts existing HTML content in items and bookmarks
// to Markdown using the same converter as the feed parser (md.FromHTML).
//
// Idempotent: uses a _content_converted tracking table so already-converted
// rows are skipped even if the migration function is called again.
func convertHTMLToMarkdown(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS _content_converted (item_id INTEGER PRIMARY KEY)`); err != nil {
		return err
	}

	converted, err := convertColumn(db, "items", "content", false)
	if err != nil {
		return err
	}

	convertedTrans, err := convertColumn(db, "items", "translated_content", true)
	if err != nil {
		return err
	}

	slog.Info("HTML to Markdown conversion complete",
		"content_rows", converted,
		"translated_content_rows", convertedTrans,
	)
	return nil
}

// convertColumn converts HTML to Markdown for a single column.
// If nullable is true, the column is read as sql.NullString.
func convertColumn(db *sql.DB, table, column string, nullable bool) (int, error) {
	query := `SELECT id, ` + column + ` FROM ` + table +
		` WHERE ` + column + ` IS NOT NULL AND ` + column + ` != ''` +
		` AND id NOT IN (SELECT item_id FROM _content_converted)`

	rows, err := db.Query(query)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type row struct {
		id      int64
		content string
	}

	var pending []row
	for rows.Next() {
		var id int64
		if nullable {
			var content sql.NullString
			if err := rows.Scan(&id, &content); err != nil {
				return 0, err
			}
			if !content.Valid || !htmlTagPattern.MatchString(content.String) {
				continue
			}
			pending = append(pending, row{id, content.String})
		} else {
			var content string
			if err := rows.Scan(&id, &content); err != nil {
				return 0, err
			}
			if !htmlTagPattern.MatchString(content) {
				continue
			}
			pending = append(pending, row{id, content})
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	for _, r := range pending {
		converted, err := md.FromHTML(r.content)
		if err != nil {
			slog.Warn("failed to convert content, keeping original",
				"table", table, "column", column, "id", r.id, "error", err,
			)
			converted = r.content
		}

		if _, err := db.Exec(
			`UPDATE `+table+` SET `+column+` = :content WHERE id = :id`,
			sql.Named("content", converted),
			sql.Named("id", r.id),
		); err != nil {
			return 0, err
		}

		// Mark as converted even on conversion error (we kept original, no point retrying).
		if _, err := db.Exec(
			`INSERT OR IGNORE INTO _content_converted (item_id) VALUES (:id)`,
			sql.Named("id", r.id),
		); err != nil {
			return 0, err
		}
	}

	return len(pending), nil
}
