/**
 * Azure AI Search Emulator - JavaScript SDK sample
 *
 * Demonstrates all emulator API endpoints using the official @azure/search-documents SDK.
 *
 * Environment variables:
 *   SEARCH_ENDPOINT  - Emulator base URL, e.g. http://localhost:8080
 *   SEARCH_API_KEY   - API key configured via the emulator's API_KEY env var
 */

import { SearchClient, SearchIndexClient, AzureKeyCredential } from "@azure/search-documents";

const ENDPOINT = process.env.SEARCH_ENDPOINT ?? "http://localhost:8080";
const API_KEY = process.env.SEARCH_API_KEY ?? "dev-api-key";
const INDEX_NAME = "hotels";

const credential = new AzureKeyCredential(API_KEY);
const indexClient = new SearchIndexClient(ENDPOINT, credential);

async function createIndex() {
  const index = {
    name: INDEX_NAME,
    fields: [
      { name: "hotelId", type: "Edm.String", key: true },
      { name: "hotelName", type: "Edm.String", searchable: true },
      { name: "description", type: "Edm.String", searchable: true },
      { name: "category", type: "Edm.String", filterable: true },
      { name: "rating", type: "Edm.Double", filterable: true, sortable: true },
    ],
  };
  const result = await indexClient.createOrUpdateIndex(index);
  console.log(`[createIndex] Created: ${result.name}`);
}

async function listIndexes() {
  const names = [];
  for await (const index of indexClient.listIndexes()) {
    names.push(index.name);
  }
  console.log(`[listIndexes] Found ${names.length} index(es): ${names.join(", ")}`);
}

async function getIndex() {
  const index = await indexClient.getIndex(INDEX_NAME);
  console.log(`[getIndex] Name=${index.name}, Fields=${index.fields.length}`);
}

async function getIndexStats() {
  const stats = await indexClient.getIndexStatistics(INDEX_NAME);
  console.log(
    `[getIndexStats] DocumentCount=${stats.documentCount}, StorageSize=${stats.storageSize}`
  );
}

async function uploadDocuments() {
  const client = new SearchClient(ENDPOINT, INDEX_NAME, credential);
  const documents = [
    {
      hotelId: "1",
      hotelName: "Grand Tokyo Hotel",
      description: "A luxury hotel in the heart of Tokyo.",
      category: "Luxury",
      rating: 4.8,
    },
    {
      hotelId: "2",
      hotelName: "Budget Inn Osaka",
      description: "An affordable stay in Osaka city center.",
      category: "Budget",
      rating: 3.5,
    },
    {
      hotelId: "3",
      hotelName: "Seaside Resort Okinawa",
      description: "Beachfront resort with stunning ocean views.",
      category: "Resort",
      rating: 4.6,
    },
  ];
  const result = await client.uploadDocuments(documents);
  const succeeded = result.results.filter((r) => r.succeeded).length;
  console.log(`[uploadDocuments] Uploaded ${succeeded}/${documents.length} documents`);
}

async function getDocument() {
  const client = new SearchClient(ENDPOINT, INDEX_NAME, credential);
  const doc = await client.getDocument("1");
  console.log(`[getDocument] hotelId=${doc.hotelId}, hotelName=${doc.hotelName}`);
}

async function getDocumentCount() {
  const client = new SearchClient(ENDPOINT, INDEX_NAME, credential);
  const count = await client.getDocumentsCount();
  console.log(`[getDocumentCount] Count=${count}`);
}

async function searchDocuments(query = "*") {
  const client = new SearchClient(ENDPOINT, INDEX_NAME, credential);
  const results = await client.search(query);
  const docs = [];
  for await (const result of results.results) {
    docs.push(result.document);
  }
  console.log(`[searchDocuments] query=${JSON.stringify(query)}, found=${docs.length}`);
  for (const doc of docs) {
    console.log(`  hotelId=${doc.hotelId}, hotelName=${doc.hotelName}`);
  }
}

async function batchOperations() {
  const client = new SearchClient(ENDPOINT, INDEX_NAME, credential);
  const batch = [
    { "@search.action": "upload", hotelId: "4", hotelName: "Mountain Lodge Hokkaido", rating: 4.2 },
    { "@search.action": "mergeOrUpload", hotelId: "2", hotelName: "Budget Inn Osaka", rating: 3.8 },
    { "@search.action": "delete", hotelId: "3" },
  ];
  const result = await client.indexDocuments(batch);
  const succeeded = result.results.filter((r) => r.succeeded).length;
  console.log(`[batchOperations] Processed ${succeeded}/${batch.length} actions`);
}

async function deleteIndex() {
  await indexClient.deleteIndex(INDEX_NAME);
  console.log(`[deleteIndex] Deleted: ${INDEX_NAME}`);
}

async function main() {
  console.log("=== Azure AI Search Emulator - JavaScript Sample ===\n");
  await createIndex();
  await listIndexes();
  await getIndex();
  await uploadDocuments();
  await getDocument();
  await getDocumentCount();
  await getIndexStats();
  await searchDocuments("*");
  await searchDocuments("Tokyo");
  await batchOperations();
  await searchDocuments("*");
  await deleteIndex();
  console.log("\nDone.");
}

main().catch(console.error);
