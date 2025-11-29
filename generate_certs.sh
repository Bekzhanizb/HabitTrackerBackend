#!/bin/bash

# generate_certs.sh - Генерация TLS сертификатов для HTTPS

echo "🔐 Generating TLS certificates for HTTPS..."

# Создаём директорию для сертификатов
mkdir -p certs

# Генерация приватного ключа и сертификата
openssl req -x509 -newkey rsa:4096 \
  -keyout certs/server.key \
  -out certs/server.crt \
  -days 365 \
  -nodes \
  -subj "/C=KZ/ST=Almaty/L=Almaty/O=HabitTracker/OU=Development/CN=localhost" \
  -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"

# Установка правильных прав доступа
chmod 600 certs/server.key
chmod 644 certs/server.crt

echo "✅ Certificates generated successfully!"
echo ""
echo "📁 Files created:"
echo "   - certs/server.key (private key)"
echo "   - certs/server.crt (certificate)"
echo ""
echo "🚀 Now you can run the application with HTTPS support"
echo "   Set GIN_MODE=release to enable HTTPS automatically"
echo ""
echo "🌐 Access your application at:"
echo "   https://localhost:8080"
echo ""
echo "⚠️  Note: This is a self-signed certificate for development."
echo "   Browsers will show a security warning. For production,"
echo "   use certificates from a trusted CA (Let's Encrypt, etc.)"