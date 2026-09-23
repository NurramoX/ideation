CREATE TABLE idea (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  title       TEXT    NOT NULL,
  body        TEXT    NOT NULL,
  version     INTEGER NOT NULL,
  created_at  TEXT    NOT NULL,
  updated_at  TEXT    NOT NULL,
  reviewed_at TEXT
);
CREATE INDEX idea_created_at ON idea(created_at);
CREATE INDEX idea_updated_at ON idea(updated_at);
CREATE INDEX idea_reviewed_at ON idea(reviewed_at);

CREATE TABLE idea_tag (
  idea_id INTEGER NOT NULL REFERENCES idea(id) ON DELETE CASCADE,
  tag     TEXT    NOT NULL,
  PRIMARY KEY (idea_id, tag)
) WITHOUT ROWID;
CREATE INDEX idea_tag_tag ON idea_tag(tag);

CREATE TABLE idea_attribute (
  idea_id      INTEGER NOT NULL REFERENCES idea(id) ON DELETE CASCADE,
  key          TEXT    NOT NULL,
  value        TEXT    NOT NULL,
  value_folded TEXT    NOT NULL,
  PRIMARY KEY (idea_id, key)
) WITHOUT ROWID;
CREATE INDEX idea_attribute_key_value ON idea_attribute(key, value_folded);

CREATE VIRTUAL TABLE idea_fts USING fts5(
  title, body,
  content='idea', content_rowid='id',
  tokenize='porter unicode61 remove_diacritics 2'
);
CREATE TRIGGER idea_ai AFTER INSERT ON idea BEGIN
  INSERT INTO idea_fts(rowid, title, body) VALUES (new.id, new.title, new.body);
END;
CREATE TRIGGER idea_ad AFTER DELETE ON idea BEGIN
  INSERT INTO idea_fts(idea_fts, rowid, title, body) VALUES ('delete', old.id, old.title, old.body);
END;
CREATE TRIGGER idea_au AFTER UPDATE ON idea BEGIN
  INSERT INTO idea_fts(idea_fts, rowid, title, body) VALUES ('delete', old.id, old.title, old.body);
  INSERT INTO idea_fts(rowid, title, body) VALUES (new.id, new.title, new.body);
END;
INSERT INTO idea_fts(idea_fts, rank) VALUES('rank', 'bm25(10.0, 1.0)');
