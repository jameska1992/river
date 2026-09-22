package backfill

import (
	"fmt"
	"log"

	"river-api/internal/models"

	"gorm.io/gorm"
)

// creditRepointTable is one cast/crew join table whose person_id references
// need to be re-pointed from a duplicate person onto the canonical one during
// the merge. mediaCol is the media foreign-key column; hasJob marks the crew
// tables whose primary key also includes job (so a collision must match on it).
type creditRepointTable struct {
	table    string
	mediaCol string
	hasJob   bool
}

var creditRepointTables = []creditRepointTable{
	{"movie_casts", "movie_id", false},
	{"movie_crews", "movie_id", true},
	{"tv_show_casts", "tv_show_id", false},
	{"tv_show_crews", "tv_show_id", true},
}

// MergeDuplicatePeople collapses pre-existing duplicate manually-added people
// (tmdb_id IS NULL) that share a name (case-insensitive) into a single
// canonical row, re-pointing their cast/crew credits, then deleting the extras.
//
// This is the one-time data cleanup behind #193: before the find-or-create fix,
// every manual credit inserted a brand-new Person, so the same human could hold
// many rows and surface repeatedly in search. Idempotent + best-effort: once
// merged there are no duplicate groups left, so a later boot is a single cheap
// query. Runs after the schema migration, like the size backfill.
func MergeDuplicatePeople(db *gorm.DB) {
	// Names (lowercased) that have more than one manual person row.
	var dupNames []string
	if err := db.Model(&models.Person{}).
		Where("tmdb_id IS NULL").
		Group("LOWER(name)").
		Having("COUNT(*) > 1").
		Pluck("LOWER(name)", &dupNames).Error; err != nil {
		log.Printf("WARN merge-people: find duplicate names: %v", err)
		return
	}
	if len(dupNames) == 0 {
		return
	}

	merged := 0
	for _, lname := range dupNames {
		n, err := mergeOneNameGroup(db, lname)
		if err != nil {
			log.Printf("WARN merge-people: merge %q: %v", lname, err)
			continue
		}
		merged += n
	}
	if merged > 0 {
		log.Printf("INFO merge-people: merged %d duplicate person row(s) across %d name(s)", merged, len(dupNames))
	}
}

// mergeOneNameGroup merges all manual people sharing lname into the oldest one,
// in a single transaction. Returns how many duplicate rows were removed.
func mergeOneNameGroup(db *gorm.DB, lname string) (int, error) {
	removed := 0
	err := db.Transaction(func(tx *gorm.DB) error {
		var people []models.Person
		if err := tx.Where("tmdb_id IS NULL AND LOWER(name) = ?", lname).
			Order("created_at ASC, id ASC").
			Find(&people).Error; err != nil {
			return err
		}
		if len(people) < 2 {
			return nil // another worker got here first, or the group is gone
		}
		canonical := people[0].ID.String()
		for _, dup := range people[1:] {
			if err := repointCredits(tx, dup.ID.String(), canonical); err != nil {
				return err
			}
			if err := tx.Delete(&models.Person{}, "id = ?", dup.ID).Error; err != nil {
				return err
			}
			removed++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return removed, nil
}

// repointCredits moves every cast/crew credit from dupID onto canonicalID
// across all four join tables. A dup credit that would collide with an existing
// canonical credit on the same title (and job, for crew) is dropped first so
// the re-point can't violate the composite primary key.
func repointCredits(tx *gorm.DB, dupID, canonicalID string) error {
	for _, t := range creditRepointTables {
		collision := fmt.Sprintf(
			"SELECT 1 FROM %[1]s c WHERE c.%[2]s = %[1]s.%[2]s AND c.person_id = ?",
			t.table, t.mediaCol)
		if t.hasJob {
			collision += fmt.Sprintf(" AND c.job = %s.job", t.table)
		}
		del := fmt.Sprintf("DELETE FROM %[1]s WHERE person_id = ? AND EXISTS (%[2]s)", t.table, collision)
		if err := tx.Exec(del, dupID, canonicalID).Error; err != nil {
			return fmt.Errorf("dedupe %s: %w", t.table, err)
		}
		upd := fmt.Sprintf("UPDATE %s SET person_id = ? WHERE person_id = ?", t.table)
		if err := tx.Exec(upd, canonicalID, dupID).Error; err != nil {
			return fmt.Errorf("repoint %s: %w", t.table, err)
		}
	}
	return nil
}
