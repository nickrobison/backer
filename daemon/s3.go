package daemon

import (
	"io"
	"os"
	"path"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// S3Uploader struct which contains configuration settings for managing S3 Buckets
type S3Uploader struct {
	session *session.Session
	client  *s3.S3
	config  *s3Options
}

// NewS3Uploader creates a new S3 manager to sync objects
func NewS3Uploader(options *s3Options) *S3Uploader {
	logger.Println("Creating new S3 Client")
	sess := session.New(&aws.Config{
		Region:      aws.String(options.Region),
		Credentials: credentials.NewStaticCredentials(options.Credentials.AccessKeyID, options.Credentials.SecretAccessKey, options.Credentials.SessionToken),
	})

	s3Uploader := &S3Uploader{
		session: sess,
		client:  s3.New(sess),
		config:  options,
	}

	// Create the bucket
	s3Uploader.createBucket()

	return s3Uploader
}

func (s *S3Uploader) createBucket() {
	logger.Println("Creating bucket:", s.config.Bucket)
	_, err := s.client.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(s.config.Bucket),
	})

	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			switch awsErr.Code() {
			case "BucketAlreadyExists":
				logger.Panicf("Bucket %s already exists in the S3 system\nBuckets must have unique names.", s.config.Bucket)
			case "BucketAlreadyOwnedByYou":
				logger.Printf("Bucket %s already exists", s.config.Bucket)
			default:
				logger.Panicln(awsErr)
			}
		}
	}

	// Enable Versioning of S3 Bucket, if enabled in config
	if s.config.Versioning {
		s.client.PutBucketVersioning(&s3.PutBucketVersioningInput{
			Bucket: aws.String(s.config.Bucket),
			VersioningConfiguration: &s3.VersioningConfiguration{
				Status: aws.String(s3.BucketVersioningStatusEnabled),
			},
		})
	}
}

// func (s *S3Uploader) GetObject(name string) error {
// 	resp, err := s.client.GetObject(&s3.GetObjectInput{
// 		Bucket: aws.String(s.config.Bucket),
// 		Key:    aws.String(name),
// 	})
// 	if err != nil {
// 		return err
// 	}
// 	defer resp.Body.Close()
// 	return nil
// }

func (s *S3Uploader) uploadObject(name string, remoteRoot string, object io.Reader, checksum string) error {
	// Setup the metadata
	logger.Println("Metadata:", checksum)
	metadata := make(map[string]*string)
	metadata["checksum"] = aws.String(checksum)

	// Set the redundency
	var storage string
	if s.config.ReducedRedundancy {
		storage = "REDUCED_REDUNDANCY"
	} else {
		storage = "STANDARD"
	}

	uploader := s3manager.NewUploader(s.session)
	objectKey := s.buildObjectKey(name, remoteRoot)
	logger.Println("Uploading:", objectKey)
	result, err := uploader.Upload(&s3manager.UploadInput{
		Body:         object,
		Bucket:       aws.String(s.config.Bucket),
		Key:          aws.String(objectKey),
		Metadata:     metadata,
		StorageClass: aws.String(storage),
	})
	if err != nil {
		return err
	}
	logger.Printf("Uploaded %s to %s", name, result.Location)
	return nil
}

func (s *S3Uploader) getObjectDetails(key string) *s3.HeadObjectOutput {
	resp, err := s.client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			switch awsErr.Code() {
			case "NoSuchKey":
				return nil
			default:
				logger.Panicln(awsErr)
			}
		}
	}
	return resp
}

func (s *S3Uploader) deleteObject(key string) {
	_, err := s.client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(s.config.Bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		logger.Println(err)
	}
}

func (s *S3Uploader) deleteVersionedObject(key string) {
	logger.Println("Removing versioned object:", key)
	// Get the first page of versions
	var versions []*string
	isTruncated := true
	var nextVersion *string
	for isTruncated {
		resp, err := s.client.ListObjectVersions(&s3.ListObjectVersionsInput{
			Bucket:          aws.String(s.config.Bucket),
			Prefix:          aws.String(key),
			VersionIdMarker: nextVersion,
		})
		if err != nil {
			logger.Println(err)
		}
		logger.Println(resp)
		for _, version := range resp.Versions {
			if *version.Key == key {
				versions = append(versions, version.VersionId)
			}
		}
		isTruncated = *resp.IsTruncated
		nextVersion = resp.NextVersionIdMarker
	}

	// Delete all the versions
	deleteObjects := []*s3.ObjectIdentifier{}
	for _, version := range versions {
		deleteObjects = append(deleteObjects, &s3.ObjectIdentifier{
			Key:       aws.String(key),
			VersionId: version,
		})
	}
	logger.Println(deleteObjects)
	if len(deleteObjects) > 0 {
		deleteRequest := &s3.DeleteObjectsInput{
			Bucket: aws.String(s.config.Bucket),
			Delete: &s3.Delete{
				Objects: deleteObjects,
			},
		}
		// Now, delete them
		_, err := s.client.DeleteObjects(deleteRequest)
		if err != nil {
			logger.Println(err)
		}
	}
}

// UploadFile to S3 Bucket
func (s *S3Uploader) UploadFile(name string, remoteRoot string) {

	// Check if the file exists and if it matches what I need
	// objectHead := s.getObjectDetails(s.buildObjectKey(name, remoteRoot))
	// if objectHead != nil {
	// 	logger.Println("Object exists")
	// 	logger.Println(objectHead.Metadata)
	// }

	// Should I lock this file?
	file, err := os.Open(name)
	if err != nil {
		logger.Println(err)
	}
	defer file.Close()

	checksumChannel := make(chan string)
	go func() {
		checksumChannel <- generateSHA256Hash(file)
	}()

	err = s.uploadObject(name, remoteRoot, file, <-checksumChannel)
	if err != nil {
		logger.Println(err)
	}
}

// DeleteFile from S3 Bucket
func (s *S3Uploader) DeleteFile(name string, remoteRoot string) {
	if s.config.Versioning {
		s.deleteVersionedObject(s.buildObjectKey(name, remoteRoot))
	} else {
		s.deleteObject(s.buildObjectKey(name, remoteRoot))
	}
}

func (s *S3Uploader) buildObjectKey(file string, watcherPath string) string {
	// if !s.config.Versioning {
	// 	// Build versioned string
	// 	versionedFileName := path.Base(file) + time.Now().String()
	// 	return path.Join(s.config.BucketRoot, watcherPath, versionedFileName)
	// }
	return path.Join(s.config.BucketRoot, watcherPath, path.Base(file))
}
