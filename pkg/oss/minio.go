// Package algo -----------------------------
// @file      : oss.go
// @author    : xiangtao
// @contact   : xiangtao1994@gmail.com
// @time      : 2025/6/9 11:39
// Description: MinIO对象存储客户端封装
// -------------------------------------------
package oss

import (
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/xiangtao94/golib/pkg/errors"
	"github.com/xiangtao94/golib/pkg/zlog"
)

type MinioConf struct {
	AK       string `yaml:"ak"`
	SK       string `yaml:"sk"`
	Endpoint string `yaml:"endpoint"`
	UseSSL   bool   `yaml:"useSSL"` // 是否使用SSL
	Region   string `yaml:"region"` // 区域
}

// MinioClient MinIO客户端封装
type MinioClient struct {
	client *minio.Client
	config MinioConf
}

// UploadOptions 上传选项
type UploadOptions struct {
	ContentType string            // 文件类型
	UserMeta    map[string]string // 用户元数据
	ServerSide  bool              // 服务端加密
}

// DownloadInfo 下载信息
type DownloadInfo struct {
	ObjectName   string
	Size         int64
	LastModified time.Time
	ContentType  string
	ETag         string
}

// NewMClientByAK 通过AK/SK创建MinIO客户端
func NewMClientByAK(endpoint string, accessKey, secretKey string) (*minio.Client, error) {
	endpointUrl, err := url.Parse(endpoint)
	if err != nil {
		return nil, errors.ErrorSystemError
	}
	minioClient, err := minio.New(endpointUrl.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, err
	}
	return minioClient, nil
}

// NewMinioClient 创建MinIO客户端封装
func NewMinioClient(config MinioConf) (*MinioClient, error) {
	endpointUrl, err := url.Parse(config.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint: %w", err)
	}

	minioClient, err := minio.New(endpointUrl.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AK, config.SK, ""),
		Secure: config.UseSSL,
		Region: config.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	return &MinioClient{
		client: minioClient,
		config: config,
	}, nil
}

// CreateBucket 创建存储桶
func (mc *MinioClient) CreateBucket(ctx *gin.Context, bucketName string, location string) error {
	start := time.Now()

	exists, err := mc.client.BucketExists(ctx, bucketName)
	if err != nil {
		zlog.Errorf(ctx, "failed to check bucket exists: %v", err)
		return fmt.Errorf("failed to check bucket exists: %w", err)
	}

	if exists {
		zlog.Infof(ctx, "bucket %s already exists", bucketName)
		return nil
	}

	if location == "" {
		location = mc.config.Region
	}

	err = mc.client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{Region: location})
	if err != nil {
		zlog.Errorf(ctx, "failed to create bucket %s: %v", bucketName, err)
		return fmt.Errorf("failed to create bucket: %w", err)
	}

	zlog.Infof(ctx, "bucket %s created successfully, cost: %v", bucketName, time.Since(start))
	return nil
}

// UploadFile 上传文件
func (mc *MinioClient) UploadFile(ctx *gin.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts *UploadOptions) (minio.UploadInfo, error) {
	start := time.Now()

	if opts == nil {
		opts = &UploadOptions{}
	}

	// 设置默认Content-Type
	if opts.ContentType == "" {
		opts.ContentType = getContentType(objectName)
	}

	putOptions := minio.PutObjectOptions{
		ContentType:  opts.ContentType,
		UserMetadata: opts.UserMeta,
	}

	uploadInfo, err := mc.client.PutObject(ctx, bucketName, objectName, reader, objectSize, putOptions)
	if err != nil {
		zlog.Errorf(ctx, "failed to upload file %s/%s: %v", bucketName, objectName, err)
		return minio.UploadInfo{}, fmt.Errorf("failed to upload file: %w", err)
	}

	zlog.Infof(ctx, "file uploaded successfully: %s/%s, size: %d, etag: %s, cost: %v",
		bucketName, objectName, uploadInfo.Size, uploadInfo.ETag, time.Since(start))

	return uploadInfo, nil
}

// UploadFileFromPath 从本地路径上传文件
func (mc *MinioClient) UploadFileFromPath(ctx *gin.Context, bucketName, objectName, filePath string, opts *UploadOptions) (minio.UploadInfo, error) {
	start := time.Now()

	if opts == nil {
		opts = &UploadOptions{}
	}

	// 设置默认Content-Type
	if opts.ContentType == "" {
		opts.ContentType = getContentType(filePath)
	}

	putOptions := minio.PutObjectOptions{
		ContentType:  opts.ContentType,
		UserMetadata: opts.UserMeta,
	}

	uploadInfo, err := mc.client.FPutObject(ctx, bucketName, objectName, filePath, putOptions)
	if err != nil {
		zlog.Errorf(ctx, "failed to upload file from path %s to %s/%s: %v", filePath, bucketName, objectName, err)
		return minio.UploadInfo{}, fmt.Errorf("failed to upload file from path: %w", err)
	}

	zlog.Infof(ctx, "file uploaded successfully from path: %s -> %s/%s, size: %d, etag: %s, cost: %v",
		filePath, bucketName, objectName, uploadInfo.Size, uploadInfo.ETag, time.Since(start))

	return uploadInfo, nil
}

// DownloadFile 下载文件
func (mc *MinioClient) DownloadFile(ctx *gin.Context, bucketName, objectName string) (io.ReadCloser, *DownloadInfo, error) {
	start := time.Now()

	// 获取对象信息
	objInfo, err := mc.client.StatObject(ctx, bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		zlog.Errorf(ctx, "failed to get object info %s/%s: %v", bucketName, objectName, err)
		return nil, nil, fmt.Errorf("failed to get object info: %w", err)
	}

	// 获取对象
	reader, err := mc.client.GetObject(ctx, bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		zlog.Errorf(ctx, "failed to download file %s/%s: %v", bucketName, objectName, err)
		return nil, nil, fmt.Errorf("failed to download file: %w", err)
	}

	downloadInfo := &DownloadInfo{
		ObjectName:   objectName,
		Size:         objInfo.Size,
		LastModified: objInfo.LastModified,
		ContentType:  objInfo.ContentType,
		ETag:         objInfo.ETag,
	}

	zlog.Infof(ctx, "file download started: %s/%s, size: %d, cost: %v",
		bucketName, objectName, objInfo.Size, time.Since(start))

	return reader, downloadInfo, nil
}

// DownloadFileToPath 下载文件到本地路径
func (mc *MinioClient) DownloadFileToPath(ctx *gin.Context, bucketName, objectName, filePath string) error {
	start := time.Now()

	err := mc.client.FGetObject(ctx, bucketName, objectName, filePath, minio.GetObjectOptions{})
	if err != nil {
		zlog.Errorf(ctx, "failed to download file %s/%s to path %s: %v", bucketName, objectName, filePath, err)
		return fmt.Errorf("failed to download file to path: %w", err)
	}

	zlog.Infof(ctx, "file downloaded successfully: %s/%s -> %s, cost: %v",
		bucketName, objectName, filePath, time.Since(start))

	return nil
}

// GetPresignedURL 获取预签名URL
func (mc *MinioClient) GetPresignedURL(ctx *gin.Context, bucketName, objectName string, expiry time.Duration, method string) (string, error) {
	start := time.Now()

	if expiry <= 0 {
		expiry = 24 * time.Hour // 默认24小时
	}

	var presignedURL *url.URL
	var err error

	switch strings.ToUpper(method) {
	case "GET":
		presignedURL, err = mc.client.PresignedGetObject(ctx, bucketName, objectName, expiry, nil)
	case "PUT":
		presignedURL, err = mc.client.PresignedPutObject(ctx, bucketName, objectName, expiry)
	default:
		presignedURL, err = mc.client.PresignedGetObject(ctx, bucketName, objectName, expiry, nil)
	}

	if err != nil {
		zlog.Errorf(ctx, "failed to get presigned URL for %s/%s: %v", bucketName, objectName, err)
		return "", fmt.Errorf("failed to get presigned URL: %w", err)
	}

	zlog.Infof(ctx, "presigned URL generated: %s/%s, method: %s, expiry: %v, cost: %v",
		bucketName, objectName, method, expiry, time.Since(start))

	return presignedURL.String(), nil
}

// GetDownloadURL 获取下载URL（GET方法的预签名URL）
func (mc *MinioClient) GetDownloadURL(ctx *gin.Context, bucketName, objectName string, expiry time.Duration) (string, error) {
	return mc.GetPresignedURL(ctx, bucketName, objectName, expiry, "GET")
}

// GetUploadURL 获取上传URL（PUT方法的预签名URL）
func (mc *MinioClient) GetUploadURL(ctx *gin.Context, bucketName, objectName string, expiry time.Duration) (string, error) {
	return mc.GetPresignedURL(ctx, bucketName, objectName, expiry, "PUT")
}

// DeleteFile 删除文件
func (mc *MinioClient) DeleteFile(ctx *gin.Context, bucketName, objectName string) error {
	start := time.Now()

	err := mc.client.RemoveObject(ctx, bucketName, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		zlog.Errorf(ctx, "failed to delete file %s/%s: %v", bucketName, objectName, err)
		return fmt.Errorf("failed to delete file: %w", err)
	}

	zlog.Infof(ctx, "file deleted successfully: %s/%s, cost: %v",
		bucketName, objectName, time.Since(start))

	return nil
}

// ListObjects 列出对象
func (mc *MinioClient) ListObjects(ctx *gin.Context, bucketName, prefix string, recursive bool) ([]minio.ObjectInfo, error) {
	start := time.Now()

	var objects []minio.ObjectInfo
	objectCh := mc.client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: recursive,
	})

	for object := range objectCh {
		if object.Err != nil {
			zlog.Errorf(ctx, "error listing objects in bucket %s: %v", bucketName, object.Err)
			return nil, fmt.Errorf("error listing objects: %w", object.Err)
		}
		objects = append(objects, object)
	}

	zlog.Infof(ctx, "listed %d objects in bucket %s with prefix %s, cost: %v",
		len(objects), bucketName, prefix, time.Since(start))

	return objects, nil
}

// ObjectExists 检查对象是否存在
func (mc *MinioClient) ObjectExists(ctx *gin.Context, bucketName, objectName string) (bool, error) {
	start := time.Now()

	_, err := mc.client.StatObject(ctx, bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			zlog.Infof(ctx, "object %s/%s does not exist, cost: %v", bucketName, objectName, time.Since(start))
			return false, nil
		}
		zlog.Errorf(ctx, "failed to check object existence %s/%s: %v", bucketName, objectName, err)
		return false, fmt.Errorf("failed to check object existence: %w", err)
	}

	zlog.Infof(ctx, "object %s/%s exists, cost: %v", bucketName, objectName, time.Since(start))
	return true, nil
}

// GetObjectInfo 获取对象信息
func (mc *MinioClient) GetObjectInfo(ctx *gin.Context, bucketName, objectName string) (*DownloadInfo, error) {
	start := time.Now()

	objInfo, err := mc.client.StatObject(ctx, bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		zlog.Errorf(ctx, "failed to get object info %s/%s: %v", bucketName, objectName, err)
		return nil, fmt.Errorf("failed to get object info: %w", err)
	}

	downloadInfo := &DownloadInfo{
		ObjectName:   objectName,
		Size:         objInfo.Size,
		LastModified: objInfo.LastModified,
		ContentType:  objInfo.ContentType,
		ETag:         objInfo.ETag,
	}

	zlog.Infof(ctx, "got object info: %s/%s, size: %d, cost: %v",
		bucketName, objectName, objInfo.Size, time.Since(start))

	return downloadInfo, nil
}

// CopyObject 复制对象
func (mc *MinioClient) CopyObject(ctx *gin.Context, srcBucket, srcObject, destBucket, destObject string) error {
	start := time.Now()

	srcOpts := minio.CopySrcOptions{
		Bucket: srcBucket,
		Object: srcObject,
	}

	dstOpts := minio.CopyDestOptions{
		Bucket: destBucket,
		Object: destObject,
	}

	_, err := mc.client.CopyObject(ctx, dstOpts, srcOpts)
	if err != nil {
		zlog.Errorf(ctx, "failed to copy object %s/%s to %s/%s: %v", srcBucket, srcObject, destBucket, destObject, err)
		return fmt.Errorf("failed to copy object: %w", err)
	}

	zlog.Infof(ctx, "object copied successfully: %s/%s -> %s/%s, cost: %v",
		srcBucket, srcObject, destBucket, destObject, time.Since(start))

	return nil
}

// Close 关闭客户端连接
func (mc *MinioClient) Close() {
	// MinIO Go客户端不需要显式关闭连接
	// 这里主要是为了保持接口一致性
}

// getContentType 根据文件扩展名获取Content-Type
func getContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain"
	case ".html":
		return "text/html"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".zip":
		return "application/zip"
	case ".mp4":
		return "video/mp4"
	case ".mp3":
		return "audio/mpeg"
	default:
		return "application/octet-stream"
	}
}
