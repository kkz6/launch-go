echo "Configure firewall with SSH port, HTTP and HTTPS"

sudo ufw allow {{ .Server.SSHPort }}
sudo ufw allow 80
sudo ufw allow 443
sudo yes | sudo ufw enable
sudo service ufw restart
