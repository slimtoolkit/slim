# Maintenance

The compose-go library has to be kept up-to-date with approved changes in the [Compose specification](https://github.com/compose-spec/compose-spec).
As we define new attributes to be added to the spec, this typically requires:

1. Updating `schema` to latest version from compose-spec
1. Creating the matching struct/field in `types`
1. Creating the matching `CheckXX` method in `compatibility`
1. If the new attribute replaces a legacy one we want to deprecate, creating the adequate logic in `normalize.go`