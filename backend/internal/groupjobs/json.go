// backend/internal/groupjobs/json.go
package groupjobs

import (
	"encoding/json"
)

// jsonMarshal is a small wrapper so the audit-log insert can stay readable
// inline. The encoding package is the only third-party-style dep this
// helper file pulls in.
func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }
