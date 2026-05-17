# Steam Free Claim POC

This Wails2 + Vue + TypeScript experiment validates a narrow SteamScope workflow:

1. Fetch current Steam Store search results for Windows specials where the final price is free and the discount is 100%.
2. Keep a local state file for candidates and manual statuses.
3. Open the official Steam login page or an individual app store page in the system browser.
4. Let the user claim each item manually, then mark it as claimed, skipped, or failed in the app.

## Safety Boundary

- Does not store Steam passwords.
- Does not import Steam Guard secrets or local tokens.
- Does not automate bulk claiming.
- Does not reuse browser cookies or bypass Steam verification.
- Stores only local candidate/status data under the user config directory.

## Development

```powershell
npm install --prefix frontend
.\dev.ps1
```

The app can also be validated with:

```powershell
go test ./...
npm run build --prefix frontend
```

If `wails dev` stops at `Executing: go mod tidy`, run:

```powershell
$env:GOPROXY = "direct"
go mod tidy
wails dev -m
```

## Notes

The current source is intentionally simple: Steam Store search result HTML returned by the public search endpoint. If this proves useful, the parsing and source abstraction are good candidates for `steam-go`.
