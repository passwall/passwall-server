package domain

import "testing"

func TestCollectionPermissionRoundTrip(t *testing.T) {
	for _, p := range []CollectionPermission{
		CollectionPermissionView, CollectionPermissionViewExceptPasswords,
		CollectionPermissionEdit, CollectionPermissionEditExceptPasswords,
		CollectionPermissionManage,
	} {
		r, w, a, h := p.Flags()
		if got := CollectionPermissionFromFlags(r, w, a, h); got != p {
			t.Fatalf("%s round-tripped to %s", p, got)
		}
	}
	// Manage never hides passwords.
	if got := CollectionPermissionFromFlags(true, true, true, true); got != CollectionPermissionManage {
		t.Fatalf("expected manage, got %s", got)
	}
}

func TestGrantRequestNormalize(t *testing.T) {
	req := &GrantCollectionAccessRequest{Permission: CollectionPermissionEditExceptPasswords}
	if !req.Normalize() || !req.CanRead || !req.CanWrite || req.CanAdmin || !req.HidePasswords {
		t.Fatalf("permission not applied to flags: %+v", req)
	}

	legacy := &GrantCollectionAccessRequest{CanWrite: true}
	if !legacy.Normalize() || !legacy.CanRead || legacy.Permission != CollectionPermissionEdit {
		t.Fatalf("legacy flags not normalized: %+v", legacy)
	}

	manageHidden := &GrantCollectionAccessRequest{CanRead: true, CanAdmin: true, HidePasswords: true}
	if !manageHidden.Normalize() || manageHidden.HidePasswords || manageHidden.Permission != CollectionPermissionManage {
		t.Fatalf("manage must not hide passwords: %+v", manageHidden)
	}

	if (&GrantCollectionAccessRequest{Permission: "owner"}).Normalize() {
		t.Fatal("unknown permission accepted")
	}
	if (&GrantCollectionAccessRequest{}).Normalize() {
		t.Fatal("empty grant accepted")
	}
}

func TestItemPermissionsFor(t *testing.T) {
	p := ItemPermissionsFor(CollectionPermissionViewExceptPasswords)
	if !p.View || p.ViewPassword || p.Edit || p.Delete {
		t.Fatalf("unexpected view_except_passwords permissions: %+v", p)
	}
	p = ItemPermissionsFor(CollectionPermissionEditExceptPasswords)
	if !p.Edit || p.EditPassword || p.ViewPassword || p.Manage {
		t.Fatalf("unexpected edit_except_passwords permissions: %+v", p)
	}
	p = ItemPermissionsFor(CollectionPermissionManage)
	if !p.Manage || !p.ViewPassword || !p.Delete {
		t.Fatalf("unexpected manage permissions: %+v", p)
	}
}
