// Package items contains the item domain: validation, identifiers and the
// repository abstraction. The HTTP layer depends only on this package.
package items

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	maxNameLength        = 120
	maxDescriptionLength = 500
)

// ErrNotFound is returned when an item does not exist.
var ErrNotFound = errors.New("item not found")

// ValidationError describes invalid caller input. It is safe to return to clients.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

// Item is the public representation of a stored item.
type Item struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateInput is the validated input for creating an item.
type CreateInput struct {
	Name        string
	Description string
}

// Page is a window over the collection.
type Page struct {
	Items  []Item
	Total  int
	Limit  int
	Offset int
}

// Repository is the storage port. Implementations must be safe for concurrent use.
type Repository interface {
	List(ctx context.Context, offset, limit int) (Page, error)
	Get(ctx context.Context, id string) (Item, error)
	Insert(ctx context.Context, item Item) error
}

// Service applies domain rules on top of a Repository.
type Service struct {
	repo  Repository
	clock func() time.Time
	newID func() (string, error)
}

// NewService returns a Service backed by repo.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, clock: time.Now, newID: randomID}
}

// List returns one page of items.
func (s *Service) List(ctx context.Context, offset, limit int) (Page, error) {
	return s.repo.List(ctx, offset, limit)
}

// Get returns a single item by ID.
func (s *Service) Get(ctx context.Context, id string) (Item, error) {
	return s.repo.Get(ctx, id)
}

// Create validates the input, assigns an ID and persists the item.
func (s *Service) Create(ctx context.Context, in CreateInput) (Item, error) {
	clean, err := validate(in)
	if err != nil {
		return Item{}, err
	}
	id, err := s.newID()
	if err != nil {
		return Item{}, fmt.Errorf("generate item id: %w", err)
	}
	item := Item{ID: id, Name: clean.Name, Description: clean.Description, CreatedAt: s.clock().UTC()}
	if err := s.repo.Insert(ctx, item); err != nil {
		return Item{}, fmt.Errorf("insert item: %w", err)
	}
	return item, nil
}

func validate(in CreateInput) (CreateInput, error) {
	out := CreateInput{
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
	}
	switch {
	case out.Name == "":
		return CreateInput{}, &ValidationError{Message: "name is required"}
	case len([]rune(out.Name)) > maxNameLength:
		return CreateInput{}, &ValidationError{Message: fmt.Sprintf("name must be at most %d characters", maxNameLength)}
	case len([]rune(out.Description)) > maxDescriptionLength:
		return CreateInput{}, &ValidationError{Message: fmt.Sprintf("description must be at most %d characters", maxDescriptionLength)}
	}
	return out, nil
}

func randomID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
