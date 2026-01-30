#!/bin/bash
# Script de instalação do BATQA Proxy
# Uso: sudo ./install.sh

set -e

INSTALL_DIR="/usr/local/bin"
SERVICE_FILE="/etc/systemd/system/batqa-proxy.service"
BINARY_NAME="batqa-proxy"

# Cores
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}╔════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║       BATQA Proxy - Instalador         ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════╝${NC}"
echo ""

# Verifica se é root
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}❌ Execute como root: sudo ./install.sh${NC}"
    exit 1
fi

# Detecta arquitetura
ARCH=$(uname -m)
case $ARCH in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64)
        ARCH="arm64"
        ;;
    *)
        echo -e "${RED}❌ Arquitetura não suportada: $ARCH${NC}"
        exit 1
        ;;
esac

echo -e "${YELLOW}📦 Arquitetura detectada: $ARCH${NC}"

# Verifica se Go está instalado para compilar
if command -v go &> /dev/null; then
    echo -e "${GREEN}✅ Go encontrado, compilando...${NC}"
    go build -o $BINARY_NAME main.go
else
    echo -e "${YELLOW}⚠️  Go não encontrado.${NC}"
    echo -e "${YELLOW}   Baixe o binário pré-compilado ou instale Go 1.21+${NC}"
    
    if [ -f "$BINARY_NAME" ]; then
        echo -e "${GREEN}✅ Binário encontrado no diretório atual${NC}"
    else
        echo -e "${RED}❌ Binário não encontrado. Compile com: go build -o batqa-proxy main.go${NC}"
        exit 1
    fi
fi

# Copia binário
echo -e "${YELLOW}📁 Copiando binário para $INSTALL_DIR...${NC}"
cp $BINARY_NAME $INSTALL_DIR/
chmod +x $INSTALL_DIR/$BINARY_NAME

# Detecta porta do TeamSpeak
TS_PORT="10011"
if netstat -tlnp 2>/dev/null | grep -q ":10101"; then
    echo -e "${YELLOW}📍 TeaSpeak detectado (porta 10101)${NC}"
    TS_PORT="10101"
    PROXY_PORT="10203"
elif netstat -tlnp 2>/dev/null | grep -q ":10011"; then
    echo -e "${YELLOW}📍 TeamSpeak detectado (porta 10011)${NC}"
    TS_PORT="10011"
    PROXY_PORT="10202"
else
    echo -e "${YELLOW}⚠️  Nenhum servidor detectado, usando padrão (10011)${NC}"
    PROXY_PORT="10202"
fi

# Pergunta configurações
echo ""
read -p "Porta do proxy [$PROXY_PORT]: " INPUT_PROXY_PORT
PROXY_PORT=${INPUT_PROXY_PORT:-$PROXY_PORT}

read -p "Porta do TeamSpeak [$TS_PORT]: " INPUT_TS_PORT
TS_PORT=${INPUT_TS_PORT:-$TS_PORT}

# Detecta usuário do TeamSpeak
TS_USER="root"
if id "teamspeak" &>/dev/null; then
    TS_USER="teamspeak"
elif id "ts3" &>/dev/null; then
    TS_USER="ts3"
elif id "teaspeak" &>/dev/null; then
    TS_USER="teaspeak"
fi

echo ""
echo -e "${YELLOW}📝 Configuração:${NC}"
echo -e "   Porta do Proxy: $PROXY_PORT"
echo -e "   Porta do TS: $TS_PORT"
echo -e "   Usuário: $TS_USER"
echo ""

# Cria serviço systemd
echo -e "${YELLOW}🔧 Criando serviço systemd...${NC}"

cat > $SERVICE_FILE << EOF
[Unit]
Description=BATQA Proxy for TeamSpeak ServerQuery
After=network.target teamspeak3-server.service ts3server.service teaspeak.service
Wants=network-online.target

[Service]
Type=simple
User=$TS_USER
Group=$TS_USER
ExecStart=$INSTALL_DIR/$BINARY_NAME -listen :$PROXY_PORT -target localhost:$TS_PORT
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# Segurança
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

# Recarrega systemd
systemctl daemon-reload

# Pergunta se quer iniciar
echo ""
read -p "Iniciar o serviço agora? [S/n]: " START_NOW
START_NOW=${START_NOW:-S}

if [[ "$START_NOW" =~ ^[Ss]$ ]]; then
    systemctl enable batqa-proxy
    systemctl start batqa-proxy
    
    sleep 2
    
    if systemctl is-active --quiet batqa-proxy; then
        echo -e "${GREEN}✅ Serviço iniciado com sucesso!${NC}"
    else
        echo -e "${RED}❌ Falha ao iniciar. Verifique: journalctl -u batqa-proxy${NC}"
    fi
fi

# Configura firewall se disponível
if command -v ufw &> /dev/null; then
    echo ""
    read -p "Liberar porta $PROXY_PORT no UFW? [S/n]: " OPEN_UFW
    OPEN_UFW=${OPEN_UFW:-S}
    
    if [[ "$OPEN_UFW" =~ ^[Ss]$ ]]; then
        ufw allow $PROXY_PORT/tcp
        echo -e "${GREEN}✅ Porta liberada no UFW${NC}"
    fi
fi

echo ""
echo -e "${GREEN}╔════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║       Instalação Concluída! 🎉         ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════╝${NC}"
echo ""
echo -e "Comandos úteis:"
echo -e "  ${YELLOW}systemctl status batqa-proxy${NC}  - Ver status"
echo -e "  ${YELLOW}systemctl restart batqa-proxy${NC} - Reiniciar"
echo -e "  ${YELLOW}journalctl -u batqa-proxy -f${NC}  - Ver logs"
echo ""
echo -e "No BATQA App, use:"
echo -e "  Host: seu-servidor.com"
echo -e "  Porta: ${GREEN}$PROXY_PORT${NC}"
echo ""
