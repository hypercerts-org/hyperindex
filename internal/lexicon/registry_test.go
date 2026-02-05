package lexicon

import (
	"testing"
)

func TestRegistryBasic(t *testing.T) {
	json := `{
		"lexicon": 1,
		"id": "xyz.statusphere.status",
		"defs": {
			"main": {
				"type": "record",
				"record": {
					"type": "object",
					"required": ["text"],
					"properties": {
						"text": {"type": "string"}
					}
				}
			}
		}
	}`

	registry := NewRegistry()

	lexicon, err := registry.ParseAndRegister(json)
	if err != nil {
		t.Fatalf("ParseAndRegister failed: %v", err)
	}

	if lexicon.ID != "xyz.statusphere.status" {
		t.Errorf("Expected ID 'xyz.statusphere.status', got '%s'", lexicon.ID)
	}

	// Verify we can retrieve it
	retrieved, ok := registry.GetLexicon("xyz.statusphere.status")
	if !ok {
		t.Fatal("Expected to find lexicon in registry")
	}
	if retrieved.ID != lexicon.ID {
		t.Error("Retrieved lexicon doesn't match registered lexicon")
	}

	// Verify record def is indexed
	recordDef, ok := registry.GetRecordDef("xyz.statusphere.status")
	if !ok {
		t.Fatal("Expected to find record def in registry")
	}
	if recordDef.Type != "record" {
		t.Errorf("Expected record type 'record', got '%s'", recordDef.Type)
	}

	// Verify count
	if registry.Count() != 1 {
		t.Errorf("Expected count 1, got %d", registry.Count())
	}
}

func TestRegistryWithObjectDefs(t *testing.T) {
	json := `{
		"lexicon": 1,
		"id": "app.bsky.embed.images",
		"defs": {
			"main": {
				"type": "object",
				"properties": {
					"images": {"type": "array"}
				}
			},
			"image": {
				"type": "object",
				"required": ["alt"],
				"properties": {
					"alt": {"type": "string"},
					"aspectRatio": {"type": "ref", "ref": "#aspectRatio"}
				}
			},
			"aspectRatio": {
				"type": "object",
				"required": ["width", "height"],
				"properties": {
					"width": {"type": "integer"},
					"height": {"type": "integer"}
				}
			}
		}
	}`

	registry := NewRegistry()
	_, err := registry.ParseAndRegister(json)
	if err != nil {
		t.Fatalf("ParseAndRegister failed: %v", err)
	}

	// Should be able to find object defs by ref
	imageDef, ok := registry.GetObjectDef("app.bsky.embed.images#image")
	if !ok {
		t.Fatal("Expected to find image object def")
	}
	if imageDef.Type != "object" {
		t.Errorf("Expected type 'object', got '%s'", imageDef.Type)
	}

	aspectRatioDef, ok := registry.GetObjectDef("app.bsky.embed.images#aspectRatio")
	if !ok {
		t.Fatal("Expected to find aspectRatio object def")
	}
	if len(aspectRatioDef.RequiredFields) != 2 {
		t.Errorf("Expected 2 required fields, got %d", len(aspectRatioDef.RequiredFields))
	}

	// Main object type should also be accessible
	mainDef, ok := registry.GetObjectDef("app.bsky.embed.images")
	if !ok {
		t.Fatal("Expected to find main object def")
	}
	if mainDef.Type != "object" {
		t.Errorf("Expected main type 'object', got '%s'", mainDef.Type)
	}
}

func TestRegistryUnregister(t *testing.T) {
	json := `{
		"lexicon": 1,
		"id": "test.lexicon",
		"defs": {
			"main": {
				"type": "record",
				"record": {
					"type": "object",
					"properties": {
						"field": {"type": "string"}
					}
				}
			}
		}
	}`

	registry := NewRegistry()
	if _, err := registry.ParseAndRegister(json); err != nil {
		t.Fatalf("ParseAndRegister failed: %v", err)
	}

	if registry.Count() != 1 {
		t.Errorf("Expected count 1, got %d", registry.Count())
	}

	registry.Unregister("test.lexicon")

	if registry.Count() != 0 {
		t.Errorf("Expected count 0 after unregister, got %d", registry.Count())
	}

	_, ok := registry.GetLexicon("test.lexicon")
	if ok {
		t.Error("Expected lexicon to be unregistered")
	}
}

func TestRegistryResolveRef(t *testing.T) {
	json := `{
		"lexicon": 1,
		"id": "app.bsky.richtext.facet",
		"defs": {
			"main": {
				"type": "object",
				"properties": {
					"index": {"type": "ref", "ref": "#byteSlice"}
				}
			},
			"byteSlice": {
				"type": "object",
				"required": ["byteStart", "byteEnd"],
				"properties": {
					"byteStart": {"type": "integer"},
					"byteEnd": {"type": "integer"}
				}
			}
		}
	}`

	registry := NewRegistry()
	if _, err := registry.ParseAndRegister(json); err != nil {
		t.Fatalf("ParseAndRegister failed: %v", err)
	}

	// Resolve local ref
	def, ok := registry.ResolveRef("#byteSlice", "app.bsky.richtext.facet")
	if !ok {
		t.Fatal("Expected to resolve #byteSlice ref")
	}

	objDef, isObj := def.(*ObjectDef)
	if !isObj {
		t.Fatal("Expected ObjectDef")
	}
	if len(objDef.RequiredFields) != 2 {
		t.Errorf("Expected 2 required fields, got %d", len(objDef.RequiredFields))
	}

	// Resolve fully-qualified ref
	def2, ok := registry.ResolveRef("app.bsky.richtext.facet#byteSlice", "ignored")
	if !ok {
		t.Fatal("Expected to resolve fully-qualified ref")
	}
	if def != def2 {
		t.Error("Expected same definition for both refs")
	}
}

func TestRegistryFromLexicons(t *testing.T) {
	lexicon1, _ := Parse(`{
		"lexicon": 1,
		"id": "test.lexicon.one",
		"defs": {
			"main": {
				"type": "record",
				"record": {
					"type": "object",
					"properties": {"field1": {"type": "string"}}
				}
			}
		}
	}`)

	lexicon2, _ := Parse(`{
		"lexicon": 1,
		"id": "test.lexicon.two",
		"defs": {
			"main": {
				"type": "record",
				"record": {
					"type": "object",
					"properties": {"field2": {"type": "integer"}}
				}
			}
		}
	}`)

	registry := NewRegistryFromLexicons([]*Lexicon{lexicon1, lexicon2})

	if registry.Count() != 2 {
		t.Errorf("Expected count 2, got %d", registry.Count())
	}

	ids := registry.GetAllLexiconIDs()
	if len(ids) != 2 {
		t.Errorf("Expected 2 lexicon IDs, got %d", len(ids))
	}
}

func TestRegistryGetCollectionLexicons(t *testing.T) {
	// Record-type main (collection)
	lexicon1, _ := Parse(`{
		"lexicon": 1,
		"id": "test.collection",
		"defs": {
			"main": {
				"type": "record",
				"record": {
					"type": "object",
					"properties": {"field": {"type": "string"}}
				}
			}
		}
	}`)

	// Object-type main (not a collection)
	lexicon2, _ := Parse(`{
		"lexicon": 1,
		"id": "test.object",
		"defs": {
			"main": {
				"type": "object",
				"properties": {"field": {"type": "string"}}
			}
		}
	}`)

	// No main (defs only)
	lexicon3, _ := Parse(`{
		"lexicon": 1,
		"id": "test.defs",
		"defs": {
			"helper": {
				"type": "object",
				"properties": {"field": {"type": "string"}}
			}
		}
	}`)

	registry := NewRegistryFromLexicons([]*Lexicon{lexicon1, lexicon2, lexicon3})

	collections := registry.GetCollectionLexicons()
	if len(collections) != 1 {
		t.Errorf("Expected 1 collection, got %d", len(collections))
	}
	if collections[0].ID != "test.collection" {
		t.Errorf("Expected 'test.collection', got '%s'", collections[0].ID)
	}

	defs := registry.GetDefsLexicons()
	if len(defs) != 1 {
		t.Errorf("Expected 1 defs lexicon, got %d", len(defs))
	}
	if defs[0].ID != "test.defs" {
		t.Errorf("Expected 'test.defs', got '%s'", defs[0].ID)
	}
}
