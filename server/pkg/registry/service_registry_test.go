package registry_test

import (
	"testing"

	"github.com/Homiakus/WebGate/server/pkg/domain"
	"github.com/Homiakus/WebGate/server/pkg/registry"
)

func TestServiceRegistry(t *testing.T) {
	reg := registry.NewServiceRegistry()

	svc := &domain.ProtectedService{
		ID:          "svc_docs",
		TenantID:    "tenant_1",
		WorkspaceID: "ws_docs",
		Name:        "Corporate Documentation",
		Slug:        "docs",
		UpstreamURL: "http://127.0.0.1:8081",
		Status:      domain.ServiceStatusActive,
	}

	if err := reg.Register(svc); err != nil {
		t.Fatalf("failed to register service: %v", err)
	}

	// Lookup by ID
	found, err := reg.GetByID("svc_docs")
	if err != nil || found.Slug != "docs" {
		t.Fatalf("unexpected lookup by ID: %v, %v", found, err)
	}

	// Lookup by Slug
	resolved, err := reg.ResolveBySlug("docs")
	if err != nil || resolved.ID != "svc_docs" {
		t.Fatalf("unexpected lookup by slug: %v, %v", resolved, err)
	}

	// Duplicate registration error
	dup := &domain.ProtectedService{
		ID:          "svc_docs_dup",
		TenantID:    "tenant_1",
		WorkspaceID: "ws_docs",
		Name:        "Docs 2",
		Slug:        "docs", // same slug
		UpstreamURL: "http://127.0.0.1:8082",
		Status:      domain.ServiceStatusActive,
	}
	if err := reg.Register(dup); err != registry.ErrSlugCollision {
		t.Fatalf("expected ErrSlugCollision, got %v", err)
	}

	// Update status
	if err := reg.UpdateStatus("svc_docs", domain.ServiceStatusDisabled); err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	updated, _ := reg.GetByID("svc_docs")
	if updated.Status != domain.ServiceStatusDisabled || updated.Version != 2 {
		t.Fatalf("status update failed: %v", updated)
	}
}
