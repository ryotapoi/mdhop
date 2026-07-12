package core

import "testing"

func newEmptyResolveMaps() *resolveMaps {
	return &resolveMaps{
		pathSet:                 make(map[string]string),
		basenameToPath:          make(map[string]string),
		rootBasenameToPath:      make(map[string]string),
		pathToID:                make(map[string]int64),
		basenameCounts:          make(map[string]int),
		assetPathSet:            make(map[string]string),
		assetBasenameToPath:     make(map[string]string),
		assetRootBasenameToPath: make(map[string]string),
		assetPathToID:           make(map[string]int64),
		assetBasenameCounts:     make(map[string]int),
	}
}

func TestAddNote_Basic(t *testing.T) {
	rm := newEmptyResolveMaps()
	rm.addNote("sub/Note.md")

	// pathSet should have 2 entries: with ext and without ext
	if got, ok := rm.pathSet["sub/note.md"]; !ok || got != "sub/Note.md" {
		t.Errorf("pathSet[sub/note.md] = %q, ok=%v; want sub/Note.md", got, ok)
	}
	if got, ok := rm.pathSet["sub/note"]; !ok || got != "sub/Note.md" {
		t.Errorf("pathSet[sub/note] = %q, ok=%v; want sub/Note.md", got, ok)
	}

	if got := rm.basenameCounts["note"]; got != 1 {
		t.Errorf("basenameCounts[note] = %d; want 1", got)
	}

	// Not a root file, so rootBasenameToPath should be empty
	if _, ok := rm.rootBasenameToPath["note"]; ok {
		t.Error("rootBasenameToPath should not have entry for sub dir file")
	}

	// pathToID should NOT be set by addNote
	if _, ok := rm.pathToID["sub/Note.md"]; ok {
		t.Error("addNote should not set pathToID")
	}
}

func TestAddNote_RootFile(t *testing.T) {
	rm := newEmptyResolveMaps()
	rm.addNote("A.md")

	if got, ok := rm.rootBasenameToPath["a"]; !ok || got != "A.md" {
		t.Errorf("rootBasenameToPath[a] = %q, ok=%v; want A.md", got, ok)
	}
}

func TestRemoveNote_RoundTrip(t *testing.T) {
	rm := newEmptyResolveMaps()
	rm.addNote("sub/Note.md")
	rm.registerNote("sub/Note.md", 42)

	rm.removeNote("sub/Note.md")

	if len(rm.pathSet) != 0 {
		t.Errorf("pathSet should be empty, got %v", rm.pathSet)
	}
	if len(rm.basenameCounts) != 0 {
		t.Errorf("basenameCounts should be empty, got %v", rm.basenameCounts)
	}
	if len(rm.pathToID) != 0 {
		t.Errorf("pathToID should be empty, got %v", rm.pathToID)
	}
}

func TestRemoveNote_BasenameDuplicate(t *testing.T) {
	rm := newEmptyResolveMaps()
	rm.addNote("sub/A.md")
	rm.addNote("other/A.md")

	if got := rm.basenameCounts["a"]; got != 2 {
		t.Fatalf("basenameCounts[a] = %d; want 2", got)
	}

	rm.registerNote("sub/A.md", 1)
	rm.removeNote("sub/A.md")

	if got := rm.basenameCounts["a"]; got != 1 {
		t.Errorf("basenameCounts[a] = %d; want 1", got)
	}
	// pathSet should still have other/A.md entries
	if _, ok := rm.pathSet["other/a.md"]; !ok {
		t.Error("pathSet should still have other/a.md")
	}
}

func TestRemoveNote_ZeroCountCleanup(t *testing.T) {
	rm := newEmptyResolveMaps()
	rm.addNote("A.md")
	rm.registerNote("A.md", 1)

	rm.removeNote("A.md")

	if _, ok := rm.basenameCounts["a"]; ok {
		t.Error("basenameCounts should delete entry when count reaches zero")
	}
	if _, ok := rm.rootBasenameToPath["a"]; ok {
		t.Error("rootBasenameToPath should be cleaned up for root file")
	}
}

func TestAddNote_MixedCase(t *testing.T) {
	rm := newEmptyResolveMaps()
	rm.addNote("Sub/Note.MD")

	// pathSet keys should be lowercase, values preserve case
	if got, ok := rm.pathSet["sub/note.md"]; !ok || got != "Sub/Note.MD" {
		t.Errorf("pathSet[sub/note.md] = %q, ok=%v; want Sub/Note.MD", got, ok)
	}
	if got, ok := rm.pathSet["sub/note"]; !ok || got != "Sub/Note.MD" {
		t.Errorf("pathSet[sub/note] = %q, ok=%v; want Sub/Note.MD", got, ok)
	}
}

func TestRegisterNote_NormalizesPath(t *testing.T) {
	rm := newEmptyResolveMaps()
	rm.registerNote("sub//Note.md", 42)

	if got, ok := rm.pathToID["sub/Note.md"]; !ok || got != 42 {
		t.Errorf("pathToID[sub/Note.md] = %d, ok=%v; want 42", got, ok)
	}
}

// --- addAsset / removeAsset ---

func TestAddAsset_Basic(t *testing.T) {
	rm := newEmptyResolveMaps()
	rm.addAsset("sub/image.png")

	// assetPathSet has only 1 entry (with ext)
	if got, ok := rm.assetPathSet["sub/image.png"]; !ok || got != "sub/image.png" {
		t.Errorf("assetPathSet[sub/image.png] = %q, ok=%v; want sub/image.png", got, ok)
	}
	if len(rm.assetPathSet) != 1 {
		t.Errorf("assetPathSet should have 1 entry, got %d", len(rm.assetPathSet))
	}

	if got := rm.assetBasenameCounts["image.png"]; got != 1 {
		t.Errorf("assetBasenameCounts[image.png] = %d; want 1", got)
	}

	// Not root
	if _, ok := rm.assetRootBasenameToPath["image.png"]; ok {
		t.Error("assetRootBasenameToPath should not have entry for sub dir file")
	}

	// assetPathToID should NOT be set
	if _, ok := rm.assetPathToID["sub/image.png"]; ok {
		t.Error("addAsset should not set assetPathToID")
	}
}

func TestAddAsset_RootFile(t *testing.T) {
	rm := newEmptyResolveMaps()
	rm.addAsset("image.png")

	if got, ok := rm.assetRootBasenameToPath["image.png"]; !ok || got != "image.png" {
		t.Errorf("assetRootBasenameToPath = %q, ok=%v; want image.png", got, ok)
	}
}

func TestRemoveAsset_RoundTrip(t *testing.T) {
	rm := newEmptyResolveMaps()
	rm.addAsset("sub/image.png")
	rm.registerAsset("sub/image.png", 99)

	rm.removeAsset("sub/image.png")

	if len(rm.assetPathSet) != 0 {
		t.Errorf("assetPathSet should be empty, got %v", rm.assetPathSet)
	}
	if len(rm.assetBasenameCounts) != 0 {
		t.Errorf("assetBasenameCounts should be empty, got %v", rm.assetBasenameCounts)
	}
	if len(rm.assetPathToID) != 0 {
		t.Errorf("assetPathToID should be empty, got %v", rm.assetPathToID)
	}
}

func TestRemoveAsset_ZeroCountCleanup(t *testing.T) {
	rm := newEmptyResolveMaps()
	rm.addAsset("photo.jpg")
	rm.registerAsset("photo.jpg", 1)

	rm.removeAsset("photo.jpg")

	if _, ok := rm.assetBasenameCounts["photo.jpg"]; ok {
		t.Error("assetBasenameCounts should delete entry when count reaches zero")
	}
	if _, ok := rm.assetRootBasenameToPath["photo.jpg"]; ok {
		t.Error("assetRootBasenameToPath should be cleaned up for root file")
	}
}

func TestAddAsset_MixedCase(t *testing.T) {
	rm := newEmptyResolveMaps()
	rm.addAsset("Sub/IMAGE.PNG")

	if got, ok := rm.assetPathSet["sub/image.png"]; !ok || got != "Sub/IMAGE.PNG" {
		t.Errorf("assetPathSet[sub/image.png] = %q, ok=%v; want Sub/IMAGE.PNG", got, ok)
	}
}

func TestRegisterAsset_NormalizesPath(t *testing.T) {
	rm := newEmptyResolveMaps()
	rm.registerAsset("sub//image.png", 99)

	if got, ok := rm.assetPathToID["sub/image.png"]; !ok || got != 99 {
		t.Errorf("assetPathToID[sub/image.png] = %d, ok=%v; want 99", got, ok)
	}
}

// --- rebuildBasenameToPath ---

func TestRebuildBasenameToPath_UniqueOnly(t *testing.T) {
	rm := newEmptyResolveMaps()
	rm.addNote("A.md")
	rm.addNote("sub/B.md")
	rm.registerNote("A.md", 1)
	rm.registerNote("sub/B.md", 2)

	rm.rebuildBasenameToPath(nil)

	if got, ok := rm.basenameToPath["a"]; !ok || got != "A.md" {
		t.Errorf("basenameToPath[a] = %q, ok=%v; want A.md", got, ok)
	}
	if got, ok := rm.basenameToPath["b"]; !ok || got != "sub/B.md" {
		t.Errorf("basenameToPath[b] = %q, ok=%v; want sub/B.md", got, ok)
	}
}

func TestRebuildBasenameToPath_ExcludesDuplicates(t *testing.T) {
	rm := newEmptyResolveMaps()
	rm.addNote("sub/A.md")
	rm.addNote("other/A.md")
	rm.registerNote("sub/A.md", 1)
	rm.registerNote("other/A.md", 2)

	rm.rebuildBasenameToPath(nil)

	if _, ok := rm.basenameToPath["a"]; ok {
		t.Error("basenameToPath should not include duplicate basename")
	}
}

func TestRebuildBasenameToPath_ExtraPaths(t *testing.T) {
	rm := newEmptyResolveMaps()
	rm.addNote("A.md") // count=1, but not in pathToID

	rm.rebuildBasenameToPath([]string{"A.md"})

	if got, ok := rm.basenameToPath["a"]; !ok || got != "A.md" {
		t.Errorf("basenameToPath[a] = %q, ok=%v; want A.md", got, ok)
	}
}

func TestRebuildBasenameToPath_NilExtraPaths(t *testing.T) {
	rm := newEmptyResolveMaps()
	rm.addNote("A.md")
	rm.registerNote("A.md", 1)

	rm.rebuildBasenameToPath(nil)

	if got, ok := rm.basenameToPath["a"]; !ok || got != "A.md" {
		t.Errorf("basenameToPath[a] = %q, ok=%v; want A.md", got, ok)
	}
}

// --- rebuildAssetBasenameToPath ---

func TestRebuildAssetBasenameToPath_UniqueOnly(t *testing.T) {
	rm := newEmptyResolveMaps()
	rm.addAsset("img.png")
	rm.registerAsset("img.png", 1)

	rm.rebuildAssetBasenameToPath()

	if got, ok := rm.assetBasenameToPath["img.png"]; !ok || got != "img.png" {
		t.Errorf("assetBasenameToPath[img.png] = %q, ok=%v; want img.png", got, ok)
	}
}

func TestRebuildAssetBasenameToPath_ExcludesDuplicates(t *testing.T) {
	rm := newEmptyResolveMaps()
	rm.addAsset("sub/img.png")
	rm.addAsset("other/img.png")
	rm.registerAsset("sub/img.png", 1)
	rm.registerAsset("other/img.png", 2)

	rm.rebuildAssetBasenameToPath()

	if _, ok := rm.assetBasenameToPath["img.png"]; ok {
		t.Error("assetBasenameToPath should not include duplicate basename")
	}
}
