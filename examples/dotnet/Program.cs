// Azure AI Search Emulator - C# SDK sample
//
// Demonstrates all emulator API endpoints using the official Azure.Search.Documents SDK.
//
// Environment variables:
//   SEARCH_ENDPOINT  - Emulator base URL, e.g. http://localhost:8080
//   SEARCH_API_KEY   - API key configured via the emulator's API_KEY env var

using Azure;
using Azure.Search.Documents;
using Azure.Search.Documents.Indexes;
using Azure.Search.Documents.Indexes.Models;
using Azure.Search.Documents.Models;

var endpoint = new Uri(Environment.GetEnvironmentVariable("SEARCH_ENDPOINT") ?? "http://localhost:8080");
var apiKey = Environment.GetEnvironmentVariable("SEARCH_API_KEY") ?? "dev-api-key";
const string IndexName = "hotels";

var credential = new AzureKeyCredential(apiKey);
var indexClient = new SearchIndexClient(endpoint, credential);

await CreateIndexAsync();
await ListIndexesAsync();
await GetIndexAsync();
await UploadDocumentsAsync();
await GetDocumentAsync();
await GetDocumentCountAsync();
await GetIndexStatsAsync();
await SearchDocumentsAsync("*");
await SearchDocumentsAsync("Tokyo");
await BatchOperationsAsync();
await SearchDocumentsAsync("*");
await DeleteIndexAsync();

Console.WriteLine("\nDone.");

async Task CreateIndexAsync()
{
    var fields = new FieldBuilder().Build(typeof(Hotel));
    var index = new SearchIndex(IndexName, fields);
    var result = await indexClient.CreateOrUpdateIndexAsync(index);
    Console.WriteLine($"[CreateIndex] Created: {result.Value.Name}");
}

async Task ListIndexesAsync()
{
    var names = new List<string>();
    await foreach (var index in indexClient.GetIndexesAsync())
    {
        names.Add(index.Name);
    }
    Console.WriteLine($"[ListIndexes] Found {names.Count} index(es): {string.Join(", ", names)}");
}

async Task GetIndexAsync()
{
    var index = await indexClient.GetIndexAsync(IndexName);
    Console.WriteLine($"[GetIndex] Name={index.Value.Name}, Fields={index.Value.Fields.Count}");
}

async Task GetIndexStatsAsync()
{
    var stats = await indexClient.GetIndexStatisticsAsync(IndexName);
    Console.WriteLine($"[GetIndexStats] DocumentCount={stats.Value.DocumentCount}, StorageSize={stats.Value.StorageSize}");
}

async Task UploadDocumentsAsync()
{
    var client = new SearchClient(endpoint, IndexName, credential);
    var documents = new[]
    {
        new Hotel { HotelId = "1", HotelName = "Grand Tokyo Hotel", Description = "A luxury hotel in the heart of Tokyo.", Category = "Luxury", Rating = 4.8 },
        new Hotel { HotelId = "2", HotelName = "Budget Inn Osaka", Description = "An affordable stay in Osaka city center.", Category = "Budget", Rating = 3.5 },
        new Hotel { HotelId = "3", HotelName = "Seaside Resort Okinawa", Description = "Beachfront resort with stunning ocean views.", Category = "Resort", Rating = 4.6 },
    };
    var result = await client.UploadDocumentsAsync(documents);
    var succeeded = result.Value.Results.Count(r => r.Succeeded);
    Console.WriteLine($"[UploadDocuments] Uploaded {succeeded}/{documents.Length} documents");
}

async Task GetDocumentAsync()
{
    var client = new SearchClient(endpoint, IndexName, credential);
    var doc = await client.GetDocumentAsync<Hotel>("1");
    Console.WriteLine($"[GetDocument] HotelId={doc.Value.HotelId}, HotelName={doc.Value.HotelName}");
}

async Task GetDocumentCountAsync()
{
    var client = new SearchClient(endpoint, IndexName, credential);
    var count = await client.GetDocumentCountAsync();
    Console.WriteLine($"[GetDocumentCount] Count={count.Value}");
}

async Task SearchDocumentsAsync(string query)
{
    var client = new SearchClient(endpoint, IndexName, credential);
    var results = await client.SearchAsync<Hotel>(query);
    var docs = new List<Hotel>();
    await foreach (var result in results.Value.GetResultsAsync())
    {
        docs.Add(result.Document);
    }
    Console.WriteLine($"[SearchDocuments] query={query}, found={docs.Count}");
    foreach (var doc in docs)
    {
        Console.WriteLine($"  HotelId={doc.HotelId}, HotelName={doc.HotelName}");
    }
}

async Task BatchOperationsAsync()
{
    var client = new SearchClient(endpoint, IndexName, credential);
    var batch = IndexDocumentsBatch.Create(
        IndexDocumentsAction.Upload(new Hotel { HotelId = "4", HotelName = "Mountain Lodge Hokkaido", Rating = 4.2 }),
        IndexDocumentsAction.MergeOrUpload(new Hotel { HotelId = "2", HotelName = "Budget Inn Osaka", Rating = 3.8 }),
        IndexDocumentsAction.Delete(new Hotel { HotelId = "3" })
    );
    var result = await client.IndexDocumentsAsync(batch);
    var succeeded = result.Value.Results.Count(r => r.Succeeded);
    Console.WriteLine($"[BatchOperations] Processed {succeeded}/{batch.Actions.Count} actions");
}

async Task DeleteIndexAsync()
{
    await indexClient.DeleteIndexAsync(IndexName);
    Console.WriteLine($"[DeleteIndex] Deleted: {IndexName}");
}

public class Hotel
{
    [SimpleField(IsKey = true)]
    public string? HotelId { get; set; }

    [SearchableField]
    public string? HotelName { get; set; }

    [SearchableField]
    public string? Description { get; set; }

    [SimpleField(IsFilterable = true)]
    public string? Category { get; set; }

    [SimpleField(IsFilterable = true, IsSortable = true)]
    public double? Rating { get; set; }
}
