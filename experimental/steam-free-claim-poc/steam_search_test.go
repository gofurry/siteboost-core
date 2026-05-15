package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSteamSearchSourceFetchFiltersFreeToKeepCandidates(t *testing.T) {
	html := `
<a href="https://store.steampowered.com/app/100/Free_Game/" data-ds-appid="100" class="search_result_row">
  <div class="search_capsule"><img src="https://cdn.example/free.jpg"></div>
  <span class="title">Free Game</span>
  <div class="search_released">May 15, 2026</div>
  <div class="discount_block" data-price-final="0" data-discount="100">
    <div class="discount_original_price">$9.99</div>
    <div class="discount_final_price">$0.00</div>
  </div>
</a>
<a href="https://store.steampowered.com/app/200/Always_Free/" data-ds-appid="200" class="search_result_row">
  <span class="title">Always Free</span>
  <div class="discount_block" data-price-final="0" data-discount="100">
    <div class="discount_final_price">$0.00</div>
  </div>
</a>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, `{"success":1,"results_html":%q,"total_count":2}`, html)
	}))
	defer server.Close()

	source := NewSteamSearchSource(server.Client())
	source.searchURL = server.URL

	items, err := source.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].AppID != "100" || items[0].Title != "Free Game" {
		t.Fatalf("unexpected item: %#v", items[0])
	}
	if items[0].OriginalPrice != "$9.99" || items[0].FinalPrice != "$0.00" {
		t.Fatalf("unexpected prices: %#v", items[0])
	}
}
