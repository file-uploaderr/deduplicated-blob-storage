package server

import (
	"context"
	"fmt"

	"github.com/Auction-Application/be-auction-item/internal/apperror"
	"github.com/Auction-Application/be-auction-item/internal/database/filestore"
	uploadRoom "github.com/Auction-Application/be-auction-item/rpc/gen/upload_room/v1"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/jackc/pgx/v5"
)

type LotServer struct {
	uploadRoom.UnimplementedUploadRoomServiceServer
	dbStorageQuery *filestore.Queries
	conn           *pgx.Conn
}

func (s *LotServer) CreateRoom(ctx context.Context, req *uploadRoom.CreateRoomRequest) (*uploadRoom.CreateRoomResponse, error) {
	const operation apperror.Operation = "server.CreateRoom"
	id, err := s.dbStorageQuery.CreateUploadRoom(ctx, filestore.CreateUploadRoomParams{Title: *req.Room.Title, Description: req.Room.Description})
	if err != nil {
		return nil, apperror.InternalServerError(operation, fmt.Errorf("creation of upload room failed,%w", err), "internal error").MarkAsClientErr()
	}

	return &uploadRoom.CreateRoomResponse{UploadRoomId: aws.Uint64(uint64(id))}, nil
}

func NewLotServer(databaseConnection *pgx.Conn) *LotServer {
	return &LotServer{
		dbStorageQuery: filestore.New(databaseConnection),
		conn:           databaseConnection,
	}
}
