package tarix

// FileIndex represents information about a file's position in the TAR
type FileIndex struct {
	Start int64 `json:"start"` // Starting byte position in TAR
	Size  int64 `json:"size"`  // Size of the file in bytes
}

// TarIndex represents the full index of a TAR file
type TarIndex struct {
	Files map[string]FileIndex `json:"files"` // List of files in the TAR
}

