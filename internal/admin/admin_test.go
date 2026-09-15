package admin

import "testing"

func TestNormalizeQuery(t *testing.T) {
	query := NormalizeQuery(Query{Search: "  demo  ", Status: " ACTIVE ", Page: -1, PageSize: 500})
	if query.Search != "demo" || query.Status != "active" || query.Page != 1 || query.PageSize != 100 {
		t.Fatalf("unexpected normalized query: %+v", query)
	}
}

func TestNewPage(t *testing.T) {
	page := newPage([]string{"one"}, Query{Page: 2, PageSize: 20}, 41)
	if page.TotalPages != 3 || page.Total != 41 || page.Page != 2 {
		t.Fatalf("unexpected page: %+v", page)
	}
}
