package bitwarden

import (
	"encoding/json"
	"reflect"
)

func cloneDestination(raw map[string]json.RawMessage) map[string]json.RawMessage {
	destination := make(map[string]json.RawMessage, 4)
	for _, key := range []string{"name", "folderId", "organizationId", "collectionIds"} {
		if value, ok := raw[key]; ok {
			destination[key] = append(json.RawMessage(nil), value...)
		}
	}
	return destination
}

func cloneContentsMatch(expected, actual map[string]json.RawMessage) bool {
	if value := actual["deletedDate"]; len(value) > 0 && string(value) != "null" {
		return false
	}
	for _, key := range []string{"name", "type", "organizationId", "folderId", "collectionIds", "login", "card", "identity", "sshKey", "secureNote", "notes", "fields", "favorite", "reprompt"} {
		var a, b any
		if value := expected[key]; len(value) > 0 {
			if json.Unmarshal(value, &a) != nil {
				return false
			}
		}
		if value := actual[key]; len(value) > 0 {
			if json.Unmarshal(value, &b) != nil {
				return false
			}
		}
		if !reflect.DeepEqual(a, b) {
			return false
		}
	}
	return true
}
