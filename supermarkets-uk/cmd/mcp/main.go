package main

import (
	"log"

	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/auth"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/client"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/datasource"
	"github.com/jbeshir/mcp-servers/supermarkets-uk/internal/server"
)

func main() {
	logins := auth.LoadLoginFlags()
	if len(logins) > 0 {
		for id := range logins {
			log.Printf("login enabled for %s", id)
		}
	}

	cookieDir, err := auth.DefaultCookieDir()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("cookie directory: %s", cookieDir)
	store, err := auth.NewCookieStore(cookieDir)
	if err != nil {
		log.Fatal(err)
	}

	cached := auth.LoadCachedCookies(logins, store)

	// Determine which flagged supermarkets still need interactive login.
	needLogin := make(map[datasource.SupermarketID]bool)
	for id := range logins {
		if len(cached[id]) == 0 {
			needLogin[id] = true
			log.Printf("%s will prompt for login on first use", id)
		}
	}

	c := client.NewClient(client.Config{
		Cookies:   cached,
		NeedLogin: needLogin,
		Store:     store,
	})
	srv := server.NewServer(c)

	err = srv.Run()
	c.Close()
	if err != nil {
		log.Fatal(err)
	}
}
