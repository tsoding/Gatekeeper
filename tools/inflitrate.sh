#!/bin/bash
#      ^
#      We only support bash

set -e

export PGVER=17.2
export GOVER=1.23.4
export GATEKEEPER_PREFIX="$HOME/Gatekeeper"
export PGDATA="$GATEKEEPER_PREFIX/data/db" # NOTE(rexim): Tells PostgreSQL where the database is
export PATH="$GATEKEEPER_PREFIX/pkg/go/bin:$GATEKEEPER_PREFIX/pkg/postgresql-$PGVER/bin/:$PATH"
export GATEKEEPER_COMMANDS=()

GATEKEEPER_COMMANDS+=(setup-everything)
setup-everything() {
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

    setup-deps
    setup-postgres
    setup-go
    setup-gatekeeper

    echo "To enter the inflitrated environment do"
    echo ""
    echo "    source $GATEKEEPER_PREFIX/inflitrate.sh"
    echo ""
    echo "in your bash"
}

# TODO(rexim): do not try to call sudo if all the necessary dependencies are already installed
GATEKEEPER_COMMANDS+=(setup-deps)
setup-deps() {
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

GATEKEEPER_COMMANDS+=(setup-postgres)
setup-postgres() {
    if [ ! -e "$GATEKEEPER_PREFIX/pkg/postgresql-$PGVER/" ]; then
        if [ ! -e "$GATEKEEPER_PREFIX/src/postgresql-$PGVER/" ]; then
            cd "$GATEKEEPER_PREFIX/src"
            wget https://ftp.postgresql.org/pub/source/v$PGVER/postgresql-$PGVER.tar.gz
            tar fvx postgresql-$PGVER.tar.gz
        else
            echo "$GATEKEEPER_PREFIX/src/postgresql-$PGVER/ already exists"
        fi

        cd "$GATEKEEPER_PREFIX/src/postgresql-$PGVER/"
        # TODO(rexim): Do we need to build postgres with ssl support?
        # Doesn't feel like we do cause this script implies that we are running bot and db
        # on the same machine and the db only listens to local connections. But who knows?
        # Maybe this script will support multiple machines setup in the future. But even
        # in the case of multiple machine setup it is easier to just running everything
        # inside of a VPN and listen only to the local VPN connections.
        ./configure --prefix="$GATEKEEPER_PREFIX/pkg/postgresql-$PGVER/"
        make -j$(nproc)
        make install
    else
        echo "$GATEKEEPER_PREFIX/pkg/postgresql-$PGVER/ already exists"
    fi

    mkdir -vp "$GATEKEEPER_PREFIX/data/logs"

    if [ ! -e "$PGDATA" ]; then
        initdb -U postgres
        db-start # TODO(rexim): if there is already running stock Postgres on the machine this step will fail
        createuser gatekeeper -U postgres
        createdb gatekeeper -U postgres -O gatekeeper
        db-stop
    else
        echo "$PGDATA already exists"
    fi
}

GATEKEEPER_COMMANDS+=(setup-go)
setup-go() {
    if [ -e "$GATEKEEPER_PREFIX/pkg/go/" ]; then
        echo "$GATEKEEPER_PREFIX/pkg/go/ already exists"
        return
    fi

    cd "$GATEKEEPER_PREFIX/pkg"
    wget https://go.dev/dl/go$GOVER.linux-amd64.tar.gz
    tar fvx go$GOVER.linux-amd64.tar.gz
}

GATEKEEPER_COMMANDS+=(setup-gatekeeper)
setup-gatekeeper() {
    if [ ! -e "$GATEKEEPER_PREFIX/src/gatekeeper" ]; then
        cd "$GATEKEEPER_PREFIX/src"

        # TODO(rexim): iirc Go has its own sort of standardized layout of installing packages.
        # It has something to do with $GOPATH and $GOROOT or whatever (I'm not a Go dev, I don't know)
        # Maybe we can utilize this mechanism here somehow.

        # TODO(rexim): constant interrupt questions like this do not allow to just start inflitrating and walk away
        # We should probably introduce some sort of preconfigure step were you can set your preferences
        read -p 'Clone Gatekeeper source code from the SSH url? [y/n] ' yn
        while true; do
            case $yn in
                [Yy]*)
                    git clone "git@github.com:tsoding/gatekeeper.git"
                    break
                    ;;
                [Nn]*)
                    git clone "https://github.com/tsoding/gatekeeper"
                    break
                    ;;
            esac
        done
    else
        echo "$GATEKEEPER_PREFIX/src/gatekeeper already exists"
    fi

    if [ ! -e "$GATEKEEPER_PREFIX/data/secret" ]; then
        echo "Generating $GATEKEEPER_PREFIX/data/secret"
        # TODO(rexim): walk the user throw the entire process of acquiring them for both Twitch and Discord.
        cat > "$GATEKEEPER_PREFIX/data/secret" <<END
#export GATEKEEPER_DISCORD_TOKEN=""   # Discord token https://discord.com/developers/docs/topics/oauth2
#export GATEKEEPER_TWITCH_IRC_NICK="" # Twitch Login
#export GATEKEEPER_TWITCH_IRC_PASS="" # Twitch Password https://twitchapps.com/tmi/
export GATEKEEPER_PGSQL_CONNECTION="postgres://gatekeeper@localhost:5432/gatekeeper?sslmode=disable" # PostgreSQL connection URL https://www.postgresql.org/docs/current/libpq-connect.html#id-1.7.3.8.3.6
#export GATEKEEPER_MPV_IPC_ADDRESS="127.0.0.1:8080" # Address of the MPV control server
END
    else
        echo "$GATEKEEPER_PREFIX/data/secret already exists"
    fi

    if [ ! -e "$GATEKEEPER_PREFIX/inflitrate.sh" ]; then
        ln -sv "$GATEKEEPER_PREFIX/src/gatekeeper/tools/inflitrate.sh" "$GATEKEEPER_PREFIX/inflitrate.sh"
    else
        echo "$GATEKEEPER_PREFIX/inflitrate.sh already exists"
    fi
}

GATEKEEPER_COMMANDS+=(db-start)
db-start() {
    pg_ctl start -l "$GATEKEEPER_PREFIX/data/logs/postgres.log"
}

GATEKEEPER_COMMANDS+=(db-stop)
db-stop() {
    pg_ctl stop
}

GATEKEEPER_COMMANDS+=(db-status)
db-status() {
    pg_ctl status
}

GATEKEEPER_COMMANDS+=(db-psql)
db-psql() {
    . "$GATEKEEPER_PREFIX/data/secret"
    psql "$GATEKEEPER_PGSQL_CONNECTION"
}

GATEKEEPER_COMMANDS+=(db-logs)
db-logs() {
    tail -f "$GATEKEEPER_PREFIX/data/logs/postgres.log"
}

GATEKEEPER_COMMANDS+=(bot-start)
bot-start() {
    . "$GATEKEEPER_PREFIX/data/secret"
    cd "$GATEKEEPER_PREFIX/src/gatekeeper"
    go build ./cmd/gatekeeper && ./gatekeeper
}

GATEKEEPER_COMMANDS+=(bot-pull)
bot-pull() {
    cd "$GATEKEEPER_PREFIX/src/gatekeeper/"
    git fetch --prune origin
    git merge origin/master
}

GATEKEEPER_COMMANDS+=(secret-edit)
secret-edit() {
    echo "#######################################################################"
    echo "# WARNING! You are about to show sensitive information on the screen! #"
    echo "# WARNING! You are about to show sensitive information on the screen! #"
    echo "# WARNING! You are about to show sensitive information on the screen! #"
    echo "#######################################################################"
    read -p "^C to cancel"
    vim "$GATEKEEPER_PREFIX/data/secret"
}

GATEKEEPER_COMMANDS+=(cmd-list)
cmd-list() {
    echo "Available commands:"
    for cmd in ${GATEKEEPER_COMMANDS[@]}; do
        echo "    ${cmd}"
    done
}

# TODO(rexim): some sort of simple sanity check for all non-"init" commands that the environment was "init"-ed
# For instance, just check that $GATEKEEPER_PREFIX exists.

# Stolen from https://stackoverflow.com/questions/2683279/how-to-detect-if-a-script-is-being-sourced
if (return 0 2>/dev/null); then
    # TODO(rexim): check if we are already inflitrated and maybe say something about that?
    # I don't really know if it makes sense to even care about user doing `env` while within `env`...
    set +e              # Do not enable exit-on-error in the user facing environment
    # TODO(rexim): print the available commands and just a general help message
    cmd-list
    PS1="[inflitrated] $PS1"
elif [ -z "$@" ]; then
    setup-everything
else
    $@
fi

# TODO(rexim): subcommand to make/restore backups
# TODO(rexim): how would you autostart the whole system with this kind of setup?
