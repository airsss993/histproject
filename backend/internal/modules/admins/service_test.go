package admins

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestCreateValidatesInput(t *testing.T) {
	tests := []struct {
		name  string
		input CreateInput
		field string
	}{
		{
			name:  "login",
			input: CreateInput{Login: "x", Password: "long-enough-password", Email: "admin@example.test", Role: RoleAdmin},
			field: "login",
		},
		{
			name:  "password",
			input: CreateInput{Login: "new-admin", Password: "short", Email: "admin@example.test", Role: RoleAdmin},
			field: "password",
		},
		{
			name:  "email",
			input: CreateInput{Login: "new-admin", Password: "long-enough-password", Email: "not-email", Role: RoleAdmin},
			field: "email",
		},
		{
			name:  "name",
			input: CreateInput{Login: "new-admin", Name: "  ", Password: "long-enough-password", Role: RoleAdmin},
			field: "name",
		},
		{
			name:  "role",
			input: CreateInput{Login: "new-admin", Password: "long-enough-password", Email: "admin@example.test", Role: Role("root")},
			field: "role",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &adminStoreStub{}
			service := NewService(store)

			_, err := service.Create(t.Context(), test.input)
			var validationError *ValidationError
			if !errors.As(err, &validationError) {
				t.Fatalf("Create() error = %v, want ValidationError", err)
			}
			if validationError.Fields[test.field] == "" {
				t.Fatalf("validation fields = %#v, want %s", validationError.Fields, test.field)
			}
			if store.createCalled {
				t.Fatal("store.Create was called for invalid input")
			}
		})
	}
}

func TestCreateHashesPasswordAndReturnsAdmin(t *testing.T) {
	store := &adminStoreStub{
		createdAdmin: Admin{ID: 8, Login: "new-admin", Email: "admin@example.test", Role: RoleAdmin},
	}
	service := NewService(store)

	admin, err := service.Create(t.Context(), CreateInput{
		Login:    " new-admin ",
		Name:     " Иван Петров ",
		Password: "long-enough-password",
		Email:    " admin@example.test ",
		Role:     RoleAdmin,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if admin != store.createdAdmin {
		t.Fatalf("Create() = %#v", admin)
	}
	if store.createInput.Login != "new-admin" || store.createInput.Name != "Иван Петров" || store.createInput.Email != "admin@example.test" {
		t.Fatalf("normalized input = %#v", store.createInput)
	}
	if store.createInput.PasswordHash == "long-enough-password" {
		t.Fatal("password was stored without hashing")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(store.createInput.PasswordHash), []byte("long-enough-password")); err != nil {
		t.Fatalf("stored password hash does not match: %v", err)
	}
}

func TestBootstrapCreatesInitialSuperAdmin(t *testing.T) {
	store := &adminStoreStub{
		bootstrappedAdmin: Admin{ID: 1, Login: "root", Email: "root@example.test", Role: RoleSuperAdmin},
	}
	service := NewService(store)

	admin, err := service.Bootstrap(t.Context(), CreateInput{
		Login:    " root ",
		Password: "long-enough-password",
		Email:    " root@example.test ",
	})
	if err != nil {
		t.Fatalf("Bootstrap() error = %v", err)
	}
	if admin != store.bootstrappedAdmin {
		t.Fatalf("Bootstrap() = %#v", admin)
	}
	if store.bootstrapInput.Login != "root" || store.bootstrapInput.Email != "root@example.test" {
		t.Fatalf("normalized input = %#v", store.bootstrapInput)
	}
	if store.bootstrapInput.Role != RoleSuperAdmin {
		t.Fatalf("bootstrap role = %q, want super_admin", store.bootstrapInput.Role)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(store.bootstrapInput.PasswordHash), []byte("long-enough-password")); err != nil {
		t.Fatalf("bootstrap password hash does not match: %v", err)
	}
}

func TestBootstrapPropagatesExistingSuperAdminConflict(t *testing.T) {
	store := &adminStoreStub{bootstrapError: ErrSuperAdminExists}
	service := NewService(store)

	_, err := service.Bootstrap(t.Context(), CreateInput{
		Login:    "root",
		Password: "long-enough-password",
	})
	if !errors.Is(err, ErrSuperAdminExists) {
		t.Fatalf("Bootstrap() error = %v, want ErrSuperAdminExists", err)
	}
}

func TestChangeRoleRejectsSelfMutation(t *testing.T) {
	store := &adminStoreStub{}
	service := NewService(store)

	_, err := service.ChangeRole(t.Context(), 7, 7, RoleAdmin)
	if !errors.Is(err, ErrSelfMutation) {
		t.Fatalf("ChangeRole() error = %v, want ErrSelfMutation", err)
	}
	if store.changeRoleCalled {
		t.Fatal("store.ChangeRole was called for self mutation")
	}
}

func TestDeleteRejectsSelfMutation(t *testing.T) {
	store := &adminStoreStub{}
	service := NewService(store)

	err := service.Delete(t.Context(), 7, 7)
	if !errors.Is(err, ErrSelfMutation) {
		t.Fatalf("Delete() error = %v, want ErrSelfMutation", err)
	}
	if store.deleteCalled {
		t.Fatal("store.Delete was called for self mutation")
	}
}

func TestChangeRoleRejectsInvalidRole(t *testing.T) {
	store := &adminStoreStub{}
	service := NewService(store)

	_, err := service.ChangeRole(t.Context(), 1, 2, Role("root"))
	var validationError *ValidationError
	if !errors.As(err, &validationError) || validationError.Fields["role"] == "" {
		t.Fatalf("ChangeRole() error = %v, want role ValidationError", err)
	}
}

func TestServicePropagatesLastSuperAdminConflict(t *testing.T) {
	store := &adminStoreStub{changeRoleError: ErrLastSuperAdmin, deleteError: ErrLastSuperAdmin}
	service := NewService(store)

	if _, err := service.ChangeRole(t.Context(), 1, 2, RoleAdmin); !errors.Is(err, ErrLastSuperAdmin) {
		t.Fatalf("ChangeRole() error = %v", err)
	}
	if err := service.Delete(t.Context(), 1, 2); !errors.Is(err, ErrLastSuperAdmin) {
		t.Fatalf("Delete() error = %v", err)
	}
}

type adminStoreStub struct {
	createCalled      bool
	createInput       CreateRecord
	createdAdmin      Admin
	createError       error
	bootstrapInput    CreateRecord
	bootstrappedAdmin Admin
	bootstrapError    error
	changeRoleCalled  bool
	changedAdmin      Admin
	changeRoleError   error
	deleteCalled      bool
	deleteError       error
	admins            []Admin
	listError         error
}

func (stub *adminStoreStub) Create(_ context.Context, input CreateRecord) (Admin, error) {
	stub.createCalled = true
	stub.createInput = input
	return stub.createdAdmin, stub.createError
}

func (stub *adminStoreStub) CreateInitialSuperAdmin(_ context.Context, input CreateRecord) (Admin, error) {
	stub.bootstrapInput = input
	return stub.bootstrappedAdmin, stub.bootstrapError
}

func (stub *adminStoreStub) List(context.Context) ([]Admin, error) {
	return stub.admins, stub.listError
}

func (stub *adminStoreStub) ChangeRole(_ context.Context, targetID int64, role Role) (Admin, error) {
	stub.changeRoleCalled = true
	return stub.changedAdmin, stub.changeRoleError
}

func (stub *adminStoreStub) Delete(context.Context, int64) error {
	stub.deleteCalled = true
	return stub.deleteError
}
