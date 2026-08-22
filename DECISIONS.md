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
