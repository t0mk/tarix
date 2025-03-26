# Tarix

Tarix is a command-line utility for efficient extraction from tar files. It creates an index and then it can load a file from tar archive based on start and end offsets.

I use it for fast access to hundreds of thousands of files stored in Fuse-mount S3-like storage. It allows me to work with one archive instead of 800k file (faster transfer, easier handling ,..). Without Fuse mount, I would use HTTP Range requests.

## CLI Usage

```bash
# Create an index for a tar file
tarix index -tar <tar-file> -output <index-file>

# Extract a specific file using the index
tarix extract -tar <tar-file> -index <index-file> -file <file-path> -output <output-file>

# List contents of a tar archive using its index
tarix list -index <index-file>

# Print file contents directly to stdout
tarix printfrompath -tar <tar-file> -index <index-file> -file <file-path>
```

## Lookup Usage in Go

```golang

	DataTar = os.Getenv(dataTarEnv)
	CheckFileExists(DataTar)
	DataIndex = os.Getenv(dataIndexEnv)
	CheckFileExists(DataIndex)

	DataHandle, err = tarix.NewTarixHandle(DataTar, DataIndex)
	if err != nil {
		panic(err)
	}
    
    // [...]

	bs, err := DataHandle.ExtractBytesOfFile(key)
	if err != nil {
		return nil, err
	}
```

## How it works

Tarix creates an index that maps file paths to their exact positions within the tar archive. This enables direct access to files without scanning through the entire archive. File paths are hashed using MD5 (truncated to 16 characters) for efficient lookup.

The index is stored in CSV format with the following structure:
```
key,start,size
```
where:
- `key`: MD5 hash of the file path (16 characters)
- `start`: Starting position of the file in the tar archive
- `size`: Size of the file in bytes


## License

MIT License

