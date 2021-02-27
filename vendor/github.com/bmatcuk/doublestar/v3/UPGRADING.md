# Upgrading from v2 to v3

v3 introduced using `!` to negate character classes, in addition to `^`. If any
of your patterns include a character class that starts with an exclamation mark
(ie, `[!...]`), you'll need to update the pattern to escape or move the
exclamation mark. Note that, like the caret (`^`), it only negates the
character class if it is the first character in the character class.

# Upgrading from v1 to v2

The change from v1 to v2 was fairly minor: the return type of the `Open` method
on the `OS` interface was changed from `*os.File` to `File`, a new interface
exported by doublestar. The new `File` interface only defines the functionality
doublestar actually needs (`io.Closer` and `Readdir`), making it easier to use
doublestar with [go-billy](https://github.com/src-d/go-billy),
[afero](https://github.com/spf13/afero), or something similar. If you were
using this functionality, updating should be as easy as updating `Open's`
return type, since `os.File` already implements `doublestar.File`.

If you weren't using this functionality, updating should be as easy as changing
your dependencies to point to v2.
