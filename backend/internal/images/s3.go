package images

import (
	"context"
	"errors"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// S3 guarda as imagens em um bucket privado. A aplicação faz o proxy das imagens em /images/...,
// então o bucket não precisa ser público. As credenciais vêm da cadeia padrão da AWS
// (role da instância EC2, ou AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY/AWS_SESSION_TOKEN).
type S3 struct {
	client *s3.Client
	bucket string
	prefix string
}

func NewS3(ctx context.Context, region, bucket, prefix string) (*S3, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, err
	}
	return &S3{client: s3.NewFromConfig(cfg), bucket: bucket, prefix: prefix}, nil
}

func (s *S3) objectKey(key string) *string { return aws.String(s.prefix + key) }

func (s *S3) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	if !ValidKey(key) {
		return errors.New("chave inválida")
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           s.objectKey(key),
		Body:          r,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
		CacheControl:  aws.String("public, max-age=86400"),
	})
	return err
}

func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if !ValidKey(key) {
		return nil, ErrNotFound
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: s.objectKey(key)})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return out.Body, nil
}

func (s *S3) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: s.objectKey(key)})
	if err == nil {
		return true, nil
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NotFound" || apiErr.ErrorCode() == "NoSuchKey") {
		return false, nil
	}
	return false, err
}

func (s *S3) Delete(ctx context.Context, key string) error {
	if !ValidKey(key) {
		return errors.New("chave inválida")
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: s.objectKey(key)})
	return err
}

// Ping confirma que o bucket existe e que a aplicação tem permissão para acessá-lo.
func (s *S3) Ping(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	return err
}
