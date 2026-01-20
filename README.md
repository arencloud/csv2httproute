# CSV to HTTPRoute Generator 🚀

A powerful and generic CLI tool built with **Go** and **Cobra** to transform CSV-based endpoint definitions into Kubernetes **Gateway API HTTPRoute** resources.

## ✨ Features

- **Generic & Reusable**: Highly configurable via CLI flags to fit any environment.
- **Single Source of Truth**: Follows the `HTTPRoute` v1 specification (Gateway API).
- **Flexible Input**: Process an entire directory of CSVs or a single specific file.
- **Smart URL Rewriting**: Automatically generates `URLRewrite` filters when a `Prefix` is specified in the CSV.
- **Two-Rule Strategy**:
    - **Rule 1**: Matches the prefix and strips it (using `ReplacePrefixMatch: /`) before forwarding.
    - **Rule 2**: Lists all specific endpoints for direct access.
- **Namespace Support**: Configure namespaces for the Route, Backend Services, and Parent Gateways independently.
- **Custom Hostnames**: Easily assign hostnames to your generated routes.
- **Robust Parsing**: Skips comments (lines starting with `#`), handles variable CSV fields, and sanitizes resource names.

---

## 🛠 Installation

Ensure you have **Go 1.25+** installed.

### From Source
```bash
git clone <repository-url>
cd csv2httproute
go build -o csv2httproute main.go
```

### From Releases
You can download pre-compiled binaries for various operating systems (Linux, macOS, Windows) and architectures (amd64, arm64, 386, arm) from the [Releases](<repository-url>/releases) page.

---

## 🚀 Usage

### Versioning
Check the current version:
```bash
./csv2httproute --version
```

### Quick Start
Process all CSV files in the default directory (`facts/endpoints`) and output to `generated/`:

```bash
./csv2httproute --service my-service --gateway my-gateway --namespace my-app
```

### Advanced Example
Specify custom paths, hostnames, and cross-namespace references:

```bash
./csv2httproute \
  --input data/api-endpoints.csv \
  --output k8s/routes \
  --hostname api.example.com \
  --service api-svc \
  --service-namespace backend \
  --gateway shared-gw \
  --gateway-namespace infra \
  --namespace production
```

---

## 🚩 CLI Flags

| Flag | Shorthand | Description | Default |
| :--- | :--- | :--- | :--- |
| `--input` | `-i` | Directory or CSV file to process | `facts/endpoints` |
| `--output` | `-o` | Output directory for YAML files | `generated` |
| `--service` | `-s` | Default backend service name | `my-service` |
| `--port` | `-p` | Default backend service port | `80` |
| `--service-namespace` | | Namespace for the backend service | (empty) |
| `--gateway` | `-g` | Parent gateway name | `my-gateway` |
| `--gateway-namespace` | | Namespace for the parent gateway | (matches `--namespace`) |
| `--namespace` | `-n` | Namespace for the HTTPRoute resource | `default` |
| `--hostname` | | Hostname for the HTTPRoute | (empty) |

---

## 📄 CSV Format

The tool expects CSV files with a header row. Supported columns (case-insensitive):

- `Method`: HTTP Method (GET, POST, etc.)
- `URL`: The path to match.
- `Prefix` (Optional): If provided, a rewrite rule will be created to strip this prefix.
- `Comment` (Optional): Ignored by the tool, used for documentation.

**Example `endpoints.csv`**:
```csv
Method,URL,Prefix,Comment
GET,/api/v1/users,/user,Get all users
POST,/api/v1/login,/user,Authentication
GET,/health,,Health check (no prefix rewrite)
```

---

## 🔄 URL Rewrite Logic

When a `Prefix` is detected in the CSV (e.g., `/user` for a URL `/api/v1/users`), the tool generates:

1. A rule matching `PathPrefix: /user` with a filter `ReplacePrefixMatch: /`. This allows requests like `/user/api/v1/users` to be routed to the backend as `/api/v1/users`.
2. A separate rule matching the original URL `/api/v1/users` directly.

This ensures backward compatibility and flexible routing transitions.

---

## 🏗 Project Structure

- `main.go`: The core logic and CLI definition.
- `facts/crd/`: Contains the HTTPRoute CRD specification used as a reference.
- `facts/endpoints/`: Default location for input CSV files.
- `generated/`: Default output directory for YAML manifests.

---

## ⚖️ License

MIT
