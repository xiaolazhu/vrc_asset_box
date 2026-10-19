CREATE TABLE categories(id TEXT PRIMARY KEY,name TEXT NOT NULL,name_key TEXT NOT NULL UNIQUE,position INTEGER NOT NULL);
CREATE TABLE media(id TEXT PRIMARY KEY,sha256 TEXT NOT NULL UNIQUE,original_path TEXT NOT NULL,thumbnail_path TEXT NOT NULL,mime TEXT NOT NULL,width INTEGER NOT NULL,height INTEGER NOT NULL,created_at TEXT NOT NULL);
CREATE TABLE assets(
 id TEXT PRIMARY KEY,booth_id TEXT,name TEXT NOT NULL CHECK(length(trim(name))>0),name_key TEXT NOT NULL,
 category_id TEXT REFERENCES categories(id) ON DELETE SET NULL,cover_id TEXT REFERENCES media(id),
 universal INTEGER NOT NULL DEFAULT 0 CHECK(universal IN (0,1)),notes TEXT NOT NULL DEFAULT '',folder_path TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL,updated_at TEXT NOT NULL,revision INTEGER NOT NULL DEFAULT 1,
 CHECK(booth_id IS NULL OR (length(booth_id)>0 AND booth_id NOT GLOB '*[^0-9]*'))
);
CREATE UNIQUE INDEX assets_booth_unique ON assets(booth_id) WHERE booth_id IS NOT NULL;
CREATE TABLE tags(id TEXT PRIMARY KEY,kind TEXT NOT NULL CHECK(kind IN ('avatar','free')),name TEXT NOT NULL CHECK(length(trim(name))>0));
CREATE TABLE tag_names(tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,kind TEXT NOT NULL CHECK(kind IN ('avatar','free')),name TEXT NOT NULL,name_key TEXT NOT NULL,canonical INTEGER NOT NULL,PRIMARY KEY(kind,name_key));
CREATE UNIQUE INDEX tag_one_canonical ON tag_names(tag_id) WHERE canonical=1;
CREATE TABLE asset_tags(asset_id TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,PRIMARY KEY(asset_id,tag_id));
CREATE TABLE settings(key TEXT PRIMARY KEY,value_json TEXT NOT NULL);
CREATE TABLE import_runs(id TEXT PRIMARY KEY,source_sha256 TEXT NOT NULL,imported_at TEXT NOT NULL);
CREATE TABLE import_sources(source_sha256 TEXT NOT NULL,sheet TEXT NOT NULL,row_number INTEGER NOT NULL,asset_id TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,run_id TEXT NOT NULL REFERENCES import_runs(id),PRIMARY KEY(source_sha256,sheet,row_number));
CREATE INDEX assets_category ON assets(category_id);
CREATE INDEX assets_created ON assets(created_at,id);
CREATE INDEX assets_updated ON assets(updated_at,id);
CREATE INDEX asset_tags_reverse ON asset_tags(tag_id,asset_id);
