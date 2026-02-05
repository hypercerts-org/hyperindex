package lexicon

import (
	"strings"
	"unicode"
)

// NSID utilities for working with AT Protocol Namespaced Identifiers.
//
// NSIDs follow the format: "domain.name.thing" (e.g., "app.bsky.feed.post")
// They are used to identify lexicons, collections, and other namespaced resources.

// ToTypeName converts an NSID to a GraphQL type name (PascalCase).
//
// Examples:
//
//	ToTypeName("xyz.statusphere.status")  // "XyzStatusphereStatus"
//	ToTypeName("app.bsky.feed.post")      // "AppBskyFeedPost"
func ToTypeName(nsid string) string {
	parts := strings.Split(nsid, ".")
	var result strings.Builder
	for _, part := range parts {
		result.WriteString(capitalizeFirst(part))
	}
	return result.String()
}

// ToFieldName converts an NSID to a GraphQL field name (camelCase).
//
// Examples:
//
//	ToFieldName("xyz.statusphere.status")  // "xyzStatusphereStatus"
//	ToFieldName("app.bsky.feed.post")      // "appBskyFeedPost"
func ToFieldName(nsid string) string {
	parts := strings.Split(nsid, ".")
	if len(parts) == 0 {
		return nsid
	}

	var result strings.Builder
	result.WriteString(parts[0]) // First part stays lowercase
	for _, part := range parts[1:] {
		result.WriteString(capitalizeFirst(part))
	}
	return result.String()
}

// ToCollectionName extracts the collection name from an NSID (last segment).
//
// Examples:
//
//	ToCollectionName("xyz.statusphere.status")  // "status"
//	ToCollectionName("app.bsky.feed.post")      // "post"
func ToCollectionName(nsid string) string {
	parts := strings.Split(nsid, ".")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// ToDomainParts extracts the domain parts from an NSID (all but last segment).
//
// Examples:
//
//	ToDomainParts("xyz.statusphere.status")  // ["xyz", "statusphere"]
//	ToDomainParts("app.bsky.feed.post")      // ["app", "bsky", "feed"]
func ToDomainParts(nsid string) []string {
	parts := strings.Split(nsid, ".")
	if len(parts) <= 1 {
		return []string{}
	}
	return parts[:len(parts)-1]
}

// IsValidNSID checks if a string is a valid NSID format.
// Valid NSIDs have at least 3 segments and only contain lowercase letters, numbers, and hyphens.
func IsValidNSID(nsid string) bool {
	parts := strings.Split(nsid, ".")
	if len(parts) < 3 {
		return false
	}

	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, r := range part {
			if !unicode.IsLower(r) && !unicode.IsDigit(r) && r != '-' {
				return false
			}
		}
		// Cannot start or end with hyphen
		if strings.HasPrefix(part, "-") || strings.HasSuffix(part, "-") {
			return false
		}
	}
	return true
}

// ParseRef parses a lexicon ref into the lexicon ID and definition name.
//
// Examples:
//
//	ParseRef("app.bsky.embed.images#image")  // ("app.bsky.embed.images", "image", true)
//	ParseRef("app.bsky.feed.post")           // ("app.bsky.feed.post", "", true)
//	ParseRef("#localRef")                    // ("", "localRef", true)
func ParseRef(ref string) (lexiconID, defName string, ok bool) {
	if ref == "" {
		return "", "", false
	}

	// Handle local refs starting with #
	if strings.HasPrefix(ref, "#") {
		return "", strings.TrimPrefix(ref, "#"), true
	}

	// Split on #
	parts := strings.SplitN(ref, "#", 2)
	if len(parts) == 1 {
		return parts[0], "", true
	}
	return parts[0], parts[1], true
}

// IDFromRef extracts just the lexicon ID from a ref.
//
// Examples:
//
//	IDFromRef("app.bsky.embed.images#image")  // "app.bsky.embed.images"
//	IDFromRef("app.bsky.feed.post")           // "app.bsky.feed.post"
func IDFromRef(ref string) string {
	id, _, _ := ParseRef(ref)
	return id
}

// DefNameFromRef extracts just the definition name from a ref.
//
// Examples:
//
//	DefNameFromRef("app.bsky.embed.images#image")  // "image"
//	DefNameFromRef("app.bsky.feed.post")           // ""
//	DefNameFromRef("#localRef")                    // "localRef"
func DefNameFromRef(ref string) string {
	_, name, _ := ParseRef(ref)
	return name
}

// IsLocalRef returns true if the ref is a local reference (starts with #).
func IsLocalRef(ref string) bool {
	return strings.HasPrefix(ref, "#")
}

// ResolveLocalRef resolves a local ref (#name) to a fully-qualified ref.
//
// Examples:
//
//	ResolveLocalRef("#image", "app.bsky.embed.images")  // "app.bsky.embed.images#image"
//	ResolveLocalRef("app.bsky.feed.post", "ignored")   // "app.bsky.feed.post"
func ResolveLocalRef(ref, lexiconID string) string {
	if IsLocalRef(ref) {
		return lexiconID + ref
	}
	return ref
}

// MakeRef creates a fully-qualified ref from a lexicon ID and definition name.
//
// Examples:
//
//	MakeRef("app.bsky.embed.images", "image")  // "app.bsky.embed.images#image"
//	MakeRef("app.bsky.feed.post", "")          // "app.bsky.feed.post"
func MakeRef(lexiconID, defName string) string {
	if defName == "" {
		return lexiconID
	}
	return lexiconID + "#" + defName
}

// capitalizeFirst capitalizes the first letter of a string.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
