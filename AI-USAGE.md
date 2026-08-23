# AI Usage

ChatGPT (Web) was used as an interactive pair-programming assistant throughout development for:
* **Scaffolding & Adapters**: Generating initial Go boilerplate, domain structs, and HTTP client logic.
* **Test Case Generation**: Writing table-driven unit and integration tests (`httptest`) for degradation and pagination edge cases.
* **Documentation**: Formatting the OpenAPI 3.0 specification for the embedded Swagger UI.

All architecture choices, failure policies, and code were reviewed, tested, and verified by the developer.
