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
	var stdout, stderr bytes.Buffer

	// Remove contents but keep the directory itself
	err := c.Exec(ctx, ExecOptions{
		Namespace:     namespace,
		PodName:       podName,
		ContainerName: container,
		Command:       []string{"sh", "-c", fmt.Sprintf("rm -rf %s/* %s/.[!.]* %s/..?* 2>/dev/null; true", path, path, path)},
		Stdout:        &stdout,
		Stderr:        &stderr,
		TTY:           false,
	})

	if err != nil {
		return fmt.Errorf("failed to clear directory: %w (stderr: %s)", err, stderr.String())
	}

	return nil
}

// UploadResult contains the result of an upload operation
type UploadResult struct {
	FileCount int
	Files     []string
}

// UploadDirectory uploads a local directory to a container path
// This mimics kubectl cp behavior using tar
func (c *Client) UploadDirectory(ctx context.Context, namespace, podName, container, localPath, remotePath string) (*UploadResult, error) {
	result := &UploadResult{
		Files: make([]string, 0),
	}

	// First, create the target directory
	var stdout, stderr bytes.Buffer
	err := c.Exec(ctx, ExecOptions{
		Namespace:     namespace,
		PodName:       podName,
		ContainerName: container,
		Command:       []string{"sh", "-c", fmt.Sprintf("mkdir -p '%s'", remotePath)},
		Stdout:        &stdout,
		Stderr:        &stderr,
		TTY:           false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create target directory: %w", err)
	}

	// Create a tar archive of the local directory
	var tarBuffer bytes.Buffer
	tw := tar.NewWriter(&tarBuffer)

	err = filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
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
			result.FileCount++
			result.Files = append(result.Files, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create tar archive: %w", err)
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("failed to close tar writer: %w", err)
	}

	// Upload using tar extraction in container
	// This is similar to how kubectl cp works
	stdout.Reset()
	stderr.Reset()
	err = c.Exec(ctx, ExecOptions{
		Namespace:     namespace,
		PodName:       podName,
		ContainerName: container,
		Command:       []string{"tar", "-xf", "-", "-C", remotePath},
		Stdin:         &tarBuffer,
		Stdout:        &stdout,
		Stderr:        &stderr,
		TTY:           false,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to extract files in container: %w (stderr: %s)", err, stderr.String())
	}

	return result, nil
}

// UploadFile uploads a single file to a container path (with gzip support like your script)
func (c *Client) UploadFile(ctx context.Context, namespace, podName, container, localFile, remotePath string) error {
	// Read file content
	content, err := os.ReadFile(localFile)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	fileName := filepath.Base(localFile)
	remoteFile := filepath.Join(remotePath, fileName)
	remoteFile = strings.ReplaceAll(remoteFile, "\\", "/")

	// Create tar with single file
	var tarBuffer bytes.Buffer
	tw := tar.NewWriter(&tarBuffer)

	header := &tar.Header{
		Name: fileName,
		Mode: 0644,
		Size: int64(len(content)),
	}

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if _, err := tw.Write(content); err != nil {
		return err
	}

	tw.Close()

	// Upload using tar
	var stdout, stderr bytes.Buffer
	err = c.Exec(ctx, ExecOptions{
		Namespace:     namespace,
		PodName:       podName,
		ContainerName: container,
		Command:       []string{"tar", "-xf", "-", "-C", remotePath},
		Stdin:         &tarBuffer,
		Stdout:        &stdout,
		Stderr:        &stderr,
		TTY:           false,
	})

	if err != nil {
		return fmt.Errorf("failed to upload file: %w (stderr: %s)", err, stderr.String())
	}

	// If it's a JS file, also create gzipped version like your script does
	if strings.HasSuffix(localFile, ".js") {
		var gzBuffer bytes.Buffer
		gzWriter := gzip.NewWriter(&gzBuffer)
		gzWriter.Write(content)
		gzWriter.Close()

		gzFileName := fileName + ".gz"

		var gzTarBuffer bytes.Buffer
		gzTw := tar.NewWriter(&gzTarBuffer)

		gzHeader := &tar.Header{
			Name: gzFileName,
			Mode: 0644,
			Size: int64(gzBuffer.Len()),
		}

		if err := gzTw.WriteHeader(gzHeader); err != nil {
			return err
		}

		if _, err := gzTw.Write(gzBuffer.Bytes()); err != nil {
			return err
		}

		gzTw.Close()

		stdout.Reset()
		stderr.Reset()
		err = c.Exec(ctx, ExecOptions{
			Namespace:     namespace,
			PodName:       podName,
			ContainerName: container,
			Command:       []string{"tar", "-xf", "-", "-C", remotePath},
			Stdin:         &gzTarBuffer,
			Stdout:        &stdout,
			Stderr:        &stderr,
			TTY:           false,
		})

		if err != nil {
			return fmt.Errorf("failed to upload gzipped file: %w", err)
		}
	}

	return nil
}
