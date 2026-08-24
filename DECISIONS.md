# Decisions

![](https://img.shields.io/badge/01-Stack-0969da?style=flat-square)
- Using Go with its standard library for HTTP, JSON, XML, concurrency, timeouts, and testing.
- Chosen for its simplicity, low dependency footprint, and suitability for the integration requirements.

![](https://img.shields.io/badge/02-Architecture-bc4c00?style=flat-square)
- Separate adapters for each upstream service.
- Keeps the sources independent from the aggregation logic.

![](https://img.shields.io/badge/03-Scope-1a7f37?style=flat-square)
- Floor first.
- Stretch features only after the required functionality works.

![](https://img.shields.io/badge/04-Dependencies-8250df?style=flat-square)
- Upstream clients are created separately from HTTP handlers.
- Keeps upstream communication isolated and makes failure behavior testable.

![](https://img.shields.io/badge/05-Concurrency-0969da?style=flat-square)
- Parallel fan-out to upstream services using goroutines and buffered channels.
- Prevents slow upstream latency from compounding sequentially.

![](https://img.shields.io/badge/06-Degradation-cf222e?style=flat-square)
- Return partial data with nullable entity pointers and a structured source status map.
- Upstream failure reports unavailable in metadata instead of failing the caller.

#### Degradation policy — per failure mode

| Failure mode | What the caller receives | How the caller knows |
|:--|:--|:--|
| Source returns 500 (transient) | Retried up to 3× with 50ms backoff; on exhaustion, partial response with data from the other source | `_meta.sources.<source>.status: "unavailable"`, `_meta.partial: true` |
| Source timeout | Same as 500 exhaustion — retries fire, then partial fallback | Same as above |
| Source unreachable / connection refused | Same as 500 exhaustion | Same as above |
| 404 — record not found | `null` entity in response body, 200 status | `_meta.sources.<source>.status: "not_found"` |
| Circuit open (3 consecutive post-retry failures) | Instant fail-fast, no upstream call made, partial response with other source data | `_meta.sources.<source>.status: "circuit_open"`, `_meta.partial: true` |
| Stale cache served during outage | Last known good data returned | `_meta.sources.<source>.status: "stale"`, message indicates cached snapshot |
| Both sources fail simultaneously | Empty response body with both sources reporting their respective status | `_meta.partial: true`, both source entries non-`ok` |

![](https://img.shields.io/badge/07-Pagination-0969da?style=flat-square)
- Resident Index pages are fetched sequentially and deduplicated by stable resident ID, preserving first-seen order.
- The catalogue is only marked complete when the unique count matches the total reported by the source.
- Pagination failures and anomalies are surfaced through the pagination receipt and source metadata rather than silently returning incomplete data.

![](https://img.shields.io/badge/08-API_Documentation-8250df?style=flat-square)
- Using OpenAPI 3.0 with Swagger UI for API documentation.
- Gives judges self-serve, interactive endpoint exploration without adding dependencies to the Go code.

![](https://img.shields.io/badge/09-Day_2_Resilience-0969da?style=flat-square)
- Handled 40% Benefits Register failure rate with bounded retries (3 attempts, 50ms backoff) isolated to the Benefits adapter.
- Transient errors retry; 404s fail fast. Exhaustion falls back to existing degradation with zero changes to aggregation logic.

---

![](https://img.shields.io/badge/STRETCH-FEATURES-8250df?style=for-the-badge)

![](https://img.shields.io/badge/10-Circuit_Breaker-cf222e?style=flat-square)
- 3 consecutive failed operations trip the circuit to open for 5s cooldown before a single-flight trial probe.
- Counted at the operation level post-retries to protect against sustained outages without false positives from the 40% noise.
- Rejections fail fast with `circuit_open` status; 404s are definitive and never trip the breaker.

![](https://img.shields.io/badge/11-TTL_Cache-0969da?style=flat-square)
- 5-minute TTL cache layered inside the Benefits adapter ahead of retries and circuit breaker.
- Accepts up to 5m staleness on administrative records in exchange for instant reads and stale fallback during outages.
- Stale data is explicitly labeled; 404s authoritatively evict cache entries to prevent ghost records.

![](https://img.shields.io/badge/12-Identity_Resolution-1a7f37?style=flat-square)
- Cross-source identity matching via an evidence ladder, not numeric scores: a merge happens only when exactly one candidate clears an explicit rule (exact DOB + normalized name, else name + town + street).
- Ties are declined and reported as `ambiguous` with every candidate listed; "could not search" is reported as `unavailable`, never disguised as `no_match`.

![](https://img.shields.io/badge/13-Catalogue_Cache-0969da?style=flat-square)
- Identity matching requires the full benefits catalogue (`GET /records`), fetched lazily once, cached in memory with the same 5-minute TTL as per-ref records.
- The catalogue fetch shares the same circuit breaker and bounded retry policy as per-ref lookups: any operation against the Benefits Register is evidence of its health, so successful catalogue fetches reset the failure counter and failed fetches contribute to tripping it.
- On live failure an expired snapshot is still served (flagged `stale`) so identity resolution survives outages.
