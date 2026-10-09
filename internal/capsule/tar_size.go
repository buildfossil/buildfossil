package capsule

const tarBlockSize int64 = 512

func tarEntrySize(contentSize int64) int64 {
	return tarBlockSize +
		((contentSize + tarBlockSize - 1) / tarBlockSize * tarBlockSize)
}

func expectedTarSize(manifestSize int64, workspace Workspace) int64 {
	size := tarEntrySize(manifestSize)

	for _, file := range workspace.Files {
		size += tarEntrySize(file.Size)
	}

	// Two zero blocks mark the end of the tar archive.
	return size + 2*tarBlockSize
}
