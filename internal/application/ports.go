package application

import "go.klarlabs.de/vitra/domain"

// GrantRepository stores capability grants.
type GrantRepository interface {
	Save(grant *domain.CapabilityGrant) error
	Get(name domain.GrantName) (*domain.CapabilityGrant, error)
	List() ([]*domain.CapabilityGrant, error)
}

// WindowRepository stores window aggregates.
type WindowRepository interface {
	Save(window *domain.Window) error
	Get(id domain.WindowID) (*domain.Window, error)
	Delete(id domain.WindowID) error
	List() ([]*domain.Window, error)
}

// CommandRepository stores command definitions.
type CommandRepository interface {
	Save(cmd *domain.CommandDefinition) error
	Get(name domain.CommandName) (*domain.CommandDefinition, error)
	List() ([]*domain.CommandDefinition, error)
}

// ResourceRepository stores typed resource handles.
type ResourceRepository interface {
	Save(handle *ResourceHandle) error
	Get(id ResourceID) (*ResourceHandle, error)
	Delete(id ResourceID) error
	ListByOwner(window domain.WindowID) ([]*ResourceHandle, error)
}

// SubscriptionRepository stores event subscriptions.
type SubscriptionRepository interface {
	Save(sub *domain.Subscription) error
	Get(id domain.SubscriptionID) (*domain.Subscription, error)
	Delete(id domain.SubscriptionID) error
	ListByOwner(window domain.WindowID) ([]*domain.Subscription, error)
	ListByEvent(event domain.EventName) ([]*domain.Subscription, error)
}

// CommandExecutorLookup resolves executors by command name.
type CommandExecutorLookup interface {
	Get(name domain.CommandName) (domain.CommandExecutor, bool)
}
