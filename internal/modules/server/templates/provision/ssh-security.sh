echo "Enhance SSH security"

sudo sed -i "/PasswordAuthentication yes/d" /etc/ssh/sshd_config
echo "PasswordAuthentication no" | sudo tee -a /etc/ssh/sshd_config > /dev/null
sudo service ssh restart

echo "Setup SSH keys for root"

if [ ! -d /root/.ssh ]
then
    sudo mkdir -p /root/.ssh
    sudo touch /root/.ssh/authorized_keys
fi
