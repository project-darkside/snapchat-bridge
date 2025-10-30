# Snapchat Bridge Deployment Guide

## Quick Start

### 1. Build the Bridge

```bash
# Clone the repository
git clone https://github.com/lalomorales22/snapchat-bridge
cd snapchat-bridge

# Build with nocrypto tag (recommended - no libolm dependency)
go build -tags nocrypto -o snapchat-bridge

# Verify build
./snapchat-bridge --version
```

### 2. Generate Configuration

```bash
# Generate example config
./snapchat-bridge -generate-config

# This creates config.yaml and registration.yaml
```

### 3. Configure Matrix Homeserver

Edit `config.yaml`:

```yaml
homeserver:
  address: https://matrix.example.com
  domain: example.com

appservice:
  address: http://localhost:29318
  hostname: 0.0.0.0
  port: 29318
  id: snapchat
  bot_token: "generate-random-token-here"
  hs_token: "generate-random-token-here"
  as_token: "generate-random-token-here"
```

Generate secure tokens:
```bash
# Generate three different random tokens
openssl rand -base64 64  # bot_token
openssl rand -base64 64  # hs_token
openssl rand -base64 64  # as_token
```

### 4. Configure Snapchat Settings

Edit the `snapchat` section in `config.yaml`:

```yaml
snapchat:
  base_url: https://app.snapchat.com
  user_agent: "Snapchat/11.30.0.35 (iPhone; iOS 15.0; Scale/3.00)"
  app_version: "11.30.0.35"
  device_id: ""  # Will be auto-generated
  client_auth: ""  # Will be auto-generated
  rate_limit:
    requests: 10
    window: 60  # seconds
```

### 5. Register Appservice with Matrix

**For Synapse:**

1. Copy `registration.yaml` to your Synapse config directory
2. Edit `homeserver.yaml`:
   ```yaml
   app_service_config_files:
     - /path/to/snapchat-bridge/registration.yaml
   ```
3. Restart Synapse:
   ```bash
   systemctl restart matrix-synapse
   ```

**For Dendrite:**

1. Copy `registration.yaml` to your Dendrite config directory
2. Edit `dendrite.yaml`:
   ```yaml
   app_service_api:
     config_files:
       - /path/to/snapchat-bridge/registration.yaml
   ```
3. Restart Dendrite:
   ```bash
   systemctl restart dendrite
   ```

### 6. Start the Bridge

```bash
# Run in foreground (for testing)
./snapchat-bridge -config config.yaml

# Or use systemd (see below for service file)
sudo systemctl start snapchat-bridge
```

### 7. Login to Snapchat

1. Message the bridge bot: `@snapchatbot:example.com`
2. Send login command: `!sc login your-username your-password`
3. Wait for authentication confirmation
4. Your conversations will sync automatically

## Production Deployment

### Systemd Service

Create `/etc/systemd/system/snapchat-bridge.service`:

```ini
[Unit]
Description=Snapchat Matrix Bridge
After=network.target

[Service]
Type=simple
User=snapchat-bridge
Group=snapchat-bridge
WorkingDirectory=/opt/snapchat-bridge
ExecStart=/opt/snapchat-bridge/snapchat-bridge -config /opt/snapchat-bridge/config.yaml
Restart=on-failure
RestartSec=10
StandardOutput=journal
StandardError=journal

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/snapchat-bridge
CapabilityBoundingSet=

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl daemon-reload
sudo systemctl enable snapchat-bridge
sudo systemctl start snapchat-bridge
sudo systemctl status snapchat-bridge
```

View logs:
```bash
sudo journalctl -u snapchat-bridge -f
```

### Docker Deployment

Create `Dockerfile`:

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY . .

# Build with nocrypto tag
RUN go build -tags nocrypto -o snapchat-bridge

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /opt/snapchat-bridge

COPY --from=builder /app/snapchat-bridge .
COPY --from=builder /app/config.example.yaml ./config.yaml

EXPOSE 29318

CMD ["./snapchat-bridge", "-config", "config.yaml"]
```

Build and run:
```bash
# Build image
docker build -t snapchat-bridge .

# Run container
docker run -d \
  --name snapchat-bridge \
  -v $(pwd)/config.yaml:/opt/snapchat-bridge/config.yaml \
  -v $(pwd)/data:/opt/snapchat-bridge/data \
  -p 29318:29318 \
  snapchat-bridge
```

### Docker Compose

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  snapchat-bridge:
    build: .
    container_name: snapchat-bridge
    restart: unless-stopped
    volumes:
      - ./config.yaml:/opt/snapchat-bridge/config.yaml:ro
      - ./data:/opt/snapchat-bridge/data
    ports:
      - "29318:29318"
    environment:
      - TZ=UTC
    networks:
      - matrix

networks:
  matrix:
    external: true
```

Start:
```bash
docker-compose up -d
```

## Monitoring

### Health Checks

The bridge exposes health endpoints:

- **Liveness**: `GET /_matrix/mau/live` - Returns 200 if bridge is running
- **Readiness**: `GET /_matrix/mau/ready` - Returns 200 when bridge is ready to accept traffic

### Prometheus Metrics (Optional)

Add to config.yaml:
```yaml
metrics:
  enabled: true
  listen: "0.0.0.0:8001"
```

## Troubleshooting

### Build Issues

**Error: `olm/olm.h` not found**

Solution: Use the `nocrypto` build tag:
```bash
go build -tags nocrypto -o snapchat-bridge
```

Or install libolm:
```bash
# macOS
brew install libolm

# Ubuntu/Debian
sudo apt-get install libolm-dev

# Alpine
apk add olm-dev
```

### Authentication Issues

**Bridge fails to login:**

1. Check Snapchat credentials are correct
2. Verify base_url in config is `https://app.snapchat.com`
3. Check logs for detailed error messages
4. Try creating a new Snapchat account for testing

**Rate limiting errors:**

Adjust rate limit settings in config.yaml:
```yaml
snapchat:
  rate_limit:
    requests: 5  # Reduce requests per window
    window: 120  # Increase window size
```

### Connection Issues

**Bridge can't connect to homeserver:**

1. Verify homeserver URL in config
2. Check firewall rules
3. Ensure tokens match in both config.yaml and registration.yaml

**Messages not syncing:**

1. Check bridge is running: `systemctl status snapchat-bridge`
2. Review logs for errors
3. Verify Snapchat session is still valid
4. Try manual sync: `!sc sync`

### Database Issues

**SQLite database locked:**

If using SQLite, ensure only one bridge instance is running:
```bash
ps aux | grep snapchat-bridge
```

## Security Considerations

1. **Use strong tokens**: Generate cryptographically random tokens
2. **Restrict network access**: Use firewall rules to limit access to bridge port
3. **Keep credentials secure**: Use environment variables or secrets management
4. **Regular updates**: Keep the bridge and dependencies updated
5. **Separate user**: Run bridge as dedicated non-root user
6. **Monitor logs**: Watch for suspicious activity or API errors

## Backup and Recovery

### Backup Database

```bash
# Backup SQLite database
cp data/snapchat.db data/snapchat.db.backup

# Or use systemd timer for automated backups
```

### Restore Database

```bash
# Stop bridge
systemctl stop snapchat-bridge

# Restore database
cp data/snapchat.db.backup data/snapchat.db

# Start bridge
systemctl start snapchat-bridge
```

## Performance Tuning

### Database Optimization

For high-traffic bridges, consider PostgreSQL instead of SQLite:

```yaml
database:
  type: postgres
  uri: "postgresql://user:pass@localhost/snapchat_bridge?sslmode=disable"
```

### Rate Limiting

Adjust based on your needs:
```yaml
snapchat:
  rate_limit:
    requests: 20  # Higher for more active users
    window: 60
```

## Support

- **GitHub Issues**: https://github.com/lalomorales22/snapchat-bridge/issues
- **Matrix Room**: [#snapchat-bridge:example.com]
- **Documentation**: See README.md and HANDOFF.md

## License

MIT License - See LICENSE file for details
