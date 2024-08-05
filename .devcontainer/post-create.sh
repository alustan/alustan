#!/bin/bash

# this runs at Codespace creation - not part of pre-build

echo "post-create start"
echo "$(date)    post-create start" >> "$HOME/status"

# update the repos
git -C /workspaces/imdb-app pull
git -C /workspaces/webvalidate pull

# Check if yq is installed
if ! command -v yq &> /dev/null; then
    echo "yq is not installed. Installing..."
    sudo wget -qO /usr/local/bin/yq https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64
    sudo chmod a+x /usr/local/bin/yq
fi

sudo apt-get install curl -y
sudo apt-get install make -y

installgo() {
  sudo rm -rf /usr/local/go
  curl -OL https://golang.org/dl/go1.22.2.linux-amd64.tar.gz
  sudo tar -C /usr/local -xvf go1.22.2.linux-amd64.tar.gz 

  # Add Go to PATH
  echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
  
  # Source the updated profile
  source ~/.profile
}

installgo

echo "post-create complete"
echo "$(date +'%Y-%m-%d %H:%M:%S')    post-create complete" >> "$HOME/status"


