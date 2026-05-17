package mongo

import (
	"net/url"
	"strings"
)

// DatabaseName extracts the database name from a MongoDB URI.
// Example: mongodb://host:27017/mydb -> "mydb"
func DatabaseName(mongoURI string) string {
	u, err := url.Parse(mongoURI)
	if err != nil {
		return ""
	}
	db := strings.TrimPrefix(u.Path, "/")
	return db
}

