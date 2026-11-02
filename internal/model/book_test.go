package model

import (
	"encoding/json"
	"testing"
)

func TestBookInputRequiresRelations(t *testing.T) {
	for _, in := range []BookInput{{}, {Title: "x", ISBN: "1", CategoryID: 1}, {Title: "x", ISBN: "1", AuthorIDs: []uint{1}}, {Title: " ", ISBN: "1", CategoryID: 1, AuthorIDs: []uint{1}}} {
		if in.Valid() {
			t.Fatalf("accepted invalid input: %+v", in)
		}
	}
}
func TestBookInputRejectsDuplicateAuthors(t *testing.T) {
	if (BookInput{Title: "x", ISBN: "1", CategoryID: 1, AuthorIDs: []uint{1, 1}}).Valid() {
		t.Fatal("duplicate IDs accepted")
	}
}
func TestBookInputRejectsZeroAuthor(t *testing.T) {
	if (BookInput{Title: "x", ISBN: "1", CategoryID: 1, AuthorIDs: []uint{0}}).Valid() {
		t.Fatal("zero author accepted")
	}
}
func TestBookInputAcceptsMultipleAuthors(t *testing.T) {
	if !(BookInput{Title: "x", ISBN: "1", CategoryID: 1, AuthorIDs: []uint{1, 2}}).Valid() {
		t.Fatal("valid book rejected")
	}
}
func TestCopyStatuses(t *testing.T) {
	for _, s := range []string{"available", "borrowed", "lost"} {
		if !ValidCopyStatus(s) {
			t.Fatal(s)
		}
	}
	for _, s := range []string{"", "broken", "AVAILABLE"} {
		if ValidCopyStatus(s) {
			t.Fatal(s)
		}
	}
}
func TestBookOptionalFieldsRemainNull(t *testing.T) {
	raw, err := json.Marshal(Book{Authors: []Author{}})
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"published_year", "description"} {
		v, present := out[key]
		if !present || v != nil {
			t.Fatalf("%s should be present and null", key)
		}
	}
}
