package application

// SearchParams holds all OData query parameters for a search request.
type SearchParams struct {
	Search       string
	Filter       string   // $filter
	OrderBy      string   // $orderby
	Select       []string // $select (empty = all fields)
	SearchFields []string // searchFields (empty = all fields)
	Top          int      // $top  (0 = use default)
	Skip         int      // $skip
	IncludeCount bool     // $count
}

// SearchResult is returned by DocumentService.SearchDocuments.
type SearchResult struct {
	Value []map[string]interface{}
	Total int64 // total matching docs before TOP/SKIP
}

// DefaultTop is the page size used when $top is not specified.
const DefaultTop = 50

// unexported alias kept for internal use
const defaultTop = DefaultTop
