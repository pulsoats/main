package system

import (
	"time"

	"github.com/google/uuid"
)

type HostKind string

type Host struct {
	ID       uuid.UUID
	Kind     HostKind
	Name     string
	Exchange string
	Account  string
	Version  string
}

type Node struct {
	Host
	Addr       string
	LastSeenAt time.Time
	CreatedAt  time.Time
}
