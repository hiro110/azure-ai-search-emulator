# Azure AI Search Emulator

## Description

Azure AI Search Emulator is a lightweight server that emulates the basic REST API behavior of Azure Cognitive Search.  
It allows you to create, update, and delete indexes, add and search documents, and perform batch operations using a simple full-text search engine powered by SQLite.  
This project is ideal for local development, testing, and understanding Azure Search API specifications without needing an actual Azure subscription.

## Features

- Create, update, retrieve, and delete indexes
- Manage index schema (fields, suggesters, analyzers, etc.)
- Add, update, and delete documents (single and batch)
- Full-text search (case-insensitive, partial match)
- Retrieve document count and index statistics
- Simple API key authentication

## What you can do

- Use Azure Search-compatible REST API endpoints locally
- No setup required—runs with Gin and SQLite
- Manage schema and documents in JSON format
- Ideal for development, testing, and learning purposes

## How to run

1. Install dependencies:
   ```sh
   go mod tidy
   ```
2. Start the server:
   ```sh
   air
   ```
3. Access API endpoints at `http://localhost:8080`  
   (Add `api-key: your-api-key` header to your requests)

## Notes

- This emulator is not suitable for production or high-load environments.
- Only basic full-text search is supported; advanced queries and ranking are not implemented.
- Not fully compatible with all Azure Search features—only main APIs are supported.