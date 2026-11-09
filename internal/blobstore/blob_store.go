package blobstore

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Auction-Application/be-auction-item/internal/apperror"
	"github.com/Auction-Application/be-auction-item/internal/database/filestore"
	uploadFile "github.com/Auction-Application/be-auction-item/rpc/gen/upload_file/v1"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/jackc/pgx/v5"
)

type BlobStore struct {
	uploadFile.UnimplementedFileUploadServiceServer
	dbStorageQuery *filestore.Queries
	s3Storage      *s3Storage
	conn           *pgx.Conn
}

func (blobStore *BlobStore) CompleteMultiPartUpload(ctx context.Context, completeMultipartRequest *uploadFile.CompleteMultiPartUploadRequest) (*uploadFile.CompleteMultiPartUploadResponse, error) {
	etagParts := convertToCompletedPart(completeMultipartRequest.PartEtags)
	blobID, err := blobStore.completeMultiPartUpload(ctx, *completeMultipartRequest.MultipartAttemptId, "arn:aws:s3:ap-south-1:433154991296:accesspoint/auction-lot-service-access-point", etagParts)
	if err != nil {
		return nil, err
	}

	return &uploadFile.CompleteMultiPartUploadResponse{
		BlobId: new(uint64(blobID)),
	}, nil
}

func convertToCompletedPart(wireFormatEtagParts []*uploadFile.PartEtag) []types.CompletedPart {
	etagParts := make([]types.CompletedPart, 0, len(wireFormatEtagParts))
	for _, etagPart := range wireFormatEtagParts {
		etagParts = append(etagParts, types.CompletedPart{
			ETag:       etagPart.Etag,
			PartNumber: new(int32(*etagPart.Part)),
		})
	}

	return etagParts
}

func (blobStore *BlobStore) GeneratePresignedUrl(ctx context.Context, presignRequest *uploadFile.GeneratePresignedUrlRequest) (*uploadFile.GeneratePresignedUrlResponse, error) {
	uploadfileRequestResult, err := blobStore.initiateUpload(ctx, convertToUploadFiles(presignRequest.Files), *presignRequest.UploadRoomId)
	if err != nil {
		return nil, err
	}

	dulicateFiles, err := convertToFile(uploadfileRequestResult.duplicateFiles)
	if err != nil {
		return nil, err
	}
	alreadyUploadedFiles, err := convertToFile(uploadfileRequestResult.alreadyUploadedFiles)
	if err != nil {
		return nil, err
	}

	presignedFiles, err := convertToPresignedFiles(uploadfileRequestResult.presignedFileUrls)
	if err != nil {
		return nil, err
	}

	response := &uploadFile.GeneratePresignedUrlResponse{
		PresignResult: &uploadFile.PresignFileResult{
			DuplicateFiles:       dulicateFiles,
			AlreadyUploadedFiles: alreadyUploadedFiles,
			PresignedFiles:       presignedFiles,
		},
	}

	return response, nil
}

func convertToPresignedFiles(presignedUrls []presignedFileUrl) ([]*uploadFile.PresignedFile, error) {
	presignedFileUrls := make([]*uploadFile.PresignedFile, 0, len(presignedUrls))
	for _, v := range presignedUrls {
		fileUrl, err := convertToPresignedFile(v)
		if err != nil {
			return nil, err
		}
		presignedFileUrls = append(presignedFileUrls, fileUrl)
	}

	return presignedFileUrls, nil
}

func convertToPresignedFile(presignedUrl presignedFileUrl) (*uploadFile.PresignedFile, error) {
	file, err := convertToInternalFile(presignedUrl.fileUploadData)
	if err != nil {
		return nil, err
	}
	presignedUploadUrl := presignedUrl.presignedUploadUrl

	var presignedFileUploadUrl *uploadFile.PresignedUploadUrl

	if presignedUploadUrl.Single.request != nil {
		singleUpload := &uploadFile.PresignedUploadUrl_Single{
			Single: &uploadFile.SinglePresignedUploadUrl{
				Request: convertToSinglePresignedHttpRequest(presignedUploadUrl.Single.request),
			},
		}
		presignedFileUploadUrl = &uploadFile.PresignedUploadUrl{
			PresignedUrl: singleUpload,
		}

	} else {
		multiUpload := &uploadFile.PresignedUploadUrl_Multi{
			Multi: &uploadFile.MultiPresignedUploadUrl{
				MultiParts:         convertToMultiPartPresignedHttpRequest(presignedUploadUrl.Multi),
				MultipartAttemptId: &presignedUploadUrl.Multi.multipartAttemptId,
				PartSize:           &presignedUrl.Multi.partSize,
				UploadedEtags:      convertToUploadedEtags(presignedUploadUrl.Multi.fileEtags),
			},
		}
		presignedFileUploadUrl = &uploadFile.PresignedUploadUrl{
			PresignedUrl: multiUpload,
		}

	}
	return &uploadFile.PresignedFile{
		UploadFile:         file,
		PresignedUploadUrl: presignedFileUploadUrl,
	}, nil
}

func convertToUploadedEtags(fileTags []fileEtag) []*uploadFile.PartEtag {
	uploadedEtags := make([]*uploadFile.PartEtag, 0, len(fileTags))
	for _, tag := range fileTags {
		uploadedEtags = append(uploadedEtags, &uploadFile.PartEtag{
			Etag: &tag.etag,
			Part: new(uint32(tag.partNumber)),
		})
	}
	return uploadedEtags
}

func convertToMultiPresignedHttpRequest(multiPresignedRequest multiPresignedRequest) *uploadFile.MultiPresignedHTTPRequest {
	signedHeader := make(map[string]*uploadFile.HeaderValues, len(multiPresignedRequest.request.SignedHeader))
	for key, values := range multiPresignedRequest.request.SignedHeader {
		signedHeader[key] = &uploadFile.HeaderValues{
			Values: values,
		}
	}

	presignRequest := &uploadFile.MultiPresignedHTTPRequest{
		Request: &uploadFile.Request{
			Url:          new(multiPresignedRequest.request.URL),
			Method:       new(multiPresignedRequest.request.Method),
			SignedHeader: signedHeader,
		},
		Part: new(int32(multiPresignedRequest.part)),
	}
	return presignRequest
}

func convertToSinglePresignedHttpRequest(presignedRequest *v4.PresignedHTTPRequest) *uploadFile.Request {
	signedHeader := make(map[string]*uploadFile.HeaderValues, len(presignedRequest.SignedHeader))
	for key, values := range presignedRequest.SignedHeader {
		signedHeader[key] = &uploadFile.HeaderValues{
			Values: values,
		}
	}

	presignRequest := &uploadFile.Request{
		Url:          new(presignedRequest.URL),
		Method:       new(presignedRequest.Method),
		SignedHeader: signedHeader,
	}

	return presignRequest
}

func convertToMultiPartPresignedHttpRequest(multiPresignedUrl multiPresignedUrl) []*uploadFile.MultiPresignedHTTPRequest {
	presignedRequests := make([]*uploadFile.MultiPresignedHTTPRequest, 0, len(multiPresignedUrl.requests))
	for _, request := range multiPresignedUrl.requests {
		presignedRequests = append(presignedRequests, convertToMultiPresignedHttpRequest(request))
	}

	return presignedRequests
}

func convertToUploadFiles(files []*uploadFile.File) []fileUploadData {
	uploadFiles := make([]fileUploadData, 0, len(files))
	for _, file := range files {
		uploadFiles = append(uploadFiles, convertToUploadFile(file))
	}

	return uploadFiles
}

func convertToUploadFile(file *uploadFile.File) fileUploadData {
	uploadFile := fileUploadData{
		Sha256:       *file.Sha256,
		ClientFileId: strconv.Itoa(int(*file.ClientFileId)),
		FileSize:     uint(*file.FileSize),
	}
	return uploadFile
}

func convertToFile(files []fileUploadData) ([]*uploadFile.File, error) {
	internalFiles := make([]*uploadFile.File, 0, len(files))
	for _, file := range files {
		iFile, err := convertToInternalFile(file)
		if err != nil {
			return nil, err
		}
		internalFiles = append(internalFiles, iFile)

	}

	return internalFiles, nil
}

func convertToInternalFile(file fileUploadData) (*uploadFile.File, error) {
	const operation apperror.Operation = "blobstore.convertToInternalFile"
	clientId, err := strconv.Atoi(file.ClientFileId)
	if err != nil {
		return nil, apperror.InternalServerError(operation, fmt.Errorf("cannot convert client file id %s to integer:%w", file.ClientFileId, err), "internal error").MarkAsClientErr()
	}
	internalFile := &uploadFile.File{
		Sha256: new(file.Sha256),
		// FileName:     new(file.FileName),
		ClientFileId: new(int32(clientId)),
		FileSize:     new(uint64(file.FileSize)),
	}
	return internalFile, nil
}

const bucketName = "arn:aws:s3:ap-south-1:433154991296:accesspoint/auction-lot-service-access-point"

func NewBlobStore(databaseConnection *pgx.Conn) *BlobStore {
	s3Storage, err := newS3Storage(bucketName)
	if err != nil {
		fmt.Println("Error creating s3Storage")
		fmt.Println(err)
		return nil
	}
	return &BlobStore{
		dbStorageQuery: filestore.New(databaseConnection),
		s3Storage:      s3Storage,
		conn:           databaseConnection,
	}
}
