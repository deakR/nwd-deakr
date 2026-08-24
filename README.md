# Problem 3: No Wrong Door — Unified Resident Integration Service

A resilient Go integration service providing a unified resident view across two unreliable legacy municipal sources (Resident Index REST and Benefits Register XML), engineered for slow response times, dynamic pagination boundary slips, and a 40% upstream failure rate.

---

## 1. Running the System

### Option A — Docker (all platforms, one command)

Runs the entire system — both mock upstreams plus the unified API — without installing Go or Python. Requires only [Docker Desktop](https://www.docker.com/products/docker-desktop/) (Windows / macOS) or Docker Engine with the Compose plugin (Linux):

```bash
docker compose up --build
```

This builds three containers (Resident Index mock, Benefits Register mock, unified API) and wires them together over an internal network. The API container starts only after both mocks pass their health checks. Once you see `Unified API running`, open 👉 **[http://127.0.0.1:8080/docs](http://127.0.0.1:8080/docs)**

**Ports used (Docker):**

| Port | Where | What |
| :--- | :--- | :--- |
| `8080` | **host machine** | Unified API + Swagger UI (the only port occupied on your machine) |
| `8081` / `8082` | inside the Compose network only | Mock upstreams — not reachable from (and not occupying) your host |

> If `8080` is already taken on your machine, change the mapping in `docker-compose.yml` from `"8080:8080"` to e.g. `"9090:8080"` and use `http://127.0.0.1:9090/docs`.

Stop with `Ctrl+C` in the terminal, or if started detached (`docker compose up -d --build`), with:

```bash
docker compose down
```

### Option B — Manual (Go + Python toolchains)

This project uses **only the standard libraries** of Go and Python — zero external packages (`pip install`, `npm install`, or third-party Go modules are never needed). You only need the toolchains themselves:

#### Prerequisites

**Ubuntu / Debian Linux**
```bash
sudo apt update
sudo apt install -y golang python3
```

**macOS (Homebrew)**
```bash
brew install go python
```

**Windows (PowerShell with Winget or direct installer)**
```powershell
winget install GoLang.Go Python.Python.3.12
# Or download directly from:
# Go: https://go.dev/dl/ (Go 1.22+)
# Python: https://www.python.org/downloads/ (Python 3.8+)
```

Verify:
```bash
go version      # Minimum Go 1.22+
python3 --version || python --version # Minimum Python 3.8+
```

#### Step 1: Start Upstream Mock Services
The repository includes the mock upstream services in `services/`. In a separate terminal, start both services. Note that unlike the Docker option, this mode occupies **all three ports on your machine**: `8080` (API), `8081` (Resident Index), `8082` (Benefits Register):

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

#### Step 2: Start the Unified API Server
In a new terminal inside the project directory:

```bash
go run .
```

The server will compile immediately and start on `http://127.0.0.1:8080`.

---

## 2. Interactive Testing (Swagger UI)

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

## 3. Running the Automated Test Suite

Run the full suite of 22 unit and integration tests (covering graceful degradation, 3-attempt retries on 500s, boundary deduplication, context cancellations, and timeouts):

```bash
go test -v ./...
```
