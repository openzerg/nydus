PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

-- ── Event Bus ────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS events (
    event_id           TEXT PRIMARY KEY,
    event_type         TEXT NOT NULL,
    source_instance_id TEXT NOT NULL DEFAULT '',
    timestamp          INTEGER NOT NULL,
    data               TEXT NOT NULL DEFAULT '{}'  -- JSON
);

CREATE TABLE IF NOT EXISTS subscribers (
    instance_id   TEXT PRIMARY KEY,
    ip            TEXT NOT NULL DEFAULT '',
    event_types   TEXT NOT NULL DEFAULT '[]',  -- JSON array
    subscribed_at INTEGER NOT NULL
);

-- ── Chatrooms ────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS chatrooms (
    chatroom_id TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS members (
    chatroom_id TEXT NOT NULL,
    member_id   TEXT NOT NULL,
    member_type TEXT NOT NULL DEFAULT 'user',  -- user | instance
    role        TEXT NOT NULL DEFAULT 'member', -- owner | admin | member
    joined_at   INTEGER NOT NULL,
    PRIMARY KEY (chatroom_id, member_id),
    FOREIGN KEY (chatroom_id) REFERENCES chatrooms(chatroom_id) ON DELETE CASCADE
);

-- ── Messages ─────────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS messages (
    message_id  TEXT PRIMARY KEY,
    chatroom_id TEXT NOT NULL,
    sender_id   TEXT NOT NULL,
    sender_type TEXT NOT NULL DEFAULT 'user',
    content     TEXT NOT NULL DEFAULT '',
    metadata    TEXT NOT NULL DEFAULT '{}',
    created_at  INTEGER NOT NULL,
    message_type TEXT NOT NULL DEFAULT 'text',
    updated_at  INTEGER NOT NULL DEFAULT 0,
    deleted     INTEGER NOT NULL DEFAULT 0,
    reply_to_id TEXT NOT NULL DEFAULT '',
    attachments TEXT NOT NULL DEFAULT '[]',
    mentions    TEXT NOT NULL DEFAULT '[]',
    FOREIGN KEY (chatroom_id) REFERENCES chatrooms(chatroom_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS reactions (
    message_id  TEXT NOT NULL,
    emoji       TEXT NOT NULL,
    user_id     TEXT NOT NULL,
    created_at  INTEGER NOT NULL,
    PRIMARY KEY (message_id, emoji, user_id)
);

CREATE INDEX IF NOT EXISTS idx_events_type      ON events(event_type);
CREATE INDEX IF NOT EXISTS idx_events_ts        ON events(timestamp);
CREATE INDEX IF NOT EXISTS idx_messages_chatroom ON messages(chatroom_id, created_at);
CREATE INDEX IF NOT EXISTS idx_members_chatroom  ON members(chatroom_id);
CREATE INDEX IF NOT EXISTS idx_reactions_message ON reactions(message_id);
