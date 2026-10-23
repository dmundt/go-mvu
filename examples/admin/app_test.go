package main

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/dmundt/go-mvu/validation"
)

func TestDirectoryPreparesRowsTotalsAndPageBounds(t *testing.T) {
	model, _, err := (App{}).Init(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		query, role                         string
		page, wantPage, total, first, count int
	}{
		{"", "", -1, 1, 18, 1, 5},
		{"", "", 2, 2, 18, 6, 5},
		{"", "", int(^uint(0) >> 1), 4, 18, 16, 3},
		{"EXAMPLE", roleAdmin, 2, 2, 6, 16, 1},
		{"MEMBER02@", "", 1, 1, 1, 2, 1},
		{"missing", "", 99, 1, 0, 0, 0},
	} {
		t.Run(test.query+"/"+test.role, func(t *testing.T) {
			model.Query, model.Role, model.Page = test.query, test.role, test.page
			result := model.directory()
			if result.Total != test.total || result.Page != test.wantPage ||
				len(result.Members) != test.count {
				t.Fatalf("directory = %+v", result)
			}
			if test.count > 0 && result.Members[0].ID != test.first {
				t.Fatalf("first member = %+v", result.Members[0])
			}
		})
	}
}

func TestMemberOperationsRejectUnknownIDsWithoutChangingModel(t *testing.T) {
	model, _, err := (App{}).Init(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, message := range []Message{
		Edit{ID: memberCount + 1},
		Save{Form: validation.Form[Editor]{Value: Editor{ID: memberCount + 1}}},
		Save{Form: validation.Form[Editor]{Value: Editor{ID: memberCount + 1}, Valid: true}},
	} {
		next, _, err := (App{}).Update(context.Background(), model, message)
		if !errors.Is(err, errUnknownMember) || !reflect.DeepEqual(next, model) {
			t.Fatalf("%T: model changed or error missing: %v", message, err)
		}
	}
}

func TestSaveClampsFilteredPageAndPreservesOtherMemberFields(t *testing.T) {
	model, _, err := (App{}).Init(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	model.Role, model.Page = roleAdmin, 2
	before := model.Members[15]
	next, _, err := (App{}).Update(context.Background(), model, Save{
		Form: validation.Form[Editor]{
			Value: Editor{ID: before.ID, Name: "Updated member", Role: roleViewer},
			Valid: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := before
	want.Name, want.Role = "Updated member", roleViewer
	if next.Members[15] != want || model.Members[15] != before ||
		next.Page != 1 || next.Changes != 1 || next.Form.Value.ID != 0 || next.Notice == "" {
		t.Fatalf("save result = %+v", next)
	}
}

func BenchmarkDirectory(b *testing.B) {
	model, _, err := (App{}).Init(context.Background())
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		model.directory()
	}
}
