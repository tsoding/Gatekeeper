#!/bin/bash

echo "##################################################################"
echo "# WARNING! This script is a part of an on going effort to create #"
echo "# Nyr-style (See https://github.com/Nyr/wireguard-install)       #"
echo "# setup script for Gatekeeper and it's not finished yet.         #"
echo "# It is not recommended to use it right now.                     #"
echo "##################################################################"
echo ""
echo "Press Enter to continue or ^C to cancel..."
read # TODO(rexim): this read is skipped when you `curl https://url/to/inflitrate.sh | bash` it

# NOTE(rexim): Rough Layout of $HOME/Gatekeeper
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
mkdir -vp $HOME/Gatekeeper/src
mkdir -vp $HOME/Gatekeeper/pkg
mkdir -vp $HOME/Gatekeeper/data

setup_deps() {
    . /etc/os-release
    case $ID in
        "void")
            sudo xbps-install -y git bison flex readline readline-devel
            ;;
        "debian")
            echo "TODO(rexim): Debian is not supported yet"
            ;;
        *)
            echo "------------------------------------------------------------"
            echo "$NAME current is not supported."
            echo "PostgreSQL build may fail due to missing dependencies."
            echo "------------------------------------------------------------"
    esac
}

setup_postgres() {
    if [ -e "$HOME/Gatekeeper/pkg/postgresql-17.2/" ]; then
        echo "PostgreSQL is already setup"
        return
    fi

    cd $HOME/Gatekeeper/src

    wget https://ftp.postgresql.org/pub/source/v17.2/postgresql-17.2.tar.gz
    tar fvx postgresql-17.2.tar.gz
    cd ./postgresql-17.2/
    ./configure --prefix=$HOME/Gatekeeper/pkg/postgresql-17.2/
    make -j$(nproc)
    make install

    $HOME/Gatekeeper/pkg/postgresql-17.2/bin/pg_ctl init -D $HOME/Gatekeeper/data/db
    # TODO(rexim): create "gatekeeper" user and database
}

setup_go() {
    if [ -e "$HOME/Gatekeeper/pkg/go/" ]; then
        echo "Go is already setup"
        return
    fi

    cd $HOME/Gatekeeper/pkg
    wget https://go.dev/dl/go1.23.4.linux-amd64.tar.gz
    tar fvx go1.23.4.linux-amd64.tar.gz
}

setup_gatekeeper() {
    if [ -e "$HOME/Gatekeeper/pkg/gatekeeper" ]; then
        echo "Gatekeeper is already setup"
        return
    fi

    cd $HOME/Gatekeeper/src

    # TODO(rexim): iirc Go has its own sort of standardized layout of installing packages.
    # It has something to do with $GOPATH and $GOROOT or whatever (I'm not a Go dev, I don't know)
    # Maybe we can utilize this mechanism here somehow.

    git clone https://github.com/tsoding/gatekeeper
    cd gatekeeper
    $HOME/Gatekeeper/pkg/go/bin/go build ./cmd/gatekeeper
    cp -v $HOME/Gatekeeper/src/gatekeeper/gatekeeper $HOME/Gatekeeper/pkg/

    # TODO(rexim): setup Bots credentials.
    # Preferably walk the user throw the entire process of acquiring them for both Twitch and Discord.
}

# TODO(rexim): some sort of simple sanity check for all non-"init" commands that the environment was "init"-ed
# For instance, just check that $HOME/Gatekeeper exists.
# TODO(rexim): help command that prints all the available subcommands
case "$1" in
    "" | "init")
        setup_postgres
        setup_go
        setup_gatekeeper
        ;;
    "start")
        # TODO(rexim): redirect PostgreSQL log file somewhere to $HOME/Gatekeeper/data
        $HOME/Gatekeeper/pkg/postgresql-17.2/bin/pg_ctl start -D $HOME/Gatekeeper/data/db
        # TODO(rexim): start the bot process in daemon mode somehow?
        ;;
    "stop")
        $HOME/Gatekeeper/pkg/postgresql-17.2/bin/pg_ctl stop -D $HOME/Gatekeeper/data/db
        # TODO(rexim): stop the bot process if it's running as well
        ;;
    *)
        echo "ERROR: unknown subcommand '$1'"
        exit 1
        ;;
esac

# TODO(rexim): subcommand that starts/stops the gatekeeper
#   The whole things, both PostgreSQL and the Bot process
# TODO(rexim): subcommand to make/restore backups
# TODO(rexim): subcommand to pull new changes for the Bot executable
# TODO(rexim): how do you update the inflitrate.sh script itself?
#   It is expected to be downloaded standalone into $HOME or what not,
#   but what if it is updated? Maybe part of the "init" should be replacing
#   this file with a symlink to $HOME/Gatekeeper/src/gatekeeper/tools/inflitrate.sh
#   so it's always updated?
