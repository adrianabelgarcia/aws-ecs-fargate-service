package items

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeRepo struct {
	insertErr error
	inserted  []Item
	getErr    error
}

func (f *fakeRepo) List(context.Context, int, int) (Page, error) { return Page{}, nil }
func (f *fakeRepo) Get(context.Context, string) (Item, error) {
	return Item{}, f.getErr
}
func (f *fakeRepo) Insert(_ context.Context, it Item) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.inserted = append(f.inserted, it)
	return nil
}

func TestService_Create_TrimsAndStamps(t *testing.T) {
	t.Parallel()
	repo := &fakeRepo{}
	svc := NewService(repo)
	svc.newID = func() (string, error) { return "abc123", nil }
	svc.clock = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.FixedZone("x", 3600)) }

	got, err := svc.Create(context.Background(), CreateInput{Name: "  Widget ", Description: " blue "})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if got.ID != "abc123" || got.Name != "Widget" || got.Description != "blue" {
		t.Errorf("Create() = %+v", got)
	}
	if got.CreatedAt.Location() != time.UTC {
		t.Errorf("CreatedAt not normalised to UTC: %v", got.CreatedAt.Location())
	}
	if len(repo.inserted) != 1 {
		t.Fatalf("expected one insert, got %d", len(repo.inserted))
	}
}

func TestService_Create_Validation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   CreateInput
		want string
	}{
		{"empty name", CreateInput{Name: "   "}, "name is required"},
		{"name too long", CreateInput{Name: strings.Repeat("a", maxNameLength+1)}, "name must be at most"},
		{"description too long", CreateInput{Name: "ok", Description: strings.Repeat("é", maxDescriptionLength+1)}, "description must be at most"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := &fakeRepo{}
			_, err := NewService(repo).Create(context.Background(), tt.in)
			var vErr *ValidationError
			if !errors.As(err, &vErr) || !strings.Contains(vErr.Error(), tt.want) {
				t.Fatalf("Create() error = %v, want ValidationError containing %q", err, tt.want)
			}
			if len(repo.inserted) != 0 {
				t.Error("invalid input must not be persisted")
			}
		})
	}
}

func TestService_Create_PropagatesFailures(t *testing.T) {
	t.Parallel()
	t.Run("id generation", func(t *testing.T) {
		t.Parallel()
		svc := NewService(&fakeRepo{})
		svc.newID = func() (string, error) { return "", errors.New("entropy exhausted") }
		if _, err := svc.Create(context.Background(), CreateInput{Name: "x"}); err == nil || !strings.Contains(err.Error(), "generate item id") {
			t.Fatalf("Create() error = %v, want id generation failure", err)
		}
	})
	t.Run("insert", func(t *testing.T) {
		t.Parallel()
		svc := NewService(&fakeRepo{insertErr: errors.New("disk on fire")})
		if _, err := svc.Create(context.Background(), CreateInput{Name: "x"}); err == nil || !strings.Contains(err.Error(), "insert item") {
			t.Fatalf("Create() error = %v, want insert failure", err)
		}
	})
}

func TestService_DelegatesReadsToRepository(t *testing.T) {
	t.Parallel()
	svc := NewService(&fakeRepo{getErr: ErrNotFound})
	if _, err := svc.Get(context.Background(), "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}
	page, err := svc.List(context.Background(), 0, 10)
	if err != nil || page.Items != nil {
		t.Errorf("List() = %+v, %v", page, err)
	}
}

func TestRandomID_IsUniqueHex(t *testing.T) {
	t.Parallel()
	a, err := randomID()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := randomID()
	if len(a) != 32 || a == b {
		t.Errorf("randomID() = %q, %q; want 32 hex chars and distinct values", a, b)
	}
}

func TestMemoryRepository_Pagination(t *testing.T) {
	t.Parallel()
	repo := NewMemoryRepository()
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if err := repo.Insert(ctx, Item{ID: string(rune('a' + i)), Name: "n"}); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name      string
		offset    int
		limit     int
		wantLen   int
		wantFirst string
		wantTotal int
	}{
		{"first page", 0, 2, 2, "a", 5},
		{"partial last page", 4, 10, 1, "e", 5},
		{"offset past end", 9, 10, 0, "", 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, err := repo.List(ctx, tt.offset, tt.limit)
			if err != nil {
				t.Fatal(err)
			}
			if len(page.Items) != tt.wantLen || page.Total != tt.wantTotal {
				t.Fatalf("List() len=%d total=%d, want len=%d total=%d", len(page.Items), page.Total, tt.wantLen, tt.wantTotal)
			}
			if tt.wantFirst != "" && page.Items[0].ID != tt.wantFirst {
				t.Errorf("first ID = %q, want %q", page.Items[0].ID, tt.wantFirst)
			}
		})
	}
	if _, err := repo.List(ctx, -1, 1); err == nil {
		t.Error("negative offset should be rejected")
	}
	if _, err := repo.List(ctx, 0, 0); err == nil {
		t.Error("zero limit should be rejected")
	}
}

func TestMemoryRepository_GetAndDuplicateInsert(t *testing.T) {
	t.Parallel()
	repo := NewMemoryRepository()
	ctx := context.Background()
	if _, err := repo.Get(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(missing) error = %v, want ErrNotFound", err)
	}
	if err := repo.Insert(ctx, Item{ID: "x", Name: "one"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Insert(ctx, Item{ID: "x", Name: "two"}); err == nil {
		t.Fatal("duplicate insert should fail")
	}
	got, err := repo.Get(ctx, "x")
	if err != nil || got.Name != "one" {
		t.Fatalf("Get(x) = %+v, %v", got, err)
	}
}
