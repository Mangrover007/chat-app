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

---------------------------------- OLD SCHEMA BELOW ----------------------------------
-- CREATE EXTENSION IF NOT EXISTS pgcrypto ;

-- CREATE TABLE users (
--     id       UUID       PRIMARY KEY DEFAULT gen_random_uuid(),
--     username VARCHAR(255),
--     password TEXT
-- ) ;

-- CREATE TABLE guilds (
--     id   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
--     name TEXT
-- ) ;

-- CREATE TABLE guild_user (
--     member_id   UUID    REFERENCES users(id),
--     guild_id    UUID    REFERENCES guilds(id),
--     PRIMARY KEY (member_id, guild_id)
-- ) ;

-- INSERT INTO users (username, password) VALUES
--     ('alice', 'alice123'),
--     ('bob', 'bob123'),
--     ('guava', 'guava123')
-- ;

-- INSERT INTO guilds (name) VALUES
--     ('guild_A'),
--     ('guild_B')
-- ;

-- INSERT INTO guild_user (member_id, guild_id)
-- VALUES
-- (
--     (SELECT id FROM users WHERE username = 'alice'),
--     (SELECT id FROM guilds WHERE name = 'guild_A')
-- ),
-- (
--     (SELECT id FROM users WHERE username = 'bob'),
--     (SELECT id FROM guilds WHERE name = 'guild_A')
-- ),
-- (
--     (SELECT id FROM users WHERE username = 'guava'),
--     (SELECT id FROM guilds WHERE name = 'guild_A')
-- ),
-- (
--     (SELECT id FROM users WHERE username = 'alice'),
--     (SELECT id FROM guilds WHERE name = 'guild_B')
-- ),
-- (
--     (SELECT id FROM users WHERE username = 'bob'),
--     (SELECT id FROM guilds WHERE name = 'guild_B')
-- );


---------------------------------- NEW SCHEMA BELOW ----------------------------------
CREATE EXTENSION IF NOT EXISTS pgcrypto ;

CREATE TABLE users (
    id         UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    username   VARCHAR(255)  NOT NULL UNIQUE,
    password   TEXT          NOT NULL,
    created_at TIMESTAMP     NOT NULL
) ;

CREATE TABLE guilds (
    id         UUID      PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT      NOT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL
) ;

CREATE TABLE guild_members (
    member_id   UUID    REFERENCES users(id)  NOT NULL,
    guild_id    UUID    REFERENCES guilds(id) NOT NULL,
    PRIMARY KEY (member_id, guild_id)
) ;


-- choosing UUID for channel ID for now
-- in the future, consider autoincrementing integer for simplicity, and add
-- redis_timestamp and redis_sequence_number for ordering messages gloabally
CREATE TABLE channels (
    id              UUID       PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT       NOT NULL,
    parent_guild_id UUID       REFERENCES guilds(id) NOT NULL,
    created_at      TIMESTAMP  NOT NULL,
    CONSTRAINT unique_channel_per_guild UNIQUE (parent_guild_id, name)
) ;

CREATE TABLE channel_messages (
    id           UUID      PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_id    UUID      REFERENCES users(id)    NOT NULL,
    channel_id   UUID      REFERENCES channels(id) NOT NULL,
    text_content TEXT NOT NULL,
    created_at   TIMESTAMP NOT NULL
) ;

INSERT INTO users (username, password, created_at) VALUES
    ('alice', 'alice123', NOW()),
    ('bob', 'bob123', NOW()),
    ('guava', 'guava123', NOW())
;

INSERT INTO guilds (name, created_at) VALUES
    ('guild_A', NOW()),
    ('guild_B', NOW())
;

INSERT INTO guild_members (member_id, guild_id)
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

INSERT INTO channels (name, parent_guild_id, created_at) VALUES
    ('general', (SELECT id FROM guilds WHERE name = 'guild_A'), NOW()),
    ('random', (SELECT id FROM guilds WHERE name = 'guild_A'), NOW()),
    ('general', (SELECT id FROM guilds WHERE name = 'guild_B'), NOW())
;

INSERT INTO channel_messages (sender_id, channel_id, text_content, created_at) VALUES
(
    (SELECT id FROM users WHERE username = 'alice'),
    (SELECT id FROM channels WHERE name = 'general' AND parent_guild_id = (SELECT id FROM guilds WHERE name = 'guild_A')),
    'Hello from guild A general channel!',
    NOW()
),
(
    (SELECT id FROM users WHERE username = 'bob'),
    (SELECT id FROM channels WHERE name = 'general' AND parent_guild_id = (SELECT id FROM guilds WHERE name = 'guild_A')),
    'Hi Alice!',
    NOW()
),
(
    (SELECT id FROM users WHERE username = 'guava'),
    (SELECT id FROM channels WHERE name = 'general' AND parent_guild_id = (SELECT id FROM guilds WHERE name = 'guild_A')),
    'Sup guys',
    NOW()
),
(
    (SELECT id FROM users WHERE username = 'alice'),
    (SELECT id FROM channels WHERE name = 'general' AND parent_guild_id = (SELECT id FROM guilds WHERE name = 'guild_B')),
    'Hello from guild B!',
    NOW()
) ;

