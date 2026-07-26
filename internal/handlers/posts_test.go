package handlers

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestParseHomeFilters(t *testing.T) {
	request := httptest.NewRequest("GET", "/?filter=liked&category=Go&q=%20sqlite%20&sort=liked", nil)

	got := parseHomeFilters(request)
	want := homeFilters{
		filter:   "liked",
		category: "Go",
		search:   "sqlite",
		sort:     "liked",
	}

	if got != want {
		t.Fatalf("parseHomeFilters() = %#v, want %#v", got, want)
	}
}

func TestParsePostFormParsesMultipartValues(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := []struct {
		name  string
		value string
	}{
		{name: "title", value: "Recall"},
		{name: "content", value: "Practice HTTP parsing"},
		{name: "categories", value: "Go"},
		{name: "categories", value: "Web"},
	}
	for _, field := range fields {
		if err := writer.WriteField(field.name, field.value); err != nil {
			t.Fatalf("WriteField() error: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("multipart writer close error: %v", err)
	}

	request := httptest.NewRequest("POST", "/post/create", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())

	got, err := parsePostForm(request)
	if err != nil {
		t.Fatalf("parsePostForm() unexpected error: %v", err)
	}
	if got.title != "Recall" || got.content != "Practice HTTP parsing" {
		t.Fatalf("single-value fields parsed incorrectly: %#v", got)
	}
	if want := []string{"Go", "Web"}; !reflect.DeepEqual(got.categories, want) {
		t.Fatalf("categories = %#v, want %#v", got.categories, want)
	}
}

func TestNormalizeSortRejectsUnknownValues(t *testing.T) {
	if got := normalizeSort("DROP TABLE posts"); got != "newest" {
		t.Fatalf("normalizeSort() = %q, want newest", got)
	}
}

func TestValidatePostForm(t *testing.T) {
	tests := []struct {
		name    string
		form    postForm
		wantErr string
	}{
		{
			name:    "empty title",
			form:    postForm{content: "body", categories: []string{"Go"}},
			wantErr: "Title and Content cannot be empty",
		},
		{
			name:    "missing category",
			form:    postForm{title: "title", content: "body"},
			wantErr: "At least one category is required",
		},
		{
			name: "valid",
			form: postForm{title: "title", content: "body", categories: []string{"Go"}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validatePostForm(test.form)
			if test.wantErr == "" && err != nil {
				t.Fatalf("validatePostForm() unexpected error: %v", err)
			}
			if test.wantErr != "" && (err == nil || err.Error() != test.wantErr) {
				t.Fatalf("validatePostForm() error = %v, want %q", err, test.wantErr)
			}
		})
	}
}

func TestBuildHomeQueryKeepsPlaceholdersAndArgumentsAligned(t *testing.T) {
	filters := homeFilters{
		filter:   "liked",
		category: "Go",
		search:   "goroutine",
		sort:     "discussed",
	}

	query, args := buildHomeQuery(filters, 42, true)

	if got, want := strings.Count(query, "?"), len(args); got != want {
		t.Fatalf("placeholder count = %d, argument count = %d", got, want)
	}
	wantArgs := []any{"Go", 42, "%goroutine%", "%goroutine%", "%goroutine%", "%goroutine%"}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", args, wantArgs)
	}
	if !strings.Contains(query, "ORDER BY comment_count DESC") {
		t.Fatalf("query does not contain discussed ordering: %s", query)
	}
}
