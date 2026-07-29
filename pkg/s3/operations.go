package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	transfertypes "github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager/types"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go/middleware"
)

// Encryption is an S3-compatible server-side encryption mode.
type Encryption string

const (
	EncryptionNone   Encryption = ""
	EncryptionAES256 Encryption = "AES256"
	EncryptionAWSKMS Encryption = "aws:kms"
)

// UploadOptions controls portable S3 upload behavior.
type UploadOptions struct {
	ContentType          string
	Metadata             map[string]string
	ServerSideEncryption Encryption
	KMSKeyID             string
}

// UploadResult describes a completed upload without exposing AWS SDK types.
type UploadResult struct {
	Key       string
	Size      int64
	ETag      string
	VersionID string
}

// DownloadedObject combines a streaming body with its object metadata.
// The caller must close Body.
type DownloadedObject struct {
	Body io.ReadCloser
	Info ObjectInfo
}

// ListOptions controls prefix and recursive listing behavior. Non-recursive
// lists include common prefixes as ObjectInfo values with IsPrefix set.
type ListOptions struct {
	Prefix    string
	Recursive bool
	PageSize  int32
}

// CopyResult describes a server-side copy.
type CopyResult struct {
	ETag         string
	LastModified time.Time
	VersionID    string
}

// CreateBucket creates a bucket if it does not already exist.
func (client *Client) CreateBucket(ctx context.Context, bucket string) error {
	if err := validateBucket(bucket); err != nil {
		return err
	}

	_, err := client.api.HeadBucket(ctx, &awss3.HeadBucketInput{
		Bucket: new(bucket),
	})
	if err == nil {
		return nil
	}
	headErr := wrapOperationError("head bucket", bucket, "", err)
	if !errors.Is(headErr, ErrNotFound) {
		return headErr
	}

	input := &awss3.CreateBucketInput{Bucket: new(bucket)}
	// AWS requires LocationConstraint outside us-east-1. Custom endpoints
	// derive placement from their own endpoint and reject AWS region names in
	// some implementations, so no AWS location payload is sent to them.
	if client.config.Endpoint == "" &&
		client.config.Region != "" &&
		client.config.Region != "us-east-1" {
		input.CreateBucketConfiguration = &types.CreateBucketConfiguration{
			LocationConstraint: types.BucketLocationConstraint(client.config.Region),
		}
	}

	_, err = client.api.CreateBucket(ctx, input)
	if err != nil {
		if errorCode(err) == "BucketAlreadyOwnedByYou" {
			return nil
		}
		return wrapOperationError("create bucket", bucket, "", err)
	}
	return nil
}

// PutObject uploads a stream. Set size to -1 when it is unknown.
func (client *Client) PutObject(
	ctx context.Context,
	bucket string,
	key string,
	body io.Reader,
	size int64,
	options *UploadOptions,
) (UploadResult, error) {
	if err := validateObjectAddress(bucket, key); err != nil {
		return UploadResult{}, err
	}
	if body == nil {
		return UploadResult{}, errors.New("s3: upload body is required")
	}
	if size < -1 {
		return UploadResult{}, errors.New("s3: upload size must be -1 or greater")
	}
	normalized, err := normalizeUploadOptions(key, options)
	if err != nil {
		return UploadResult{}, err
	}

	input := &transfermanager.UploadObjectInput{
		Bucket:      new(bucket),
		Key:         new(key),
		Body:        body,
		ContentType: new(normalized.ContentType),
		Metadata:    cloneMetadata(normalized.Metadata),
	}
	if size >= 0 {
		input.ContentLength = new(size)
	}
	if normalized.ServerSideEncryption != EncryptionNone {
		input.ServerSideEncryption = transfertypes.ServerSideEncryption(
			normalized.ServerSideEncryption,
		)
	}
	if normalized.KMSKeyID != "" {
		input.SSEKMSKeyID = new(normalized.KMSKeyID)
	}

	if size >= 0 && size <= client.config.SinglePutThreshold {
		putOptions := []func(*awss3.Options){}
		if _, seekable := body.(io.Seeker); !seekable {
			putOptions = append(putOptions, useUnsignedPayload)
		}
		result, err := client.api.PutObject(ctx, &awss3.PutObjectInput{
			Bucket:               input.Bucket,
			Key:                  input.Key,
			Body:                 input.Body,
			ContentLength:        input.ContentLength,
			ContentType:          input.ContentType,
			Metadata:             input.Metadata,
			ServerSideEncryption: types.ServerSideEncryption(input.ServerSideEncryption),
			SSEKMSKeyId:          input.SSEKMSKeyID,
		}, putOptions...)
		if err != nil {
			return UploadResult{}, wrapOperationError("put", bucket, key, err)
		}
		return UploadResult{
			Key:       key,
			Size:      size,
			ETag:      trimETag(aws.ToString(result.ETag)),
			VersionID: aws.ToString(result.VersionId),
		}, nil
	}

	select {
	case client.multipartSlots <- struct{}{}:
		defer func() { <-client.multipartSlots }()
	case <-ctx.Done():
		return UploadResult{}, ctx.Err()
	}
	result, err := client.transfers.UploadObject(ctx, input)
	if err != nil {
		return UploadResult{}, wrapOperationError("put", bucket, key, err)
	}
	return UploadResult{
		Key:       key,
		Size:      size,
		ETag:      trimETag(aws.ToString(result.ETag)),
		VersionID: aws.ToString(result.VersionID),
	}, nil
}

func useUnsignedPayload(options *awss3.Options) {
	options.APIOptions = append(options.APIOptions, func(stack *middleware.Stack) error {
		if err := v4.RemoveContentSHA256HeaderMiddleware(stack); err != nil {
			return err
		}
		if err := v4.RemoveComputePayloadSHA256Middleware(stack); err != nil {
			return err
		}
		return v4.AddUnsignedPayloadMiddleware(stack)
	})
}

// PutFile uploads a local file.
func (client *Client) PutFile(
	ctx context.Context,
	bucket string,
	key string,
	sourcePath string,
	options *UploadOptions,
) (UploadResult, error) {
	file, err := os.Open(sourcePath)
	if err != nil {
		return UploadResult{}, fmt.Errorf("s3: open upload source %q: %w", sourcePath, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return UploadResult{}, fmt.Errorf("s3: stat upload source %q: %w", sourcePath, err)
	}
	if !stat.Mode().IsRegular() {
		return UploadResult{}, fmt.Errorf("s3: upload source %q is not a regular file", sourcePath)
	}

	if options == nil {
		options = &UploadOptions{}
	} else {
		cloned := *options
		options = &cloned
	}
	if options.ContentType == "" {
		options.ContentType = inferContentType(sourcePath)
	}
	return client.PutObject(ctx, bucket, key, file, stat.Size(), options)
}

// GetObject starts downloading an object.
func (client *Client) GetObject(
	ctx context.Context,
	bucket string,
	key string,
) (*DownloadedObject, error) {
	if err := validateObjectAddress(bucket, key); err != nil {
		return nil, err
	}

	result, err := client.api.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: new(bucket),
		Key:    new(key),
	})
	if err != nil {
		return nil, wrapOperationError("get", bucket, key, err)
	}
	return &DownloadedObject{
		Body: result.Body,
		Info: ObjectInfo{
			Key:          key,
			Size:         aws.ToInt64(result.ContentLength),
			LastModified: aws.ToTime(result.LastModified),
			ContentType:  aws.ToString(result.ContentType),
			ETag:         trimETag(aws.ToString(result.ETag)),
			Metadata:     cloneMetadata(result.Metadata),
			VersionID:    aws.ToString(result.VersionId),
			StorageClass: string(result.StorageClass),
		},
	}, nil
}

// GetFile downloads an object to a local path. The destination is replaced
// only after the complete response has been written successfully.
func (client *Client) GetFile(
	ctx context.Context,
	bucket string,
	key string,
	destinationPath string,
) (returnErr error) {
	object, err := client.GetObject(ctx, bucket, key)
	if err != nil {
		return err
	}
	defer func() {
		returnErr = errors.Join(returnErr, object.Body.Close())
	}()

	directory := filepath.Dir(destinationPath)
	tempFile, err := os.CreateTemp(directory, "."+filepath.Base(destinationPath)+".*.part")
	if err != nil {
		return fmt.Errorf("s3: create temporary download file: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if _, err := io.Copy(tempFile, object.Body); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("s3: download %s/%s: %w", bucket, key, err)
	}
	if err := tempFile.Sync(); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("s3: sync temporary download file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("s3: close temporary download file: %w", err)
	}
	if err := os.Rename(tempPath, destinationPath); err != nil {
		return fmt.Errorf("s3: replace download destination: %w", err)
	}
	return nil
}

// DeleteObject removes an object. S3 deletion is idempotent.
func (client *Client) DeleteObject(
	ctx context.Context,
	bucket string,
	key string,
) error {
	if err := validateObjectAddress(bucket, key); err != nil {
		return err
	}
	_, err := client.api.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: new(bucket),
		Key:    new(key),
	})
	return wrapOperationError("delete", bucket, key, err)
}

// ObjectExists reports whether an object exists.
func (client *Client) ObjectExists(
	ctx context.Context,
	bucket string,
	key string,
) (bool, error) {
	_, err := client.StatObject(ctx, bucket, key)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ListObjects lazily yields matching objects and follows continuation tokens
// only as the caller advances the sequence. Errors are yielded once and end
// the sequence.
func (client *Client) ListObjects(
	ctx context.Context,
	bucket string,
	options ListOptions,
) iter.Seq2[ObjectInfo, error] {
	return func(yield func(ObjectInfo, error) bool) {
		if err := validateBucket(bucket); err != nil {
			yield(ObjectInfo{}, err)
			return
		}
		if options.PageSize < 0 {
			yield(ObjectInfo{}, errors.New("s3: page size must not be negative"))
			return
		}

		input := &awss3.ListObjectsV2Input{
			Bucket: new(bucket),
			Prefix: new(options.Prefix),
		}
		if !options.Recursive {
			input.Delimiter = new("/")
		}
		if options.PageSize > 0 {
			input.MaxKeys = new(options.PageSize)
		}

		paginator := awss3.NewListObjectsV2Paginator(client.api, input)
		for paginator.HasMorePages() {
			page, err := paginator.NextPage(ctx)
			if err != nil {
				yield(
					ObjectInfo{},
					wrapOperationError("list", bucket, options.Prefix, err),
				)
				return
			}
			for _, object := range objectsFromPage(page) {
				if !yield(object, nil) {
					return
				}
			}
		}
	}
}

func objectsFromPage(page *awss3.ListObjectsV2Output) []ObjectInfo {
	objects := make([]ObjectInfo, 0, len(page.Contents)+len(page.CommonPrefixes))
	for _, object := range page.Contents {
		objects = append(objects, ObjectInfo{
			Key:          aws.ToString(object.Key),
			Size:         aws.ToInt64(object.Size),
			LastModified: aws.ToTime(object.LastModified),
			ETag:         trimETag(aws.ToString(object.ETag)),
			StorageClass: string(object.StorageClass),
		})
	}
	for _, prefix := range page.CommonPrefixes {
		objects = append(objects, ObjectInfo{
			Key:      aws.ToString(prefix.Prefix),
			IsPrefix: true,
		})
	}
	slices.SortFunc(objects, func(left, right ObjectInfo) int {
		return strings.Compare(left.Key, right.Key)
	})
	return objects
}

// CopyObject copies an object within one S3 service without downloading it.
func (client *Client) CopyObject(
	ctx context.Context,
	sourceBucket string,
	sourceKey string,
	destinationBucket string,
	destinationKey string,
) (CopyResult, error) {
	if err := validateObjectAddress(sourceBucket, sourceKey); err != nil {
		return CopyResult{}, err
	}
	if err := validateObjectAddress(destinationBucket, destinationKey); err != nil {
		return CopyResult{}, err
	}

	result, err := client.api.CopyObject(ctx, &awss3.CopyObjectInput{
		Bucket:     new(destinationBucket),
		Key:        new(destinationKey),
		CopySource: new(url.PathEscape(sourceBucket + "/" + sourceKey)),
	})
	if err != nil {
		return CopyResult{}, wrapOperationError(
			"copy",
			destinationBucket,
			destinationKey,
			err,
		)
	}

	copyResult := CopyResult{VersionID: aws.ToString(result.VersionId)}
	if result.CopyObjectResult != nil {
		copyResult.ETag = trimETag(aws.ToString(result.CopyObjectResult.ETag))
		copyResult.LastModified = aws.ToTime(result.CopyObjectResult.LastModified)
	}
	return copyResult, nil
}

func validateBucket(bucket string) error {
	if strings.TrimSpace(bucket) == "" {
		return errors.New("s3: bucket is required")
	}
	return nil
}

func normalizeUploadOptions(key string, options *UploadOptions) (UploadOptions, error) {
	if options == nil {
		options = &UploadOptions{}
	}
	normalized := *options
	normalized.Metadata = cloneMetadata(options.Metadata)
	if normalized.ContentType == "" {
		normalized.ContentType = inferContentType(key)
	}
	switch normalized.ServerSideEncryption {
	case EncryptionNone, EncryptionAES256, EncryptionAWSKMS:
	default:
		return UploadOptions{}, fmt.Errorf(
			"s3: unsupported server-side encryption %q",
			normalized.ServerSideEncryption,
		)
	}
	if normalized.KMSKeyID != "" &&
		normalized.ServerSideEncryption != EncryptionAWSKMS {
		return UploadOptions{}, errors.New(
			"s3: KMS key ID requires aws:kms server-side encryption",
		)
	}
	return normalized, nil
}

func inferContentType(name string) string {
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
	if contentType == "" {
		return "application/octet-stream"
	}
	return contentType
}
