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

![](https://img.shields.io/badge/07-Pagination-0969da?style=flat-square)
- Resident Index pages are fetched sequentially and deduplicated by stable resident ID, preserving first-seen order.
- The catalogue is only marked complete when the unique count matches the total reported by the source.
- Pagination failures and anomalies are surfaced through the pagination receipt and source metadata rather than silently returning incomplete data.

![](https://img.shields.io/badge/08-API_Documentation-8250df?style=flat-square)
- Using OpenAPI 3.0 with Swagger UI for API documentation.
- Gives judges self-serve, interactive endpoint exploration without adding dependencies to the Go code.

