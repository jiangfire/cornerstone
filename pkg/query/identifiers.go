package query

import (
	"fmt"
	"regexp"
	"strings"
)

// MaxIdentifierLength is the max length of a single identifier segment.
const MaxIdentifierLength = 128

// allowedJoinOps only these two comparison operators are allowed for Join.On;
// other operators lack clear equivalent semantics and significantly expand the injection surface, so they are all rejected.
var allowedJoinOps = map[string]struct{}{
	"=":  {},
	"<>": {},
}

// identifierSegmentPattern single identifier segment: must start with a letter or underscore, containing only alphanumeric characters and underscores.
var identifierSegmentPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// jsonPathSegmentPattern same rules as identifiers; JSON path segments must not contain quotes, wildcards, spaces, etc.
var jsonPathSegmentPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ValidateIdentifier validates table names, field names, and qualified identifiers.
// Allows multi-segment references like `table.field` or `alias.field.sub`; each segment is validated independently.
//
// Rejects: single/double quotes, semicolons, spaces, comment markers, SQL keyword punctuation, etc.
// Design intent: all identifiers entering SQL concatenation (whether or not passed through quoteIdentifier)
// must pass this check first, so even if quoteIdentifier is missed later, no literal can break out of the quotes.
func ValidateIdentifier(name string) error {
	if name == "" {
		return fmt.Errorf("identifier cannot be empty")
	}
	if len(name) > MaxIdentifierLength*8 {
		return fmt.Errorf("identifier too long (max %d segments including separators)", MaxIdentifierLength)
	}
	for segment := range strings.SplitSeq(name, ".") {
		if err := validateIdentifierSegment(segment); err != nil {
			return fmt.Errorf("invalid identifier %q: %w", name, err)
		}
	}
	return nil
}

func validateIdentifierSegment(seg string) error {
	if seg == "" {
		return fmt.Errorf("empty segment")
	}
	if len(seg) > MaxIdentifierLength {
		return fmt.Errorf("segment length exceeds %d", MaxIdentifierLength)
	}
	if !identifierSegmentPattern.MatchString(seg) {
		return fmt.Errorf("segment %q contains invalid characters; only letters, digits, and underscores allowed, and the first character must not be a digit", seg)
	}
	return nil
}

// ValidateJSONPathSegment validates a single key segment in a JSON path.
// Strictly limited to `^[A-Za-z_][\w]*$`; `[`, `*`, `'`, `"`, spaces, `$`, `.`, etc. are not allowed.
func ValidateJSONPathSegment(seg string) error {
	if seg == "" {
		return fmt.Errorf("JSON path segment cannot be empty")
	}
	if len(seg) > MaxIdentifierLength {
		return fmt.Errorf("JSON path segment length exceeds %d", MaxIdentifierLength)
	}
	if !jsonPathSegmentPattern.MatchString(seg) {
		return fmt.Errorf("JSON path segment %q contains invalid characters", seg)
	}
	return nil
}

// ValidateJSONPath validates a complete JSON path with multiple segments joined by `.`, e.g. `status` or `payload.user.id`.
func ValidateJSONPath(path string) error {
	if path == "" {
		return fmt.Errorf("JSON path cannot be empty")
	}
	for seg := range strings.SplitSeq(path, ".") {
		if err := ValidateJSONPathSegment(seg); err != nil {
			return err
		}
	}
	return nil
}

// ValidateJoinOp is the whitelist for Join.On.Op.
func ValidateJoinOp(op string) error {
	if _, ok := allowedJoinOps[op]; !ok {
		return fmt.Errorf("invalid JOIN operator %q: only = / <> are allowed", op)
	}
	return nil
}
