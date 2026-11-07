package handlers

import (
	"encoding/csv"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nikyjain/whatomate/internal/contactutil"
	"github.com/nikyjain/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/xuri/excelize/v2"
	"github.com/zerodha/fastglue"
)

const (
	rsvpGuestImportMaxSize = 10 << 20
	rsvpGuestImportMaxRows = 10000
)

type addRSVPGuestsRequest struct {
	ContactIDs []string `json:"contact_ids"`
}

type rsvpGuestCandidate struct {
	ID           uuid.UUID         `json:"id"`
	ProfileName  string            `json:"profile_name"`
	PhoneNumber  string            `json:"phone_number"`
	Tags         models.JSONBArray `json:"tags"`
	AlreadyAdded bool              `json:"already_added"`
}

func (a *App) ListRSVPGuestCandidates(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceRSVP, models.ActionRead); err != nil {
		return nil
	}
	eventID, err := parsePathUUID(r, "id", "RSVP event")
	if err != nil {
		return nil
	}
	if _, err = findByIDAndOrg[models.RSVPEvent](a.DB, r, eventID, orgID, "RSVP event"); err != nil {
		return nil
	}
	pg := parsePagination(r)
	q := a.DB.Model(&models.Contact{}).Where("organization_id = ? AND is_active = ?", orgID, true)
	if search := strings.TrimSpace(string(r.RequestCtx.QueryArgs().Peek("search"))); search != "" {
		like := "%" + search + "%"
		q = q.Where("profile_name ILIKE ? OR phone_number LIKE ?", like, like)
	}
	var total int64
	q.Count(&total)
	var contacts []models.Contact
	if err := pg.Apply(q.Order("profile_name, phone_number")).Find(&contacts).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list contacts", nil, "")
	}
	ids := make([]uuid.UUID, 0, len(contacts))
	for i := range contacts {
		ids = append(ids, contacts[i].ID)
	}
	added := map[uuid.UUID]bool{}
	if len(ids) > 0 {
		var rows []models.RSVPResponse
		a.DB.Select("contact_id").Where("rsvp_event_id = ? AND contact_id IN ?", eventID, ids).Find(&rows)
		for i := range rows {
			added[rows[i].ContactID] = true
		}
	}
	out := make([]rsvpGuestCandidate, 0, len(contacts))
	for i := range contacts {
		c := contacts[i]
		out = append(out, rsvpGuestCandidate{ID: c.ID, ProfileName: c.ProfileName, PhoneNumber: c.PhoneNumber, Tags: c.Tags, AlreadyAdded: added[c.ID]})
	}
	return r.SendEnvelope(map[string]interface{}{"contacts": out, "total": total, "page": pg.Page, "limit": pg.Limit})
}

func (a *App) AddRSVPGuests(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceRSVP, models.ActionWrite); err != nil {
		return nil
	}
	eventID, err := parsePathUUID(r, "id", "RSVP event")
	if err != nil {
		return nil
	}
	event, err := findByIDAndOrg[models.RSVPEvent](a.DB, r, eventID, orgID, "RSVP event")
	if err != nil {
		return nil
	}
	var req addRSVPGuestsRequest
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}
	added, existing, invalid := 0, 0, 0
	seen := map[uuid.UUID]bool{}
	for _, raw := range req.ContactIDs {
		id, parseErr := uuid.Parse(raw)
		if parseErr != nil || seen[id] {
			invalid++
			continue
		}
		seen[id] = true
		var contact models.Contact
		if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&contact).Error; err != nil {
			invalid++
			continue
		}
		if a.rsvpGuestListed(eventID, id) {
			existing++
			continue
		}
		a.seedPendingRSVPResponse(orgID, event, id, contact.PhoneNumber, models.RSVPGuestSourceContactSelection)
		added++
	}
	return r.SendEnvelope(map[string]int{"added": added, "already_added": existing, "invalid": invalid})
}

type rsvpGuestRosterRow struct {
	models.RSVPResponse
	JourneyStatus  string     `gorm:"column:journey_status" json:"journey_status"`
	ReminderCount  int        `gorm:"column:reminder_count" json:"reminder_count"`
	LastReminderAt *time.Time `gorm:"column:last_reminder_at" json:"last_reminder_at,omitempty"`
}

func (a *App) ListRSVPGuests(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceRSVP, models.ActionRead); err != nil {
		return nil
	}
	eventID, err := parsePathUUID(r, "id", "RSVP event")
	if err != nil {
		return nil
	}
	event, err := findByIDAndOrg[models.RSVPEvent](a.DB, r, eventID, orgID, "RSVP event")
	if err != nil {
		return nil
	}
	pg := parsePagination(r)
	journeySQL := "CASE WHEN rsvp_responses.responded_at IS NOT NULL THEN 'responded' WHEN rsvp_responses.rsvp_started_at IS NOT NULL THEN 'in_progress' ELSE 'not_started' END"
	q := a.DB.Model(&models.RSVPResponse{}).
		Where("rsvp_responses.organization_id = ? AND rsvp_responses.rsvp_event_id = ?", orgID, eventID).
		Joins(`LEFT JOIN (SELECT rsvp_response_id, COUNT(*) AS reminder_count, MAX(attempted_at) AS last_reminder_at
			FROM rsvp_reminder_deliveries WHERE status = 'sent' AND deleted_at IS NULL GROUP BY rsvp_response_id) rd
			ON rd.rsvp_response_id = rsvp_responses.id`)
	if v := strings.TrimSpace(string(r.RequestCtx.QueryArgs().Peek("journey_status"))); v != "" {
		q = q.Where(journeySQL+" = ?", v)
	}
	if v := strings.TrimSpace(string(r.RequestCtx.QueryArgs().Peek("attendance"))); v != "" {
		q = q.Where("rsvp_responses.attendance = ?", v)
	}
	if v := strings.TrimSpace(string(r.RequestCtx.QueryArgs().Peek("member_status"))); v != "" {
		attendance := map[string]string{"attending": "yes", "not_attending": "no", "maybe": "maybe", "pending": "pending"}[v]
		if attendance != "" {
			q = q.Where("rsvp_responses.attendance = ?", attendance)
		}
	}
	if v := strings.TrimSpace(string(r.RequestCtx.QueryArgs().Peek("spouse_status"))); v != "" {
		mobileField := event.SpouseMobileField
		if strings.TrimSpace(mobileField) == "" {
			mobileField = "spouse_mobile"
		}
		contributors := event.HeadcountContributors
		if len(contributors) == 0 {
			contributors = legacyHeadcountContributors(event.AttendanceField)
		}
		spouseKey := deriveSpouseAttendanceKey(contributors, event.AttendanceField)
		spouseKeyTitle := spouseKey + "_title"
		// spouseKey is user-editable configuration (event.HeadcountContributors), so
		// it is bound as a parameter below via `?`, never interpolated into the SQL
		// string, the same pattern mobileField already follows in phoneSQL.
		answerSQL := `LOWER(TRIM(COALESCE(NULLIF(rsvp_responses.answers ->> ?, ''), rsvp_responses.answers ->> ?, '')))`
		phoneSQL := `LENGTH(regexp_replace(COALESCE(rsvp_responses.answers ->> ?, ''), '[^0-9]', '', 'g'))`
		switch v {
		case "attending":
			q = q.Where(answerSQL+" IN ?", spouseKey, spouseKeyTitle, []string{"yes", "attending"})
		case "not_attending":
			q = q.Where(answerSQL+" IN ?", spouseKey, spouseKeyTitle, []string{"no", "not attending", "not_attending"})
		case "maybe":
			q = q.Where(answerSQL+" = 'maybe'", spouseKey, spouseKeyTitle)
		case "pending":
			q = q.Where("("+answerSQL+" IN ? AND "+phoneSQL+" < 10) OR NOT ("+answerSQL+" IN ? OR "+answerSQL+" IN ? OR "+answerSQL+" = 'maybe')",
				spouseKey, spouseKeyTitle, []string{"yes", "attending"}, mobileField,
				spouseKey, spouseKeyTitle, []string{"yes", "attending"},
				spouseKey, spouseKeyTitle, []string{"no", "not attending", "not_attending"},
				spouseKey, spouseKeyTitle)
		}
	}
	if v := strings.TrimSpace(string(r.RequestCtx.QueryArgs().Peek("source"))); v != "" {
		q = q.Where("rsvp_responses.source = ?", v)
	}
	if v := strings.TrimSpace(string(r.RequestCtx.QueryArgs().Peek("reminded"))); v == "yes" {
		q = q.Where("COALESCE(rd.reminder_count, 0) > 0")
	} else if v == "no" {
		q = q.Where("COALESCE(rd.reminder_count, 0) = 0")
	}
	if search := strings.TrimSpace(string(r.RequestCtx.QueryArgs().Peek("search"))); search != "" {
		like := "%" + search + "%"
		q = q.Joins("LEFT JOIN contacts roster_contact ON roster_contact.id = rsvp_responses.contact_id").
			Where("rsvp_responses.phone_number LIKE ? OR roster_contact.profile_name ILIKE ?", like, like)
	}
	var total int64
	q.Count(&total)
	var notStarted, inProgress, responded int64
	a.DB.Model(&models.RSVPResponse{}).Where("organization_id = ? AND rsvp_event_id = ? AND rsvp_started_at IS NULL AND responded_at IS NULL", orgID, eventID).Count(&notStarted)
	a.DB.Model(&models.RSVPResponse{}).Where("organization_id = ? AND rsvp_event_id = ? AND rsvp_started_at IS NOT NULL AND responded_at IS NULL", orgID, eventID).Count(&inProgress)
	a.DB.Model(&models.RSVPResponse{}).Where("organization_id = ? AND rsvp_event_id = ? AND responded_at IS NOT NULL", orgID, eventID).Count(&responded)
	var rows []rsvpGuestRosterRow
	selectSQL := "rsvp_responses.*, " + journeySQL + " AS journey_status, COALESCE(rd.reminder_count, 0) AS reminder_count, rd.last_reminder_at"
	if err := pg.Apply(q.Select(selectSQL).Preload("Contact").Order("rsvp_responses.created_at DESC")).Find(&rows).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list RSVP guests", nil, "")
	}
	return r.SendEnvelope(map[string]interface{}{"guests": rows, "total": total, "page": pg.Page, "limit": pg.Limit, "journey_counts": map[string]int64{"not_started": notStarted, "in_progress": inProgress, "responded": responded}})
}

type rsvpImportError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

func rsvpImportColumn(header []string, aliases ...string) int {
	for i, value := range header {
		v := strings.ToLower(strings.TrimSpace(value))
		for _, alias := range aliases {
			if v == alias {
				return i
			}
		}
	}
	return -1
}

func readRSVPGuestRows(name string, reader io.Reader) ([][]string, error) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".csv":
		return csv.NewReader(reader).ReadAll()
	case ".xlsx":
		book, err := excelize.OpenReader(reader)
		if err != nil {
			return nil, err
		}
		defer book.Close() //nolint:errcheck
		for _, sheet := range book.GetSheetList() {
			rows, rowErr := book.GetRows(sheet)
			if rowErr != nil {
				return nil, rowErr
			}
			if len(rows) > 0 {
				return rows, nil
			}
		}
		return nil, fmt.Errorf("spreadsheet is empty")
	default:
		return nil, fmt.Errorf("only CSV and XLSX files are supported")
	}
}

func normalizeRSVPImportPhone(raw string) string {
	digits := normalizePhoneDigits(raw)
	if len(digits) == 10 {
		digits = "91" + digits
	}
	if len(digits) < 10 || len(digits) > 15 {
		return ""
	}
	return digits
}

func (a *App) ImportRSVPGuests(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceRSVP, models.ActionWrite); err != nil {
		return nil
	}
	eventID, err := parsePathUUID(r, "id", "RSVP event")
	if err != nil {
		return nil
	}
	event, err := findByIDAndOrg[models.RSVPEvent](a.DB, r, eventID, orgID, "RSVP event")
	if err != nil {
		return nil
	}
	form, err := r.RequestCtx.MultipartForm()
	if err != nil || len(form.File["file"]) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "A CSV or XLSX file is required", nil, "")
	}
	header := form.File["file"][0]
	if header.Size > rsvpGuestImportMaxSize {
		return r.SendErrorEnvelope(fasthttp.StatusRequestEntityTooLarge, "Guest file must be 10MB or smaller", nil, "")
	}
	file, err := header.Open()
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Failed to read guest file", nil, "")
	}
	defer file.Close() //nolint:errcheck
	rows, err := readRSVPGuestRows(header.Filename, io.LimitReader(file, rsvpGuestImportMaxSize+1))
	if err != nil || len(rows) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Failed to parse guest file: "+fmt.Sprint(err), nil, "")
	}
	if len(rows)-1 > rsvpGuestImportMaxRows {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Guest file may contain at most 10000 rows", nil, "")
	}
	phoneCol := rsvpImportColumn(rows[0], "phone", "mobile", "phone_number", "phonenumber", "number")
	nameCol := rsvpImportColumn(rows[0], "name", "guest_name", "recipient_name", "profile_name")
	if phoneCol < 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "A phone or mobile column is required", nil, "")
	}
	result := struct {
		Rows            int               `json:"rows"`
		GuestsAdded     int               `json:"guests_added"`
		AlreadyAdded    int               `json:"already_added"`
		ContactsCreated int               `json:"contacts_created"`
		Skipped         int               `json:"skipped"`
		Errors          []rsvpImportError `json:"errors"`
	}{Rows: len(rows) - 1, Errors: []rsvpImportError{}}
	seen := map[string]bool{}
	for idx, row := range rows[1:] {
		rowNum := idx + 2
		if phoneCol >= len(row) {
			result.Skipped++
			result.Errors = append(result.Errors, rsvpImportError{Row: rowNum, Message: "Missing phone number"})
			continue
		}
		phone := normalizeRSVPImportPhone(row[phoneCol])
		if phone == "" {
			result.Skipped++
			result.Errors = append(result.Errors, rsvpImportError{Row: rowNum, Message: "Invalid phone number"})
			continue
		}
		if seen[phone] {
			result.Skipped++
			result.Errors = append(result.Errors, rsvpImportError{Row: rowNum, Message: "Duplicate phone number in file"})
			continue
		}
		seen[phone] = true
		name := ""
		if nameCol >= 0 && nameCol < len(row) {
			name = strings.TrimSpace(row[nameCol])
		}
		var contact models.Contact
		if found, err := contactutil.FindContactUnscoped(a.DB, orgID, phone); err == nil {
			contact = *found
			if contact.DeletedAt.Valid {
				a.DB.Unscoped().Model(&contact).Updates(map[string]interface{}{"deleted_at": nil, "is_active": true})
			}
		} else {
			contact = models.Contact{BaseModel: models.BaseModel{ID: uuid.New()}, OrganizationID: orgID, PhoneNumber: phone, ProfileName: name, WhatsAppAccount: event.WhatsAppAccount, IsActive: true, Tags: models.JSONBArray{}, Metadata: models.JSONB{}}
			if createErr := a.DB.Create(&contact).Error; createErr != nil {
				result.Skipped++
				result.Errors = append(result.Errors, rsvpImportError{Row: rowNum, Message: "Failed to create contact"})
				continue
			}
			result.ContactsCreated++
		}
		if a.rsvpGuestListed(eventID, contact.ID) {
			result.AlreadyAdded++
			continue
		}
		a.seedPendingRSVPResponse(orgID, event, contact.ID, contact.PhoneNumber, models.RSVPGuestSourceSpreadsheetImport)
		result.GuestsAdded++
	}
	return r.SendEnvelope(result)
}
