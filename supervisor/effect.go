package supervisor

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// AppendLineOnce appends "key<TAB>text" to the file at path unless a line for
// key is already there. The file is its own ledger, so a crash between this
// effect and the step checkpoint is safe: on resume the key is found and the
// line is not written again. Single writer per file is assumed (single node).
func AppendLineOnce(path, key, text string) error {
	if strings.ContainsAny(key+text, "\n\t") {
		return fmt.Errorf("supervisor: key and text must not contain tabs or newlines")
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	prefix := key + "\t"
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), prefix) {
			return nil // effect already applied
		}
	}
	if err := sc.Err(); err != nil {
		return err
	}
	// One write call so a SIGKILL cannot leave half a line behind.
	if _, err := f.WriteString(prefix + text + "\n"); err != nil {
		return err
	}
	return f.Sync()
}
