package imagestore

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

type ImageStore struct {
	uploadFile.UnimplementedFileUploadServiceServer
	dbStorageQuery *filestore.Queries
	s3Storage      *s3Storage
	conn           *pgx.Conn
}

func (imageStore *ImageStore) CompleteMultiPartUpload(ctx context.Context, completeMultipartRequest *uploadFile.CompleteMultiPartUploadRequest) (*uploadFile.CompleteMultiPartUploadResponse, error) {
	etagParts := convertToCompletedPart(completeMultipartRequest.PartEtags)
	blobID, err := imageStore.completeMultiPartUpload(ctx, *completeMultipartRequest.MultipartAttemptId, "arn:aws:s3:ap-south-1:433154991296:accesspoint/auction-lot-service-access-point", etagParts)
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

func (imageStore *ImageStore) GeneratePresignedUrl(ctx context.Context, presignRequest *uploadFile.GeneratePresignedUrlRequest) (*uploadFile.GeneratePresignedUrlResponse, error) {
	uploadfileRequestResult, err := imageStore.initiateUpload(ctx, convertToUploadFiles(presignRequest.Files), *presignRequest.UploadRoomId)
	if err != nil {
		return nil, err
	}

	dulicateImageFiles, err := convertToImageFiles(uploadfileRequestResult.duplicateFiles)
	if err != nil {
		return nil, err
	}
	alreadyUploadedImageFiles, err := convertToImageFiles(uploadfileRequestResult.alreadyUploadedFiles)
	if err != nil {
		return nil, err
	}

	presignedImageFiles, err := convertToPresignedImageFiles(uploadfileRequestResult.presignedFileUrls)
	if err != nil {
		return nil, err
	}

	response := &uploadFile.GeneratePresignedUrlResponse{
		PresignResult: &uploadFile.PresignFileResult{
			DuplicateFiles:       dulicateImageFiles,
			AlreadyUploadedFiles: alreadyUploadedImageFiles,
			PresignedFiles:       presignedImageFiles,
		},
	}

	return response, nil
}

func convertToPresignedImageFiles(presignedUrls []presignedFileUrl) ([]*uploadFile.PresignedFile, error) {
	presignedImageFileUrls := make([]*uploadFile.PresignedFile, 0, len(presignedUrls))
	for _, v := range presignedUrls {
		imageUrl, err := convertToPresignedFile(v)
		if err != nil {
			return nil, err
		}
		presignedImageFileUrls = append(presignedImageFileUrls, imageUrl)
	}

	return presignedImageFileUrls, nil
}

func convertToPresignedFile(presignedUrl presignedFileUrl) (*uploadFile.PresignedFile, error) {
	imageFile, err := convertToImageFile(presignedUrl.fileUploadData)
	if err != nil {
		return nil, err
	}
	presignedUploadUrl := presignedUrl.presignedUploadUrl

	var presignedImageUploadUrl *uploadFile.PresignedUploadUrl

	if presignedUploadUrl.Single.request != nil {
		singleUpload := &uploadFile.PresignedUploadUrl_Single{
			Single: &uploadFile.SinglePresignedUploadUrl{
				Request: convertToLotImageSinglePresignedHttpRequest(presignedUploadUrl.Single.request),
			},
		}
		presignedImageUploadUrl = &uploadFile.PresignedUploadUrl{
			PresignedUrl: singleUpload,
		}

	} else {
		multiUpload := &uploadFile.PresignedUploadUrl_Multi{
			Multi: &uploadFile.MultiPresignedUploadUrl{
				MultiParts:         convertToLotImageMultiPartPresignedHttpRequest(presignedUploadUrl.Multi),
				MultipartAttemptId: &presignedUploadUrl.Multi.multipartAttemptId,
				PartSize:           &presignedUrl.Multi.partSize,
				UploadedEtags:      convertToUploadedEtags(presignedUploadUrl.Multi.imageEtags),
			},
		}
		presignedImageUploadUrl = &uploadFile.PresignedUploadUrl{
			PresignedUrl: multiUpload,
		}

	}
	return &uploadFile.PresignedFile{
		UploadFile:         imageFile,
		PresignedUploadUrl: presignedImageUploadUrl,
	}, nil
}

func convertToUploadedEtags(imageTags []imageEtag) []*uploadFile.PartEtag {
	uploadedEtags := make([]*uploadFile.PartEtag, 0, len(imageTags))
	for _, tag := range imageTags {
		uploadedEtags = append(uploadedEtags, &uploadFile.PartEtag{
			Etag: &tag.etag,
			Part: new(uint32(tag.partNumber)),
		})
	}
	return uploadedEtags
}

func convertToLotImageMultiPresignedHttpRequest(multiPresignedRequest multiPresignedRequest) *uploadFile.MultiPresignedHTTPRequest {
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

func convertToLotImageSinglePresignedHttpRequest(presignedRequest *v4.PresignedHTTPRequest) *uploadFile.Request {
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

func convertToLotImageMultiPartPresignedHttpRequest(multiPresignedUrl multiPresignedUrl) []*uploadFile.MultiPresignedHTTPRequest {
	lotImagePresignedRequests := make([]*uploadFile.MultiPresignedHTTPRequest, 0, len(multiPresignedUrl.requests))
	for _, request := range multiPresignedUrl.requests {
		lotImagePresignedRequests = append(lotImagePresignedRequests, convertToLotImageMultiPresignedHttpRequest(request))
	}

	return lotImagePresignedRequests
}

func convertToUploadFiles(imageFiles []*uploadFile.File) []fileUploadData {
	uploadFiles := make([]fileUploadData, 0, len(imageFiles))
	for _, image := range imageFiles {
		uploadFiles = append(uploadFiles, convertToUploadFile(image))
	}

	return uploadFiles
}

func convertToUploadFile(imageFile *uploadFile.File) fileUploadData {
	uploadFile := fileUploadData{
		Sha256: *imageFile.Sha256,
		// FileName:     *imageFile.FileName,
		ClientFileId: strconv.Itoa(int(*imageFile.ClientFileId)),
		FileSize:     uint(*imageFile.FileSize),
	}
	return uploadFile
}

func convertToImageFiles(files []fileUploadData) ([]*uploadFile.File, error) {
	imageFiles := make([]*uploadFile.File, 0, len(files))
	for _, file := range files {
		iFile, err := convertToImageFile(file)
		if err != nil {
			return nil, err
		}
		imageFiles = append(imageFiles, iFile)

	}

	return imageFiles, nil
}

func convertToImageFile(file fileUploadData) (*uploadFile.File, error) {
	const operation apperror.Operation = "imagestore.convertToImageFile"
	clientId, err := strconv.Atoi(file.ClientFileId)
	if err != nil {
		return nil, apperror.InternalServerError(operation, fmt.Errorf("cannot convert client file id %s to integer:%w", file.ClientFileId, err), "internal error").MarkAsClientErr()
	}
	imageFile := &uploadFile.File{
		Sha256: new(file.Sha256),
		// FileName:     new(file.FileName),
		ClientFileId: new(int32(clientId)),
		FileSize:     new(uint64(file.FileSize)),
	}
	return imageFile, nil
}

const bucketName = "arn:aws:s3:ap-south-1:433154991296:accesspoint/auction-lot-service-access-point"

func NewImageStore(databaseConnection *pgx.Conn) *ImageStore {
	s3Storage, err := newS3Storage(bucketName)
	if err != nil {
		fmt.Println("Error creating s3Storage")
		fmt.Println(err)
		return nil
	}
	return &ImageStore{
		dbStorageQuery: filestore.New(databaseConnection),
		s3Storage:      s3Storage,
		conn:           databaseConnection,
	}
}
