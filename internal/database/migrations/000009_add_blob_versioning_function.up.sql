create or replace function bump_row_version_table_image_blob_upload_attempts() 
returns trigger
language plpgsql
as 
$$
begin

if new.version <> old.version then
raise exception 'column "version" of table "%" is not user-updatable', TG_TABLE_NAME
using errcode='restrict_violation';
end if;

if (new.sha256,new.file_size,new.content_type,new.upload_type,new.upload_id,new.part_size,new.upload_room_id,new.file_name,new.storage_key,new.username,
new.upload_state,new.valid_until,new.verified) is not distinct from (old.sha256,old.file_size,old.content_type,old.upload_type,old.upload_id,old.part_size,old.upload_room_id,old.file_name,old.storage_key,old.username,
old.upload_state,old.valid_until,old.verified) then
return new;
end if;

 new.version:=old.version+1;
return new;
end;
$$


