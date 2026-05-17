package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
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

type appDetailsResponse map[string]struct {
	Success bool `json:"success"`
	Data    struct {
		PackageGroups []struct {
			Title string `json:"title"`
			Subs  []struct {
				PackageID                int64  `json:"packageid"`
				PercentSavingsText       string `json:"percent_savings_text"`
				OptionText               string `json:"option_text"`
				IsFreeLicense            bool   `json:"is_free_license"`
				PriceInCentsWithDiscount int    `json:"price_in_cents_with_discount"`
			} `json:"subs"`
		} `json:"package_groups"`
	} `json:"data"`
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

	items, err := parseSearchResults(payload.ResultsHTML)
	if err != nil {
		return nil, err
	}

	for i := range items {
		packageID, packageTitle, err := s.ResolveFreePackage(ctx, items[i].AppID)
		if err == nil {
			items[i].PackageID = packageID
			items[i].PackageTitle = packageTitle
		} else {
			items[i].Note = err.Error()
		}
	}

	return items, nil
}

func (s *SteamSearchSource) ResolveFreePackage(ctx context.Context, appID string) (int64, string, error) {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return 0, "", fmt.Errorf("appID is required")
	}

	endpoint := "https://store.steampowered.com/api/appdetails?appids=" + url.QueryEscape(appID) + "&cc=us&l=english"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("User-Agent", "SteamScope Free Claim POC/0.1")

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, "", fmt.Errorf("appdetails returned HTTP %d", resp.StatusCode)
	}

	var payload appDetailsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, "", err
	}
	details, ok := payload[appID]
	if !ok || !details.Success {
		return 0, "", fmt.Errorf("appdetails did not include app %s", appID)
	}

	for _, group := range details.Data.PackageGroups {
		for _, sub := range group.Subs {
			if sub.PackageID == 0 || sub.PriceInCentsWithDiscount != 0 {
				continue
			}
			if sub.IsFreeLicense || strings.Contains(sub.PercentSavingsText, "100") || strings.Contains(strings.ToLower(sub.OptionText), "free") {
				title := cleanText(stripTags(sub.OptionText))
				if title == "" {
					title = group.Title
				}
				return sub.PackageID, title, nil
			}
		}
	}
	return 0, "", fmt.Errorf("no free package found for app %s", appID)
}

func parseSearchResults(resultsHTML string) ([]Freebie, error) {
	doc, err := html.Parse(strings.NewReader(resultsHTML))
	if err != nil {
		return nil, err
	}

	rows := findNodes(doc, func(node *html.Node) bool {
		return node.Type == html.ElementNode && node.Data == "a" && hasClass(node, "search_result_row")
	})

	items := make([]Freebie, 0, len(rows))
	for _, row := range rows {
		appID := strings.TrimSpace(attr(row, "data-ds-appid"))
		if appID == "" {
			continue
		}

		discountBlock := firstNode(row, func(node *html.Node) bool {
			return node.Type == html.ElementNode && hasClass(node, "discount_block")
		})
		discount := attr(discountBlock, "data-discount")
		finalCents := attr(discountBlock, "data-price-final")
		if strings.TrimSpace(discount) != "100" || strings.TrimSpace(finalCents) != "0" {
			continue
		}

		originalPrice := cleanText(textOfFirst(row, "discount_original_price"))
		if originalPrice == "" {
			continue
		}

		capsule := firstNode(row, func(node *html.Node) bool {
			return node.Type == html.ElementNode && node.Data == "img" && hasAncestorClass(node, "search_capsule")
		})

		items = append(items, Freebie{
			AppID:         appID,
			Title:         cleanText(textOfFirst(row, "title")),
			StoreURL:      cleanSteamURL(attr(row, "href")),
			CapsuleURL:    cleanSteamURL(attr(capsule, "src")),
			Released:      cleanText(textOfFirst(row, "search_released")),
			OriginalPrice: originalPrice,
			FinalPrice:    cleanText(textOfFirst(row, "discount_final_price")),
			Discount:      "-" + discount + "%",
			Source:        "Steam Store search",
			Status:        freebieStatusTodo,
		})
	}
	return items, nil
}

func cleanText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func cleanSteamURL(value string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), `\/`, `/`)
}

func stripTags(value string) string {
	doc, err := html.Parse(strings.NewReader(value))
	if err != nil {
		return value
	}
	return textContent(doc)
}

func textOfFirst(root *html.Node, class string) string {
	node := firstNode(root, func(node *html.Node) bool {
		return node.Type == html.ElementNode && hasClass(node, class)
	})
	return textContent(node)
}

func firstNode(root *html.Node, match func(*html.Node) bool) *html.Node {
	for _, node := range findNodes(root, match) {
		return node
	}
	return nil
}

func findNodes(root *html.Node, match func(*html.Node) bool) []*html.Node {
	if root == nil {
		return nil
	}
	var nodes []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if match(node) {
			nodes = append(nodes, node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return nodes
}

func textContent(root *html.Node) string {
	if root == nil {
		return ""
	}
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			builder.WriteString(node.Data)
			builder.WriteByte(' ')
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return builder.String()
}

func attr(node *html.Node, name string) string {
	if node == nil {
		return ""
	}
	for _, item := range node.Attr {
		if item.Key == name {
			return item.Val
		}
	}
	return ""
}

func hasClass(node *html.Node, class string) bool {
	for _, item := range strings.Fields(attr(node, "class")) {
		if item == class {
			return true
		}
	}
	return false
}

func hasAncestorClass(node *html.Node, class string) bool {
	for parent := node.Parent; parent != nil; parent = parent.Parent {
		if hasClass(parent, class) {
			return true
		}
	}
	return false
}
