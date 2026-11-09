package mcp

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime"
	"mime/multipart"
	"net/textproto"
	"path/filepath"
	"strings"

	"github.com/valyala/fasthttp"
)

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

// knownContentTypes pins common upload types; mime.TypeByExtension depends on
// the host OS (Windows maps .csv to application/vnd.ms-excel).
var knownContentTypes = map[string]string{
	".csv": "text/csv", ".txt": "text/plain", ".json": "application/json", ".pdf": "application/pdf",
	".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".webp": "image/webp",
	".mp4": "video/mp4", ".3gp": "video/3gpp", ".mp3": "audio/mpeg", ".ogg": "audio/ogg", ".opus": "audio/ogg",
	".aac": "audio/aac", ".amr": "audio/amr", ".wav": "audio/wav",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
}

// decodeBase64 accepts standard or URL-safe base64, padded or not, and an
// optional data: URL prefix.
func decodeBase64(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "data:") {
		if i := strings.Index(s, ","); i >= 0 {
			s = s[i+1:]
		}
	}
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, s)
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, fmt.Errorf("file_base64 is not valid base64")
}

// maxUploadBase64Len is the padded base64 length of a maxUploadBytes file.
var maxUploadBase64Len = base64.StdEncoding.EncodedLen(maxUploadBytes)

// base64PayloadLen counts the base64 characters of s (ignoring an optional
// data: URL prefix and whitespace) without copying or decoding it.
func base64PayloadLen(s string) int {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "data:") {
		if i := strings.Index(s, ","); i >= 0 {
			s = s[i+1:]
		}
	}
	n := len(s)
	for _, ws := range []string{"\n", "\r", " ", "\t"} {
		n -= strings.Count(s, ws)
	}
	return n
}

// toolUploadFile implements whatomate_upload.
func (s *Server) toolUploadFile(ctx *fasthttp.RequestCtx, c *Caller, args map[string]any) *toolResult {
	method := strings.ToUpper(scalarString(args["method"]))
	if method == "" {
		method = "POST"
	}
	if method != "POST" && method != "PUT" && method != "PATCH" {
		return errorResult("upload method must be POST, PUT or PATCH")
	}
	entry, internalPath, err := resolveCallPath(method, scalarString(args["path"]), s.app.Config.Server.BasePath)
	if err != nil {
		return errorResult("%v", err)
	}
	if entry.Multipart == nil {
		return errorResult("%s %s is not a file-upload endpoint; use whatomate_call", entry.Method, entry.Path)
	}

	fields, _ := args["fields"].(map[string]any)
	if err := guardSelfCredentialChange(c, entry.Method, entry, internalPath, mapKeys(fields)); err != nil {
		return errorResult("%v", err)
	}

	raw, _ := args["file_base64"].(string)
	// Reject oversized input before decoding anything.
	if base64PayloadLen(raw) > maxUploadBase64Len {
		return errorResult("file is larger than 16 MB")
	}
	data, err := decodeBase64(raw)
	if err != nil {
		return errorResult("%v", err)
	}
	if len(data) == 0 {
		return errorResult("file is empty")
	}
	if len(data) > maxUploadBytes {
		return errorResult("file is larger than 16 MB")
	}
	filename := filepath.Base(strings.ReplaceAll(scalarString(args["filename"]), "\\", "/"))
	if filename == "" || filename == "." || filename == "/" {
		return errorResult("filename is required")
	}
	contentType := scalarString(args["content_type"])
	if contentType == "" {
		ext := strings.ToLower(filepath.Ext(filename))
		if ct, ok := knownContentTypes[ext]; ok {
			contentType = ct
		} else {
			contentType = mime.TypeByExtension(ext)
		}
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	field := scalarString(args["file_field"])
	if field == "" {
		field = entry.Multipart.FileField
	}
	if field == "" {
		field = "file"
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := w.WriteField(k, scalarString(v)); err != nil {
			return errorResult("failed to build form: %v", err)
		}
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
		quoteEscaper.Replace(field), quoteEscaper.Replace(filename)))
	h.Set("Content-Type", contentType)
	part, err := w.CreatePart(h)
	if err != nil {
		return errorResult("failed to build form: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		return errorResult("failed to build form: %v", err)
	}
	if err := w.Close(); err != nil {
		return errorResult("failed to build form: %v", err)
	}

	resp, err := s.dispatch(ctx, c, apiRequest{
		Method:      entry.Method,
		Path:        internalPath,
		Body:        buf.Bytes(),
		ContentType: w.FormDataContentType(),
	})
	if err != nil {
		return errorResult("%v", err)
	}
	return shapeResponse(resp)
}
