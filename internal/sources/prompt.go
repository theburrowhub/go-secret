package sources

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// PromptForSource is the interactive picker used when no --source is given
// and no default exists. Returns the chosen source ID.
func PromptForSource(active []Provider) (string, error) {
	ids := make([]string, 0, len(active))
	for _, p := range active {
		ids = append(ids, p.ID())
	}
	return promptForSourceFromIO(os.Stdin, os.Stdout, ids)
}

func promptForSourceFromIO(in io.Reader, out io.Writer, ids []string) (string, error) {
	if len(ids) == 0 {
		return "", errors.New("no active sources")
	}
	if len(ids) == 1 {
		return ids[0], nil
	}
	br := bufio.NewReader(in)
	for {
		_, _ = fmt.Fprintln(out, "Select source:")
		for i, id := range ids {
			_, _ = fmt.Fprintf(out, "  %d) %s\n", i+1, id)
		}
		_, _ = fmt.Fprintf(out, "Choice [1-%d]: ", len(ids))
		line, err := br.ReadString('\n')
		if err != nil {
			return "", err
		}
		n, err := strconv.Atoi(strings.TrimSpace(line))
		if err == nil && n >= 1 && n <= len(ids) {
			return ids[n-1], nil
		}
		_, _ = fmt.Fprintln(out, "Invalid choice, try again.")
	}
}
