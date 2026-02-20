package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"gorm.io/gorm"
)

// ExportConfig defines allowed tables and their exportable columns
type ExportConfig struct {
	Model           interface{}
	Resource        string // For permission check
	AllowedColumns  []string
	DefaultColumns  []string
	ColumnLabels    map[string]string // Column name -> CSV header label
	ColumnTransform map[string]func(interface{}) string
}

// ImportConfig defines allowed tables and their importable columns
type ImportConfig struct {
	Model            interface{}
	Resource         string // For permission check
	RequiredColumns  []string
	OptionalColumns  []string
	ColumnTransform  map[string]func(string) (interface{}, error)
	UniqueColumn     string // Column to check for duplicates (e.g., "phone_number")
	BeforeCreate     func(db *gorm.DB, orgID uuid.UUID, record map[string]interface{}) error
}

// Supported export/import configurations
var exportConfigs = map[string]ExportConfig{
	"contacts": {
		Model:    &models.Contact{},
		Resource: "contacts",
		AllowedColumns: []string{
			"phone_number", "profile_name", "whats_app_account", "tags",
			"assigned_user_id", "last_message_at", "created_at", "updated_at",
		},
		DefaultColumns: []string{"phone_number", "profile_name", "tags"},
		ColumnLabels: map[string]string{
			"phone_number":      "Phone Number",
			"profile_name":      "Name",
			"whats_app_account": "WhatsApp Account",
			"tags":              "Tags",
			"assigned_user_id":  "Assigned User ID",
			"last_message_at":   "Last Message At",
			"created_at":        "Created At",
			"updated_at":        "Updated At",
		},
		ColumnTransform: map[string]func(interface{}) string{
			"tags": func(v interface{}) string {
				if v == nil {
					return ""
				}
				if tags, ok := v.(models.JSONBArray); ok {
					var tagStrs []string
					for _, t := range tags {
						if s, ok := t.(string); ok {
							tagStrs = append(tagStrs, s)
						}
					}
					return strings.Join(tagStrs, ",")
				}
				return ""
			},
			"last_message_at": func(v interface{}) string {
				if v == nil {
					return ""
				}
				if t, ok := v.(*time.Time); ok && t != nil {
					return t.Format(time.RFC3339)
				}
				return ""
			},
			"created_at": func(v interface{}) string {
				if t, ok := v.(time.Time); ok {
					return t.Format(time.RFC3339)
				}
				return ""
			},
			"updated_at": func(v interface{}) string {
				if t, ok := v.(time.Time); ok {
					return t.Format(time.RFC3339)
				}
				return ""
			},
			"assigned_user_id": func(v interface{}) string {
				if v == nil {
					return ""
				}
				if id, ok := v.(*uuid.UUID); ok && id != nil {
					return id.String()
				}
				return ""
			},
		},
	},
	"tags": {
		Model:          &models.Tag{},
		Resource:       "tags",
		AllowedColumns: []string{"name", "color", "description", "created_at"},
		DefaultColumns: []string{"name", "color", "description"},
		ColumnLabels: map[string]string{
			"name":        "Name",
			"color":       "Color",
			"description": "Description",
			"created_at":  "Created At",
		},
	},
}

var importConfigs = map[string]ImportConfig{
	"contacts": {
		Model:           &models.Contact{},
		Resource:        "contacts",
		RequiredColumns: []string{"phone_number"},
		OptionalColumns: []string{"profile_name", "whats_app_account", "tags"},
		UniqueColumn:    "phone_number",
		ColumnTransform: map[string]func(string) (interface{}, error){
			"phone_number": func(s string) (interface{}, error) {
				// Normalize phone number - remove + prefix
				phone := strings.TrimSpace(s)
				if len(phone) > 0 && phone[0] == '+' {
					phone = phone[1:]
				}
				if phone == "" {
					return nil, fmt.Errorf("phone number is required")
				}
				return phone, nil
			},
			"tags": func(s string) (interface{}, error) {
				if s == "" {
					return nil, nil
				}
				parts := strings.Split(s, ",")
				tags := make(models.JSONBArray, 0, len(parts))
				for _, p := range parts {
					p = strings.TrimSpace(p)
					if p != "" {
						tags = append(tags, p)
					}
				}
				return tags, nil
			},
		},
	},
	"tags": {
		Model:           &models.Tag{},
		Resource:        "tags",
		RequiredColumns: []string{"name"},
		OptionalColumns: []string{"color", "description"},
		UniqueColumn:    "name",
	},
}

// ExportRequest represents an export request
type ExportRequest struct {
	Table   string            `json:"table"`
	Columns []string          `json:"columns"`
	Filters map[string]string `json:"filters"`
	Format  string            `json:"format"` // csv (default), json
}

// ExportData handles generic data export
func (a *App) ExportData(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var req ExportRequest
	if err := json.Unmarshal(r.RequestCtx.PostBody(), &req); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	// Get export config
	config, ok := exportConfigs[req.Table]
	if !ok {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid table", nil, "")
	}

	// Check permission
	if !a.HasPermission(userID, config.Resource, models.ActionExport, orgID) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You do not have permission to export "+req.Table, nil, "")
	}

	// Validate and set columns
	columns := req.Columns
	if len(columns) == 0 {
		columns = config.DefaultColumns
	}

	// Validate columns against allowed set
	allowedSet := make(map[string]bool)
	for _, col := range config.AllowedColumns {
		allowedSet[col] = true
	}
	requestedCols := make(map[string]bool, len(columns))
	for _, col := range columns {
		if !allowedSet[col] {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, fmt.Sprintf("Column '%s' is not allowed for export", col), nil, "")
		}
		requestedCols[col] = true
	}

	// Build query
	query := a.DB.Model(config.Model).Where("organization_id = ?", orgID)

	// Apply filters
	if search, ok := req.Filters["search"]; ok && search != "" {
		searchPattern := "%" + search + "%"
		switch req.Table {
		case "contacts":
			// Use ILIKE for case-insensitive search on profile_name
			query = query.Where("phone_number LIKE ? OR profile_name ILIKE ?", searchPattern, searchPattern)
		case "tags":
			query = query.Where("name ILIKE ? OR description ILIKE ?", searchPattern, searchPattern)
		}
	}

	if tags, ok := req.Filters["tags"]; ok && tags != "" {
		tagList := strings.Split(tags, ",")
		conditions := make([]string, 0, len(tagList))
		args := make([]interface{}, 0, len(tagList))
		for _, tag := range tagList {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				// Use proper JSONB containment with explicit cast
				conditions = append(conditions, "tags @> ?::jsonb")
				tagJSON, _ := json.Marshal([]string{tag})
				args = append(args, string(tagJSON))
			}
		}
		if len(conditions) > 0 {
			query = query.Where("("+strings.Join(conditions, " OR ")+")", args...)
		}
	}

	// Select only needed columns plus id for scoping.
	// Build the list from server-controlled AllowedColumns to prevent SQL injection.
	// This ensures only server-defined strings are passed to GORM, not user input.
	safeColumns := make([]string, 0, len(columns))
	for _, col := range config.AllowedColumns {
		if requestedCols[col] {
			safeColumns = append(safeColumns, col)
		}
	}
	selectCols := append([]string{"id"}, safeColumns...)
	query = query.Select(selectCols)

	// Execute query
	rows, err := query.Rows()
	if err != nil {
		a.Log.Error("Failed to export data", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to export data", nil, "")
	}
	defer rows.Close()

	// Get column types
	colTypes, _ := rows.ColumnTypes()

	// Build CSV
	var buf strings.Builder
	writer := csv.NewWriter(&buf)

	// Write header using safe (server-controlled) column names
	header := make([]string, len(safeColumns))
	for i, col := range safeColumns {
		if label, ok := config.ColumnLabels[col]; ok {
			header[i] = label
		} else {
			header[i] = col
		}
	}
	_ = writer.Write(header)

	// Write rows
	for rows.Next() {
		// Create a slice of interface{} to scan into
		values := make([]interface{}, len(selectCols))
		valuePtrs := make([]interface{}, len(selectCols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			continue
		}

		// Convert to CSV row (skip id column which is at index 0)
		csvRow := make([]string, len(safeColumns))
		for i, col := range safeColumns {
			val := values[i+1] // +1 to skip id

			// Apply transform if available
			if transform, ok := config.ColumnTransform[col]; ok {
				csvRow[i] = transform(val)
			} else {
				csvRow[i] = formatExportValue(val, colTypes[i+1])
			}
		}
		// Apply phone masking for contacts export
		if req.Table == "contacts" && a.ShouldMaskPhoneNumbers(orgID) {
			for i, col := range safeColumns {
				if col == "phone_number" {
					csvRow[i] = MaskPhoneNumber(csvRow[i])
				} else if col == "profile_name" {
					csvRow[i] = MaskIfPhoneNumber(csvRow[i])
				}
			}
		}

		// Escape CSV injection: prefix dangerous first chars with a single quote
		// Only escape '=' and '@' which trigger formulas. '+' and '-' are skipped
		// because they appear in legitimate data (phone numbers, negative values).
		for j, cell := range csvRow {
			if len(cell) > 0 && (cell[0] == '=' || cell[0] == '@') {
				csvRow[j] = "'" + cell
			}
		}
		_ = writer.Write(csvRow)
	}

	writer.Flush()

	// Set response headers for CSV download
	filename := fmt.Sprintf("%s_export_%s.csv", req.Table, time.Now().Format("20060102_150405"))
	r.RequestCtx.Response.Header.Set("Content-Type", "text/csv")
	r.RequestCtx.Response.Header.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	r.RequestCtx.SetBody([]byte(buf.String()))

	return nil
}

// ImportDataRequest represents an import request metadata
type ImportDataRequest struct {
	Table         string            `json:"table"`
	ColumnMapping map[string]string `json:"column_mapping"` // CSV header -> DB column
	UpdateOnDup   bool              `json:"update_on_duplicate"`
}

// ImportData handles generic data import
func (a *App) ImportData(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Parse multipart form
	form, err := r.RequestCtx.MultipartForm()
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid multipart form", nil, "")
	}

	// Get table name
	tableValues := form.Value["table"]
	if len(tableValues) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "table is required", nil, "")
	}
	tableName := tableValues[0]

	// Get import config
	config, ok := importConfigs[tableName]
	if !ok {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid table", nil, "")
	}

	// Check permission
	if !a.HasPermission(userID, config.Resource, models.ActionImport, orgID) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You do not have permission to import "+tableName, nil, "")
	}

	// Get update_on_duplicate flag
	updateOnDup := false
	if updateValues := form.Value["update_on_duplicate"]; len(updateValues) > 0 {
		updateOnDup = updateValues[0] == "true"
	}

	// Get column mapping (optional)
	columnMapping := make(map[string]string)
	if mappingValues := form.Value["column_mapping"]; len(mappingValues) > 0 {
		_ = json.Unmarshal([]byte(mappingValues[0]), &columnMapping)
	}

	// Get CSV file
	files := form.File["file"]
	if len(files) == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "file is required", nil, "")
	}
	fileHeader := files[0]

	file, err := fileHeader.Open()
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Failed to read file", nil, "")
	}
	defer file.Close()

	// Limit CSV file size to 10MB
	const maxCSVSize = 10 << 20
	limitedReader := io.LimitReader(file, maxCSVSize+1)

	// Parse CSV
	reader := csv.NewReader(limitedReader)

	// Read header
	header, err := reader.Read()
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Failed to read CSV header", nil, "")
	}

	// Build column index mapping
	colIndex := make(map[string]int)
	for i, h := range header {
		h = strings.TrimSpace(h)
		// Apply column mapping if provided
		if mapped, ok := columnMapping[h]; ok {
			colIndex[mapped] = i
		} else {
			// Try to match by lowercase
			colIndex[strings.ToLower(h)] = i
		}
	}

	// Validate required columns exist
	for _, reqCol := range config.RequiredColumns {
		found := false
		for col := range colIndex {
			if strings.EqualFold(col, reqCol) || strings.EqualFold(col, strings.ReplaceAll(reqCol, "_", " ")) {
				found = true
				break
			}
		}
		if !found {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, fmt.Sprintf("Required column '%s' not found in CSV", reqCol), nil, "")
		}
	}

	// Normalize column index keys
	normalizedIndex := make(map[string]int)
	for col, idx := range colIndex {
		// Match against allowed columns
		for _, allowed := range append(config.RequiredColumns, config.OptionalColumns...) {
			if strings.EqualFold(col, allowed) ||
				strings.EqualFold(col, strings.ReplaceAll(allowed, "_", " ")) ||
				strings.EqualFold(col, config.getColumnLabel(allowed)) {
				normalizedIndex[allowed] = idx
				break
			}
		}
	}

	// Process rows (limit to 10,000)
	const maxImportRows = 10000
	var created, updated, skipped, errors int
	var errorMessages []string

	rowNum := 1
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		rowNum++
		if rowNum > maxImportRows+1 { // +1 for header row
			errorMessages = append(errorMessages, fmt.Sprintf("Import limited to %d rows", maxImportRows))
			break
		}
		if err != nil {
			errors++
			errorMessages = append(errorMessages, fmt.Sprintf("Row %d: failed to parse", rowNum))
			continue
		}

		// Build record map
		recordMap := make(map[string]interface{})
		recordMap["organization_id"] = orgID

		hasError := false
		for col, idx := range normalizedIndex {
			if idx >= len(record) {
				continue
			}
			val := strings.TrimSpace(record[idx])

			// Validate field length
			if len(val) > 10000 {
				hasError = true
				errorMessages = append(errorMessages, fmt.Sprintf("Row %d: %s exceeds max length", rowNum, col))
				break
			}

			// Apply transform if available
			if transform, ok := config.ColumnTransform[col]; ok {
				transformed, err := transform(val)
				if err != nil {
					hasError = true
					errorMessages = append(errorMessages, fmt.Sprintf("Row %d: %s - %s", rowNum, col, err.Error()))
					break
				}
				if transformed != nil {
					recordMap[col] = transformed
				}
			} else if val != "" {
				recordMap[col] = val
			}
		}

		if hasError {
			errors++
			continue
		}

		// Check for required fields
		for _, reqCol := range config.RequiredColumns {
			if _, ok := recordMap[reqCol]; !ok {
				hasError = true
				errorMessages = append(errorMessages, fmt.Sprintf("Row %d: missing required field '%s'", rowNum, reqCol))
				break
			}
		}

		if hasError {
			errors++
			continue
		}

		// Check for duplicate based on unique column
		if config.UniqueColumn != "" {
			uniqueVal := recordMap[config.UniqueColumn]
			var existing interface{}

			// Use reflection to create a new instance of the model type
			modelType := reflect.TypeOf(config.Model).Elem()
			existing = reflect.New(modelType).Interface()

			err := a.DB.Where("organization_id = ? AND "+config.UniqueColumn+" = ?", orgID, uniqueVal).First(existing).Error
			if err == nil {
				// Record exists
				if updateOnDup {
					// Update existing record
					delete(recordMap, "organization_id")
					delete(recordMap, config.UniqueColumn)
					if len(recordMap) > 0 {
						if err := a.DB.Model(existing).Updates(recordMap).Error; err != nil {
							errors++
							errorMessages = append(errorMessages, fmt.Sprintf("Row %d: failed to update", rowNum))
						} else {
							updated++
						}
					} else {
						skipped++
					}
				} else {
					skipped++
				}
				continue
			}
		}

		// Run BeforeCreate hook if defined
		if config.BeforeCreate != nil {
			if err := config.BeforeCreate(a.DB, orgID, recordMap); err != nil {
				errors++
				errorMessages = append(errorMessages, fmt.Sprintf("Row %d: %s", rowNum, err.Error()))
				continue
			}
		}

		// Create new record using model type
		recordMap["id"] = uuid.New()

		// Create instance of the model type and populate it via reflection
		modelType := reflect.TypeOf(config.Model).Elem()
		newRecordVal := reflect.New(modelType).Elem()

		// Populate struct fields from recordMap
		for key, val := range recordMap {
			// Convert snake_case to PascalCase for struct field names
			fieldName := snakeToPascal(key)
			field := newRecordVal.FieldByName(fieldName)
			if !field.IsValid() || !field.CanSet() {
				continue
			}

			// Set the value based on type
			if val != nil {
				valReflect := reflect.ValueOf(val)
				if valReflect.Type().AssignableTo(field.Type()) {
					field.Set(valReflect)
				} else if valReflect.Type().ConvertibleTo(field.Type()) {
					field.Set(valReflect.Convert(field.Type()))
				}
			}
		}

		// Use GORM to create the populated struct - this handles PostgreSQL properly
		newRecord := newRecordVal.Addr().Interface()
		if err := a.DB.Create(newRecord).Error; err != nil {
			errors++
			errorMessages = append(errorMessages, fmt.Sprintf("Row %d: failed to create - %s", rowNum, err.Error()))
			continue
		}
		created++
	}

	return r.SendEnvelope(map[string]interface{}{
		"created":  created,
		"updated":  updated,
		"skipped":  skipped,
		"errors":   errors,
		"messages": errorMessages,
	})
}

// GetExportConfig returns the export configuration for a table
func (a *App) GetExportConfig(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	tableName := r.RequestCtx.UserValue("table").(string)

	config, ok := exportConfigs[tableName]
	if !ok {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid table", nil, "")
	}

	// Check permission
	if !a.HasPermission(userID, config.Resource, models.ActionExport, orgID) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You do not have permission to export "+tableName, nil, "")
	}

	// Build column info
	columns := make([]map[string]string, len(config.AllowedColumns))
	for i, col := range config.AllowedColumns {
		label := col
		if l, ok := config.ColumnLabels[col]; ok {
			label = l
		}
		columns[i] = map[string]string{
			"key":   col,
			"label": label,
		}
	}

	return r.SendEnvelope(map[string]interface{}{
		"table":           tableName,
		"columns":         columns,
		"default_columns": config.DefaultColumns,
	})
}

// GetImportConfig returns the import configuration for a table
func (a *App) GetImportConfig(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	tableName := r.RequestCtx.UserValue("table").(string)

	config, ok := importConfigs[tableName]
	if !ok {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid table", nil, "")
	}

	// Check permission
	if !a.HasPermission(userID, config.Resource, models.ActionImport, orgID) {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "You do not have permission to import "+tableName, nil, "")
	}

	// Get labels from export config if available
	var columnLabels map[string]string
	if expConfig, ok := exportConfigs[tableName]; ok {
		columnLabels = expConfig.ColumnLabels
	}

	// Build column info
	requiredCols := make([]map[string]string, len(config.RequiredColumns))
	for i, col := range config.RequiredColumns {
		label := col
		if columnLabels != nil {
			if l, ok := columnLabels[col]; ok {
				label = l
			}
		}
		requiredCols[i] = map[string]string{
			"key":   col,
			"label": label,
		}
	}

	optionalCols := make([]map[string]string, len(config.OptionalColumns))
	for i, col := range config.OptionalColumns {
		label := col
		if columnLabels != nil {
			if l, ok := columnLabels[col]; ok {
				label = l
			}
		}
		optionalCols[i] = map[string]string{
			"key":   col,
			"label": label,
		}
	}

	return r.SendEnvelope(map[string]interface{}{
		"table":            tableName,
		"required_columns": requiredCols,
		"optional_columns": optionalCols,
		"unique_column":    config.UniqueColumn,
	})
}

// Helper function to convert snake_case to PascalCase
// Handles common acronyms like ID, URL, API, etc.
func snakeToPascal(s string) string {
	// Common acronyms that should be all uppercase
	acronyms := map[string]string{
		"id":   "ID",
		"url":  "URL",
		"api":  "API",
		"uuid": "UUID",
		"ip":   "IP",
		"http": "HTTP",
		"sql":  "SQL",
		"json": "JSON",
	}

	parts := strings.Split(s, "_")
	for i, part := range parts {
		lower := strings.ToLower(part)
		if acronym, ok := acronyms[lower]; ok {
			parts[i] = acronym
		} else if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}

// Helper function to format values for CSV export
func formatExportValue(v interface{}, colType interface{}) string {
	if v == nil {
		return ""
	}

	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case int, int32, int64:
		return fmt.Sprintf("%d", val)
	case float32, float64:
		return fmt.Sprintf("%f", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case time.Time:
		return val.Format(time.RFC3339)
	case *time.Time:
		if val != nil {
			return val.Format(time.RFC3339)
		}
		return ""
	case uuid.UUID:
		return val.String()
	case *uuid.UUID:
		if val != nil {
			return val.String()
		}
		return ""
	default:
		// Try JSON marshal for complex types
		if b, err := json.Marshal(val); err == nil {
			return string(b)
		}
		return fmt.Sprintf("%v", val)
	}
}

// Helper method to get column label
func (c ImportConfig) getColumnLabel(col string) string {
	if expConfig, ok := exportConfigs[c.Resource]; ok {
		if label, ok := expConfig.ColumnLabels[col]; ok {
			return label
		}
	}
	return col
}
