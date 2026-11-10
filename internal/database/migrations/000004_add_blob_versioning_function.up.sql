create or replace function upload_config.bump_row_version_table_blob_upload_attempts() 
returns trigger
language plpgsql
as 
$$
begin

if new.version <> old.version then
raise exception 'column "version" of table "%" is not user-updatable', TG_TABLE_NAME
using errcode='restrict_violation';
end if;

if (new.sha256,new.file_size,new.upload_type,new.upload_id,new.part_size,new.upload_room_id,new.storage_key,new.uploader_identity,
new.upload_state,new.valid_until,new.verified) is not distinct from (old.sha256,old.file_size,old.upload_type,old.upload_id,old.part_size,old.upload_room_id,old.storage_key,old.uploader_identity,
old.upload_state,old.valid_until,old.verified) then
return new;
end if;

 new.version:=old.version+1;
return new;
end;
$$


