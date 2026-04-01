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

CREATE EXTENSION IF NOT EXISTS pgcrypto ;

CREATE TABLE users (
    id       UUID       PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(255),
    password TEXT
) ;

CREATE TABLE guilds (
    id   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT
) ;

CREATE TABLE guild_user (
    member_id   UUID    REFERENCES users(id),
    guild_id    UUID    REFERENCES guilds(id),
    PRIMARY KEY (member_id, guild_id)
) ;

INSERT INTO users (username, password) VALUES
    ('alice', 'alice123'),
    ('bob', 'bob123'),
    ('guava', 'guava123')
;

INSERT INTO guilds (name) VALUES
    ('guild_A'),
    ('guild_B')
;

INSERT INTO guild_user (member_id, guild_id)
VALUES
(
    (SELECT id FROM users WHERE username = 'alice'),
    (SELECT id FROM guilds WHERE name = 'guild_A')
),
(
    (SELECT id FROM users WHERE username = 'bob'),
    (SELECT id FROM guilds WHERE name = 'guild_A')
),
(
    (SELECT id FROM users WHERE username = 'guava'),
    (SELECT id FROM guilds WHERE name = 'guild_A')
),
(
    (SELECT id FROM users WHERE username = 'alice'),
    (SELECT id FROM guilds WHERE name = 'guild_B')
),
(
    (SELECT id FROM users WHERE username = 'bob'),
    (SELECT id FROM guilds WHERE name = 'guild_B')
);
