# Game API (Tic-Tac-Toe)

[![CI](https://github.com/R0uT3r52/game-api/actions/workflows/CI.yml/badge.svg?branch=main)](https://github.com/R0uT3r52/game-api/actions/workflows/CI.yml)
[![Go Version](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)](https://www.postgresql.org/)

A REST API for playing Tic-Tac-Toe, featuring singleplayer against a Minimax AI bot and online multiplayer. Built in Go with layered architecture, Uber FX for dependency injection, and PostgreSQL.

## Features

- **Singleplayer**: Play against an AI opponent powered by the Minimax algorithm.
- **Multiplayer**: Create games and join open waiting lobbies.
- **Authentication**: JWT (JSON Web Tokens) with Access (24h) and Refresh (7d) tokens, Bearer authentication, and bcrypt password hashing.
- **Game History**: View history of finished matches for the authenticated player.
- **Leaderboard**: Global player rankings based on calculated winrate.
- **Containers**: Dockerfile and Docker Compose orchestration (API, PostgreSQL 16, and pgweb database viewer).
- **CI Pipeline**: Automated linting (`gofmt`, `go vet`), race detection (`go test -race`), and builds via GitHub Actions.

## Game Mechanics & Rules

### Signs and Board Representation
The board is represented as a 3x3 integer matrix (`[3][3]int`):
- `0` = **Empty cell**
- `1` = **Cross (X)** (Player 1)
- `2` = **Nought (O)** (Player 2 or Bot)

### Game Status (`status`)
| Status Code | Name | Description |
| :--- | :--- | :--- |
| `0` | **Waiting** | Game created; waiting for Player 2 to connect (multiplayer only) |
| `1` | **Turn** | Active game; players take turns making moves |
| `2` | **Draw** | Game ended in a tie (no moves remaining) |
| `3` | **Win** | Game ended; one player or bot achieved 3 in a row |

## Getting Started

### Prerequisites

- **Docker** and **Docker Compose**
- Alternatively, for local development without containers:
  - **Go 1.25+**
  - **PostgreSQL 16+**

### 1. Configuration

Copy the example environment file and adjust variables as needed:

```bash
cp .env.template .env
```

Configuration variables:

```env
DB_USER=postgres
DB_PASSWORD=postgrespassword
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgrespassword
DB_HOST=localhost
DB_PORT=5432
DB_NAME=mydb
JWT_SECRET=mysecret
PORT=8080
```

> **Note:** When using Docker Compose, `DB_HOST` is automatically routed to the `postgres` service container. `JWT_SECRET` is required for signing and validating JWT tokens.

### 2. Run with Docker Compose (Recommended)

Start the database, database management UI, and API service with one command:

```bash
docker compose up --build
```

- **API**: available at `http://localhost:8080`
- **pgweb** (web database explorer): available at `http://localhost:8081`
- The initial database schema in `migrations/database.sql` is automatically applied on first startup.

To run in the background (detached mode):
```bash
docker compose up -d
```

To stop containers:
```bash
docker compose down
```

---

## API Reference

All protected endpoints require **Bearer Token Authentication** in the `Authorization` header:

```http
Authorization: Bearer <access_token>
```

### 1. Authentication & Users

#### Register a User
- **Endpoint**: `POST /signup`
- **Auth**: Public
- **Request Body**:
  ```json
  {
    "login": "player1",
    "password": "secretpassword"
  }
  ```
- **Response**: `201 Created`

#### Authenticate / Login
- **Endpoint**: `POST /login`
- **Auth**: Public
- **Request Body**:
  ```json
  {
    "login": "player1",
    "password": "secretpassword"
  }
  ```
- **Response**: `200 OK`
  ```json
  {
    "type": "Bearer",
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }
  ```

#### Refresh Access Token
Generates a new access token while keeping the same refresh token.
- **Endpoint**: `POST /refresh-acc`
- **Auth**: Public
- **Request Body**:
  ```json
  {
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }
  ```
- **Response**: `200 OK`
  ```json
  {
    "type": "Bearer",
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }
  ```

#### Refresh Both Tokens (Rotate Refresh Token)
Generates both a new access token and a new refresh token.
- **Endpoint**: `POST /refresh-ref`
- **Auth**: Public
- **Request Body**:
  ```json
  {
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }
  ```
- **Response**: `200 OK`
  ```json
  {
    "type": "Bearer",
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  }
  ```

#### Get Current User Profile
Retrieves user information for the token holder.
- **Endpoint**: `GET /user`
- **Auth**: Bearer Token (Protected)
- **Response**: `200 OK`
  ```json
  {
    "uuid": "4f1c7d24-3486-4f4d-862d-05886616035f",
    "login": "player1"
  }
  ```

#### Get User Profile by UUID
- **Endpoint**: `GET /user/{uuid}`
- **Auth**: Bearer Token (Protected)
- **Response**: `200 OK`
  ```json
  {
    "uuid": "4f1c7d24-3486-4f4d-862d-05886616035f",
    "login": "player1"
  }
  ```

---

### 2. Games & Gameplay

#### Create Game
Create a new game (singleplayer against bot or a multiplayer lobby).
- **Endpoint**: `POST /game/new`
- **Auth**: Bearer Token (Protected)
- **Request Body**:
  ```json
  {
    "is_with_bot": true
  }
  ```
  *(Set `"is_with_bot": false` to create an open multiplayer lobby)*
- **Response**: `201 Created`
  ```json
  {
    "uuid": "d3b07384-d113-4a00-880d-85c8e312f5a6"
  }
  ```

#### List Available Games
Lists all open multiplayer games currently waiting for a second player (`status = 0`).
- **Endpoint**: `GET /games/available`
- **Auth**: Bearer Token (Protected)
- **Response**: `200 OK`
  ```json
  [
    {
      "uuid": "d3b07384-d113-4a00-880d-85c8e312f5a6",
      "field": [
        [0, 0, 0],
        [0, 0, 0],
        [0, 0, 0]
      ],
      "player1_uuid": "4f1c7d24-3486-4f4d-862d-05886616035f",
      "status": 0,
      "is_with_bot": false,
      "player1_sign": 1,
      "player2_sign": 0,
      "created_at": "2026-11-03T02:00:00Z"
    }
  ]
  ```

#### Connect to a Game
Join an open waiting game as Player 2.
- **Endpoint**: `POST /game/connect`
- **Auth**: Bearer Token (Protected)
- **Request Body**:
  ```json
  {
    "uuid": "d3b07384-d113-4a00-880d-85c8e312f5a6"
  }
  ```
- **Response**: `200 OK`
  ```json
  "connected successfully"
  ```

#### Get Active Games of Current User
Returns all games where the authenticated user participates.
- **Endpoint**: `GET /game/current`
- **Auth**: Bearer Token (Protected)
- **Response**: `200 OK`
  ```json
  [
    {
      "uuid": "d3b07384-d113-4a00-880d-85c8e312f5a6",
      "field": [
        [1, 0, 0],
        [0, 2, 0],
        [0, 0, 0]
      ],
      "player1_uuid": "4f1c7d24-3486-4f4d-862d-05886616035f",
      "player2_uuid": "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d",
      "current_turn_uuid": "4f1c7d24-3486-4f4d-862d-05886616035f",
      "status": 1,
      "is_with_bot": false,
      "player1_sign": 1,
      "player2_sign": 2,
      "created_at": "2026-11-03T02:00:00Z"
    }
  ]
  ```

#### Get Specific Game State
- **Endpoint**: `GET /game/current/{uuid}`
- **Auth**: Bearer Token (Protected)
- **Response**: `200 OK` (array with the matched `GameModel`)

#### Make a Move
Submit an updated 3x3 grid. Exactly one cell must change from empty (`0`) to your player sign (`1` for Player 1, `2` for Player 2).

In bot games, the bot calculates and executes its move immediately within the same request.
- **Endpoint**: `POST /game/{uuid}`
- **Auth**: Bearer Token (Protected)
- **Request Body**:
  ```json
  {
    "field": [
      [1, 0, 0],
      [0, 0, 0],
      [0, 0, 0]
    ]
  }
  ```
- **Response**: `200 OK`
  ```json
  {
    "uuid": "d3b07384-d113-4a00-880d-85c8e312f5a6",
    "field": [
      [1, 0, 0],
      [0, 2, 0],
      [0, 0, 0]
    ],
    "player1_uuid": "4f1c7d24-3486-4f4d-862d-05886616035f",
    "current_turn_uuid": "4f1c7d24-3486-4f4d-862d-05886616035f",
    "status": 1,
    "is_with_bot": true,
    "player1_sign": 1,
    "player2_sign": 2,
    "created_at": "2026-11-03T02:00:00Z"
  }
  ```

---

### 3. History & Leaderboard

#### Get Finished Games
Returns all completed games (`status = 2` (Draw) or `status = 3` (Win)) where the authenticated user participated, sorted newest first (`created_at DESC`).
- **Endpoint**: `GET /game/finished`
- **Auth**: Bearer Token (Protected)
- **Response**: `200 OK`
  ```json
  [
    {
      "uuid": "d3b07384-d113-4a00-880d-85c8e312f5a6",
      "field": [
        [1, 1, 1],
        [0, 2, 0],
        [2, 0, 0]
      ],
      "player1_uuid": "4f1c7d24-3486-4f4d-862d-05886616035f",
      "winner_uuid": "4f1c7d24-3486-4f4d-862d-05886616035f",
      "status": 3,
      "is_with_bot": true,
      "player1_sign": 1,
      "player2_sign": 2,
      "created_at": "2026-11-03T02:00:00Z"
    }
  ]
  ```

#### Get Leaderboard
Returns top players sorted by winrate (`wins / (losses + draws)`).
- **Endpoint**: `GET /leaderboard`
- **Auth**: Bearer Token (Protected)
- **Query Parameters**:
  - `limit` (optional, synonyms: `n`, `N`): Maximum number of players to return (default: `50`).
- **Response**: `200 OK`
  ```json
  [
    {
      "uuid": "4f1c7d24-3486-4f4d-862d-05886616035f",
      "login": "player1",
      "winrate": 0.85
    }
  ]
  ```

---

## Testing & Quality

Run unit and integration tests with race detector (requires Docker):

```bash
go test -v -race ./...
```

Run only unit tests without Postgres integration:

```bash
go test -v -race -short ./...
```

Run linter and formatting checks:

```bash
go vet ./...
test -z "$(gofmt -l .)"
```

Build binary:

```bash
go build -o game-api cmd/main.go
```
