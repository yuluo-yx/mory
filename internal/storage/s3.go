package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuluo-yx/mory/internal/atomicfile"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type s3Backend struct{ config Config }

func newS3Backend(config Config) Backend { return &s3Backend{config: config} }

func (backend *s3Backend) client(ctx context.Context) (*s3.Client, error) {
	options := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(backend.config.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			backend.config.AccessKeyID,
			backend.config.AccessKeySecret,
			backend.config.SessionToken,
		)),
	}
	if backend.config.Endpoint != "" {
		options = append(options, awsconfig.WithBaseEndpoint(backend.config.Endpoint))
	}
	config, err := awsconfig.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("configure s3 client: %w", err)
	}
	return s3.NewFromConfig(config, func(options *s3.Options) {
		options.UsePathStyle = backend.config.Endpoint != ""
	}), nil
}

func (backend *s3Backend) Pull(ctx context.Context, root string) (Summary, error) {
	client, err := backend.client(ctx)
	if err != nil {
		return Summary{}, err
	}
	prefix := strings.Trim(backend.config.Prefix, "/")
	requestPrefix := prefix
	if requestPrefix != "" {
		requestPrefix += "/"
	}
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: aws.String(backend.config.Bucket),
		Prefix: aws.String(requestPrefix),
	})
	var summary Summary
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return summary, fmt.Errorf("list s3 objects: %w", err)
		}
		for _, object := range page.Contents {
			key := aws.ToString(object.Key)
			relative, ok := objectRelative(prefix, key)
			if !ok || relative == "" || strings.HasSuffix(key, "/") {
				continue
			}
			result, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(backend.config.Bucket), Key: aws.String(key)})
			if err != nil {
				return summary, fmt.Errorf("download s3 object %q: %w", key, err)
			}
			written, copyErr := copyRemoteFile(root, relative, result.Body)
			if copyErr != nil {
				return summary, fmt.Errorf("save s3 object %q: %w", key, copyErr)
			}
			summary.Files++
			summary.Bytes += written
		}
	}
	return summary, nil
}

func (backend *s3Backend) Push(ctx context.Context, root string) (Summary, error) {
	client, err := backend.client(ctx)
	if err != nil {
		return Summary{}, err
	}
	files, err := localFiles(root)
	if err != nil {
		return Summary{}, err
	}
	var summary Summary
	for _, file := range files {
		body, err := os.Open(file.Path)
		if err != nil {
			return summary, fmt.Errorf("open local file %q: %w", file.Relative, err)
		}
		_, uploadErr := client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:        aws.String(backend.config.Bucket),
			Key:           aws.String(objectKey(backend.config.Prefix, file.Relative)),
			Body:          body,
			ContentLength: aws.Int64(file.Size),
		})
		closeErr := body.Close()
		if uploadErr != nil || closeErr != nil {
			return summary, fmt.Errorf("upload s3 object %q: %w", file.Relative, errors.Join(uploadErr, closeErr))
		}
		summary.Files++
		summary.Bytes += file.Size
	}
	return summary, nil
}

// copyRemoteFile owns source's Close, when available, and checks it before committing.
func copyRemoteFile(root, relative string, source io.Reader) (written int64, err error) {
	closed := false
	closeSource := func() error {
		if closer, ok := source.(io.Closer); ok && !closed {
			closed = true
			return closer.Close()
		}
		return nil
	}
	defer func() { err = errors.Join(err, closeSource()) }()
	root, err = filepath.Abs(root)
	if err != nil {
		return 0, err
	}
	destination, err := safeLocalPath(root, relative)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return 0, err
	}
	confined, err := os.OpenRoot(root)
	if err != nil {
		return 0, err
	}
	defer confined.Close()
	local, err := filepath.Rel(root, destination)
	if err != nil {
		return 0, err
	}
	// Root methods enforce containment during I/O, including concurrent link changes.
	if err := confined.MkdirAll(filepath.Dir(local), 0o755); err != nil {
		return 0, fmt.Errorf("create local directory: %w", err)
	}
	err = atomicfile.Replace(confined, local, 0o644, func(file *os.File) error {
		var copyErr error
		written, copyErr = io.Copy(file, source)
		return errors.Join(copyErr, closeSource())
	})
	return written, err
}
