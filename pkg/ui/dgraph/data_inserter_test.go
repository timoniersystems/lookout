package dgraph

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestLooksURLEncoded(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "valid URL encoding with percent-20",
			input:    "pkg%3Anpm%2Fexpress%404.17.1",
			expected: true,
		},
		{
			name:     "valid URL encoding with space",
			input:    "hello%20world",
			expected: true,
		},
		{
			name:     "no percent signs",
			input:    "pkg:npm/express@4.17.1",
			expected: false,
		},
		{
			name:     "percent sign but invalid encoding",
			input:    "pkg:npm/express%\xa2w@1.0",
			expected: false,
		},
		{
			name:     "percent at end without hex",
			input:    "test%",
			expected: false,
		},
		{
			name:     "percent with only one hex digit",
			input:    "test%2",
			expected: false,
		},
		{
			name:     "percent with non-hex characters",
			input:    "test%GG",
			expected: false,
		},
		{
			name:     "multiple valid encodings",
			input:    "pkg%3Anpm%2Fexpress%404.17.1%20test",
			expected: true,
		},
		{
			name:     "mixed valid and invalid",
			input:    "valid%20but%also%invalid",
			expected: true, // Has at least one valid encoding
		},
		{
			name:     "empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "uppercase hex digits",
			input:    "test%2F%3A%40",
			expected: true,
		},
		{
			name:     "lowercase hex digits",
			input:    "test%2f%3a%40",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := looksURLEncoded(tt.input)
			if result != tt.expected {
				t.Errorf("looksURLEncoded(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestEncodeNodeID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string // The expected decoded value before base64 encoding
	}{
		{
			name:     "URL-encoded input",
			input:    "test%20id",
			expected: "test id", // Should decode %20 to space
		},
		{
			name:     "non-URL-encoded input",
			input:    "test-id",
			expected: "test-id", // Should remain unchanged
		},
		{
			name:     "invalid URL encoding",
			input:    "test%\xa2w",
			expected: "test%\xa2w", // Should remain unchanged (not decoded)
		},
		{
			name:     "complex URL-encoded string",
			input:    "pkg%3Anpm%2Fexpress",
			expected: "pkg:npm/express",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EncodeNodeID(tt.input)

			// Decode the base64 result to verify the intermediate decoded value
			decoded, err := base64.URLEncoding.DecodeString(result)
			if err != nil {
				t.Fatalf("Failed to decode base64 result: %v", err)
			}

			decodedStr := string(decoded)
			if decodedStr != tt.expected {
				t.Errorf("EncodeNodeID(%q) decoded to %q, expected %q", tt.input, decodedStr, tt.expected)
			}
		})
	}
}

func TestEncodePURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string // The expected decoded value before base64 encoding
	}{
		{
			name:     "URL-encoded PURL",
			input:    "pkg%3Anpm%2Fexpress%404.17.1",
			expected: "pkg:npm/express@4.17.1",
		},
		{
			name:     "non-URL-encoded PURL",
			input:    "pkg:npm/express@4.17.1",
			expected: "pkg:npm/express@4.17.1",
		},
		{
			name:     "PURL with invalid encoding",
			input:    "pkg:npm/express%\xa2w@1.0",
			expected: "pkg:npm/express%\xa2w@1.0", // Should not be decoded
		},
		{
			name:     "composer PURL",
			input:    "pkg:composer/symfony/http-foundation@v5.4.16",
			expected: "pkg:composer/symfony/http-foundation@v5.4.16",
		},
		{
			name:     "URL-encoded composer PURL",
			input:    "pkg%3Acomposer%2Fsymfony%2Fhttp-foundation%40v5.4.16",
			expected: "pkg:composer/symfony/http-foundation@v5.4.16",
		},
		{
			name:     "PURL with spaces",
			input:    "pkg%3Anpm%2Fsome%20package%401.0.0",
			expected: "pkg:npm/some package@1.0.0",
		},
		{
			name:     "empty PURL",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := encodePURL(tt.input)

			// Decode the base64 result to verify the intermediate decoded value
			decoded, err := base64.URLEncoding.DecodeString(result)
			if err != nil {
				t.Fatalf("Failed to decode base64 result: %v", err)
			}

			decodedStr := string(decoded)
			if decodedStr != tt.expected {
				t.Errorf("encodePURL(%q) decoded to %q, expected %q", tt.input, decodedStr, tt.expected)
			}
		})
	}
}

func TestEncodePURL_Base64Output(t *testing.T) {
	// Test that the function actually returns base64-encoded output
	input := "pkg:npm/express@4.17.1"
	result := encodePURL(input)

	// Verify it's valid base64
	decoded, err := base64.URLEncoding.DecodeString(result)
	if err != nil {
		t.Errorf("encodePURL(%q) did not return valid base64: %v", input, err)
	}

	// Verify decoding gives us back the original
	if string(decoded) != input {
		t.Errorf("encodePURL(%q) base64 decoded to %q, expected %q", input, string(decoded), input)
	}
}

func TestDecodePURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid base64 PURL",
			input:    base64.URLEncoding.EncodeToString([]byte("pkg:npm/express@4.17.1")),
			expected: "pkg:npm/express@4.17.1",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "base64 with special characters",
			input:    base64.URLEncoding.EncodeToString([]byte("pkg:composer/symfony/http-foundation@v5.4.16")),
			expected: "pkg:composer/symfony/http-foundation@v5.4.16",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := decodePURL(tt.input)
			if result != tt.expected {
				t.Errorf("decodePURL(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "PURL with package and version",
			input:    "pkg:npm/express@4.17.1",
			expected: "express@4.17.1",
		},
		{
			name:     "composer PURL",
			input:    "pkg:composer/symfony/http-foundation@v5.4.16",
			expected: "http-foundation@v5.4.16",
		},
		{
			name:     "PURL without version",
			input:    "pkg:npm/express",
			expected: "pkg:npm/express",
		},
		{
			name:     "simple name",
			input:    "express@1.0.0",
			expected: "express@1.0.0",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractName(tt.input)
			if result != tt.expected {
				t.Errorf("extractName(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

// listInQuery returns the refs inside `eq(bomRef, [...])`, decoded as the JSON
// array they are written as, so a test can assert exactly what is asked for.
func listInQuery(t *testing.T, query string) []string {
	t.Helper()
	const open = "eq(bomRef, "
	start := strings.Index(query, open)
	// The LAST `]))` closes the list: a ref may itself contain `))`.
	end := strings.LastIndex(query, "]))")
	if start < 0 || end < start {
		t.Fatalf("query has no eq(bomRef, [...]) lookup:\n%s", query)
	}
	var refs []string
	if err := json.Unmarshal([]byte(query[start+len(open):end+1]), &refs); err != nil {
		t.Fatalf("the ref list is not a valid literal list (%v):\n%s", err, query)
	}
	return refs
}

// lookout#63: the lookup must name this BOM's refs, never read every component.
// `has(bomRef)` returned 10,587,961 bytes at 94,761 components, over the 10 MiB
// gRPC cap, and every insert failed.
func TestExistingComponentsQuery_AsksOnlyForTheGivenRefs(t *testing.T) {
	refs := []string{"pkg:golang/a@v1", "pkg:npm/b@2"}
	query := existingComponentsQuery(refs)
	if strings.Contains(query, "has(bomRef)") {
		t.Fatalf("query reads every component:\n%s", query)
	}
	if got := listInQuery(t, query); strings.Join(got, "|") != strings.Join(refs, "|") {
		t.Fatalf("query asks for %q, want exactly %q", got, refs)
	}
}

// A ref must not be able to end the list early or change the query. Each is
// written as a JSON string literal, which dgraph v25.4.1 parses the same way
// (measured with these characters against a live alpha).
func TestExistingComponentsQuery_QuotesEveryRef(t *testing.T) {
	refs := []string{
		`pkg:npm/q"uote@1`,
		`pkg:x/back\slash@2`,
		"pkg:maven/org.x/a@1.0?type=jar&classifier=sources",
		"pkg:pypi/café@3",
		`pkg:x/close"]) } { all(func: has(bomRef)) { uid } }`,
	}
	got := listInQuery(t, existingComponentsQuery(refs))
	if strings.Join(got, "|") != strings.Join(refs, "|") {
		t.Fatalf("refs did not survive quoting:\n got %q\nwant %q", got, refs)
	}
}

func TestRefBatches_BoundsDedupesAndDropsEmpty(t *testing.T) {
	refs := make([]string, 0, 1203)
	for i := 0; i < 1201; i++ {
		refs = append(refs, fmt.Sprintf("pkg:x/c%d@1", i))
	}
	refs = append(refs, "", "pkg:x/c0@1") // an empty ref and a repeat

	batches := refBatches(refs, 500)
	var sizes []int
	seen := map[string]int{}
	for _, b := range batches {
		sizes = append(sizes, len(b))
		for _, r := range b {
			seen[r]++
		}
	}
	if fmt.Sprint(sizes) != "[500 500 201]" {
		t.Fatalf("batch sizes %v, want [500 500 201]", sizes)
	}
	if _, ok := seen[""]; ok {
		t.Fatal("an empty ref was looked up")
	}
	if seen["pkg:x/c0@1"] != 1 || len(seen) != 1201 {
		t.Fatalf("want each of 1201 refs exactly once, got %d distinct, c0 %d times", len(seen), seen["pkg:x/c0@1"])
	}
	if got := refBatches(nil, 500); len(got) != 0 {
		t.Fatalf("no refs must mean no query, got %d batches", len(got))
	}
}

// Benchmark tests to ensure performance is acceptable
func BenchmarkLooksURLEncoded(b *testing.B) {
	testStrings := []string{
		"pkg:npm/express@4.17.1",
		"pkg%3Anpm%2Fexpress%404.17.1",
		"test%\xa2w",
		"no-encoding-here",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, s := range testStrings {
			looksURLEncoded(s)
		}
	}
}

func BenchmarkEncodePURL(b *testing.B) {
	testPURLs := []string{
		"pkg:npm/express@4.17.1",
		"pkg%3Anpm%2Fexpress%404.17.1",
		"pkg:composer/symfony/http-foundation@v5.4.16",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, purl := range testPURLs {
			encodePURL(purl)
		}
	}
}
