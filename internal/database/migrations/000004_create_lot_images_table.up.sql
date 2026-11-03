create table lot_images(
    lot_image_id bigint generated always as identity primary key,
    upload_room_id bigint not null references file_store.upload_rooms(id) on delete cascade,
    image_blob_id bigint not null references image_blobs(image_blob_id) on delete restrict,
    filename varchar(250) not null
)