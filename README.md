# Problem 3: No Wrong Door — Unified Resident Integration Service

A resilient Go integration service providing a unified resident view across two unreliable legacy municipal sources (Resident Index REST and Benefits Register XML), engineered for slow response times, dynamic pagination boundary slips, and a 40% upstream failure rate.

---

## 1. Prerequisites & Clean Environment Setup

This project uses **only the standard libraries** of Go and Python. There are **zero external packages or libraries to install** (no `pip install`, `npm install`, or third-party Go modules).

### Platform Installation Commands

#### Ubuntu / Debian Linux
```bash
sudo apt update
sudo apt install -y golang python3
```

#### macOS (Homebrew)
```bash
brew install go python
```

#### Windows (PowerShell with Winget or direct installer)
```powershell
winget install GoLang.Go Python.Python.3.12
# Or download directly from:
# Go: https://go.dev/dl/ (Go 1.22+)
# Python: https://www.python.org/downloads/ (Python 3.8+)
```

### Verify Installation
```bash
go version      # Minimum Go 1.22+
python3 --version || python --version # Minimum Python 3.8+
```

---

## 2. Running the System

### Step 1: Start Upstream Mock Services
The repository includes the mock upstream services in `services/`. In a separate terminal, start both services:

**Windows (PowerShell):**
```powershell
# Terminal 1: Resident Index (REST on port 8081)
python services/rest_service.py --port 8081

# Terminal 2: Benefits Register (XML on port 8082 with 40% failure rate)
python services/xml_service.py --port 8082 --failure-rate 0.40
```

**Linux / macOS:**
```bash
python3 services/rest_service.py --port 8081 &
python3 services/xml_service.py --port 8082 --failure-rate 0.40 &
```

---

### Step 2: Start the Unified API Server
In a new terminal inside the project directory:

```bash
go run .
```

The server will compile immediately and start on `http://127.0.0.1:8080`.

---

## 3. Interactive Testing (Swagger UI)

Once the server is running, open the interactive Swagger documentation in your browser:

👉 **[http://127.0.0.1:8080/docs](http://127.0.0.1:8080/docs)**

You can test all endpoints interactively using the **"Try it out"** buttons:

| Endpoint | Description | Sample Query / Path |
| :--- | :--- | :--- |
| `GET /unified` | Concurrent fan-out returning unified resident and benefits data with `_meta` diagnostics. | `?resident_id=R-10001&benefit_ref=CA/2016/4001` |
| `GET /residents` | Complete catalogue of all 620 deduplicated residents with pagination receipt. | `/residents` |
| `GET /residents/{id}` | Single resident lookup from Resident Index. | `/residents/R-10001` |
| `GET /benefits/{ref...}` | Single benefit record lookup from Benefits Register. | `/benefits/CA/2016/4001` |

*(Raw OpenAPI YAML spec is also available at `http://127.0.0.1:8080/openapi.yaml`)*.

---

## 4. Running the Automated Test Suite

Run the full suite of 22 unit and integration tests (covering graceful degradation, 3-attempt retries on 500s, boundary deduplication, context cancellations, and timeouts):

```bash
go test -v ./...
```
