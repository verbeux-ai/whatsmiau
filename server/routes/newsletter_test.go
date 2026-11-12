package routes

import (
	"sort"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/verbeux-ai/whatsmiau/server/controllers"
)

// TestNewsletterRoutesMatchTheDocumentedInventory keeps the Echo routes and the
// Swagger inventory in sync for the channel endpoints.
func TestNewsletterRoutesMatchTheDocumentedInventory(t *testing.T) {
	app := echo.New()
	registerNewsletter(app.Group("/v1/instance/:instance/newsletter"), controllers.NewNewsletters(nil))

	var registered []string
	for _, route := range app.Routes() {
		path := strings.ReplaceAll(route.Path, ":instance", "{instance}")
		registered = append(registered, route.Method+" "+path)
	}

	var documented []string
	for _, operation := range DocumentedV1Operations() {
		if strings.HasPrefix(operation.Path, "/v1/instance/{instance}/newsletter/") {
			documented = append(documented, operation.Method+" "+operation.Path)
		}
	}

	sort.Strings(registered)
	sort.Strings(documented)
	if len(documented) != 8 || strings.Join(registered, "\n") != strings.Join(documented, "\n") {
		t.Fatalf("registered routes:\n%s\n\ndocumented operations:\n%s", strings.Join(registered, "\n"), strings.Join(documented, "\n"))
	}
}
