#!/bin/bash
set -e

OS="$(uname -s)"
ARCH="$(uname -m)"

install_dependencies() {
    if [[ "$OS" == "Linux" ]]; then
        if command -v apt >/dev/null 2>&1; then
            . /etc/os-release
            if [[ "$ID" == "debian" || "$ID" == "ubuntu" ]]; then
                sudo apt update
                sudo apt install -y git libvips-dev openssl
            else
                echo "Unsupported Linux distribution: $ID"
                exit 1
            fi
        else
            echo "apt is not available. Please install dependencies manually."
            exit 1
        fi
    else
        echo "You will have to manually install dependencies as this is an unsupported operating system: $OS"
        exit 1
    fi
}

create_user() {
    if ! id "imagebarn" >/dev/null 2>&1; then
        sudo useradd --no-create-home --system imagebarn
        echo "User 'imagebarn' created."
    else
        echo "User 'imagebarn' already exists."
    fi
}

get_release() {
    sudo mkdir -p /opt/imagebarn
    cd /tmp

    LATEST_TAG=$(curl -s https://api.github.com/repos/kyleyannelli/imagebarn/releases/latest | grep -oP '"tag_name": "\K(.*)(?=")')

    if [[ -z "$LATEST_TAG" ]]; then
        echo "Failed to fetch the latest release tag."
        exit 1
    fi

    case "$ARCH" in
        x86_64)
            ARCH_DL="amd64"
            ;;
        aarch64|arm64)
            ARCH_DL="arm64"
            ;;
        *)
            echo "Unsupported architecture: $ARCH"
            exit 1
            ;;
    esac

    DOWNLOAD_URL="https://github.com/kyleyannelli/imagebarn/releases/download/$LATEST_TAG/imagebarn_${LATEST_TAG#v}_linux_$ARCH_DL.tar.gz"

    echo "Downloading ImageBarn from $DOWNLOAD_URL"

    curl -L -o imagebarn.tar.gz "$DOWNLOAD_URL"

    tar -xzf imagebarn.tar.gz

    sudo mv "imagebarn_${LATEST_TAG#v}_linux_$ARCH_DL" /opt/imagebarn/imagebarn

    rm imagebarn.tar.gz
    cd -

    sudo chown -R imagebarn:imagebarn /opt/imagebarn
    sudo chmod +x /opt/imagebarn/imagebarn

    echo "ImageBarn installed in /opt/imagebarn."
}

configure_environment() {
    echo "Configuring environment variables for ImageBarn..."

    read -p "Enter your Google Client ID: " GOOGLE_CLIENT_ID
    while [[ -z "$GOOGLE_CLIENT_ID" ]]; do
        echo "Google Client ID cannot be empty."
        read -p "Enter your Google Client ID: " GOOGLE_CLIENT_ID
    done

    read -p "Enter your Google Client Secret: " GOOGLE_CLIENT_SECRET
    while [[ -z "$GOOGLE_CLIENT_SECRET" ]]; do
        echo "Google Client Secret cannot be empty."
        read -p "Enter your Google Client Secret: " GOOGLE_CLIENT_SECRET
    done

    read -p "Enter the Base URI (e.g., https://yourdomain.com): " BASE_URI
    while [[ -z "$BASE_URI" ]]; do
        echo "Base URI cannot be empty."
        read -p "Enter the Base URI (e.g., https://yourdomain.com): " BASE_URI
    done

    read -p "Enter the admin user's email address: " ADMIN_USER
    while [[ -z "$ADMIN_USER" ]]; do
        echo "Admin user email cannot be empty."
        read -p "Enter the admin user's email address: " ADMIN_USER
    done

    BEARER_TOKEN=$(openssl rand -base64 24)
    echo "Generated BEARER_TOKEN: $BEARER_TOKEN"

    read -p "Enter the number of image workers [default: 2]: " IMAGE_WORKERS
    IMAGE_WORKERS=${IMAGE_WORKERS:-2}

    read -p "Enter the upload limit in MB [default: 35]: " UPLOAD_LIMIT_MB
    UPLOAD_LIMIT_MB=${UPLOAD_LIMIT_MB:-35}

    read -p "Enter any trusted proxies (comma-separated), or leave blank: " TRUSTED_PROXIES

    ENV_FILE="/opt/imagebarn/.env"
    sudo bash -c "cat > $ENV_FILE" << EOF
GOOGLE_CLIENT_ID="$GOOGLE_CLIENT_ID"
GOOGLE_CLIENT_SECRET="$GOOGLE_CLIENT_SECRET"
BASE_URI="$BASE_URI"
ADMIN_USER="$ADMIN_USER"
BEARER_TOKEN="$BEARER_TOKEN"
IMAGE_WORKERS=$IMAGE_WORKERS
UPLOAD_LIMIT_MB=$UPLOAD_LIMIT_MB
TRUSTED_PROXIES="$TRUSTED_PROXIES"
EOF

    sudo chown imagebarn:imagebarn $ENV_FILE
    sudo chmod 600 $ENV_FILE

    echo "Environment configuration complete."
}

create_systemd_service() {
    SERVICE_FILE=/etc/systemd/system/imagebarn.service
    sudo bash -c "cat > $SERVICE_FILE" << EOF
[Unit]
Description=ImageBarn Service
After=network.target

[Service]
Type=simple
User=imagebarn
WorkingDirectory=/opt/imagebarn
EnvironmentFile=/opt/imagebarn/.env
ExecStart=/opt/imagebarn/imagebarn
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF
    sudo systemctl daemon-reload
    sudo systemctl enable imagebarn
    sudo systemctl start imagebarn
}

echo "Starting ImageBarn setup..."
install_dependencies
create_user
get_release
configure_environment
create_systemd_service
sudo systemctl status imagebarn

echo "ImageBarn setup is complete."
