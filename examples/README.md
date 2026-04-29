# Examples

Sample code for each language using the official Azure AI Search SDK against the emulator.

## Covered Operations

All samples cover the same set of operations:

| Operation | Endpoint |
|---|---|
| Create / update index | `PUT /indexes/{name}` |
| List indexes | `GET /indexes` |
| Get index | `GET /indexes/{name}` |
| Get index stats | `GET /indexes/{name}/stats` |
| Upload document | `POST /indexes/{name}/docs/index` |
| Get document by key | `GET /indexes/{name}/docs/{key}` |
| Get document count | `GET /indexes/{name}/docs/$count` |
| Search documents | `POST /indexes/{name}/docs/search` |
| Batch operations (upload / mergeOrUpload / delete) | `POST /indexes/{name}/docs/index` |
| Delete index | `DELETE /indexes/{name}` |

## Python

Requires Python 3.9+.

```bash
# Start emulator
API_KEY=dev-api-key go run main.go &

cd examples/python
pip install -r requirements.txt
SEARCH_ENDPOINT=http://localhost:8080 SEARCH_API_KEY=dev-api-key python sample.py
```

## JavaScript

Requires Node.js 18+.

```bash
# Start emulator
API_KEY=dev-api-key go run main.go &

cd examples/javascript
npm install
SEARCH_ENDPOINT=http://localhost:8080 SEARCH_API_KEY=dev-api-key npm start
```

## C# (.NET 8)

The `Azure.Search.Documents` C# SDK requires an HTTPS endpoint.
Start the emulator with `TLS_PORT=8443` to enable the self-signed HTTPS listener.

```bash
# Start emulator with TLS
API_KEY=dev-api-key TLS_PORT=8443 go run main.go &

cd examples/dotnet
SEARCH_ENDPOINT=https://localhost:8443 SEARCH_API_KEY=dev-api-key dotnet run
```
