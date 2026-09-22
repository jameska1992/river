// Package backfill holds one-shot, idempotent data-maintenance passes run at
// boot, after the schema migration. They exist to populate columns that were
// added after rows already existed, or that a producer may not have supplied.
package backfill

import (
	"log"
	"os"

	"river-api/internal/models"

	"gorm.io/gorm"
)

// sizeTarget describes one file-bearing table the size backfill walks.
type sizeTarget struct {
	label string
	model any
}

var sizeTargets = []sizeTarget{
	{"movie", &models.Movie{}},
	{"episode", &models.Episode{}},
	{"track", &models.Track{}},
	{"chapter", &models.AudiobookChapter{}},
}

// row is the projection the backfill reads: just the id + file_path of rows
// that still need measuring.
type row struct {
	ID       string
	FilePath string
}

// Sizes populates size_bytes for any file-bearing row that has a file_path but
// no recorded size yet (size_bytes = 0). It stats the file on disk (FilePath is
// an absolute path river-api can read — it's what the stream handler opens) and
// writes the byte count back.
//
// This is the safety net behind the transcoders sending size_bytes at ingest:
// it fills in rows created before the column existed, rows from pre-transcoded
// libraries (no transcoder runs, so nothing sends a size), and any create that
// omitted it. Idempotent — a row measured on a previous boot has a non-zero
// size and is skipped; a file that can't be stat'd is left at 0 to be retried
// on a later boot. Best-effort: failures are logged, never fatal.
func Sizes(db *gorm.DB) {
	var measured, missing int
	for _, t := range sizeTargets {
		var rows []row
		if err := db.Model(t.model).
			Where("size_bytes = 0 AND file_path <> ''").
			Select("id, file_path").
			Scan(&rows).Error; err != nil {
			log.Printf("WARN backfill-sizes: query %s: %v", t.label, err)
			continue
		}
		for _, r := range rows {
			info, err := os.Stat(r.FilePath)
			if err != nil {
				// File not reachable from here (mount mismatch, deleted,
				// still transcoding) — leave at 0 and retry next boot.
				missing++
				continue
			}
			if err := db.Model(t.model).
				Where("id = ?", r.ID).
				UpdateColumn("size_bytes", info.Size()).Error; err != nil {
				log.Printf("WARN backfill-sizes: update %s %s: %v", t.label, r.ID, err)
				continue
			}
			measured++
		}
	}
	if measured > 0 || missing > 0 {
		log.Printf("INFO backfill-sizes: measured %d file(s), %d unreachable (left for a later boot)", measured, missing)
	}
}
