#!/bin/bash
set -e

# Deployment script for ldap-manager on srv-01
# This script deploys the ldap-manager Podman Quadlet to production

SERVER="srv-01"
CONFIG_DIR="/var/mnt/storage/config-001/ldap-manager"
QUADLET_DIR="/etc/containers/systemd"

echo "=== LDAP Manager Deployment to $SERVER ==="
echo

# 1. Create config directory
echo "Step 1: Creating config directory..."
ssh "$SERVER" "sudo mkdir -p $CONFIG_DIR && sudo chown core:core $CONFIG_DIR"
echo "✓ Config directory created"
echo

# 2. Copy production config
echo "Step 2: Copying production config.yaml..."
scp deployment/config.yaml "$SERVER:$CONFIG_DIR/config.yaml"
echo "✓ Config file deployed"
echo

# 3. Check if secrets exist, create if needed
echo "Step 3: Checking Podman secrets..."
if ! ssh "$SERVER" "podman secret ls | grep -q ldap_admin_password"; then
    echo "❌ Secret 'ldap_admin_password' not found!"
    echo "Please create it manually:"
    echo "  ssh $SERVER"
    echo "  echo 'YOUR_LDAP_ADMIN_PASSWORD' | podman secret create ldap_admin_password -"
    exit 1
fi
echo "✓ ldap_admin_password exists"

if ! ssh "$SERVER" "podman secret ls | grep -q ldap_manager_session_secret"; then
    echo "Creating ldap_manager_session_secret..."
    ssh "$SERVER" "openssl rand -base64 32 | podman secret create ldap_manager_session_secret -"
    echo "✓ Session secret created"
else
    echo "✓ ldap_manager_session_secret exists"
fi
echo

# 4. Copy Quadlet file
echo "Step 4: Deploying Quadlet file..."
scp deployment/ldap-manager.container "$SERVER:/tmp/ldap-manager.container"
ssh "$SERVER" "sudo mv /tmp/ldap-manager.container $QUADLET_DIR/ldap-manager.container && \
               sudo chown root:root $QUADLET_DIR/ldap-manager.container && \
               sudo chmod 644 $QUADLET_DIR/ldap-manager.container"
echo "✓ Quadlet file deployed"
echo

# 5. Reload systemd
echo "Step 5: Reloading systemd..."
ssh "$SERVER" "sudo systemctl daemon-reload"
echo "✓ Systemd reloaded"
echo

# 6. Enable and start service
echo "Step 6: Starting service..."
ssh "$SERVER" "sudo systemctl enable ldap-manager.service && sudo systemctl start ldap-manager.service"
echo "✓ Service started"
echo

# 7. Wait for container to be healthy
echo "Step 7: Waiting for container to be healthy..."
for i in {1..30}; do
    if ssh "$SERVER" "podman exec ldap-manager wget -qO- http://localhost:9090/health 2>/dev/null" | grep -q "ok"; then
        echo "✓ Container is healthy"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "❌ Container did not become healthy after 30 seconds"
        echo "Check logs with: ssh $SERVER 'sudo journalctl -u ldap-manager.service -f'"
        exit 1
    fi
    echo "  Waiting... ($i/30)"
    sleep 1
done
echo

# 8. Show status
echo "Step 8: Service status:"
ssh "$SERVER" "sudo systemctl status ldap-manager.service --no-pager"
echo

echo "=== Deployment Complete ==="
echo
echo "Next steps:"
echo "1. Update Authelia access control rules for passwd.heinrich.blue"
echo "2. Test public access: https://passwd.heinrich.blue/reset"
echo "3. Test admin access: https://passwd.heinrich.blue/admin/dashboard"
echo
echo "Useful commands:"
echo "  View logs:   ssh $SERVER 'sudo journalctl -u ldap-manager.service -f'"
echo "  Restart:     ssh $SERVER 'sudo systemctl restart ldap-manager.service'"
echo "  Stop:        ssh $SERVER 'sudo systemctl stop ldap-manager.service'"
echo "  Exec shell:  ssh $SERVER 'podman exec -it ldap-manager /bin/sh'"
