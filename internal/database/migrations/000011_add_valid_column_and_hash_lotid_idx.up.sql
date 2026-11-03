create unique index idx_unq_img_attmpts_sha256_upload_room_id_upload_state on image_blob_upload_attempts(sha256,upload_room_id) where upload_state='pending';

alter table image_blob_upload_attempts add column valid_until timestamptz default current_timestamp + interval '5 days';
