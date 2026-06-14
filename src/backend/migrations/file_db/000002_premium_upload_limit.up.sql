-- file_db v2 — raise size_bytes cap to 200 MiB (premium tier enforced in File service)
ALTER TABLE files DROP CONSTRAINT IF EXISTS files_size_bytes_check;
ALTER TABLE files ADD CONSTRAINT files_size_bytes_check CHECK (size_bytes > 0 AND size_bytes <= 209715200);
