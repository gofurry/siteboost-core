package main

type Freebie struct {
	AppID         string `json:"appID"`
	Title         string `json:"title"`
	StoreURL      string `json:"storeURL"`
	CapsuleURL    string `json:"capsuleURL"`
	Released      string `json:"released"`
	OriginalPrice string `json:"originalPrice"`
	FinalPrice    string `json:"finalPrice"`
	Discount      string `json:"discount"`
	Source        string `json:"source"`
	Status        string `json:"status"`
	Note          string `json:"note"`
	FirstSeenAt   string `json:"firstSeenAt"`
	LastSeenAt    string `json:"lastSeenAt"`
	UpdatedAt     string `json:"updatedAt"`
}

type FreebieSnapshot struct {
	Items         []Freebie `json:"items"`
	Total         int       `json:"total"`
	TodoCount     int       `json:"todoCount"`
	ClaimedCount  int       `json:"claimedCount"`
	SkippedCount  int       `json:"skippedCount"`
	FailedCount   int       `json:"failedCount"`
	LastRefreshAt string    `json:"lastRefreshAt"`
	SourceURL     string    `json:"sourceURL"`
}
