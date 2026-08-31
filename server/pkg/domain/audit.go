package domain

import "time"

type AuditAction string

const (
	AuditActionServiceCreated     AuditAction = "SERVICE_CREATED"
	AuditActionServiceUpdated     AuditAction = "SERVICE_UPDATED"
	AuditActionServiceDisabled    AuditAction = "SERVICE_DISABLED"
	AuditActionDeviceEnrolled     AuditAction = "DEVICE_ENROLLED"
	AuditActionDeviceActivated    AuditAction = "DEVICE_ACTIVATED"
	AuditActionDeviceRevoked      AuditAction = "DEVICE_REVOKED"
	AuditActionReleasePromoted    AuditAction = "RELEASE_PROMOTED"
	AuditActionReleaseRevoked     AuditAction = "RELEASE_REVOKED"
	AuditActionDeliveryDispatched AuditAction = "DELIVERY_DISPATCHED"
	AuditActionAccessDenied       AuditAction = "ACCESS_DENIED"
)

type AuditEvent struct {
	ID        string      `json:"id"`
	Action    AuditAction `json:"action"`
	ActorID   string      `json:"actor_id"`
	TargetID  string      `json:"target_id"`
	Details   string      `json:"details"`
	Timestamp time.Time   `json:"timestamp"`
}
