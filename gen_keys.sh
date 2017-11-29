#!/bin/sh
mkdir -p keys/server
ssh-keygen -t rsa -P '' -C '' -f keys/server/id_rsa

mkdir -p keys/client
ssh-keygen -t rsa -P '' -C '' -f keys/client/id_rsa
