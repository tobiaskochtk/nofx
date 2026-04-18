#!/bin/sh
set -e

# Railway sets the PORT environment variable
export PORT=${PORT:-8080}
echo "🚀 Starting NOFX on port $PORT..."

# Generate encryption keys (if not already set)
if [ -z "$RSA_PRIVATE_KEY" ]; then
    export RSA_PRIVATE_KEY=$(openssl genrsa 2048 2>/dev/null)
fi
if [ -z "$DATA_ENCRYPTION_KEY" ]; then
    export DATA_ENCRYPTION_KEY=$(openssl rand -base64 32)
fi

# PostgreSQL is the default runtime datastore.
# Railway-managed Postgres commonly exposes PG* variables, so map them when DB_* is unset.
export DB_TYPE=${DB_TYPE:-postgres}
if [ -z "$DB_HOST" ] && [ -n "$PGHOST" ]; then
    export DB_HOST="$PGHOST"
fi
if [ -z "$DB_PORT" ] && [ -n "$PGPORT" ]; then
    export DB_PORT="$PGPORT"
fi
if [ -z "$DB_USER" ] && [ -n "$PGUSER" ]; then
    export DB_USER="$PGUSER"
fi
if [ -z "$DB_PASSWORD" ] && [ -n "$PGPASSWORD" ]; then
    export DB_PASSWORD="$PGPASSWORD"
fi
if [ -z "$DB_NAME" ] && [ -n "$PGDATABASE" ]; then
    export DB_NAME="$PGDATABASE"
fi
export DB_SSLMODE=${DB_SSLMODE:-disable}

# Generate nginx config
cat > /etc/nginx/http.d/default.conf << NGINX_EOF
server {
    listen $PORT;
    server_name _;
    root /usr/share/nginx/html;
    index index.html;
    gzip on;
    gzip_types text/plain text/css application/json application/javascript;

    location / {
        try_files \$uri \$uri/ /index.html;
    }

    location /api/ {
        proxy_pass http://127.0.0.1:8081/api/;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_connect_timeout 300s;
        proxy_send_timeout 300s;
        proxy_read_timeout 300s;
    }

    location /health {
        return 200 'OK';
        add_header Content-Type text/plain;
    }
}
NGINX_EOF

# Start backend (port 8081)
API_SERVER_PORT=8081 /app/nofx &
sleep 2

# Start nginx (background)
nginx

echo "✅ NOFX started successfully"

# Keep the container running
tail -f /dev/null
