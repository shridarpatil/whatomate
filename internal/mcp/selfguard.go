package mcp

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
)

// selfUserRoute is the user-edit route an AI app must not use to change the
// caller's own login credentials.
const selfUserRoute = "/api/users/{id}"

// selfProtectedFields are the body keys that would change the caller's own
// login. JSON decoding in the handlers matches field names case-insensitively,
// so the check does too.
var selfProtectedFields = []string{"password", "email"}

var errSelfCredentialChange = errors.New("changing your own password or email is not available to AI apps; do it in Whatomate under your profile")

// guardSelfCredentialChange rejects a PUT/PATCH of /api/users/{id} for the
// caller's own user ID whose body sets password or email. Editing other users
// (including setting their password) is left to the handler's users:write
// permission check. keys are the top-level body or form field names.
func guardSelfCredentialChange(c *Caller, method string, e *Entry, internalPath string, keys []string) error {
	if c == nil || c.UserID == uuid.Nil || e == nil || e.Path != selfUserRoute {
		return nil
	}
	if method != "PUT" && method != "PATCH" {
		return nil
	}
	params, ok := matchTemplate(splitPath(e.Path), splitPath(decodedPath(internalPath)))
	if !ok {
		return nil
	}
	target, err := uuid.Parse(strings.TrimSpace(params["id"]))
	if err != nil || target != c.UserID {
		return nil
	}
	for _, k := range keys {
		for _, f := range selfProtectedFields {
			if strings.EqualFold(strings.TrimSpace(k), f) {
				return errSelfCredentialChange
			}
		}
	}
	return nil
}

// jsonObjectKeys returns the top-level keys of a JSON object body (nil for any
// other JSON value, which the handlers reject anyway).
func jsonObjectKeys(body []byte) []string {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// mapKeys returns the keys of a form-field argument map.
func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
