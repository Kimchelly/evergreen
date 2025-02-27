package operations

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/evergreen-ci/evergreen"
	"github.com/evergreen-ci/evergreen/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClone(t *testing.T) {
	type testCase struct {
		opts      cloneOptions
		isPassing bool
	}

	testCases := map[string]testCase{
		"SimpleHTTPS": {isPassing: true, opts: cloneOptions{
			owner:      "evergreen-ci",
			repository: "sample",
			revision:   "cf46076567e4949f9fc68e0634139d4ac495c89b",
			branch:     "main",
		}},
		"InvalidRepo": {isPassing: false, opts: cloneOptions{
			owner:      "evergreen-ci",
			repository: "foo",
			revision:   "cf46076567e4949f9fc68e0634139d4ac495c89b",
			branch:     "main",
		}},
		"InvalidRevision": {isPassing: false, opts: cloneOptions{
			owner:      "evergreen-ci",
			repository: "sample",
			revision:   "9999999999999999999999999999999999999999",
			branch:     "main",
		}},
		"InvalidToken": {isPassing: false, opts: cloneOptions{
			owner:      "10gen",
			repository: "kernel-tools",
			revision:   "cabca3defc4b251c8a0be268969606717e01f906",
			branch:     "main",
			token:      "foo",
		}},
	}

	opts := cloneOptions{
		branch: "main",
	}
	for name, test := range testCases {
		opts.owner = test.opts.owner
		opts.repository = test.opts.repository
		opts.revision = test.opts.revision

		if test.opts.token != "" {
			opts.token = test.opts.token
		}
		t.Run(name, func(t *testing.T) {
			runCloneTest(t, opts, test.isPassing)
		})
	}
}

func runCloneTest(t *testing.T, opts cloneOptions, pass bool) {
	opts.rootDir = t.TempDir()
	if !pass {
		assert.Error(t, clone(opts))
		return
	}
	assert.NoError(t, clone(opts))
}

func TestTruncateName(t *testing.T) {
	// Test with .tar in filename
	fileName := strings.Repeat("a", 300) + ".tar.gz"
	newName := truncateFilename(fileName)
	assert.NotEqual(t, newName, fileName)
	assert.Len(t, newName, 250)
	assert.Equal(t, strings.Repeat("a", 243)+".tar.gz", newName)

	// Test with 3 dots in filename
	fileName = strings.Repeat("a", 243) + "_v4.4.4.txt"
	newName = truncateFilename(fileName)
	assert.NotEqual(t, newName, fileName)
	assert.Len(t, newName, 250)
	assert.Equal(t, strings.Repeat("a", 243)+"_v4.txt", newName)

	// Test filename at max length
	fileName = strings.Repeat("a", 247) + ".js"
	newName = truncateFilename(fileName)
	assert.Equal(t, fileName, newName)

	// Test "extension" significantly longer than name
	fileName = "a." + strings.Repeat("b", 300)
	newName = truncateFilename(fileName)
	assert.Equal(t, fileName, newName)

	// Test no extension
	fileName = strings.Repeat("a", 300)
	newName = truncateFilename(fileName)
	assert.Len(t, newName, 250)
	assert.Equal(t, strings.Repeat("a", 250), newName)
}

func TestFileNameWithIndex(t *testing.T) {
	t.Run("JustFilename", func(t *testing.T) {
		assert.Equal(t, "file_(4).txt", fileNameWithIndex("file.txt", 5))
	})
	t.Run("DirectoryAndFilename", func(t *testing.T) {
		assert.Equal(t, filepath.Join("path", "to", "file_(4).txt"), fileNameWithIndex(filepath.Join("path", "to", "file.txt"), 5))
	})
	t.Run("FilenameWithoutExtensions", func(t *testing.T) {
		assert.Equal(t, "file_(4)", fileNameWithIndex("file", 5))
	})
	t.Run("DirectoryAndFilenameWithoutExtensions", func(t *testing.T) {
		assert.Equal(t, filepath.Join("path", "to", "file_(4)"), fileNameWithIndex(filepath.Join("path", "to", "file"), 5))
	})
	t.Run("DirectoryAndFilenameWithMultipleExtensions", func(t *testing.T) {
		assert.Equal(t, filepath.Join("path", "to", "file_(4).tar.gz"), fileNameWithIndex(filepath.Join("path", "to", "file.tar.gz"), 5))
	})
	t.Run("DirectoryWithPeriodsAndFilenameWithExtension", func(t *testing.T) {
		assert.Equal(t, filepath.Join("path.with.dots", "to", "file_(4).tar.gz"), fileNameWithIndex(filepath.Join("path.with.dots", "to", "file.tar.gz"), 5))
	})
}

func TestResetGitRemoteToSSH(t *testing.T) {
	opts := cloneOptions{
		owner:      "evergreen-ci",
		repository: "sample",
		revision:   "cf46076567e4949f9fc68e0634139d4ac495c89b",
		branch:     "main",
		rootDir:    t.TempDir(),
		token:      "abcdefg1234",
		isAppToken: true,
	}

	require.NoError(t, clone(opts))

	// check that the remote is reset to SSH
	cmd := exec.Command("git", "-C", opts.rootDir, "remote", "-v")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err)
	assert.Contains(t, string(output), "git@github.com:")
	assert.NotContains(t, string(output), "https:")
}

func TestGetArtifactFolderName(t *testing.T) {
	testCases := map[string]struct {
		task     service.RestTask
		expected string
	}{
		"ShortBuildVariant": {
			task: service.RestTask{
				BuildVariant: "variant",
				Requester:    evergreen.PatchVersionRequester,
				PatchNumber:  123,
				DisplayName:  "display",
			},
			expected: "artifacts-patch-123_variant_display",
		},
		"LongBuildVariant": {
			task: service.RestTask{
				BuildVariant: strings.Repeat("a", 200),
				Requester:    evergreen.PatchVersionRequester,
				PatchNumber:  123,
				DisplayName:  "display",
			},
			expected: fmt.Sprintf("artifacts-patch-123_%s_display", strings.Repeat("a", 100)),
		},
		"ShortRevision": {
			task: service.RestTask{
				BuildVariant: "variant",
				Requester:    evergreen.RepotrackerVersionRequester,
				Revision:     "abcde",
				DisplayName:  "display",
			},
			expected: "artifacts-variant_display",
		},
		"LongRevision": {
			task: service.RestTask{
				BuildVariant: "variant",
				Requester:    evergreen.RepotrackerVersionRequester,
				Revision:     "abcde1234567",
				DisplayName:  "display",
			},
			expected: "artifacts-abcde1-variant_display",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			result := getArtifactFolderName(&tc.task)
			assert.Equal(t, tc.expected, result)
		})
	}
}
