# Examples

Sample code for each language using the official Azure AI Search SDK against the emulator.

## Prerequisites

Start the emulator and set environment variables:

```bash
docker compose up -d

export SEARCH_ENDPOINT=http://localhost:8080
export SEARCH_API_KEY=<your API_KEY>
```

## Covered Operations

All samples cover the same set of operations:

| Operation | Endpoint |
|---|---|
| Create / update index | `POST /indexes` |
| List indexes | `GET /indexes` |
| Get index | `GET /indexes/{name}` |
| Get index stats | `GET /indexes/{name}/stats` |
| Upload document | `POST /indexes/{name}/docs` |
| Get document by key | `GET /indexes/{name}/docs/{key}` |
| Get document count | `GET /indexes/{name}/docs/$count` |
| Search documents | `GET /indexes/{name}/docs?search=` |
| Batch operations (upload / mergeOrUpload / delete) | `POST /indexes/{name}/docs/index` |
| Delete index | `DELETE /indexes/{name}` |

## Python

```bash
cd examples/python
pip install -r requirements.txt
python sample.py
```

## JavaScript

```bash
cd examples/javascript
npm install
npm start
```

## C# (.NET 8)

```bash
cd examples/dotnet
dotnet run
```
