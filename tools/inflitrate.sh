#!/bin/bash

set -e

export PGVER=17.2
export GOVER=1.23.4
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

    # TODO(rexim): Do we need to build postgres with ssl support?
    # Doesn't feel like we do cause this script implies that we are running bot and db
    # on the same machine and the db only listens to local connections. But who knows?
    # Maybe this script will support multiple machines setup in the future. But even
    # in the case of multiple machine setup it is easier to just running everything
    # inside of a VPN and listen only to the local VPN connections.

    wget https://ftp.postgresql.org/pub/source/v$PGVER/postgresql-$PGVER.tar.gz
    tar fvx postgresql-$PGVER.tar.gz
    cd ./postgresql-$PGVER/
    ./configure --prefix="$GATEKEEPER_PREFIX/pkg/postgresql-$PGVER/"
    make -j$(nproc)
    make install

    mkdir -vp "$GATEKEEPER_PREFIX/data/logs"
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
    wget https://go.dev/dl/go$GOVER.linux-amd64.tar.gz
    tar fvx go$GOVER.linux-amd64.tar.gz
}

setup_gatekeeper() {
    if [ ! -e "$GATEKEEPER_PREFIX/src/gatekeeper" ]; then
        cd "$GATEKEEPER_PREFIX/src"

        # TODO(rexim): iirc Go has its own sort of standardized layout of installing packages.
        # It has something to do with $GOPATH and $GOROOT or whatever (I'm not a Go dev, I don't know)
        # Maybe we can utilize this mechanism here somehow.

        git clone https://github.com/tsoding/gatekeeper
    else
        echo "Gatekeeper Source is already setup"
    fi

    if [ ! -e "$GATEKEEPER_PREFIX/data/secret" ]; then
        echo "Generating $GATEKEEPER_PREFIX/data/secret"
        # TODO(rexim): walk the user throw the entire process of acquiring them for both Twitch and Discord.
        cat > "$GATEKEEPER_PREFIX/data/secret" <<END
#export GATEKEEPER_DISCORD_TOKEN=""   # Discord token https://discord.com/developers/docs/topics/oauth2
#export GATEKEEPER_TWITCH_IRC_NICK="" # Twitch Login
#export GATEKEEPER_TWITCH_IRC_PASS="" # Twitch Password https://twitchapps.com/tmi/
export GATEKEEPER_PGSQL_CONNECTION="postgres://gatekeeper@localhost:5432/gatekeeper?sslmode=disable" # PostgreSQL connection URL https://www.postgresql.org/docs/current/libpq-connect.html#id-1.7.3.8.3.6
END
    else
        echo "$GATEKEEPER_PREFIX/data/secret already exists"
    fi

    if [ ! -e "$GATEKEEPER_PREFIX/inflitrate.sh" ]; then
        ln -sv "$GATEKEEPER_PREFIX/src/gatekeeper/tools/inflitrate.sh" "$GATEKEEPER_PREFIX/inflitrate.sh"
    else
        echo "$GATEKEEPER_PREFIX/inflitrate.sh already exist"
    fi
}

# TODO(rexim): some sort of simple sanity check for all non-"init" commands that the environment was "init"-ed
# For instance, just check that $GATEKEEPER_PREFIX exists.
# TODO(rexim): help command that prints all the available subcommands
case "$1" in
    "" | "init")
        infiltrate_init
        ;;
    "db-start")
        pg_ctl start -l "$GATEKEEPER_PREFIX/data/logs/postgres.log"
        ;;
    "db-stop")
        pg_ctl stop
        ;;
    "db-status")
        pg_ctl status
        ;;
    "db-psql")
        . "$GATEKEEPER_PREFIX/data/secret"
        psql "$GATEKEEPER_PGSQL_CONNECTION"
        ;;
    "db-logs")
        tail -f "$GATEKEEPER_PREFIX/data/logs/postgres.log"
        ;;
    "bot-start")
        . "$GATEKEEPER_PREFIX/data/secret"
        cd "$GATEKEEPER_PREFIX/src/gatekeeper"
        go build ./cmd/gatekeeper
        ./gatekeeper
        ;;
    "bot-pull")
        cd "$GATEKEEPER_PREFIX/src/gatekeeper/"
        git fetch --prune origin
        git merge origin/master
        ;;
    "secret-edit")
        vim "$GATEKEEPER_PREFIX/data/secret"
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
