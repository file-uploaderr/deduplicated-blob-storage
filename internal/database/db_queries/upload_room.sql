-- name: CreateUploadRoom :one
insert into file_store.upload_rooms(title,description) values($1,$2) returning id;