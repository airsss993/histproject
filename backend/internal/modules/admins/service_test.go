package admins

import (
	"context"
	"errors"
	"testing"
)

type adminStoreStub struct {
	created      Admin
	input        CreateRecord
	updated      UpdateRecord
	createCalled bool
}

func (s *adminStoreStub) Create(_ context.Context, in CreateRecord) (Admin, error) {
	s.createCalled = true
	s.input = in
	return s.created, nil
}
func (s *adminStoreStub) CreateInitialSuperAdmin(context.Context, CreateRecord) (Admin, error) {
	return Admin{}, nil
}
func (s *adminStoreStub) List(context.Context) ([]Admin, error)                  { return nil, nil }
func (s *adminStoreStub) ChangeRole(context.Context, int64, Role) (Admin, error) { return Admin{}, nil }
func (s *adminStoreStub) Delete(context.Context, int64) error                    { return nil }
func (s *adminStoreStub) Get(context.Context, int64) (Admin, error)              { return Admin{}, nil }
func (s *adminStoreStub) Update(_ context.Context, _ int64, in UpdateRecord) (Admin, error) {
	s.updated = in
	return Admin{}, nil
}

func TestCreateUsesEmailWithoutPassword(t *testing.T) {
	store := &adminStoreStub{}
	service := NewService(store)
	_, err := service.Create(t.Context(), CreateInput{Name: " Иван Петров ", Email: " USER@Example.Test ", Role: RoleAdmin})
	if err != nil || store.input.Name != "Иван Петров" || store.input.Email != "user@example.test" || store.input.PasswordHash != "disabled" || store.input.Login == "" {
		t.Fatalf("input = %#v, err = %v", store.input, err)
	}
	store.createCalled = false
	_, err = service.Create(t.Context(), CreateInput{Name: "Иван", Email: "", Role: RoleAdmin})
	var validation *ValidationError
	if !errors.As(err, &validation) || validation.Fields["email"] == "" || store.createCalled {
		t.Fatalf("validation = %#v, called = %v", err, store.createCalled)
	}
}

func TestRoleAndUpdateGuard(t *testing.T) {
	service := NewService(&adminStoreStub{})
	if _, err := service.ChangeRole(t.Context(), 7, 7, RoleAdmin); !errors.Is(err, ErrSelfMutation) {
		t.Fatalf("role: %v", err)
	}
	if err := service.Delete(t.Context(), 7, 7); !errors.Is(err, ErrSelfMutation) {
		t.Fatalf("delete: %v", err)
	}
	if _, err := service.Update(t.Context(), 7, 7, UpdateInput{}); !errors.Is(err, ErrSelfMutation) {
		t.Fatalf("update: %v", err)
	}
}
