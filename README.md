<p align="center"><img src="https://raw.githubusercontent.com/bnkamalesh/phispr/refs/heads/main/cmd/http/static/images/phispr-active.svg" alt="Phantom Whisperer" width="256px"/></p>

# Phispr - A Space of Phantom Whispers

[![Go Version](https://img.shields.io/badge/go-1.25.0-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

Phispr is an ephemeral chat application designed for anonymous, temporary conversations that vanish without a trace. Built with Go, it offers both web and terminal user interfaces for seamless communication in phantom rooms where privacy and anonymity are paramount.

# Genesis

[Read more here](docs/genesis.md)

## Features

- **Anonymous Chat Rooms**: No accounts, no registration, just instant anonymous communication
- **Phantom Mode**: Messages that exist only in memory, never persisted anywhere
- **Web Interface**: Modern, responsive web UI with real-time messaging via Server-Sent Events (SSE)
- **Terminal Interface**: A simplistic TUI built with Bubble Tea for command-line enthusiasts
- **Real-time Communication**: Live message delivery and member presence updates
- **Room Management**: Create public or private rooms with customizable capacity
- **QR Code Sharing**: Easy room sharing via QR codes
- **Auto-cleanup**: Configurable message history limits and room capacity management
- **PWA**: This app has [PWA capability](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps)

**Note**: Even for non-phantom rooms, messages are only persisted in-memory. So if you restart the server, messages and rooms are lost.

## Architecture

Phispr follows a clean architecture pattern with clear separation of concerns:

- **HTTP Layer**: Web server with REST API and SSE endpoints ([`cmd/http/`](cmd/http/))
- **TUI Layer**: Terminal user interface ([`cmd/tui/`](cmd/tui/))
- **Business Logic**: Core domain logic ([`internal/`](internal/))
- **Storage**: In-memory message store with configurable retention ([`internal/messages/stores/mem/`](internal/messages/stores/mem/))

## Quick Start

### Prerequisites

- Go 1.25.0 or higher
- Modern web browser (for web interface)
- [esbuild](https://github.com/evanw/esbuild) for preparing static JS and CSS assets

### Installation

1. Clone the repository:

```bash
git clone https://github.com/bnkamalesh/phispr.git
cd phispr
# make changes to config.yaml if needed
```

2. Run the web server:

```bash
# config.yaml is expected to be in the same directory
go run main.go
```

4. Open your browser and navigate to `http://localhost:8080`

### Using the Terminal Interface

For terminal-based usage, you can use the TUI client:

```bash
go run cmd/tui/cmd/main.go -config=/path/to/tui-config.yaml
```

## Configuration

Phispr uses YAML configuration files for both web server and TUI client.

### Web Server Configuration

The main configuration file ([`config.yaml`](config.yaml)) controls the web server behavior:

```yaml
http:
  host: "" # Server host (empty for all interfaces)
  port: 8080 # Server port
  read_timeout: "3s" # HTTP read timeout
  write_timeout: "2h" # HTTP write timeout (long for SSE)
  allowed_origins: # CORS allowed origins
    - "*"
  allowed_headers: # CORS allowed headers
    - "*"
  template_home: "./cmd/http/templates/home.html" # Home page template
  template_room: "./cmd/http/templates/room.html" # Room page template
  template_error: "./cmd/http/templates/error.html" # Error page template
  live_reload_template: true # Enable template live reloading
  enable_access_log: true # Enable HTTP access logging

rooms:
  capacity: 100 # Maximum number of concurrent rooms
  member_capacity: 250 # Maximum members per room
  live_viewer_broadcast_delay: "5s" # Delay between viewer count broadcasts
```

### TUI Configuration

Create a separate configuration file for the terminal interface:

```yaml
host: "http://localhost:8080" # Phispr server URL
cookies_path: "/tmp/phispr-cookies.json" # Cookie storage path
member_path: "/tmp/phispr-member.json" # Member info storage path
room: "general" # Default room to join
username: "terminal-user" # Default username
```

## Usage Guide

### Web Interface

1. **Creating a Room**:

   - Visit the homepage
   - Fill in the "New room" form with a room name and username
   - Choose options: "unlisted" (private) and/or "phantom" (no message persistence)
   - Click "Create & Join"

2. **Joining a Room**:

   - Click on any public room from the homepage, or
   - Navigate directly to `/rooms/{room-name}`

3. **Room Sharing**:
   - Click on the room name to display a QR code
   - Share the URL or QR code with others

### Terminal Interface

The TUI provides a rich terminal experience with the following features:

#### Starting the TUI

```bash
# Basic usage
go run cmd/tui/cmd/main.go -config=tui-config.yaml

# The TUI will automatically connect to the specified room if configured
```

#### TUI Commands

- **Send Message**: Type your message and press Enter
- **Clear Chat**: Type `/clear` and press Enter
- **Exit**: Type `/exit` and press Enter, or use Ctrl+C

#### TUI Features

- **Real-time Messaging**: Messages appear instantly from other users
- **Member Notifications**: See when users join or leave
- **Message History**: Scroll through previous messages
- **Connection Status**: Visual indicators for connection health
- **Cookie Persistence**: Automatically manages authentication cookies

### API Endpoints

Phispr exposes a REST API that can be consumed by any HTTP client:

- `GET /` - Homepage
- `GET /rooms/{roomID}` - Room page or JSON room details
- `POST /rooms` - Create and join a room
- `POST /rooms/{roomID}` - Join an existing room
- `POST /rooms/{roomID}/leave` - Leave a room
- `POST /rooms/{roomID}/messages` - Send a message
- `GET /rooms/{roomID}/messages` - SSE endpoint for real-time updates
- `GET /static/*` - Static file serving

## Development

### Building

```bash
# Build web server
go build -o phispr main.go

# Build TUI client
go build -o phispr-tui cmd/tui/cmd/main.go
```

## Security & Privacy

- **No Data Persistence**: Phantom rooms store no messages
- **Anonymous by Design**: No user registration or personal data collection
- **Stateless Authentication**: Cookie-based session management without server-side storage
- **CORS Protection**: Configurable origin and header restrictions

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For major changes, please open an issue first to discuss what you would like to change.
