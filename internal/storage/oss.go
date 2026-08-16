package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

type ossBackend struct{ config Config }

func newOSSBackend(config Config) Backend { return &ossBackend{config: config} }

func (backend *ossBackend) client() *oss.Client {
	config := oss.LoadDefaultConfig().
		WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			backend.config.AccessKeyID,
			backend.config.AccessKeySecret,
			backend.config.SessionToken,
		)).
		WithRegion(backend.config.Region)
	if backend.config.Endpoint != "" {
		config.WithEndpoint(backend.config.Endpoint).WithUsePathStyle(true)
	}
	return oss.NewClient(config)
}

func (backend *ossBackend) Pull(ctx context.Context, root string) (Summary, error) {
	client := backend.client()
	prefix := strings.Trim(backend.config.Prefix, "/")
	requestPrefix := prefix
	if requestPrefix != "" {
		requestPrefix += "/"
	}
	paginator := client.NewListObjectsV2Paginator(&oss.ListObjectsV2Request{
		Bucket: oss.Ptr(backend.config.Bucket),
		Prefix: oss.Ptr(requestPrefix),
	})
	var summary Summary
	for paginator.HasNext() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return summary, fmt.Errorf("list oss objects: %w", err)
		}
		for _, object := range page.Contents {
			key := oss.ToString(object.Key)
			relative, ok := objectRelative(prefix, key)
			if !ok || relative == "" || strings.HasSuffix(key, "/") {
				continue
			}
			result, err := client.GetObject(ctx, &oss.GetObjectRequest{Bucket: oss.Ptr(backend.config.Bucket), Key: oss.Ptr(key)})
			if err != nil {
				return summary, fmt.Errorf("download oss object %q: %w", key, err)
			}
			written, copyErr := copyRemoteFile(root, relative, result.Body)
			closeErr := result.Body.Close()
			if copyErr != nil || closeErr != nil {
				return summary, fmt.Errorf("save oss object %q: %w", key, errors.Join(copyErr, closeErr))
			}
			summary.Files++
			summary.Bytes += written
		}
	}
	return summary, nil
}

func (backend *ossBackend) Push(ctx context.Context, root string) (Summary, error) {
	client := backend.client()
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
		_, uploadErr := client.PutObject(ctx, &oss.PutObjectRequest{
			Bucket:        oss.Ptr(backend.config.Bucket),
			Key:           oss.Ptr(objectKey(backend.config.Prefix, file.Relative)),
			Body:          body,
			ContentLength: oss.Ptr(file.Size),
		})
		closeErr := body.Close()
		if uploadErr != nil || closeErr != nil {
			return summary, fmt.Errorf("upload oss object %q: %w", file.Relative, errors.Join(uploadErr, closeErr))
		}
		summary.Files++
		summary.Bytes += file.Size
	}
	return summary, nil
}
