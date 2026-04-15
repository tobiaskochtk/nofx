# Selfhosted AI500

This service exposes AI500-compatible endpoints for NOFX without routing signal data through claw402.

Current implementation:
- Hyperliquid-backed universe and live snapshots
- SQLite persistence for price and open-interest history
- AI500 candidate scoring
- OI, price, and modeled netflow rankings
- Coin detail endpoint for `price`, `oi`, and `netflow`

Important:
- `institution` and `personal` netflow values are modeled proxy buckets in this selfhosted mode.
- OI deltas become more accurate as the service accumulates its own history.
