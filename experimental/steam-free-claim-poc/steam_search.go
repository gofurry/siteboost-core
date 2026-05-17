package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

const steamSearchURL = "https://store.steampowered.com/search/results/?query&start=0&count=50&dynamic_data=&force_infinite=1&specials=1&maxprice=free&os=win&snr=1_7_7_7000_7&infinite=1"

type SteamSearchSource struct {
	client    *http.Client
	searchURL string
}

type steamSearchResponse struct {
	Success     int    `json:"success"`
	ResultsHTML string `json:"results_html"`
	TotalCount  int    `json:"total_count"`
}

func NewSteamSearchSource(client *http.Client) *SteamSearchSource {
	if client == nil {
		client = http.DefaultClient
	}
	return &SteamSearchSource{client: client, searchURL: steamSearchURL}
}

func (s *SteamSearchSource) Fetch(ctx context.Context) ([]Freebie, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("User-Agent", "SteamScope Free Claim POC/0.1")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("steam search returned HTTP %d", resp.StatusCode)
	}

	var payload steamSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Success != 1 {
		return nil, fmt.Errorf("steam search returned success=%d", payload.Success)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(payload.ResultsHTML))
	if err != nil {
		return nil, err
	}

	items := make([]Freebie, 0)
	doc.Find("a.search_result_row").Each(func(_ int, selection *goquery.Selection) {
		appID, _ := selection.Attr("data-ds-appid")
		appID = strings.TrimSpace(appID)
		if appID == "" {
			return
		}

		discountBlock := selection.Find(".discount_block").First()
		discount, _ := discountBlock.Attr("data-discount")
		finalCents, _ := discountBlock.Attr("data-price-final")
		if strings.TrimSpace(discount) != "100" || strings.TrimSpace(finalCents) != "0" {
			return
		}

		originalPrice := cleanText(selection.Find(".discount_original_price").First().Text())
		if originalPrice == "" {
			return
		}

		storeURL, _ := selection.Attr("href")
		capsuleURL, _ := selection.Find(".search_capsule img").First().Attr("src")

		items = append(items, Freebie{
			AppID:         appID,
			Title:         cleanText(selection.Find(".title").First().Text()),
			StoreURL:      cleanSteamURL(storeURL),
			CapsuleURL:    cleanSteamURL(capsuleURL),
			Released:      cleanText(selection.Find(".search_released").First().Text()),
			OriginalPrice: originalPrice,
			FinalPrice:    cleanText(selection.Find(".discount_final_price").First().Text()),
			Discount:      "-" + discount + "%",
			Source:        "Steam Store search",
			Status:        statusTodo,
		})
	})

	return items, nil
}

func cleanText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func cleanSteamURL(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), `\/`, `/`)
}
