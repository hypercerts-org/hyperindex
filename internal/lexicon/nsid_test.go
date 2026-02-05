package lexicon

import (
	"testing"
)

func TestToTypeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"xyz.statusphere.status", "XyzStatusphereStatus"},
		{"app.bsky.feed.post", "AppBskyFeedPost"},
		{"com.atproto.label.defs", "ComAtprotoLabelDefs"},
		{"simple", "Simple"},
	}

	for _, tt := range tests {
		result := ToTypeName(tt.input)
		if result != tt.expected {
			t.Errorf("ToTypeName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestToFieldName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"xyz.statusphere.status", "xyzStatusphereStatus"},
		{"app.bsky.feed.post", "appBskyFeedPost"},
		{"com.atproto.label.defs", "comAtprotoLabelDefs"},
		{"simple", "simple"},
	}

	for _, tt := range tests {
		result := ToFieldName(tt.input)
		if result != tt.expected {
			t.Errorf("ToFieldName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestToCollectionName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"xyz.statusphere.status", "status"},
		{"app.bsky.feed.post", "post"},
		{"com.atproto.label.defs", "defs"},
		{"simple", "simple"},
	}

	for _, tt := range tests {
		result := ToCollectionName(tt.input)
		if result != tt.expected {
			t.Errorf("ToCollectionName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestToDomainParts(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"xyz.statusphere.status", []string{"xyz", "statusphere"}},
		{"app.bsky.feed.post", []string{"app", "bsky", "feed"}},
		{"simple", []string{}},
	}

	for _, tt := range tests {
		result := ToDomainParts(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("ToDomainParts(%q) = %v, want %v", tt.input, result, tt.expected)
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("ToDomainParts(%q)[%d] = %q, want %q", tt.input, i, result[i], tt.expected[i])
			}
		}
	}
}

func TestIsValidNSID(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"app.bsky.feed.post", true},
		{"com.atproto.label.defs", true},
		{"xyz.statusphere.status", true},
		{"app.bsky", false},       // Too few segments
		{"simple", false},         // Too few segments
		{"App.Bsky.Feed", false},  // Uppercase
		{"app..bsky.feed", false}, // Empty segment
		{"app.bsky.-feed", false}, // Starts with hyphen
		{"app.bsky.feed-", false}, // Ends with hyphen
		{"app.bsky.feed-post", true},
		{"app.bsky.feed123", true},
	}

	for _, tt := range tests {
		result := IsValidNSID(tt.input)
		if result != tt.expected {
			t.Errorf("IsValidNSID(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestParseRef(t *testing.T) {
	tests := []struct {
		input         string
		wantLexiconID string
		wantDefName   string
		wantOK        bool
	}{
		{"app.bsky.embed.images#image", "app.bsky.embed.images", "image", true},
		{"app.bsky.feed.post", "app.bsky.feed.post", "", true},
		{"#localRef", "", "localRef", true},
		{"", "", "", false},
	}

	for _, tt := range tests {
		lexiconID, defName, ok := ParseRef(tt.input)
		if ok != tt.wantOK || lexiconID != tt.wantLexiconID || defName != tt.wantDefName {
			t.Errorf("ParseRef(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tt.input, lexiconID, defName, ok, tt.wantLexiconID, tt.wantDefName, tt.wantOK)
		}
	}
}

func TestIDFromRef(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"app.bsky.embed.images#image", "app.bsky.embed.images"},
		{"app.bsky.feed.post", "app.bsky.feed.post"},
		{"#localRef", ""},
	}

	for _, tt := range tests {
		result := IDFromRef(tt.input)
		if result != tt.expected {
			t.Errorf("IDFromRef(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestDefNameFromRef(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"app.bsky.embed.images#image", "image"},
		{"app.bsky.feed.post", ""},
		{"#localRef", "localRef"},
	}

	for _, tt := range tests {
		result := DefNameFromRef(tt.input)
		if result != tt.expected {
			t.Errorf("DefNameFromRef(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestIsLocalRef(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"#localRef", true},
		{"app.bsky.feed.post", false},
		{"app.bsky.embed.images#image", false},
	}

	for _, tt := range tests {
		result := IsLocalRef(tt.input)
		if result != tt.expected {
			t.Errorf("IsLocalRef(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestResolveLocalRef(t *testing.T) {
	tests := []struct {
		ref       string
		lexiconID string
		expected  string
	}{
		{"#image", "app.bsky.embed.images", "app.bsky.embed.images#image"},
		{"app.bsky.feed.post", "ignored", "app.bsky.feed.post"},
		{"#mention", "app.bsky.richtext.facet", "app.bsky.richtext.facet#mention"},
	}

	for _, tt := range tests {
		result := ResolveLocalRef(tt.ref, tt.lexiconID)
		if result != tt.expected {
			t.Errorf("ResolveLocalRef(%q, %q) = %q, want %q", tt.ref, tt.lexiconID, result, tt.expected)
		}
	}
}

func TestMakeRef(t *testing.T) {
	tests := []struct {
		lexiconID string
		defName   string
		expected  string
	}{
		{"app.bsky.embed.images", "image", "app.bsky.embed.images#image"},
		{"app.bsky.feed.post", "", "app.bsky.feed.post"},
	}

	for _, tt := range tests {
		result := MakeRef(tt.lexiconID, tt.defName)
		if result != tt.expected {
			t.Errorf("MakeRef(%q, %q) = %q, want %q", tt.lexiconID, tt.defName, result, tt.expected)
		}
	}
}
