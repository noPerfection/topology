// Package source defines the dependency parameters in the 'dev' context.
// It's used by dependency_manager to articulate with the dependencies.
package source

import (
	"fmt"
	"net/url"

	"github.com/asaskevich/govalidator"
	"github.com/sds-framework/os-lib/path"
)

// The Src struct is used to fetch the source code.
// It has the optional Branch option.
// When the Branch is set, then the dependency manager will check out that from remote.
type Src struct {
	Url      string // Remote Url of the source code
	GitUrl   string // The Git url derived from the url
	Branch   string // Branch to fetch. Leave it empty to get the certain branch.
	localUrl string // Optionally, pass the url to the local directory
}

// New dependency by its source code remote url.
// It can optionally accept the local url if it's not an empty string.
//
// It returns error in the following cases:
//   - url is not a web location that could be turned in to the git.
//   - localUrl is not a directory with `go.mod` file.
func New(url string, localUrls ...string) (*Src, error) {
	gitUrl, err := convertToGitUrl(url)
	if err != nil {
		return nil, fmt.Errorf("convertToGitUrl('%s'): %w", url, err)
	}

	localUrl := ""
	if len(localUrls) > 0 {
		localUrl = localUrls[0]
	}

	src := &Src{Url: url, GitUrl: gitUrl}
	if len(localUrl) > 0 {
		if err := src.setLocalUrl(localUrl); err != nil {
			return nil, fmt.Errorf("src.SetLocalUrl('%s'): %w", localUrl, err)
		}
	}

	return src, nil
}

// SetLocalUrl sets the already downloaded source code path in this machine.
// Returns error if the path doesn't exist or not a git repository or has no go.mod
func (src *Src) setLocalUrl(localUrl string) error {
	if src == nil {
		return fmt.Errorf("nil")
	}

	exist, err := path.DirExist(localUrl)
	if err != nil {
		return fmt.Errorf("path.DirExist('%s'): %w", localUrl, err)
	}
	if !exist {
		return fmt.Errorf("path.DirExist('%s'): false", localUrl)
	}

	filePath := path.AbsDir(localUrl, "go.mod")
	exist, err = path.FileExist(filePath)
	if err != nil {
		return fmt.Errorf("path.FileExist('%s'): %w", filePath, err)
	}
	if !exist {
		return fmt.Errorf("path.FileExist('%s'): false", filePath)
	}

	src.localUrl = localUrl

	return nil
}

// SetBranch sets the branch name of the repository.
func (src *Src) SetBranch(branch string) {
	if src == nil {
		return
	}

	src.Branch = branch
}

func (src *Src) LocalUrl() string {
	if src == nil {
		return ""
	}
	return src.localUrl
}

// convertToGitUrl converts the url without any protocol schema part into https link to the git.
// It supports only the remote urls.
// The file paths are not supported.
func convertToGitUrl(rawUrl string) (string, error) {
	_, err := url.ParseRequestURI(rawUrl)
	if err == nil {
		return "", fmt.Errorf("url should be not an absolute path")
	}

	absPath := "https://" + rawUrl + ".git"
	URL, err := url.ParseRequestURI(absPath)
	if err != nil {
		return "", fmt.Errorf("invalid '%s' url: %w", rawUrl, err)
	}

	hostName := URL.Hostname()
	if !govalidator.IsDNSName(hostName) {
		return "", fmt.Errorf("not a valid DNS Name: %s", hostName)
	}

	return absPath, nil
}
