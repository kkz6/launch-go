package testutil

import (
	"errors"
	"testing"

	"gorm.io/gorm"
)

// testEntity is a simple entity for testing the MockStore.
type testEntity struct {
	ID   string
	Name string
}

func getTestEntityID(e *testEntity) string {
	return e.ID
}

func TestMockStore_Create(t *testing.T) {
	store := NewMockStore[testEntity]()
	entity := &testEntity{ID: "1", Name: "test"}

	err := store.Create(entity, getTestEntityID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if store.Count() != 1 {
		t.Errorf("expected count 1, got %d", store.Count())
	}
}

func TestMockStore_Create_WithError(t *testing.T) {
	store := NewMockStore[testEntity]()
	expectedErr := errors.New("create failed")
	store.SetError("Create", expectedErr)

	entity := &testEntity{ID: "1", Name: "test"}
	err := store.Create(entity, getTestEntityID)

	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestMockStore_FindByID(t *testing.T) {
	store := NewMockStore[testEntity]()
	entity := &testEntity{ID: "1", Name: "test"}
	_ = store.Create(entity, getTestEntityID)

	found, err := store.FindByID("1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.Name != "test" {
		t.Errorf("expected name 'test', got %s", found.Name)
	}
}

func TestMockStore_FindByID_NotFound(t *testing.T) {
	store := NewMockStore[testEntity]()

	_, err := store.FindByID("nonexistent")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("expected gorm.ErrRecordNotFound, got %v", err)
	}
}

func TestMockStore_FindByIDOrNil(t *testing.T) {
	store := NewMockStore[testEntity]()

	found, err := store.FindByIDOrNil("nonexistent")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found != nil {
		t.Error("expected nil, got item")
	}
}

func TestMockStore_Update(t *testing.T) {
	store := NewMockStore[testEntity]()
	entity := &testEntity{ID: "1", Name: "original"}
	_ = store.Create(entity, getTestEntityID)

	updated := &testEntity{ID: "1", Name: "updated"}
	err := store.Update(updated, getTestEntityID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	found, _ := store.FindByID("1")
	if found.Name != "updated" {
		t.Errorf("expected name 'updated', got %s", found.Name)
	}
}

func TestMockStore_Delete(t *testing.T) {
	store := NewMockStore[testEntity]()
	entity := &testEntity{ID: "1", Name: "test"}
	_ = store.Create(entity, getTestEntityID)

	err := store.Delete("1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if store.Count() != 0 {
		t.Error("expected count 0 after delete")
	}
}

func TestMockStore_FindAll(t *testing.T) {
	store := NewMockStore[testEntity]()
	_ = store.Create(&testEntity{ID: "1", Name: "one"}, getTestEntityID)
	_ = store.Create(&testEntity{ID: "2", Name: "two"}, getTestEntityID)

	all, err := store.FindAll()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(all) != 2 {
		t.Errorf("expected 2 items, got %d", len(all))
	}
}

func TestMockStore_FindWhere(t *testing.T) {
	store := NewMockStore[testEntity]()
	_ = store.Create(&testEntity{ID: "1", Name: "alice"}, getTestEntityID)
	_ = store.Create(&testEntity{ID: "2", Name: "bob"}, getTestEntityID)
	_ = store.Create(&testEntity{ID: "3", Name: "alice"}, getTestEntityID)

	found, err := store.FindWhere(func(e *testEntity) bool {
		return e.Name == "alice"
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(found) != 2 {
		t.Errorf("expected 2 items, got %d", len(found))
	}
}

func TestMockStore_FindOneWhere(t *testing.T) {
	store := NewMockStore[testEntity]()
	_ = store.Create(&testEntity{ID: "1", Name: "alice"}, getTestEntityID)
	_ = store.Create(&testEntity{ID: "2", Name: "bob"}, getTestEntityID)

	found, err := store.FindOneWhere(func(e *testEntity) bool {
		return e.Name == "bob"
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if found.ID != "2" {
		t.Errorf("expected ID '2', got %s", found.ID)
	}
}

func TestMockStore_FindOneWhere_NotFound(t *testing.T) {
	store := NewMockStore[testEntity]()
	_ = store.Create(&testEntity{ID: "1", Name: "alice"}, getTestEntityID)

	_, err := store.FindOneWhere(func(e *testEntity) bool {
		return e.Name == "nonexistent"
	})

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("expected gorm.ErrRecordNotFound, got %v", err)
	}
}

func TestMockStore_Exists(t *testing.T) {
	store := NewMockStore[testEntity]()
	_ = store.Create(&testEntity{ID: "1", Name: "test"}, getTestEntityID)

	if !store.Exists("1") {
		t.Error("expected Exists to return true")
	}
	if store.Exists("2") {
		t.Error("expected Exists to return false for non-existent ID")
	}
}

func TestMockStore_CountWhere(t *testing.T) {
	store := NewMockStore[testEntity]()
	_ = store.Create(&testEntity{ID: "1", Name: "alice"}, getTestEntityID)
	_ = store.Create(&testEntity{ID: "2", Name: "bob"}, getTestEntityID)
	_ = store.Create(&testEntity{ID: "3", Name: "alice"}, getTestEntityID)

	count := store.CountWhere(func(e *testEntity) bool {
		return e.Name == "alice"
	})

	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}
}

func TestMockStore_Clear(t *testing.T) {
	store := NewMockStore[testEntity]()
	_ = store.Create(&testEntity{ID: "1", Name: "test"}, getTestEntityID)
	store.SetError("Create", errors.New("test error"))

	store.Clear()

	if store.Count() != 0 {
		t.Error("expected count 0 after Clear")
	}

	// Errors should persist after Clear
	if store.getError("Create") == nil {
		t.Error("expected error to persist after Clear")
	}
}

func TestMockStore_Reset(t *testing.T) {
	store := NewMockStore[testEntity]()
	_ = store.Create(&testEntity{ID: "1", Name: "test"}, getTestEntityID)
	store.SetError("Create", errors.New("test error"))

	store.Reset()

	if store.Count() != 0 {
		t.Error("expected count 0 after Reset")
	}
	if store.getError("Create") != nil {
		t.Error("expected no error after Reset")
	}
}

func TestMockStore_Seed(t *testing.T) {
	store := NewMockStore[testEntity]()
	items := []*testEntity{
		{ID: "1", Name: "one"},
		{ID: "2", Name: "two"},
		{ID: "3", Name: "three"},
	}

	store.Seed(items, getTestEntityID)

	if store.Count() != 3 {
		t.Errorf("expected count 3, got %d", store.Count())
	}
}

func TestMockStore_GetAll(t *testing.T) {
	store := NewMockStore[testEntity]()
	_ = store.Create(&testEntity{ID: "1", Name: "test"}, getTestEntityID)

	all := store.GetAll()
	if len(all) != 1 {
		t.Errorf("expected 1 item, got %d", len(all))
	}

	// Verify it returns a copy by modifying and checking original
	delete(all, "1")
	if store.Count() != 1 {
		t.Error("GetAll should return a copy, not the original map")
	}
}

func TestMockStore_ClearError(t *testing.T) {
	store := NewMockStore[testEntity]()
	store.SetError("Create", errors.New("error"))

	store.ClearError("Create")

	if store.getError("Create") != nil {
		t.Error("expected error to be cleared")
	}
}

func TestMockStore_ClearAllErrors(t *testing.T) {
	store := NewMockStore[testEntity]()
	store.SetError("Create", errors.New("error1"))
	store.SetError("FindByID", errors.New("error2"))

	store.ClearAllErrors()

	if store.getError("Create") != nil || store.getError("FindByID") != nil {
		t.Error("expected all errors to be cleared")
	}
}
