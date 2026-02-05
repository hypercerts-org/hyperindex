package lexicon

import (
	"fmt"
	"sync"
)

// Registry holds all loaded lexicons and provides cross-reference lookups.
//
// The registry allows looking up definitions by fully-qualified refs like
// "app.bsky.embed.images#image" across all loaded lexicons.
type Registry struct {
	mu sync.RWMutex

	// lexicons indexed by ID
	lexicons map[string]*Lexicon

	// objectDefs indexed by fully-qualified ref (e.g., "app.bsky.embed.images#image")
	objectDefs map[string]*ObjectDef

	// recordDefs indexed by lexicon ID (for main record definitions)
	recordDefs map[string]*RecordDef
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		lexicons:   make(map[string]*Lexicon),
		objectDefs: make(map[string]*ObjectDef),
		recordDefs: make(map[string]*RecordDef),
	}
}

// NewRegistryFromLexicons creates a registry from a list of lexicons.
func NewRegistryFromLexicons(lexicons []*Lexicon) *Registry {
	r := NewRegistry()
	for _, lex := range lexicons {
		r.Register(lex)
	}
	return r
}

// Register adds a lexicon to the registry.
func (r *Registry) Register(lexicon *Lexicon) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.lexicons[lexicon.ID] = lexicon

	// Index main definition
	if lexicon.Defs.Main != nil {
		r.recordDefs[lexicon.ID] = lexicon.Defs.Main

		// If main is an object type, also add to objectDefs
		if lexicon.Defs.Main.Type == "object" {
			objDef := &ObjectDef{
				Type:           lexicon.Defs.Main.Type,
				RequiredFields: nil, // Main defs don't track required separately
				Properties:     lexicon.Defs.Main.Properties,
			}
			r.objectDefs[lexicon.ID] = objDef
		}
	}

	// Index all other definitions
	for name, def := range lexicon.Defs.Others {
		ref := MakeRef(lexicon.ID, name)
		if def.IsObject() {
			r.objectDefs[ref] = def.Object
		} else if def.IsRecord() {
			r.recordDefs[ref] = def.Record
		}
	}
}

// Unregister removes a lexicon from the registry.
func (r *Registry) Unregister(lexiconID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	lexicon, ok := r.lexicons[lexiconID]
	if !ok {
		return
	}

	// Remove from lexicons
	delete(r.lexicons, lexiconID)
	delete(r.recordDefs, lexiconID)
	delete(r.objectDefs, lexiconID)

	// Remove all other definitions
	for name := range lexicon.Defs.Others {
		ref := MakeRef(lexiconID, name)
		delete(r.objectDefs, ref)
		delete(r.recordDefs, ref)
	}
}

// GetLexicon retrieves a lexicon by ID.
func (r *Registry) GetLexicon(id string) (*Lexicon, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	lex, ok := r.lexicons[id]
	return lex, ok
}

// GetObjectDef retrieves an object definition by fully-qualified ref.
//
// Examples:
//
//	GetObjectDef("app.bsky.embed.images#image")
//	GetObjectDef("app.bsky.richtext.facet#mention")
func (r *Registry) GetObjectDef(ref string) (*ObjectDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	obj, ok := r.objectDefs[ref]
	return obj, ok
}

// GetRecordDef retrieves a record definition by lexicon ID.
func (r *Registry) GetRecordDef(lexiconID string) (*RecordDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rec, ok := r.recordDefs[lexiconID]
	return rec, ok
}

// ResolveRef resolves a ref (potentially local) within the context of a lexicon.
//
// Examples:
//
//	ResolveRef("#image", "app.bsky.embed.images")  // Returns ObjectDef for app.bsky.embed.images#image
//	ResolveRef("app.bsky.feed.post", "ignored")    // Returns RecordDef for app.bsky.feed.post
func (r *Registry) ResolveRef(ref, contextLexiconID string) (any, bool) {
	resolvedRef := ResolveLocalRef(ref, contextLexiconID)

	// Try object def first
	if obj, ok := r.GetObjectDef(resolvedRef); ok {
		return obj, true
	}

	// Try record def
	if rec, ok := r.GetRecordDef(resolvedRef); ok {
		return rec, true
	}

	return nil, false
}

// GetAllLexiconIDs returns all registered lexicon IDs.
func (r *Registry) GetAllLexiconIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.lexicons))
	for id := range r.lexicons {
		ids = append(ids, id)
	}
	return ids
}

// GetAllObjectRefs returns all registered object definition refs.
func (r *Registry) GetAllObjectRefs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	refs := make([]string, 0, len(r.objectDefs))
	for ref := range r.objectDefs {
		refs = append(refs, ref)
	}
	return refs
}

// GetAllRecordRefs returns all registered record definition refs (lexicon IDs).
func (r *Registry) GetAllRecordRefs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	refs := make([]string, 0, len(r.recordDefs))
	for ref := range r.recordDefs {
		refs = append(refs, ref)
	}
	return refs
}

// Count returns the number of registered lexicons.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.lexicons)
}

// GetAllLexicons returns all registered lexicons.
func (r *Registry) GetAllLexicons() []*Lexicon {
	r.mu.RLock()
	defer r.mu.RUnlock()

	lexicons := make([]*Lexicon, 0, len(r.lexicons))
	for _, lex := range r.lexicons {
		lexicons = append(lexicons, lex)
	}
	return lexicons
}

// ParseAndRegister parses a lexicon JSON string and registers it.
func (r *Registry) ParseAndRegister(jsonStr string) (*Lexicon, error) {
	lexicon, err := Parse(jsonStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse lexicon: %w", err)
	}

	r.Register(lexicon)
	return lexicon, nil
}

// GetCollectionLexicons returns all lexicons that have a main record definition.
// These are the "collection" lexicons that define actual record types.
func (r *Registry) GetCollectionLexicons() []*Lexicon {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var collections []*Lexicon
	for _, lex := range r.lexicons {
		if lex.Defs.Main != nil && lex.Defs.Main.Type == "record" {
			collections = append(collections, lex)
		}
	}
	return collections
}

// GetDefsLexicons returns all lexicons that only contain helper definitions (no main record).
// These are typically "defs" lexicons like "app.bsky.embed.defs".
func (r *Registry) GetDefsLexicons() []*Lexicon {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var defs []*Lexicon
	for _, lex := range r.lexicons {
		if lex.Defs.Main == nil && len(lex.Defs.Others) > 0 {
			defs = append(defs, lex)
		}
	}
	return defs
}
