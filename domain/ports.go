package domain

import "context"

// GrantRepository stores capability grants.
type GrantRepository interface {
	Save(grant *CapabilityGrant) error
	Get(name GrantName) (*CapabilityGrant, error)
	List() ([]*CapabilityGrant, error)
}

// WindowRepository stores window aggregates.
type WindowRepository interface {
	Save(window *Window) error
	Get(id WindowID) (*Window, error)
	Delete(id WindowID) error
	List() ([]*Window, error)
}

// CommandRepository stores command definitions.
type CommandRepository interface {
	Save(cmd *CommandDefinition) error
	Get(name CommandName) (*CommandDefinition, error)
	List() ([]*CommandDefinition, error)
}

// ResourceRepository stores typed resource handles.
type ResourceRepository interface {
	Save(handle *ResourceHandle) error
	Get(id ResourceID) (*ResourceHandle, error)
	Delete(id ResourceID) error
	ListByOwner(window WindowID) ([]*ResourceHandle, error)
}

// CommandExecutor executes a registered command after authorization.
type CommandExecutor interface {
	Execute(ctx context.Context, name CommandName, input any) (any, error)
}

// CommandExecutorLookup resolves executors by command name.
type CommandExecutorLookup interface {
	Get(name CommandName) (CommandExecutor, bool)
}
