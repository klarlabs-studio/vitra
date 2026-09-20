package domain

import "errors"

// CommandDefinition is an explicitly registered application operation.
// Exported Go methods are never automatically commands — registration is
// required, and each command declares the permission it needs.
type CommandDefinition struct {
	name        CommandName
	description string
	permission  PermissionName
	plugin      PluginID // empty = application-owned
}

// NewCommandDefinition creates a validated command definition.
func NewCommandDefinition(name CommandName, description string, permission PermissionName) (*CommandDefinition, error) {
	if name == "" {
		return nil, errors.New("command name is required")
	}
	if permission == "" {
		return nil, errors.New("command permission is required")
	}
	return &CommandDefinition{
		name:        name,
		description: description,
		permission:  permission,
	}, nil
}

// WithPlugin marks the command as contributed by a plugin.
func (c *CommandDefinition) WithPlugin(id PluginID) *CommandDefinition {
	c.plugin = id
	return c
}

// Name returns the command name.
func (c *CommandDefinition) Name() CommandName { return c.name }

// Description returns the human-readable description.
func (c *CommandDefinition) Description() string { return c.description }

// Permission returns the permission required to invoke this command.
func (c *CommandDefinition) Permission() PermissionName { return c.permission }

// Plugin returns the contributing plugin id, if any.
func (c *CommandDefinition) Plugin() PluginID { return c.plugin }
