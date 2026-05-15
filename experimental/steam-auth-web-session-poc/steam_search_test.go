package main

import "testing"

func TestParseSearchResultsKeepsOnlyFreePromotionalApps(t *testing.T) {
	html := `
<a class="search_result_row" href="https:\/\/store.steampowered.com\/app\/3587490\/Demo" data-ds-appid="3587490">
  <div class="search_capsule"><img src="https:\/\/cdn.example\/capsule.jpg"></div>
  <span class="title">Terrors to Unveil - Day Off</span>
  <div class="search_released">Dec 12, 2025</div>
  <div class="discount_block" data-discount="100" data-price-final="0">
    <div class="discount_original_price">$4.99</div>
    <div class="discount_final_price">Free</div>
  </div>
</a>
<a class="search_result_row" href="https://store.steampowered.com/app/10/Paid" data-ds-appid="10">
  <span class="title">Paid</span>
  <div class="discount_block" data-discount="50" data-price-final="499"></div>
</a>`

	items, err := parseSearchResults(html)
	if err != nil {
		t.Fatalf("parseSearchResults() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].AppID != "3587490" {
		t.Fatalf("AppID = %q, want 3587490", items[0].AppID)
	}
	if items[0].StoreURL != "https://store.steampowered.com/app/3587490/Demo" {
		t.Fatalf("StoreURL = %q", items[0].StoreURL)
	}
	if items[0].CapsuleURL != "https://cdn.example/capsule.jpg" {
		t.Fatalf("CapsuleURL = %q", items[0].CapsuleURL)
	}
}
