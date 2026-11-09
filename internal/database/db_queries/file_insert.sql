
-- name: RetrieveIdenticalBlobs :many
select encode(sha256,'hex') sha256 from upload_config.image_blob_upload_attempts where sha256 = any(select decode(h.sha256,'hex') from unnest(
@sha256s::char(64)[]) h(sha256)) and verified=true;



-- name: InsertSinglePartUpload :many
INSERT INTO upload_config.image_blob_upload_attempts
            (
                        sha256,
                        file_size,
                        upload_type,
                        upload_room_id,
                        uploader_identity
            )
SELECT Decode(i.sha256,'hex'),
       i.file_size,
       @upload_type::upload_config.upload_type,
       @upload_room_id:: bigint,
       @uploader_identity::text
FROM   unnest( @sha256s::char(64)[],@file_sizes::bigint[]) i(sha256,file_size)
ON conflict (sha256,upload_room_id)
WHERE  upload_state='pending'
do update  set storage_key=image_blob_upload_attempts.storage_key returning encode(sha256, 'hex') sha256,storage_key,(old is null)::boolean as is_inserted;




-- name: InsertAndValidateMultiPartUpload :many
insert into upload_config.image_blob_upload_attempts(
  sha256, file_size, upload_type, 
  upload_id, part_size, upload_room_id, 
  uploader_identity, storage_key
) 
select 
  decode(i.sha256, 'hex') sha256, 
  i.file_size, 
  @upload_type :: upload_config.upload_type, 
  i.upload_id, 
  @part_size :: bigint, 
  @upload_room_id :: bigint, 
  @uploader_identity :: varchar(128), 
  i.storage_key 
from 
  unnest(
    @sha256s :: char(64) [], 
  @file_sizes :: bigint[], 
  @upload_ids :: text[], 
  @storage_keys :: uuid[]) i(
    sha256, file_size, upload_id, storage_key
  ) on conflict (sha256, upload_room_id) 
where 
  upload_state = 'pending' do 
update 
set 
  part_size = case when image_blob_upload_attempts.valid_until >= transaction_timestamp() then image_blob_upload_attempts.part_size else @part_size :: bigint end, 
  valid_until = case when image_blob_upload_attempts.valid_until >= transaction_timestamp() then image_blob_upload_attempts.valid_until else transaction_timestamp()+ interval '5 days' end 
returning 
  part_size, 
  id, 
  encode(sha256,'hex') sha256, 
  storage_key, 
  upload_id,
    ARRAY(
    SELECT
      generate_series(1, part_count, 1)
  )::smallint[] AS parts,
  (old is null):: boolean as is_inserted, 
coalesce(old.valid_until >=Now(),false)::boolean as is_valid;


-- name: GetMultiPartUploadItem :one
select upload_id,storage_key,part_count from upload_config.image_blob_upload_attempts where id= sqlc.arg(multipart_attempt_id);








