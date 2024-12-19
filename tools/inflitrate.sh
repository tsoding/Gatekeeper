#!/bin/bash

echo "##################################################################"
echo "# WARNING! This script is a part of an on going effort to create #"
echo "# Nyr-style (See https://github.com/Nyr/wireguard-install)       #"
echo "# setup script for Gatekeeper and it's not finished yet.         #"
echo "# It is not recommended to use it right now.                     #"
echo "##################################################################"
echo ""
echo "Press Enter to continue or ^C to cancel..."
read

set -xe

# Rough Layout of $HOME/Gatekeeper
# 
# `-$HOME/Gatekeeper/
#   |
#   `-src/                # Source code
#   | `-postgres/...
#   | `-gatekeeper/...
#   `-pkg/                # Binaries build from the Source code
#   | `-postgres/...
#   | `-gatekeeper/...
#   | `-go/...
#   `-data/               # Applications data
#     `-postgres/...
#     `-gatekeeper/...
mkdir -p $HOME/Gatekeeper/src
mkdir -p $HOME/Gatekeeper/pkg
mkdir -p $HOME/Gatekeeper/data

. /etc/os-release 
case $ID in
    "void")
        sudo xbps-install -y git bison flex readline readline-devel
        ;;
    "debian")
        echo "TODO: Debian is not supported yet"
        ;;
    *)
        echo "------------------------------------------------------------"
        echo "$NAME current is not supported."
        echo "PostgreSQL build may fail due to missing dependencies."
        echo "------------------------------------------------------------"
esac

setup_postgres() {
    cd $HOME/Gatekeeper/src

    wget https://ftp.postgresql.org/pub/source/v17.2/postgresql-17.2.tar.gz
    tar fvx postgresql-17.2.tar.gz
    cd ./postgresql-17.2/
    ./configure --prefix=$HOME/Gatekeeper/pkg/postgresql-17.2/
    make -j$(nproc)
    make install
}

setup_postgres
