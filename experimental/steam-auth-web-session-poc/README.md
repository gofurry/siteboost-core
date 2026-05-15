# Steam Auth Web Session POC

This Wails v2 + Vue + Go experiment validates a narrow local capability:

1. Start a Steam WebBrowser auth session via QR code or account credentials.
2. Handle Steam Guard code submission and confirmation polling.
3. Save only the refresh token, encrypted locally with Windows DPAPI.
4. Exchange the refresh token for Steam Web cookies in the Go backend.
5. Verify the web session without returning tokens or cookies to Vue.

## Safety Boundary

- Only intended for the user's own Steam account.
- Does not save Steam passwords.
- Does not read browser cookies or Steam client local state.
- Does not expose refresh tokens, access tokens, `steamLoginSecure`, `sessionid`, or `Set-Cookie` values to the frontend.
- Does not claim games, automate purchases, bypass verification, or run background batch actions.

## Development

```powershell
npm install --prefix frontend
npm install --prefix frontend qrcode @types/qrcode
wails generate module
npm run build --prefix frontend
go test ./...
wails dev -m
```

If module download stalls on a proxy, use:

```powershell
$env:GOPROXY = "direct"
go mod tidy
```
