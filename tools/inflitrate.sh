#!/bin/bash

set -e

export PGVER=17.2
export GATEKEEPER_PREFIX="$HOME/Gatekeeper"
export PGDATA="$GATEKEEPER_PREFIX/data/db" # NOTE(rexim): Tells PostgreSQL where the database is
export PATH="$GATEKEEPER_PREFIX/pkg/go/bin:$GATEKEEPER_PREFIX/pkg/postgresql-$PGVER/bin/:$PATH"

infiltrate_init() {
    echo "##################################################################"
    echo "# WARNING! This script is a part of an on going effort to create #"
    echo "# Nyr-style (See https://github.com/Nyr/wireguard-install)       #"
    echo "# setup script for Gatekeeper and it's not finished yet.         #"
    echo "# It is not recommended to use it right now.                     #"
    echo "##################################################################"
    echo ""
    read -p "Press Enter to continue or ^C to cancel..."

    # NOTE(rexim): Rough Layout of $GATEKEEPER_PREFIX
    #
    # `-$GATEKEEPER_PREFIX/
    #   |
    #   `-src/                # Source code
    #   | `-postgres/...
    #   | `-gatekeeper/...
    #   `-pkg/                # Binaries build from the Source code
    #   | `-postgres/...
    #   | `-gatekeeper/...
    #   | `-go/...
    #   `-data/               # Applications data
    #     `-db/...
    mkdir -vp "$GATEKEEPER_PREFIX/src"
    mkdir -vp "$GATEKEEPER_PREFIX/pkg"
    mkdir -vp "$GATEKEEPER_PREFIX/data"

    setup_deps
    setup_postgres
    setup_go
    setup_gatekeeper
}

setup_deps() {
    . /etc/os-release
    # TODO(rexim): test on different distros via Docker
    case $ID in
        "void")
            # TODO(rexim): update xbps
            sudo xbps-install -y git pkg-config bison flex readline readline-devel
            ;;
        "debian")
            # TODO(rexim): update apt
            su -c "apt install git pkg-config gcc libicu-dev bison flex libreadline-dev zlib1g-dev"
            ;;
        *)
            echo "------------------------------------------------------------"
            echo "$NAME currently is not supported."
            echo "------------------------------------------------------------"
            exit 1
    esac
}

setup_postgres() {
    if [ -e "$GATEKEEPER_PREFIX/pkg/postgresql-$PGVER/" ]; then
        echo "PostgreSQL is already setup"
        return
    fi

    cd "$GATEKEEPER_PREFIX/src"

    wget https://ftp.postgresql.org/pub/source/v$PGVER/postgresql-$PGVER.tar.gz
    tar fvx postgresql-$PGVER.tar.gz
    cd ./postgresql-$PGVER/
    ./configure --prefix="$GATEKEEPER_PREFIX/pkg/postgresql-$PGVER/"
    make -j$(nproc)
    make install

    initdb -U postgres
    pg_ctl start                # TODO(rexim): if there is already running stock Postgres on the machine this step will fail
    createuser gatekeeper -U postgres
    createdb gatekeeper -U postgres -O gatekeeper
    pg_ctl stop
}

setup_go() {
    if [ -e "$GATEKEEPER_PREFIX/pkg/go/" ]; then
        echo "Go is already setup"
        return
    fi

    cd "$GATEKEEPER_PREFIX/pkg"
    wget https://go.dev/dl/go1.23.4.linux-amd64.tar.gz
    tar fvx go1.23.4.linux-amd64.tar.gz
}

setup_gatekeeper() {
    if [ -e "$GATEKEEPER_PREFIX/pkg/gatekeeper" ]; then
        echo "Gatekeeper is already setup"
        return
    fi

    cd "$GATEKEEPER_PREFIX/src"

    # TODO(rexim): iirc Go has its own sort of standardized layout of installing packages.
    # It has something to do with $GOPATH and $GOROOT or whatever (I'm not a Go dev, I don't know)
    # Maybe we can utilize this mechanism here somehow.

    git clone https://github.com/tsoding/gatekeeper
    cd gatekeeper
    go build ./cmd/gatekeeper
    cp -v "$GATEKEEPER_PREFIX/src/gatekeeper/gatekeeper" "$GATEKEEPER_PREFIX/pkg/"

    # TODO(rexim): setup Bots credentials.
    # Preferably walk the user throw the entire process of acquiring them for both Twitch and Discord.
}

# TODO(rexim): some sort of simple sanity check for all non-"init" commands that the environment was "init"-ed
# For instance, just check that $GATEKEEPER_PREFIX exists.
# TODO(rexim): help command that prints all the available subcommands
case "$1" in
    "" | "init")
        infiltrate_init
        ;;
    "start")
        # TODO(rexim): redirect PostgreSQL log file somewhere to $GATEKEEPER_PREFIX/data
        pg_ctl start
        # TODO(rexim): start the bot process in daemon mode somehow?
        ;;
    "stop")
        pg_ctl stop
        # TODO(rexim): stop the bot process if it's running as well
        ;;
    "status")
        pg_ctl status
        # TODO(rexim): check the status of the bot process if it's running
        ;;
    "update")
        cd "$GATEKEEPER_PREFIX/src/gatekeeper/"
        git fetch --prune origin
        git merge origin/master
        go build ./cmd/gatekeeper
        cp -v "$GATEKEEPER_PREFIX/src/gatekeeper/gatekeeper" "$GATEKEEPER_PREFIX/pkg/"
        ;;
    *)
        echo "ERROR: unknown subcommand '$1'"
        exit 1
        ;;
esac

# TODO(rexim): subcommand to make/restore backups
# TODO(rexim): how do you update the inflitrate.sh script itself?
#   It is expected to be downloaded standalone into $HOME or what not,
#   but what if it is updated? Maybe part of the "init" should be replacing
#   this file with a symlink to $GATEKEEPER_PREFIX/src/gatekeeper/tools/inflitrate.sh
#   so it's always updated?
# TODO(rexim): how would you autostart the whole system with this kind of setup
