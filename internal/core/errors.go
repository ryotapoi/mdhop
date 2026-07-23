package core

import "errors"

// Sentinel errors for internal/core. Production code does not currently branch
// on them; they support errors.Is verification in tests and leave room for
// future callers to branch. Wrapped errors retain their operation-specific
// context for message assertions and diagnostics.
var (
	ErrIndexNotFound         = errors.New("index not found")
	ErrFileNotRegistered     = errors.New("file not registered")
	ErrFileNotFound          = errors.New("file not found")
	ErrSourceFileMissing     = errors.New("source file not found on disk")
	ErrFileAlreadyRegistered = errors.New("file already registered")
	ErrAlreadyRegistered     = errors.New("destination already registered")
	ErrAlreadyExistsOnDisk   = errors.New("destination already exists on disk")
	ErrAmbiguousLink         = errors.New("ambiguous link")
	ErrAmbiguousName         = errors.New("ambiguous name")
	ErrLinkNotFound          = errors.New("link not found")
	ErrEntryNotFound         = errors.New("entry not found")
	ErrLinkEscapesVault      = errors.New("link escapes vault")
	ErrSourceStale           = errors.New("source file is stale")
	ErrMovedFileStale        = errors.New("moved file is stale")
	ErrAddingMakesAmbiguous  = errors.New("adding files would make existing links ambiguous")
)
