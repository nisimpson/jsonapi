package jsonapi

import (
	"encoding/json"
	"fmt"
	mathrand "math/rand"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
)

type testResourceWithRel struct {
	ID   string `jsonapi:"primary,test"`
	Name string `jsonapi:"attr,name"`
}

func (t testResourceWithRel) ResourceID() string   { return t.ID }
func (t testResourceWithRel) ResourceType() string { return "test" }
func (t testResourceWithRel) Relationships() map[string]RelationType {
	return map[string]RelationType{"author": RelationToOne}
}
func (t testResourceWithRel) MarshalRef(name string) []ResourceIdentifier {
	return nil
}

func TestWithTopMeta(t *testing.T) {
	resource := testResource{ID: "1", Name: "test"}

	data, err := Marshal(resource, WithTopMeta("total", 42))
	assert.NoError(t, err)

	var doc Document
	err = json.Unmarshal(data, &doc)
	assert.NoError(t, err)
	assert.Equal(t, float64(42), doc.Meta["total"])
}

func TestWithTopLink(t *testing.T) {
	resource := testResource{ID: "1", Name: "test"}

	data, err := Marshal(resource, WithTopLink("self", Link{Href: "http://example.com/test/1"}))
	assert.NoError(t, err)

	var doc Document
	err = json.Unmarshal(data, &doc)
	assert.NoError(t, err)
	assert.Equal(t, "http://example.com/test/1", doc.Links["self"].Href)
}

func TestWithTopHref(t *testing.T) {
	resource := testResource{ID: "1", Name: "test"}

	data, err := Marshal(resource, WithTopHref("self", "http://example.com/test/1"))
	assert.NoError(t, err)

	var doc Document
	err = json.Unmarshal(data, &doc)
	assert.NoError(t, err)
	assert.Equal(t, "http://example.com/test/1", doc.Links["self"].Href)
}

func TestWithMaxIncludeDepth(t *testing.T) {
	resource := testResource{ID: "1", Name: "test"}

	// This should not error even with depth limit
	data, err := Marshal(resource, WithMaxIncludeDepth(1))
	assert.NoError(t, err)
	assert.NotNil(t, data)
}

func TestWithTypeValidation(t *testing.T) {
	resource := testResource{ID: "1", Name: "test"}

	data, err := Marshal(resource, WithTypeValidation())
	assert.NoError(t, err)
	assert.NotNil(t, data)
}

func TestWithLinkResolver(t *testing.T) {
	resolver := &testLinkResolver{}
	resource := testResource{ID: "1", Name: "test"}

	data, err := Marshal(resource, WithLinkResolver("self", resolver))
	assert.NoError(t, err)
	assert.NotNil(t, data)
}

func TestWithDefaultLinks(t *testing.T) {
	resource := testResource{ID: "1", Name: "test"}

	data, err := Marshal(resource, WithDefaultLinks("http://example.com"))
	assert.NoError(t, err)
	assert.NotNil(t, data)
}

type testLinkResolver struct{}

func (r *testLinkResolver) ResolveResourceLink(key string, id ResourceIdentifier) (Link, bool) {
	if key == "self" {
		return Link{Href: "http://example.com/" + id.ResourceType() + "/" + id.ResourceID()}, true
	}
	return Link{}, false
}

func (r *testLinkResolver) ResolveRelationshipLink(key string, name string, id RelationshipMarshaler) (Link, bool) {
	return Link{}, false
}

func TestSelfLinkResolver_ResolveResourceLink(t *testing.T) {
	resolver := SelfLinkResolver{
		BaseURL:             "http://example.com",
		SelfResourcePattern: "%s/%s/%s",
	}
	resource := testResource{ID: "1", Name: "test"}

	link, ok := resolver.ResolveResourceLink("self", resource)
	assert.True(t, ok)
	assert.Equal(t, "http://example.com/test/1", link.Href)
}

func TestSelfLinkResolver_ResolveRelationshipLink(t *testing.T) {
	resolver := SelfLinkResolver{
		BaseURL:                    "http://example.com",
		SelfRelationshipPattern:    "%s/%s/%s/relationships/%s",
		RelatedRelationshipPattern: "%s/%s/%s/%s",
	}
	resource := testResourceWithRel{ID: "1", Name: "test"}

	link, ok := resolver.ResolveRelationshipLink("self", "author", resource)
	assert.True(t, ok)
	assert.Equal(t, "http://example.com/test/1/relationships/author", link.Href)

	link, ok = resolver.ResolveRelationshipLink("related", "author", resource)
	assert.True(t, ok)
	assert.Equal(t, "http://example.com/test/1/author", link.Href)
}

func TestBuildQueryParams_Empty(t *testing.T) {
	opts := applyOptions(nil)
	params := opts.buildQueryParams()
	assert.Empty(t, params.Encode())
}

func TestBuildQueryParams_Include(t *testing.T) {
	opts := applyOptions([]Options{WithInclude("author", "tags")})
	params := opts.buildQueryParams()
	assert.Equal(t, "author,tags", params.Get("include"))
}

func TestBuildQueryParams_Fields(t *testing.T) {
	opts := applyOptions([]Options{
		WithFields("articles", "title", "content"),
		WithFields("people", "name"),
	})
	params := opts.buildQueryParams()
	assert.Equal(t, "title,content", params.Get("fields[articles]"))
	assert.Equal(t, "name", params.Get("fields[people]"))
}

func TestBuildQueryParams_Sort(t *testing.T) {
	opts := applyOptions([]Options{WithSort("-created_at", "title")})
	params := opts.buildQueryParams()
	assert.Equal(t, "-created_at,title", params.Get("sort"))
}

func TestBuildQueryParams_PageNumber(t *testing.T) {
	opts := applyOptions([]Options{WithPageNumber(2, 25)})
	params := opts.buildQueryParams()
	assert.Equal(t, "2", params.Get("page[number]"))
	assert.Equal(t, "25", params.Get("page[size]"))
}

func TestBuildQueryParams_PageCursor(t *testing.T) {
	opts := applyOptions([]Options{WithPageCursor("abc123", 10)})
	params := opts.buildQueryParams()
	assert.Equal(t, "abc123", params.Get("page[after]"))
	assert.Equal(t, "10", params.Get("page[size]"))
}

func TestBuildQueryParams_PageParams(t *testing.T) {
	opts := applyOptions([]Options{WithPageParams(map[string]string{"offset": "10", "limit": "25"})})
	params := opts.buildQueryParams()
	assert.Equal(t, "10", params.Get("page[offset]"))
	assert.Equal(t, "25", params.Get("page[limit]"))
}

func TestBuildQueryParams_Filter(t *testing.T) {
	opts := applyOptions([]Options{WithFilter(map[string]string{"status": "published", "author": "john"})})
	params := opts.buildQueryParams()
	assert.Equal(t, "published", params.Get("filter[status]"))
	assert.Equal(t, "john", params.Get("filter[author]"))
}

func TestBuildQueryParams_Combined(t *testing.T) {
	opts := applyOptions([]Options{
		WithInclude("author", "tags"),
		WithFields("articles", "title", "content"),
		WithSort("-created_at"),
		WithPageNumber(1, 25),
		WithFilter(map[string]string{"status": "published"}),
	})
	params := opts.buildQueryParams()
	assert.Equal(t, "author,tags", params.Get("include"))
	assert.Equal(t, "title,content", params.Get("fields[articles]"))
	assert.Equal(t, "-created_at", params.Get("sort"))
	assert.Equal(t, "1", params.Get("page[number]"))
	assert.Equal(t, "25", params.Get("page[size]"))
	assert.Equal(t, "published", params.Get("filter[status]"))
}

func TestBuildQueryParams_NilPageFields(t *testing.T) {
	opts := applyOptions(nil)
	// Verify nil pointer fields don't cause panics
	assert.Nil(t, opts.queryPageNumber)
	assert.Nil(t, opts.queryPageCursor)
	params := opts.buildQueryParams()
	assert.Empty(t, params.Get("page[number]"))
	assert.Empty(t, params.Get("page[after]"))
}

// Feature: jsonapi-http-client, Property 6: Query parameters are correctly encoded
// Validates: Requirements 13.1, 13.2, 13.3, 13.4, 13.5, 13.6, 13.7, 13.8

// queryParamsInput is a generator struct for testing random query parameter combinations.
type queryParamsInput struct {
	Include      []string
	FieldTypes   []string   // resource type names for fields
	FieldValues  [][]string // field names per type (parallel with FieldTypes)
	Sort         []string
	PageNumber   int
	PageSize     int
	UsePageNum   bool // whether to use page-number pagination
	Cursor       string
	CursorSize   int
	UseCursor    bool // whether to use cursor pagination
	PageKeys     []string
	PageValues   []string
	FilterKeys   []string
	FilterValues []string
}

// safeAlphaString converts a random string to a safe alphanumeric identifier.
// This ensures generated keys/values are non-empty and contain only safe characters
// for deterministic testing, while still exercising the encoding logic.
func safeAlphaString(s string) string {
	var buf []byte
	for _, b := range []byte(s) {
		// Map to lowercase letters a-z
		buf = append(buf, 'a'+(b%26))
	}
	if len(buf) == 0 {
		return "a"
	}
	// Limit length to keep tests readable
	if len(buf) > 10 {
		buf = buf[:10]
	}
	return string(buf)
}

// Generate implements quick.Generator for queryParamsInput.
func (queryParamsInput) Generate(rand *mathrand.Rand, size int) reflect.Value {
	// Generate 0-4 include relationships
	numInclude := rand.Intn(5)
	includes := make([]string, numInclude)
	for i := range includes {
		includes[i] = safeAlphaString(fmt.Sprintf("%c%c", 'a'+rand.Intn(26), 'a'+rand.Intn(26)))
	}

	// Generate 0-3 field types, each with 1-3 fields.
	// Use unique type names to avoid accumulation across duplicate keys.
	numFieldTypes := rand.Intn(4)
	fieldTypes := make([]string, numFieldTypes)
	fieldValues := make([][]string, numFieldTypes)
	usedTypes := make(map[string]bool)
	for i := range fieldTypes {
		// Generate unique type names by appending the index.
		typ := safeAlphaString(fmt.Sprintf("type%d%d", i, rand.Intn(100)))
		for usedTypes[typ] {
			typ = safeAlphaString(fmt.Sprintf("type%d%d%d", i, rand.Intn(100), rand.Intn(100)))
		}
		usedTypes[typ] = true
		fieldTypes[i] = typ
		numFields := 1 + rand.Intn(3)
		fieldValues[i] = make([]string, numFields)
		for j := range fieldValues[i] {
			fieldValues[i][j] = safeAlphaString(fmt.Sprintf("field%d", rand.Intn(100)))
		}
	}

	// Generate 0-4 sort fields (some with "-" prefix)
	numSort := rand.Intn(5)
	sorts := make([]string, numSort)
	for i := range sorts {
		field := safeAlphaString(fmt.Sprintf("sort%d", rand.Intn(100)))
		if rand.Intn(2) == 0 {
			field = "-" + field
		}
		sorts[i] = field
	}

	// Generate 0-3 page params with unique keys.
	numPageParams := rand.Intn(4)
	pageKeys := make([]string, numPageParams)
	pageValues := make([]string, numPageParams)
	usedPageKeys := make(map[string]bool)
	for i := range pageKeys {
		pk := safeAlphaString(fmt.Sprintf("pk%d%d", i, rand.Intn(100)))
		for usedPageKeys[pk] {
			pk = safeAlphaString(fmt.Sprintf("pk%d%d%d", i, rand.Intn(100), rand.Intn(100)))
		}
		usedPageKeys[pk] = true
		pageKeys[i] = pk
		pageValues[i] = safeAlphaString(fmt.Sprintf("pv%d", rand.Intn(100)))
	}

	// Generate 0-3 filter params with unique keys.
	numFilters := rand.Intn(4)
	filterKeys := make([]string, numFilters)
	filterValues := make([]string, numFilters)
	usedFilterKeys := make(map[string]bool)
	for i := range filterKeys {
		fk := safeAlphaString(fmt.Sprintf("fk%d%d", i, rand.Intn(100)))
		for usedFilterKeys[fk] {
			fk = safeAlphaString(fmt.Sprintf("fk%d%d%d", i, rand.Intn(100), rand.Intn(100)))
		}
		usedFilterKeys[fk] = true
		filterKeys[i] = fk
		filterValues[i] = safeAlphaString(fmt.Sprintf("fv%d", rand.Intn(100)))
	}

	input := queryParamsInput{
		Include:      includes,
		FieldTypes:   fieldTypes,
		FieldValues:  fieldValues,
		Sort:         sorts,
		PageNumber:   1 + rand.Intn(100),
		PageSize:     1 + rand.Intn(100),
		UsePageNum:   rand.Intn(2) == 0,
		Cursor:       safeAlphaString(fmt.Sprintf("cur%d", rand.Intn(1000))),
		CursorSize:   1 + rand.Intn(100),
		UseCursor:    rand.Intn(2) == 0,
		PageKeys:     pageKeys,
		PageValues:   pageValues,
		FilterKeys:   filterKeys,
		FilterValues: filterValues,
	}

	return reflect.ValueOf(input)
}

func TestProperty_QueryParametersCorrectlyEncoded(t *testing.T) {
	config := &quick.Config{MaxCount: 200}

	err := quick.Check(func(input queryParamsInput) bool {
		// Build options from the generated input.
		var opts []Options

		if len(input.Include) > 0 {
			opts = append(opts, WithInclude(input.Include...))
		}

		for i, typ := range input.FieldTypes {
			if i < len(input.FieldValues) {
				opts = append(opts, WithFields(typ, input.FieldValues[i]...))
			}
		}

		if len(input.Sort) > 0 {
			opts = append(opts, WithSort(input.Sort...))
		}

		// Only use one pagination strategy at a time (page-number OR cursor, not both).
		if input.UsePageNum && !input.UseCursor {
			opts = append(opts, WithPageNumber(input.PageNumber, input.PageSize))
		} else if input.UseCursor && !input.UsePageNum {
			opts = append(opts, WithPageCursor(input.Cursor, input.CursorSize))
		}

		// Add page params (only when not using page-number or cursor to avoid conflicts).
		if !input.UsePageNum && !input.UseCursor && len(input.PageKeys) > 0 {
			pageParams := make(map[string]string)
			for i, k := range input.PageKeys {
				if i < len(input.PageValues) {
					pageParams[k] = input.PageValues[i]
				}
			}
			opts = append(opts, WithPageParams(pageParams))
		}

		if len(input.FilterKeys) > 0 {
			filterParams := make(map[string]string)
			for i, k := range input.FilterKeys {
				if i < len(input.FilterValues) {
					filterParams[k] = input.FilterValues[i]
				}
			}
			opts = append(opts, WithFilter(filterParams))
		}

		applied := applyOptions(opts)
		params := applied.buildQueryParams()

		// Verify include is comma-separated.
		if len(input.Include) > 0 {
			expected := strings.Join(input.Include, ",")
			if params.Get("include") != expected {
				t.Logf("include: got %q, want %q", params.Get("include"), expected)
				return false
			}
		} else {
			if params.Get("include") != "" {
				t.Logf("include should be empty, got %q", params.Get("include"))
				return false
			}
		}

		// Verify fields[type] is comma-separated per type.
		for i, typ := range input.FieldTypes {
			if i < len(input.FieldValues) {
				key := fmt.Sprintf("fields[%s]", typ)
				expected := strings.Join(input.FieldValues[i], ",")
				if params.Get(key) != expected {
					t.Logf("fields[%s]: got %q, want %q", typ, params.Get(key), expected)
					return false
				}
			}
		}

		// Verify sort is comma-separated.
		if len(input.Sort) > 0 {
			expected := strings.Join(input.Sort, ",")
			if params.Get("sort") != expected {
				t.Logf("sort: got %q, want %q", params.Get("sort"), expected)
				return false
			}
		} else {
			if params.Get("sort") != "" {
				t.Logf("sort should be empty, got %q", params.Get("sort"))
				return false
			}
		}

		// Verify page[number] and page[size] for page-number pagination.
		if input.UsePageNum && !input.UseCursor {
			expectedNum := strconv.Itoa(input.PageNumber)
			expectedSize := strconv.Itoa(input.PageSize)
			if params.Get("page[number]") != expectedNum {
				t.Logf("page[number]: got %q, want %q", params.Get("page[number]"), expectedNum)
				return false
			}
			if params.Get("page[size]") != expectedSize {
				t.Logf("page[size]: got %q, want %q", params.Get("page[size]"), expectedSize)
				return false
			}
		}

		// Verify page[after] and page[size] for cursor pagination.
		if input.UseCursor && !input.UsePageNum {
			expectedSize := strconv.Itoa(input.CursorSize)
			if params.Get("page[after]") != input.Cursor {
				t.Logf("page[after]: got %q, want %q", params.Get("page[after]"), input.Cursor)
				return false
			}
			if params.Get("page[size]") != expectedSize {
				t.Logf("page[size]: got %q, want %q", params.Get("page[size]"), expectedSize)
				return false
			}
		}

		// Verify page[key]=value for generic page params.
		if !input.UsePageNum && !input.UseCursor {
			for i, k := range input.PageKeys {
				if i < len(input.PageValues) {
					key := fmt.Sprintf("page[%s]", k)
					if params.Get(key) != input.PageValues[i] {
						t.Logf("page[%s]: got %q, want %q", k, params.Get(key), input.PageValues[i])
						return false
					}
				}
			}
		}

		// Verify filter[key]=value for filter params.
		for i, k := range input.FilterKeys {
			if i < len(input.FilterValues) {
				key := fmt.Sprintf("filter[%s]", k)
				if params.Get(key) != input.FilterValues[i] {
					t.Logf("filter[%s]: got %q, want %q", k, params.Get(key), input.FilterValues[i])
					return false
				}
			}
		}

		// Verify all values are properly URL-encoded by round-tripping through Encode/parse.
		encoded := params.Encode()
		parsed, parseErr := url.ParseQuery(encoded)
		if parseErr != nil {
			t.Logf("failed to parse encoded query string: %v", parseErr)
			return false
		}
		// Every key in the original params should be present after round-trip.
		for key, vals := range params {
			parsedVals, ok := parsed[key]
			if !ok {
				t.Logf("key %q missing after URL encode round-trip", key)
				return false
			}
			if len(vals) != len(parsedVals) {
				t.Logf("key %q: value count mismatch after round-trip: %d vs %d", key, len(vals), len(parsedVals))
				return false
			}
			for i, v := range vals {
				if v != parsedVals[i] {
					t.Logf("key %q[%d]: got %q, want %q after round-trip", key, i, parsedVals[i], v)
					return false
				}
			}
		}

		return true
	}, config)

	if err != nil {
		t.Errorf("Property 6 failed: %v", err)
	}
}
