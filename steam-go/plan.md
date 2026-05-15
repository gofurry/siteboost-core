# steam-go Plan

## Candidates From SteamScope Free Claim POC

- Store search client: wrap Steam Store search result endpoint with query parameters for specials, platform, price, pagination, and locale.
- Free-to-keep detector: parse store search result HTML and classify apps where `data-discount=100`, `data-price-final=0`, and an original price is present.
- Promotion source abstraction: expose a small interface for multiple freebie sources so Steam Store search, SteamDB-like pages, RSS feeds, or curated feeds can be combined later.
- App promotion model: normalize app id, title, store URL, capsule image, release date, original price, final price, discount, source, and first/last seen timestamps.
- Manual claim helper boundary: provide official Steam Store URLs and status hints without handling passwords, Steam Guard secrets, cookies, or automated claiming.
