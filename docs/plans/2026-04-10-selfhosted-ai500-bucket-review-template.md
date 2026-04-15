# Selfhosted AI500 Bucket Review Template

Purpose:

- review 12-24h live runtime after enabling bucket-aware logging
- verify whether `adaptive` or `exploration` picks actually appear
- decide whether the current selector is too strict, too loose, or balanced

## Review Window

- Start:
- End:
- Duration:
- Environment:
- Running traders:

## Commands

Selfhosted service bucket log:

```powershell
docker logs --since 24h selfhosted-ai500 2>&1 |
  Select-String -Pattern "refresh complete|bucket_mix"
```

Backend AI500 fetch bucket log:

```powershell
docker logs --since 24h nofx-trading 2>&1 |
  Select-String -Pattern "AI500 bucket mix|Successfully fetched [0-9]+ AI500 coins|Strategy engine fetched candidate coins|No candidate coins available"
```

OI Momentum Scalper focus:

```powershell
docker logs --since 24h nofx-trading 2>&1 |
  Select-String -Pattern "\\[OI Momentum Scalper\\].*(Strategy engine fetched candidate coins|No candidate coins available|Successfully fetched quantitative data)"
```

2026 trader focus:

```powershell
docker logs --since 24h nofx-trading 2>&1 |
  Select-String -Pattern "\\[2026\\].*(Strategy engine fetched candidate coins|No candidate coins available|Successfully fetched quantitative data)"
```

Current live selector state:

```powershell
$token = docker exec selfhosted-ai500 sh -lc 'printf %s "$SELFHOSTED_AI500_AUTH_TOKEN"'
Invoke-RestMethod -Uri ("http://127.0.0.1:8081/api/ai500/stats?auth=" + $token) | ConvertTo-Json -Depth 8
Invoke-RestMethod -Uri ("http://127.0.0.1:8081/api/debug/rankings?kind=ai500&limit=16&auth=" + $token) | ConvertTo-Json -Depth 8
```

## Snapshot Summary

- Universe count:
- Selected count:
- Eligible count:
- Threshold match count:
- Score threshold:
- Adaptive threshold:
- Exploration count:

## Bucket Usage

| Bucket | Observed count | Typical symbols | Notes |
| --- | --- | --- | --- |
| `primary` |  |  |  |
| `adaptive` |  |  |  |
| `fallback_eligible` |  |  |  |
| `exploration` |  |  |  |

## Trader Impact

| Trader | Candidate count trend | Empty-cycle rate | Quant fetch trend | Notes |
| --- | --- | --- | --- | --- |
| `2026` |  |  |  |  |
| `OI Momentum Scalper` |  |  |  |  |
| `Multi-Signal Swing Trader` |  |  |  |  |
| `Aggressive Altcoin Hunter` |  |  |  |  |

## Quality Check

- Did `adaptive` appear at all?
- Did `exploration` appear at all?
- If yes, were the exploration symbols mostly soft misses or obvious junk?
- Did empty-cycle frequency decrease for provider-backed strategies?
- Did quant fetch count increase together with candidate count?
- Did any low-depth or high-risk symbols slip in unexpectedly?

## Symbol Notes

Good additions:

- Symbol:
  Reason:

- Symbol:
  Reason:

Bad additions:

- Symbol:
  Reason:

- Symbol:
  Reason:

Important misses:

- Symbol:
  Reason:

- Symbol:
  Reason:

## Decision Rules

Keep as-is if:

- `adaptive` and `exploration` are rare but useful
- empty cycles drop without obvious quality decay
- no repeated weak low-depth names appear

Tighten if:

- `exploration` shows mostly weak or noisy names
- candidate count rises but quant/decision quality does not
- repeated low-conviction symbols dominate the extra slots

Loosen if:

- empty cycles remain common
- `adaptive` never appears despite thin live pools
- claw402/reference comparisons still show frequent missed opportunities

## Follow-Up Actions

- Action:
- Owner:
- Priority:

- Action:
- Owner:
- Priority:
