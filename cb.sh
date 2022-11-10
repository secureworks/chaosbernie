#!/bin/bash
# Copyright 2022 SecureWorks, Inc. All rights reserved.

# Original Author: Mark Chaffe

# This program is free software; you can redistribute it and/or
# modify it under the terms of the GNU General Public License
# version 2 as published by the Free Software Foundation.

# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
# GNU General Public License for more details.

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

DEFAULT_ARGS="-psallusers -episode 2"

PSDOOMPSCMD="${SCRIPT_DIR}/azure/bin/client --action=ps"
PSDOOMRENICECMD="${SCRIPT_DIR}/azure/bin/client --action=renice"
PSDOOMKILLCMD="${SCRIPT_DIR}/azure/bin/client --action=kill"
export PSDOOMPSCMD=$PSDOOMPSCMD
export PSDOOMRENICECMD=$PSDOOMRENICECMD
export PSDOOMKILLCMD=$PSDOOMKILLCMD


echo "PSDOOMPSCMD=${PSDOOMPSCMD}"
echo "PSDOOMRENICECMD=${PSDOOMRENICECMD}"
echo "PSDOOMKILLCMD=${PSDOOMKILLCMD}"
echo "DOOMWADPATH=${DOOMWADPATH}"

/usr/local/bin/psdoom-ng ${DEFAULT_ARGS} $@

