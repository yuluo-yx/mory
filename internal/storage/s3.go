package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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
			closeErr := result.Body.Close()
			if copyErr != nil || closeErr != nil {
				return summary, fmt.Errorf("save s3 object %q: %w", key, errors.Join(copyErr, closeErr))
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

func copyRemoteFile(root, relative string, source io.Reader) (int64, error) {
	destination, err := safeLocalPath(root, relative)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return 0, fmt.Errorf("create local directory: %w", err)
	}
	file, err := os.Create(destination)
	if err != nil {
		return 0, fmt.Errorf("create local file %q: %w", relative, err)
	}
	written, copyErr := io.Copy(file, source)
	if closeErr := file.Close(); copyErr != nil || closeErr != nil {
		return written, errors.Join(copyErr, closeErr)
	}
	return written, nil
}
