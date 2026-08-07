package model

// AllModels defines the migration scope for the db create_auto command.
// Models not listed here are invisible to the schema differ.
var AllModels = []any{
	(*User)(nil),
}
