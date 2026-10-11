package replay

import "os"

// prepareLegacyReplayWorkspace allows the unprivileged container user
// to traverse the temporary workspace created by BuildFossil.
func prepareLegacyReplayWorkspace(workspace string) error {
	return os.Chmod(workspace, 0755)
}
