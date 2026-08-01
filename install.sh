#!/bin/bash
set -e

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m'

echo -e "${GREEN}[ ] Grabba Installer${NC}"
echo "=================="

if [ ! -f "./build/grabba" ]; then
    echo -e "${YELLOW}[ ] Binary not found, building...${NC}"
    make build
    if [ $? -ne 0 ]; then
        echo -e "${RED}[-] Build failed. Please check dependencies.${NC}"
        exit 1
    fi
fi

if [ -f "/usr/local/bin/grabba" ]; then
    echo -e "${YELLOW}[ ] grabba already exists in /usr/local/bin. Overwriting...${NC}"
    sudo rm /usr/local/bin/grabba
fi

sudo cp ./build/grabba /usr/local/bin/
sudo chmod +x /usr/local/bin/grabba

echo -e "${GREEN}[+] Grabba installed successfully to /usr/local/bin/grabba${NC}"
echo "[ ] Run 'grabba --help' to get started."
