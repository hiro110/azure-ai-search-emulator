"""
Azure AI Search Emulator - Python SDK sample

Demonstrates all emulator API endpoints using the official azure-search-documents SDK.

Environment variables:
    SEARCH_ENDPOINT  - Emulator base URL, e.g. http://localhost:8080
    SEARCH_API_KEY   - API key configured via the emulator's API_KEY env var
"""

import os
from azure.core.credentials import AzureKeyCredential
from azure.search.documents import SearchClient
from azure.search.documents.indexes import SearchIndexClient
from azure.search.documents.indexes.models import (
    SearchIndex,
    SearchField,
    SearchFieldDataType,
    SimpleField,
    SearchableField,
)
from azure.search.documents.models import IndexDocumentsBatch

ENDPOINT = os.environ.get("SEARCH_ENDPOINT", "http://localhost:8080")
API_KEY = os.environ.get("SEARCH_API_KEY", "dev-api-key")
INDEX_NAME = "hotels"

credential = AzureKeyCredential(API_KEY)
index_client = SearchIndexClient(endpoint=ENDPOINT, credential=credential)


def create_index() -> None:
    fields = [
        SimpleField(name="hotelId", type=SearchFieldDataType.String, key=True),
        SearchableField(name="hotelName", type=SearchFieldDataType.String),
        SearchableField(name="description", type=SearchFieldDataType.String),
        SimpleField(name="category", type=SearchFieldDataType.String, filterable=True),
        SimpleField(
            name="rating",
            type=SearchFieldDataType.Double,
            filterable=True,
            sortable=True,
        ),
    ]
    index = SearchIndex(name=INDEX_NAME, fields=fields)
    result = index_client.create_or_update_index(index)
    print(f"[create_index] Created: {result.name}")


def list_indexes() -> None:
    indexes = list(index_client.list_indexes())
    print(f"[list_indexes] Found {len(indexes)} index(es): {[i.name for i in indexes]}")


def get_index() -> None:
    index = index_client.get_index(INDEX_NAME)
    print(f"[get_index] Name={index.name}, Fields={len(index.fields)}")


def get_index_stats() -> None:
    stats = index_client.get_index_statistics(INDEX_NAME)
    print(
        f"[get_index_stats] DocumentCount={stats.document_count}, StorageSize={stats.storage_size}"
    )


def upload_documents() -> None:
    client = SearchClient(endpoint=ENDPOINT, index_name=INDEX_NAME, credential=credential)
    documents = [
        {
            "hotelId": "1",
            "hotelName": "Grand Tokyo Hotel",
            "description": "A luxury hotel in the heart of Tokyo.",
            "category": "Luxury",
            "rating": 4.8,
        },
        {
            "hotelId": "2",
            "hotelName": "Budget Inn Osaka",
            "description": "An affordable stay in Osaka city center.",
            "category": "Budget",
            "rating": 3.5,
        },
        {
            "hotelId": "3",
            "hotelName": "Seaside Resort Okinawa",
            "description": "Beachfront resort with stunning ocean views.",
            "category": "Resort",
            "rating": 4.6,
        },
    ]
    result = client.upload_documents(documents=documents)
    succeeded = sum(1 for r in result if r.succeeded)
    print(f"[upload_documents] Uploaded {succeeded}/{len(documents)} documents")


def get_document() -> None:
    client = SearchClient(endpoint=ENDPOINT, index_name=INDEX_NAME, credential=credential)
    doc = client.get_document(key="1")
    print(f"[get_document] hotelId={doc['hotelId']}, hotelName={doc['hotelName']}")


def get_document_count() -> None:
    client = SearchClient(endpoint=ENDPOINT, index_name=INDEX_NAME, credential=credential)
    count = client.get_document_count()
    print(f"[get_document_count] Count={count}")


def search_documents(query: str = "*") -> None:
    client = SearchClient(endpoint=ENDPOINT, index_name=INDEX_NAME, credential=credential)
    results = client.search(search_text=query)
    docs = list(results)
    print(f"[search_documents] query={query!r}, found={len(docs)}")
    for doc in docs:
        print(f"  hotelId={doc['hotelId']}, hotelName={doc['hotelName']}")


def batch_operations() -> None:
    client = SearchClient(endpoint=ENDPOINT, index_name=INDEX_NAME, credential=credential)
    batch = IndexDocumentsBatch()
    batch.add_upload_actions(
        [{"hotelId": "4", "hotelName": "Mountain Lodge Hokkaido", "rating": 4.2}]
    )
    batch.add_merge_or_upload_actions(
        [{"hotelId": "2", "hotelName": "Budget Inn Osaka", "rating": 3.8}]
    )
    batch.add_delete_actions([{"hotelId": "3"}])
    result = client.index_documents(batch=batch)
    succeeded = sum(1 for r in result if r.succeeded)
    print(f"[batch_operations] Processed {succeeded}/{len(result)} actions")


def delete_index() -> None:
    index_client.delete_index(INDEX_NAME)
    print(f"[delete_index] Deleted: {INDEX_NAME}")


def main() -> None:
    print("=== Azure AI Search Emulator - Python Sample ===\n")
    create_index()
    list_indexes()
    get_index()
    upload_documents()
    get_document()
    get_document_count()
    get_index_stats()
    search_documents("*")
    search_documents("Tokyo")
    batch_operations()
    search_documents("*")
    delete_index()
    print("\nDone.")


if __name__ == "__main__":
    main()
