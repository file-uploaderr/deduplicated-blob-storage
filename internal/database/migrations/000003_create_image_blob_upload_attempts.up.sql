create type  upload_config.upload_attempt_state as enum('pending','committed');

create type upload_config.upload_type as enum('multiUpload','singleUpload');

create table if not exists upload_config.image_blob_upload_attempts(id bigint generated always as identity primary key,
sha256 bytea not null,
file_size bigint not null,
upload_type upload_config.upload_type not null,

upload_id text,
part_size bigint,
part_count bigint generated always as (case
	when upload_type = 'multiUpload' then 
	(file_size + part_size-1)/ part_size
	else null
end) stored,

upload_room_id bigint not null references file_store.upload_rooms(id) on
delete cascade,
	storage_key text not null,
	uploader_identity varchar(128) not null,
upload_state upload_config.upload_attempt_state not null default 'pending',
version integer not null default 0
);
