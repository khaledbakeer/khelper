package k8s

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ListDirectories lists directories in a path inside a container
func (c *Client) ListDirectories(ctx context.Context, namespace, podName, container, path string) ([]string, error) {
	var stdout, stderr bytes.Buffer

	err := c.Exec(ctx, ExecOptions{
		Namespace:     namespace,
		PodName:       podName,
		ContainerName: container,
		Command:       []string{"sh", "-c", fmt.Sprintf("find %s -maxdepth 1 -type d 2>/dev/null | tail -n +2 | xargs -I{} basename {}", path)},
		Stdout:        &stdout,
		Stderr:        &stderr,
		TTY:           false,
	})

	if err != nil {
		// Try with ls if find is not available
		stdout.Reset()
		stderr.Reset()
		err = c.Exec(ctx, ExecOptions{
			Namespace:     namespace,
			PodName:       podName,
			ContainerName: container,
			Command:       []string{"sh", "-c", fmt.Sprintf("ls -d %s/*/ 2>/dev/null | xargs -I{} basename {}", path)},
			Stdout:        &stdout,
			Stderr:        &stderr,
			TTY:           false,
		})
		if err != nil {
			return nil, fmt.Errorf("this pod doesn't appear to be a fragment-loader pod (path %s not found)", path)
		}
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return []string{}, nil
	}

	folders := strings.Split(output, "\n")
	// Filter empty strings
	result := make([]string, 0, len(folders))
	for _, f := range folders {
		f = strings.TrimSpace(f)
		if f != "" {
			result = append(result, f)
		}
	}

	return result, nil
}

// ClearDirectory removes all files and directories inside a path
func (c *Client) ClearDirectory(ctx context.Context, namespace, podName, container, path string) error {
	var stderr bytes.Buffer

	// Remove contents but keep the directory itself
	err := c.Exec(ctx, ExecOptions{
		Namespace:     namespace,
		PodName:       podName,
		ContainerName: container,
		Command:       []string{"sh", "-c", fmt.Sprintf("rm -rf %s/* %s/.[!.]* %s/..?* 2>/dev/null; true", path, path, path)},
		Stderr:        &stderr,
		TTY:           false,
	})

	if err != nil {
		return fmt.Errorf("failed to clear directory: %w (stderr: %s)", err, stderr.String())
	}

	return nil
}

// UploadDirectory uploads a local directory to a container path
func (c *Client) UploadDirectory(ctx context.Context, namespace, podName, container, localPath, remotePath string) (int, error) {
	// Create a tar archive of the local directory
	var tarBuffer bytes.Buffer
	fileCount := 0

	err := filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(localPath, path)
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if relPath == "." {
			return nil
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		// Write header
		tw := tar.NewWriter(&tarBuffer)
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// If it's a file, write its contents
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tw, file); err != nil {
				return err
			}
			fileCount++
		}

		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to create tar archive: %w", err)
	}

	// We need to properly close the tar writer
	var finalTarBuffer bytes.Buffer
	tw := tar.NewWriter(&finalTarBuffer)

	err = filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(localPath, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tw, file); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("failed to create tar archive: %w", err)
	}

	tw.Close()

	// Compress the tar archive
	var gzBuffer bytes.Buffer
	gzWriter := gzip.NewWriter(&gzBuffer)
	if _, err := io.Copy(gzWriter, &finalTarBuffer); err != nil {
		return 0, fmt.Errorf("failed to compress archive: %w", err)
	}
	gzWriter.Close()

	// Upload using base64 encoding through exec
	// First, create the target directory if it doesn't exist
	var stderr bytes.Buffer
	err = c.Exec(ctx, ExecOptions{
		Namespace:     namespace,
		PodName:       podName,
		ContainerName: container,
		Command:       []string{"sh", "-c", fmt.Sprintf("mkdir -p %s", remotePath)},
		Stderr:        &stderr,
		TTY:           false,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to create target directory: %w", err)
	}

	// Upload files one by one using cat
	fileCount = 0
	err = filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(localPath, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		targetFile := filepath.Join(remotePath, relPath)
		// Convert to unix path
		targetFile = strings.ReplaceAll(targetFile, "\\", "/")

		if info.IsDir() {
			// Create directory
			err = c.Exec(ctx, ExecOptions{
				Namespace:     namespace,
				PodName:       podName,
				ContainerName: container,
				Command:       []string{"sh", "-c", fmt.Sprintf("mkdir -p '%s'", targetFile)},
				TTY:           false,
			})
			if err != nil {
				return fmt.Errorf("failed to create directory %s: %w", targetFile, err)
			}
		} else {
			// Upload file content
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			// Create parent directory
			parentDir := filepath.Dir(targetFile)
			parentDir = strings.ReplaceAll(parentDir, "\\", "/")
			err = c.Exec(ctx, ExecOptions{
				Namespace:     namespace,
				PodName:       podName,
				ContainerName: container,
				Command:       []string{"sh", "-c", fmt.Sprintf("mkdir -p '%s'", parentDir)},
				TTY:           false,
			})
			if err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			// Write file using cat and stdin
			err = c.Exec(ctx, ExecOptions{
				Namespace:     namespace,
				PodName:       podName,
				ContainerName: container,
				Command:       []string{"sh", "-c", fmt.Sprintf("cat > '%s'", targetFile)},
				Stdin:         bytes.NewReader(content),
				TTY:           false,
			})
			if err != nil {
				return fmt.Errorf("failed to upload file %s: %w", targetFile, err)
			}
			fileCount++
		}

		return nil
	})

	if err != nil {
		return fileCount, err
	}

	return fileCount, nil
}
