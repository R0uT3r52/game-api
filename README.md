# Game API (Tic-Tac-Toe)

[![CI](https://github.com/R0uT3r52/game-api/actions/workflows/CI.yml/badge.svg?branch=main)](https://github.com/R0uT3r52/game-api/actions/workflows/CI.yml)
[![Go Version](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-4169E1?logo=postgresql&logoColor=white)](https://www.postgresql.org/)

A REST API for playing Tic-Tac-Toe, featuring singleplayer against a Minimax AI bot and online multiplayer. Built in Go with layered architecture, Uber FX for DI, and PostgreSQL.

## Features

- **Singleplayer**: Play against an AI opponent powered by the Minimax algorithm.
- **Multiplayer**: Create games and join open lobbies.
- **Authentication**: HTTP Basic Auth with bcrypt password hashing.
- **Containers**: Dockerfile and Docker Compose orchestration.
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

- **Docker** and **Docker compose**
- Alternatively, for local development:
  - **Go 1.25+**
  - **PostgreSQL 16+**

### 1. Configuration

Copy the example environment file and adjust credentials if needed:

```bash
cp .env.template .env
```

Default configuration variables:

```
DB_USER=postgres
DB_PASSWORD=postgrespassword
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgrespassword
DB_HOST=localhost
DB_PORT=5432
DB_NAME=mydb
PORT=8080
```

### 2. Run with Docker Compose (Recommended)

Start the database and the API service with one command:

```bash
docker compose up --build
```

The database schema in `migrations/database.sql` will be automatically applied on initial startup. The API will be available at `http://localhost:8080`.

To run in the background:
```bash
docker compose up -d
```

To stop containers:
```bash
docker compose down
```

## API Reference

All protected endpoints require **HTTP Basic Authentication** (`Authorization: Basic <base64(login:password)>`).

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
- **Auth**: Basic Auth
- **Response**: `200 OK`
  ```json
  {
    "uuid": "4f1c7d24-3486-4f4d-862d-05886616035f"
  }
  ```

#### Get User Profile
- **Endpoint**: `GET /user/{uuid}`
- **Auth**: Basic Auth (Protected)
- **Response**: `200 OK`
  ```json
  {
    "uuid": "4f1c7d24-3486-4f4d-862d-05886616035f",
    "login": "player1"
  }
  ```

### 2. Games

#### Create Game
Create a new game.
- **Endpoint**: `POST /game/new`
- **Auth**: Basic Auth (Protected)
- **Request Body**:
  ```json
  {
    "is_with_bot": true
  }
  ```
  *(Set `"is_with_bot": false` to create a multiplayer lobby)*
- **Response**: `201 Created`
  ```json
  {
    "uuid": "d3b07384-d113-4a00-880d-85c8e312f5a6"
  }
  ```

#### List Available Games
List all open multiplayer games currently waiting for a second player.
- **Endpoint**: `GET /games/available`
- **Auth**: Basic Auth (Protected)
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
      "player2_sign": 0
    }
  ]
  ```

#### Connect to a Game
Join an open waiting game as Player 2.
- **Endpoint**: `POST /game/connect`
- **Auth**: Basic Auth (Protected)
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
Returns all games where authenticated user participates.
- **Endpoint**: `GET /game/current`
- **Auth**: Basic Auth (Protected)
- **Response**: `200 OK` ; array of `GameModel`

#### Get Specific Game State
- **Endpoint**: `GET /game/current/{uuid}`
- **Auth**: Basic Auth (Protected)
- **Response**: `200 OK` ; array with the matched `GameModel`

#### Make a Move
Submit an updated 3x3 grid. Exactly one cell must be changed from empty (`0`) to your player sign (`1` for Player 1, `2` for Player 2). 

In bot games, the bot calculates and executes its move immediately within the same request.
- **Endpoint**: `POST /game/{uuid}`
- **Auth**: Basic Auth (Protected)
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
    "player2_sign": 2
  }
  ```

## Testing

Run unit and integration tests:

```bash
go test -v -race ./...
```

Run linter and formatting checks:

```bash
go vet ./...
test -z "$(gofmt -l .)"
```

Build:

```bash
go build -o game-api cmd/main.go
```
