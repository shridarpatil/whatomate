package contactutil

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nikyjain/whatomate/internal/models"
	"gorm.io/gorm"
)

// tombstonePrefix marks a duplicate that was merged but could not be hard
// deleted (some reference could not move). Its phone is rewritten so it never
// matches a lookup again and frees the canonical value for the survivor.
const tombstonePrefix = "merged:"

// MergeReport summarises one MergeDuplicateContacts run.
type MergeReport struct {
	Groups     int // phone numbers that had more than one contact
	Merged     int // duplicates folded into a survivor and deleted
	Tombstoned int // duplicates folded in but kept as a soft-deleted tombstone
	Renamed    int // single contacts whose phone was rewritten to canonical form
}

type refColumn struct {
	Table  string
	Column string
}

// MergeDuplicateContacts folds contacts that are the same number written
// differently (+91…, 91…, bare 10-digit, leading 0) into one contact per
// organisation. Every row that points at a duplicate (messages, sessions,
// notes, RSVP responses, …) is repointed to the survivor, the survivor keeps
// the best of each field, and its phone is stored in canonical form.
// Idempotent: a second run finds nothing to do.
func MergeDuplicateContacts(db *gorm.DB) (MergeReport, error) {
	var report MergeReport

	var orgIDs []uuid.UUID
	if err := db.Raw(`SELECT DISTINCT organization_id FROM contacts
		WHERE phone_number !~ '^[1-9][0-9]{10,14}$' AND phone_number NOT LIKE ?`, tombstonePrefix+"%").
		Scan(&orgIDs).Error; err != nil {
		return report, fmt.Errorf("find organisations with non-canonical phones: %w", err)
	}
	if len(orgIDs) == 0 {
		return report, nil
	}

	refs, err := contactReferenceColumns(db)
	if err != nil {
		return report, err
	}

	for _, orgID := range orgIDs {
		if err := mergeOrganisation(db, refs, orgID, &report); err != nil {
			return report, err
		}
	}
	return report, nil
}

func mergeOrganisation(db *gorm.DB, refs []refColumn, orgID uuid.UUID, report *MergeReport) error {
	var rows []models.Contact
	if err := db.Unscoped().
		Select("id", "organization_id", "phone_number", "deleted_at", "last_message_at", "created_at").
		Where("organization_id = ? AND phone_number NOT LIKE ?", orgID, tombstonePrefix+"%").
		Find(&rows).Error; err != nil {
		return fmt.Errorf("load contacts for org %s: %w", orgID, err)
	}

	groups := map[string][]uuid.UUID{}
	needsWork := map[string]bool{}
	for _, row := range rows {
		if !looksLikePhone(row.PhoneNumber) {
			continue
		}
		canonical := CanonicalPhone(row.PhoneNumber)
		if !IsMergeablePhone(canonical) {
			continue
		}
		groups[canonical] = append(groups[canonical], row.ID)
		if row.PhoneNumber != canonical {
			needsWork[canonical] = true
		}
	}

	keys := make([]string, 0, len(groups))
	for k, ids := range groups {
		if len(ids) > 1 || needsWork[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	for _, canonical := range keys {
		ids := groups[canonical]
		err := db.Transaction(func(tx *gorm.DB) error {
			return mergeGroup(tx, refs, canonical, ids, report)
		})
		if err != nil {
			return fmt.Errorf("merge contacts for %s in org %s: %w", canonical, orgID, err)
		}
	}
	return nil
}

func mergeGroup(tx *gorm.DB, refs []refColumn, canonical string, ids []uuid.UUID, report *MergeReport) error {
	var members []models.Contact
	if err := tx.Unscoped().Where("id IN ?", ids).Find(&members).Error; err != nil {
		return err
	}
	if len(members) == 0 {
		return nil
	}
	sortSurvivorFirst(members, canonical)
	survivor, dups := members[0], members[1:]
	now := time.Now().UTC()

	if len(dups) > 0 {
		report.Groups++
	}
	for _, dup := range dups {
		movedAll := moveReferences(tx, refs, dup.ID, survivor.ID)
		if movedAll && hardDelete(tx, dup.ID) {
			report.Merged++
			continue
		}
		if err := tx.Unscoped().Model(&models.Contact{}).Where("id = ?", dup.ID).Updates(map[string]any{
			"phone_number": tombstonePrefix + dup.ID.String(),
			"deleted_at":   now,
			"updated_at":   now,
		}).Error; err != nil {
			return err
		}
		report.Tombstoned++
	}

	updates := mergedFields(survivor, dups)
	updates["phone_number"] = canonical
	updates["updated_at"] = now
	if err := tx.Unscoped().Model(&models.Contact{}).Where("id = ?", survivor.ID).Updates(updates).Error; err != nil {
		return err
	}
	if len(dups) == 0 {
		report.Renamed++
	}
	return nil
}

// sortSurvivorFirst orders a group so the contact that should survive comes
// first: live before deleted, already-canonical, most recent chat, oldest.
func sortSurvivorFirst(members []models.Contact, canonical string) {
	sort.SliceStable(members, func(i, j int) bool {
		a, b := members[i], members[j]
		if a.DeletedAt.Valid != b.DeletedAt.Valid {
			return !a.DeletedAt.Valid
		}
		if (a.PhoneNumber == canonical) != (b.PhoneNumber == canonical) {
			return a.PhoneNumber == canonical
		}
		if at, bt := timeOrZero(a.LastMessageAt), timeOrZero(b.LastMessageAt); !at.Equal(bt) {
			return at.After(bt)
		}
		return a.CreatedAt.Before(b.CreatedAt)
	})
}

// moveReferences repoints every contact reference from dup to survivor. A
// table whose update fails (e.g. a unique constraint such as one RSVP
// response per event and contact) is left in place and reported false.
func moveReferences(tx *gorm.DB, refs []refColumn, dup, survivor uuid.UUID) bool {
	ok := true
	for _, ref := range refs {
		tx.SavePoint("contact_merge_ref")
		sql := fmt.Sprintf("UPDATE %s SET %s = ? WHERE %s = ?",
			quoteIdent(ref.Table), quoteIdent(ref.Column), quoteIdent(ref.Column))
		if err := tx.Exec(sql, survivor, dup).Error; err != nil {
			tx.RollbackTo("contact_merge_ref")
			ok = false
		}
	}
	return ok
}

func hardDelete(tx *gorm.DB, id uuid.UUID) bool {
	tx.SavePoint("contact_merge_delete")
	if err := tx.Exec("DELETE FROM contacts WHERE id = ?", id).Error; err != nil {
		tx.RollbackTo("contact_merge_delete")
		return false
	}
	return true
}

// contactReferenceColumns finds every uuid column that points at contacts:
// columns named contact_id plus any declared foreign key to contacts.
func contactReferenceColumns(db *gorm.DB) ([]refColumn, error) {
	var refs []refColumn
	err := db.Raw(`
		SELECT c.table_name AS "table", c.column_name AS "column"
		FROM information_schema.columns c
		JOIN information_schema.tables t
		  ON t.table_schema = c.table_schema AND t.table_name = c.table_name AND t.table_type = 'BASE TABLE'
		WHERE c.table_schema = current_schema() AND c.column_name = 'contact_id'
		  AND c.data_type = 'uuid' AND c.table_name <> 'contacts'
		UNION
		SELECT cl.relname AS "table", a.attname AS "column"
		FROM pg_constraint con
		JOIN pg_class cl ON cl.oid = con.conrelid
		JOIN pg_namespace n ON n.oid = cl.relnamespace
		JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = ANY(con.conkey)
		WHERE con.contype = 'f' AND n.nspname = current_schema()
		  AND con.confrelid = to_regclass(format('%I.contacts', current_schema()))
		  AND cl.relname <> 'contacts'
		ORDER BY 1, 2`).Scan(&refs).Error
	if err != nil {
		return nil, fmt.Errorf("discover contact references: %w", err)
	}
	return refs, nil
}

// looksLikePhone rejects identifiers that merely contain digits.
func looksLikePhone(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return false
	}
	for _, r := range trimmed {
		if !(r >= '0' && r <= '9') && !strings.ContainsRune("+ -().", r) {
			return false
		}
	}
	return true
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func timeOrZero(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
