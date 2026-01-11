echo "Configure swap"

if [ ! -f /swapfile ]; then
    sudo fallocate -l {{ .SwapInMegabytes }}M /swapfile
    sudo chmod 600 /swapfile
    sudo mkswap /swapfile
    sudo swapon /swapfile || true

    # Use tee to append to /etc/fstab with sudo privileges
    echo "/swapfile none swap sw 0 0" | sudo tee -a /etc/fstab > /dev/null

    # Use tee to append to /etc/sysctl.conf with sudo privileges
    echo "vm.swappiness={{ .Swappiness }}" | sudo tee -a /etc/sysctl.conf > /dev/null
    echo "vm.vfs_cache_pressure=50" | sudo tee -a /etc/sysctl.conf > /dev/null
fi
