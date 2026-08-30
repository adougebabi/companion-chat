package core

import (
	"context"
	"testing"
)

type fakeRepository struct{}

func (fakeRepository) Ping(context.Context) error                             { return nil }
func (fakeRepository) ResolveSession(context.Context, string) (string, error) { return "human-1", nil }
func (fakeRepository) ListFluctlights(context.Context, string) ([]Fluctlight, error) {
	return []Fluctlight{{ID: "fl-1", Status: "active"}}, nil
}
func (fakeRepository) GetFluctlight(context.Context, string, string) (Fluctlight, error) {
	return Fluctlight{ID: "fl-1"}, nil
}
func (fakeRepository) DirectConversationID(context.Context, string, string) (string, error) {
	return "conversation-1", nil
}
func (fakeRepository) History(context.Context, string, string, *int, int) (ConversationPage, error) {
	return ConversationPage{}, nil
}

func TestRepositoryContractCanBeImplementedWithoutTransportCoupling(t *testing.T) {
	var repository Repository = fakeRepository{}
	if err := repository.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
}
