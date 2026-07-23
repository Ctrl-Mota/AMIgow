#!/bin/bash
set -e

APP_NAME=amigow
APP_DIR=/opt/amigow
SERVICE_FILE=/etc/systemd/system/${APP_NAME}.service
JOURNAL_CONF=/etc/systemd/journald.conf

echo "==> Criando diretório da aplicação"
sudo mkdir -p $APP_DIR

echo "==> Copiando binário e config.json"
sudo cp ~/amigow/$APP_NAME $APP_DIR/
sudo cp ~/amigow/config.json $APP_DIR/

echo "==> Ajustando permissões"
sudo chown -R asterisk:asterisk $APP_DIR
sudo chmod 750 $APP_DIR
sudo chmod +x $APP_DIR/$APP_NAME

echo "==> Criando service systemd (stdout + journal)"
sudo tee $SERVICE_FILE > /dev/null <<EOF
[Unit]
Description=Amigow - Go AMI Worker
After=network.target asterisk.service
Requires=asterisk.service

[Service]
Type=simple
User=asterisk
Group=asterisk
WorkingDirectory=$APP_DIR
ExecStart=$APP_DIR/$APP_NAME
Restart=always
RestartSec=5

StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

echo "==> Configurando limites do journald (5MB por arquivo)"
sudo sed -i 's/^#SystemMaxUse=.*/SystemMaxUse=200M/' $JOURNAL_CONF || true
sudo sed -i 's/^#SystemMaxFileSize=.*/SystemMaxFileSize=5M/' $JOURNAL_CONF || true

echo "==> Reiniciando journald"
sudo systemctl restart systemd-journald

echo "==> Recarregando systemd"
sudo systemctl daemon-reload

echo "==> Habilitando serviço no boot"
sudo systemctl enable $APP_NAME

echo "==> Iniciando serviço"
sudo systemctl start $APP_NAME

echo "==> Status final"
sudo systemctl status $APP_NAME --no-pager

echo "Configure nginx to proxy to amigow" 
echo "Cole o seguinte no arquivo de configuração do nginx:

sudo nano /etc/nginx/sites-available/vitalpbx 

dentro de server 80 e 443, adicione:

location /amigow/ {
    proxy_pass http://127.0.0.1:8080/;

    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;

    # 1) Remove CSP que venha do serviço em 8080 (senão fica duplicado e bloqueia)
    proxy_hide_header Content-Security-Policy;
    proxy_hide_header Content-Security-Policy-Report-Only;

    # 2) Define um CSP único (TUDO EM UMA LINHA)
    add_header Content-Security-Policy "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval' https://unpkg.com; style-src 'self' 'unsafe-inline' https://unpkg.com; img-src 'self' data: blob:; font-src 'self' https://unpkg.com data:; connect-src 'self';" always;
"


# Implementação recomendada — regra sudoers NOPASSWD para suportar o tip.sh
# sudo nano /etc/sudoers.d/amigow
# asterisk ALL=(root) NOPASSWD: /usr/sbin/fwconsole, /usr/bin/fail2ban-client

# sudo visudo -cf /etc/sudoers.d/amigow   # valida sintaxe
# sudo chmod 440 /etc/sudoers.d/amigow

