CREATE TABLE IF NOT EXISTS users (
    uuid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    login VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

DROP TABLE IF EXISTS games;

CREATE TABLE games (
    uuid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    field JSONB NOT NULL,
    player1_uuid UUID NOT NULL REFERENCES users(uuid),
    player2_uuid UUID REFERENCES users(uuid),
    current_turn_uuid UUID REFERENCES users(uuid),
    status INTEGER NOT NULL,
    is_with_bot BOOLEAN NOT NULL DEFAULT FALSE,
    winner_uuid UUID REFERENCES users(uuid),
    player1_sign INTEGER NOT NULL,
    player2_sign INTEGER NOT NULL,
    changed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
