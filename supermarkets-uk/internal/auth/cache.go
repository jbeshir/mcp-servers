package auth

import (
	"log"
	"net/http"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
)

// LoadCachedCookies loads previously saved cookies for flagged supermarkets.
// It does not trigger interactive login — only returns what's on disk.
func LoadCachedCookies(
	logins map[datasource.SupermarketID]bool,
	store *CookieStore,
) map[datasource.SupermarketID][]*http.Cookie {
	result := make(map[datasource.SupermarketID][]*http.Cookie)

	for id := range logins {
		cookies, err := store.Load(id)
		if err != nil {
			log.Printf(
				"warning: failed to load cached cookies for %s: %v",
				id, err,
			)
			continue
		}
		if len(cookies) > 0 && HasSessionCookie(id, cookies) {
			result[id] = cookies
			log.Printf("loaded cached session for %s (%d cookies)", id, len(cookies))
		} else if len(cookies) > 0 {
			log.Printf(
				"cached cookies for %s missing session cookie, will re-login",
				id,
			)
		}
	}

	return result
}
