#!/bin/bash
set -e

sftpgo_db="${SFTPGo_DB_NAME:-sftpgo}"
sftpgo_user="${SFTPGo_DB_USER:-sftpgo}"
sftpgo_password="${SFTPGo_DB_PASSWORD:?SFTPGo_DB_PASSWORD is required}"

psql=(psql --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" --set ON_ERROR_STOP=1)

"${psql[@]}" --set=sftpgo_db="$sftpgo_db" --set=sftpgo_user="$sftpgo_user" --set=sftpgo_password="$sftpgo_password" <<'SQL'
CREATE USER :"sftpgo_user" WITH PASSWORD :'sftpgo_password';
CREATE DATABASE :"sftpgo_db" OWNER :"sftpgo_user";
SQL

