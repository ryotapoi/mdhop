package core

import "errors"

// Sentinel errors for internal/core. Use errors.Is at call sites to branch
// on these. Messages match existing error-string prefixes so that legacy
// strings.Contains assertions in tests continue to work.
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
	ErrLinkEscapesVault      = errors.New("link escapes vault")
	ErrSourceStale           = errors.New("source file is stale")
	ErrMovedFileStale        = errors.New("moved file is stale")
	ErrAddingMakesAmbiguous  = errors.New("adding files would make existing links ambiguous")
)
