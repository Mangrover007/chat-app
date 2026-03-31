-- Table users {
--   ID string [PK]
--   Username string
-- }

-- Table guilds {
--   ID string [PK]
-- }

-- Table guild_user {
--   member_id string
--   guild_id string

--   indexes {
--     (member_id, guild_id) [PK]
--   }
-- }

-- Ref: users.ID < guild_user.member_id
-- Ref: guilds.ID < guild_user.guild_id

CREATE TABLE users (
    id       TEXT       PRIMARY KEY,
    username VARCHAR(255)
) ;

CREATE TABLE guilds (
    id TEXT PRIMARY KEY
) ;

CREATE TABLE guild_user (
    member_id   TEXT    REFERENCES users(id),
    guild_id    TEXT    REFERENCES guilds(id),
    PRIMARY KEY (member_id, guild_id)
) ;

INSERT INTO users (id, username) VALUES
    ('alice123', 'alice'),
    ('bob123', 'bob'),
    ('guava123', 'guava')
;

INSERT INTO guilds (id) VALUES
    ('guild_A'),
    ('guild_B')
;

INSERT INTO guild_user (member_id, guild_id) VALUES
    ('alice123', 'guild_A'),
    ('alice123', 'guild_B'),
    ('bob123', 'guild_A'),
    ('guava123', 'guild_B')
;
